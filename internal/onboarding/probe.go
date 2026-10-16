// probe.go —— SQLite 探针与注入点（供 onboarding 迁移校验与测试替换）。
//
// 拆出独立文件是为了让 onboarding 包不直接 import 业务 store 包
//（store.Open 语义是「打开并执行迁移」，探针只需要「能打开」）。
package onboarding

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/plugins/sqlite"
)

// openProbe 打开 SQLite（只读用途，不跑迁移——迁移由业务启动流程统一执行）。
func openProbe(ctx context.Context, path string) (*sqlite.DB, error) {
	conn, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, "SELECT count(*) FROM sqlite_master")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("onboarding: 读取 sqlite_master 失败: %w", err)
	}
	if len(rows.Values) == 0 {
		conn.Close()
		return nil, fmt.Errorf("onboarding: sqlite_master 查询无结果")
	}
	return conn, nil
}

// closeProbe 关闭探针连接。
func closeProbe(db *sqlite.DB) error {
	return db.Close()
}
