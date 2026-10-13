// Package store 是全仓库唯一允许写 SQL 的包（AGENTS.md §4.1）。
//
// 职责：
//   - 持有 SQLite 连接（mygo 内置 plugins/sqlite，purego 构建、无需 cgo）；
//   - 启动时执行版本化迁移（embedded SQL，每个迁移在单个事务内执行，幂等）；
//   - 提供 settings/todos/notes/quick_links 四表的存取方法，供 service 层调用。
//
// 并发约定：mygo 的 sqlite.DB 可被任意 goroutine 使用，连接内部对操作串行化；
// 时间统一存 UTC RFC3339 文本（AGENTS.md §4.1）。
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// schemaMigrations 记录已应用迁移版本号的表名（由迁移器自行创建）。
const schemaMigrations = "schema_migrations"

// DB 持有 SQLite 连接。
type DB struct {
	conn *sqlite.DB
}

// Open 打开数据库文件并执行全部未应用的迁移。
//
// path 可以是文件路径（父目录必须已存在，由调用方负责，例如 main.go 用
// mygo 的 userData 目录）或 ":memory:"（测试用）。
func Open(ctx context.Context, path string) (*DB, error) {
	conn, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, fmt.Errorf("store: 打开数据库 %q 失败: %w", path, err)
	}
	db := &DB{conn: conn}
	if err := db.migrate(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// Close 关闭底层连接。重复调用安全。
func (db *DB) Close() error {
	if db == nil || db.conn == nil {
		return nil
	}
	if err := db.conn.Close(); err != nil {
		return fmt.Errorf("store: 关闭数据库失败: %w", err)
	}
	return nil
}

// Version 返回当前 schema 版本（已应用的最大迁移号）；空库返回 0。
// 供测试与诊断使用。
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM "+schemaMigrations)
	if err != nil {
		return 0, fmt.Errorf("store: 读取 schema 版本失败: %w", err)
	}
	if len(rows.Values) == 0 || rows.Values[0][0] == nil {
		return 0, nil
	}
	v, ok := rows.Values[0][0].(int64)
	if !ok {
		return 0, fmt.Errorf("store: schema 版本类型异常: %T", rows.Values[0][0])
	}
	return int(v), nil
}

// nowTime 返回当前 UTC 时间（RFC3339 格式化统一在调用处完成）。
func nowTime() time.Time { return time.Now().UTC() }

// migrate 执行全部未应用迁移。
//
// 设计（AGENTS.md §4.4）：
//   - 迁移文件按文件名编号升序执行；
//   - 每个迁移的 SQL 与版本记录写入在**单个事务**内完成，保证崩溃后不会
//     出现「表建了一半、版本未记录」的状态；
//   - 已应用的版本号不重跑（幂等），重复调用 Open 安全。
func (db *DB) migrate(ctx context.Context) error {
	return db.migrateFS(ctx, migrationsFS)
}

// migrateFS 是 migrate 的内部实现，接受任意 fs.FS；测试用 fstest.MapFS
// 注入坏迁移以验证事务回滚。
func (db *DB) migrateFS(ctx context.Context, mfs fs.FS) error {
	// 先建版本记录表；它自身不在事务内也能保证安全——建表是幂等的
	// （CREATE TABLE IF NOT EXISTS），且失败时整个 Open 直接报错退出。
	if _, err := db.conn.Execute(ctx,
		`CREATE TABLE IF NOT EXISTS `+schemaMigrations+` (
		   version    INTEGER PRIMARY KEY,
		   applied_at TEXT NOT NULL
		 )`); err != nil {
		return fmt.Errorf("store: 创建 %s 表失败: %w", schemaMigrations, err)
	}

	// 读取已应用版本。
	rows, err := db.conn.Query(ctx, "SELECT version FROM "+schemaMigrations)
	if err != nil {
		return fmt.Errorf("store: 读取已应用迁移失败: %w", err)
	}
	applied := make(map[int]bool, len(rows.Values))
	for _, r := range rows.Values {
		if v, ok := r[0].(int64); ok {
			applied[int(v)] = true
		}
	}

	// 从 embed FS 枚举迁移文件，按编号升序。
	entries, err := fs.Glob(mfs, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("store: 枚举迁移文件失败: %w", err)
	}
	sort.Strings(entries)

	for _, entry := range entries {
		version, err := fileNameVersion(entry)
		if err != nil {
			return fmt.Errorf("store: 解析迁移文件名 %q: %w", entry, err)
		}
		if applied[version] {
			continue // 已应用，跳过（幂等）
		}
		sqlBytes, err := fs.ReadFile(mfs, entry)
		if err != nil {
			return fmt.Errorf("store: 读取迁移 %q 失败: %w", entry, err)
		}
		if err := db.applyMigration(ctx, version, string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration 在单个事务内执行一个迁移并写入版本记录。
//
// mygo sqlite 的 Transaction 使用 BEGIN IMMEDIATE，任一语句失败即整体回滚；
// 它会拒绝事务控制语句，因此这里不手写 BEGIN/COMMIT。
func (db *DB) applyMigration(ctx context.Context, version int, sqlText string) error {
	// 拆分 SQL 文件为独立语句（见 splitSQL 的注释）。
	statements, err := splitSQL(sqlText)
	if err != nil {
		return fmt.Errorf("store: 迁移 %d 拆分失败: %w", version, err)
	}
	statements = append(statements, sqlite.Statement{
		SQL:  "INSERT INTO " + schemaMigrations + " (version, applied_at) VALUES (?, ?)",
		Args: []any{version, nowTime().Format(time.RFC3339)},
	})
	if _, err := db.conn.Transaction(ctx, statements); err != nil {
		return fmt.Errorf("store: 迁移 %d 执行失败: %w", version, err)
	}
	return nil
}

// fileNameVersion 从 "migrations/0001_init.sql" 提取版本号 1。
func fileNameVersion(entry string) (int, error) {
	base := entry[strings.LastIndexByte(entry, '/')+1:]
	dot := strings.IndexByte(base, '.')
	if dot <= 0 {
		return 0, errors.New("store: 迁移文件名缺少编号")
	}
	num := base[:dot]
	// 编号格式：下划线前的纯数字（如 0001）。
	if i := strings.IndexByte(num, '_'); i >= 0 {
		num = num[:i]
	}
	v, err := strconv.Atoi(num)
	if err != nil {
		return 0, fmt.Errorf("store: 编号 %q 不是数字: %w", num, err)
	}
	return v, nil
}

// splitSQL 把一个迁移文件拆成独立语句。
//
// 说明：mygo sqlite 的 Execute 一次只能执行一条语句，因此多个 DDL 需要
// 按分号拆分。拆分是文本级的：SQLite 中字符串字面量以单引号包裹且不
// 跨行转义，'--' 行注释与 '/* */' 块注释中的分号也随之保留在语句内——
// 而 0001_init.sql 只有注释与 DDL，不含字符串字面量分号，故此级拆分
// 对当前迁移是安全的；若未来迁移包含含分号的字符串，需改为更严格的解析。
func splitSQL(sqlText string) ([]sqlite.Statement, error) {
	// 去掉 -- 行注释后按 ; 拆分，避免把注释里的分号当分隔符。
	var b strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	cleaned := b.String()

	var statements []sqlite.Statement
	for _, part := range strings.Split(cleaned, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		statements = append(statements, sqlite.Statement{SQL: part})
	}
	if len(statements) == 0 {
		return nil, errors.New("store: 迁移文件没有语句")
	}
	return statements, nil
}
