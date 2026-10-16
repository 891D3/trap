// onboarding.go —— 首次安装初始化的数据目录策略（PRD Onboarding D1）。
//
// 目录口径（用户 2026-10-16 裁定）：所有配置与用户文件统一存放在
// `~/.trap`（home 下点目录），结构：
//
//	~/.trap/
//	  trap.db        SQLite（含全部迁移）
//	  copies/        头像等副本文件
//	  exports/       信息记录导出物
//
// 兼容：存量安装的数据在 mygo userData（`~/Library/Application Support/Trap`）。
// 首次启动检测到旧库且新库不存在 → 自动迁移（复制 → 校验可打开 → 旧库改名
// 备份保留）；任一步失败回退旧路径，不丢数据、不阻塞启动。
//
// 本文件不含 SQL（AGENTS.md §4.1）；全部为文件系统操作。
package onboarding

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// Result 描述目录解析与迁移结果（main 启动日志与 DataDir 展示用）。
type Result struct {
	Dir        string // 实际使用的数据目录（~/.trap 或回退后的旧路径）
	Migrated   bool   // 本次启动是否执行了旧 userData → ~/.trap 迁移
	Fallback   bool   // 迁移失败是否回退到旧路径
	LegacyPath string // 旧 userData 路径（用于日志/提示；无则为空）
}

// legacyDirFor 是「旧 userData 目录」的注入点（测试覆盖为临时目录）。
// 生产实现取 mygo userData（~/Library/Application Support/<AppName>）。
var legacyDirFor = func() (string, error) {
	return mygo.App.Path(mygo.PathUserData)
}

// legacyDBPath 返回旧 userData 下的旧库路径（可能不存在）。
func legacyDBPath() (string, error) {
	dir, err := legacyDirFor()
	if err != nil {
		return "", fmt.Errorf("onboarding: 解析旧数据目录失败: %w", err)
	}
	return filepath.Join(dir, "trap.db"), nil
}

// homeDirFor 是「用户主目录」的注入点（测试覆盖为临时目录）。
var homeDirFor = os.UserHomeDir

// Resolve 确定并准备数据目录：
//  1. 目标 `~/.trap` 不存在则创建（含 copies/、exports/）；
//  2. 目标库已存在 → 直接使用（正常路径）；
//  3. 目标库不存在但旧 userData 库存在 → 自动迁移；
//  4. 迁移失败 → 回退旧路径（Result.Fallback=true，调用方提示）。
//
// 首次安装（两边都无库）走 1 的全新创建，Result.Migrated=false。
func Resolve(ctx context.Context) (Result, error) {
	home, err := homeDirFor()
	if err != nil {
		return Result{}, fmt.Errorf("onboarding: 解析用户主目录失败: %w", err)
	}
	target := filepath.Join(home, ".trap")
	legacyDB, err := legacyDBPath()
	if err != nil {
		return Result{}, err
	}
	legacyDir := filepath.Dir(legacyDB)

	targetDB := filepath.Join(target, "trap.db")
	_, targetStat := os.Stat(targetDB)
	_, legacyStat := os.Stat(legacyDB)

	res := Result{Dir: target, LegacyPath: legacyDB}

	// 1) 目录结构（幂等）。
	if err := ensureDir(target); err != nil {
		return res, err
	}
	for _, sub := range []string{"copies", "exports"} {
		if err := ensureDir(filepath.Join(target, sub)); err != nil {
			return res, err
		}
	}

	// 2) 新库已在 → 直接用。
	if targetStat == nil {
		return res, nil
	}

	// 3) 全新安装（旧库也没有）→ 目标已建好，直接用。
	if legacyStat != nil {
		return res, nil
	}

	// 4) 存量安装 → 迁移旧库（复制 → 校验 → 备份旧库）。
	if err := migrateDB(ctx, legacyDB, targetDB); err != nil {
		res.Fallback = true
		res.Dir = legacyDir
		return res, fmt.Errorf("onboarding: 数据迁移失败，回退旧目录 %s: %w", legacyDir, err)
	}

	// 附带迁移头像副本（copies/ 下；不存在则跳过）。
	copyTree(filepath.Join(legacyDir, "copies"), filepath.Join(target, "copies"))

	res.Migrated = true
	return res, nil
}

// ensureDir 幂等建目录。
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("onboarding: 创建目录 %s 失败: %w", dir, err)
	}
	return nil
}

// migrateDB 复制旧库到新路径并用 SQLite 打开校验，成功后旧库改名备份。
// 旧库始终保留（trap.db.bak-<ts>），不删除。
func migrateDB(ctx context.Context, legacyDB, targetDB string) error {
	if err := copyFile(legacyDB, targetDB); err != nil {
		return err
	}
	// 校验：目标库可被 SQLite 打开并读到 schema_migrations（只读探针）。
	if err := probeSQLite(ctx, targetDB); err != nil {
		_ = os.Remove(targetDB) // 清掉坏副本；旧库未动
		return err
	}
	backup := legacyDB + ".bak-migrated"
	if err := os.Rename(legacyDB, backup); err != nil {
		return fmt.Errorf("onboarding: 旧库备份改名失败: %w", err)
	}
	return nil
}

// probeSQLite 只读探针：能打开并 SELECT 1 即认为库完好。
// 通过 sqlite 插件避免引入 cgo 驱动（与业务同栈）。
func probeSQLite(ctx context.Context, path string) error {
	db, err := openProbe(ctx, path)
	if err != nil {
		return fmt.Errorf("onboarding: 迁移后库校验失败: %w", err)
	}
	return closeProbe(db)
}

// copyFile 复制单个文件（保留权限）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("onboarding: 打开源文件失败: %w", err)
	}
	defer in.Close()
	inStat, err := in.Stat()
	if err != nil {
		return fmt.Errorf("onboarding: 读取源文件信息失败: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, inStat.Mode())
	if err != nil {
		return fmt.Errorf("onboarding: 创建目标文件失败: %w", err)
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("onboarding: 复制失败: %w", err)
	}
	if err = out.Close(); err != nil {
		return fmt.Errorf("onboarding: 关闭目标文件失败: %w", err)
	}
	return nil
}

// copyTree 递归复制目录（不存在源目录则跳过；单项失败不中断——副本属非关键数据）。
func copyTree(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	_ = ensureDir(dst)
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(s, d)
			continue
		}
		_ = copyFile(s, d)
	}
}
