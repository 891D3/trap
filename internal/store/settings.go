// settings.go：键值设置表的存取（唯一 SQL 所在包）。
package store

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/plugins/sqlite"
)

// SettingsKey 是 settings 表使用的键。集中定义避免散落的字符串字面量。
type SettingsKey string

const (
	KeyUserName     SettingsKey = "user.name"        // 用户姓名
	KeyUserAvatar   SettingsKey = "user.avatar_path" // 头像文件路径
	KeyAppTheme     SettingsKey = "app.theme"        // 主题：system/light/dark
	KeyAppExportDir SettingsKey = "app.export_dir"   // 信息记录导出目录
	// KeyAppInitialized 是首次安装初始化的完成标记（Onboarding D2）。
	// 判定权威：存在且为 "true" → 已初始化；存量用户由启动 sweep 兜底补写。
	KeyAppInitialized SettingsKey = "app.initialized"
)

// IsInitialized 读取初始化标记（读不到/非 true 均视为未初始化）。
func (db *DB) IsInitialized(ctx context.Context) (bool, error) {
	kv, err := db.GetSettings(ctx)
	if err != nil {
		return false, err
	}
	return kv[string(KeyAppInitialized)] == "true", nil
}

// GetSettings 读取全部键值对；空表返回空 map（非 nil）。
func (db *DB) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.conn.Query(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("store: 读取 settings 失败: %w", err)
	}
	kv := make(map[string]string, len(rows.Values))
	for _, r := range rows.Values {
		kv[columnText(r[0])] = columnText(r[1])
	}
	return kv, nil
}

// SetSettings 在单个事务内批量写入（upsert）。
//
// AGENTS.md §4.4 要求所有写操作在事务内；单条 upsert 本身是原子的，
// 但批量写入（如 UpdateProfile 的姓名+头像）必须同事务，故统一走
// db.Transaction（BEGIN IMMEDIATE，任一语句失败整体回滚）。
func (db *DB) SetSettings(ctx context.Context, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	ts := nowTime().Format(timeRFC3339)
	stmts := make([]sqlite.Statement, 0, len(kv))
	for k, v := range kv {
		stmts = append(stmts, sqlite.Statement{
			SQL: `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			      ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			Args: []any{k, v, ts},
		})
	}
	if _, err := db.conn.Transaction(ctx, stmts); err != nil {
		return fmt.Errorf("store: 写入 settings 失败: %w", err)
	}
	return nil
}
