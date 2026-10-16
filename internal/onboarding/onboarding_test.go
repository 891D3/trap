// onboarding_test.go —— 目录策略与迁移测试（PRD Onboarding 验收 1）。
//
// 家目录不能在测试里真改：用 Resolve 的测试注入点（homeDirFor 变量）
// 指到 t.TempDir()，行为路径与生产一致。
package onboarding

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
)

// withHome 把「家目录」与「旧 userData 目录」都注入到临时目录并还原。
// 旧目录按 macOS 布局模拟：<home>/Library/Application Support/Trap。
func withHome(t *testing.T, home string) {
	t.Helper()
	oldHome, oldLegacy := homeDirFor, legacyDirFor
	homeDirFor = func() (string, error) { return home, nil }
	legacyDirFor = func() (string, error) {
		return filepath.Join(home, "Library", "Application Support", "Trap"), nil
	}
	t.Cleanup(func() { homeDirFor, legacyDirFor = oldHome, oldLegacy })
}

func TestResolveFreshInstall(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)

	res, err := Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Migrated || res.Fallback {
		t.Errorf("全新安装不应迁移/回退: %+v", res)
	}
	if res.Dir != filepath.Join(home, ".trap") {
		t.Errorf("Dir = %q", res.Dir)
	}
	// 目录结构。
	for _, sub := range []string{"", "copies", "exports"} {
		if fi, err := os.Stat(filepath.Join(home, ".trap", sub)); err != nil || !fi.IsDir() {
			t.Errorf("缺少目录 %q", sub)
		}
	}
	// 旧库不存在 → 无备份文件。
	if _, err := os.Stat(filepath.Join(home, ".trap", "trap.db")); err == nil {
		t.Errorf("全新安装不应创建 trap.db（由业务启动流程执行迁移）")
	}
}

func TestResolveMigratesLegacyData(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)

	// 造一个旧 userData 库（合法 SQLite：任意表）。
	legacyDir := filepath.Join(home, "Library", "Application Support", "Trap")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyDB := filepath.Join(legacyDir, "trap.db")
	seedLegacyDB(t, legacyDB)

	res, err := Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.Migrated {
		t.Errorf("应执行迁移: %+v", res)
	}
	// 新库存在且可打开。
	if _, err := os.Stat(filepath.Join(home, ".trap", "trap.db")); err != nil {
		t.Fatalf("新库缺失: %v", err)
	}
	// 旧库已改名备份（不存在原路径，存在 .bak-migrated）。
	if _, err := os.Stat(legacyDB); !os.IsNotExist(err) {
		t.Errorf("旧库应已改名备份")
	}
	if _, err := os.Stat(legacyDB + ".bak-migrated"); err != nil {
		t.Errorf("旧库备份缺失: %v", err)
	}
}

func TestResolveFallbackOnCorruptLegacy(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)

	// 旧「库」是坏文件（非 SQLite）。
	legacyDir := filepath.Join(home, "Library", "Application Support", "Trap")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "trap.db"), []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Resolve(context.Background())
	if err == nil {
		t.Fatal("坏库迁移应报错")
	}
	if !res.Fallback || res.Dir != legacyDir {
		t.Errorf("应回退旧目录: %+v", res)
	}
	// 旧库未被破坏。
	data, err := os.ReadFile(filepath.Join(legacyDir, "trap.db"))
	if err != nil || string(data) != "not a database" {
		t.Errorf("旧库应原样保留")
	}
	// 坏副本应被清理。
	if _, err := os.Stat(filepath.Join(home, ".trap", "trap.db")); !os.IsNotExist(err) {
		t.Errorf("坏副本应被清理")
	}
}

// seedLegacyDB 在指定路径创建一个合法 SQLite 库（探针可打开）。
func seedLegacyDB(t *testing.T, path string) {
	t.Helper()
	conn, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{})
	if err != nil {
		t.Fatalf("创建旧库: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Execute(context.Background(),
		"CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatalf("建表: %v", err)
	}
}
