// migrate_test.go：迁移执行器的行为测试。
//
// 覆盖（验收标准）：
//   - 干净库从零重放 0001：四表与 schema_migrations 表建立，版本=1；
//   - 幂等：已应用版本不重跑（重复 Open 不报错、版本不变、数据保留）；
//   - 失败回滚：后续迁移语句失败时，其变更与版本记录一并回滚。
package store

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/egoist/mygo/plugins/sqlite"
)

// openTempFile 在临时目录开一个文件库。
func openTempFile(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "trap.db"))
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// tableExists 查询 sqlite_master 判断表是否存在。
func tableExists(t *testing.T, db *DB, table string) bool {
	t.Helper()
	rows, err := db.conn.Query(context.Background(),
		"SELECT 1 FROM sqlite_master WHERE type='table' AND name = ?", table)
	if err != nil {
		t.Fatalf("查询 sqlite_master: %v", err)
	}
	return len(rows.Values) > 0
}

func TestMigrateFreshReplay(t *testing.T) {
	db := openTempFile(t)

	// 五表全部建立（0002 新增 projects）。
	for _, table := range []string{"settings", "todos", "notes", "quick_links", "projects"} {
		if !tableExists(t, db, table) {
			t.Errorf("干净库迁移后缺少表 %q", table)
		}
	}
	// 版本记录到 0002。
	v, err := db.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("读取版本: %v", err)
	}
	if v != 2 {
		t.Errorf("版本 = %d, 期望 2", v)
	}
}

// TestMigrateIdempotent：重复 Open（即零重放后再执行迁移）不报错，
// 版本不重跑，且已有数据保留。间接验证 applyMigration 的事务边界。
func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trap.db")
	ctx := context.Background()

	db1, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("第一次打开: %v", err)
	}
	created, err := db1.AddTodo(ctx, TodoInput{Title: "迁移前的数据"})
	if err != nil {
		t.Fatalf("写入数据: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("第一次关闭: %v", err)
	}

	// 重新打开：应跳过 0001/0002，数据无损。
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("第二次打开: %v", err)
	}
	defer db2.Close()

	v, err := db2.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("读取版本: %v", err)
	}
	if v != 2 {
		t.Errorf("版本 = %d, 期望仍为 2（不重跑）", v)
	}
	todo, err := db2.GetTodo(ctx, created.ID)
	if err != nil {
		t.Fatalf("重开后读回待办: %v", err)
	}
	if todo.Title != "迁移前的数据" {
		t.Errorf("重开后数据变化: %+v", todo)
	}
}

// TestMigrateRollbackOnFailure：坏迁移（0003）内一条语句失败时，整个迁移
// （含同事务内的其他语句与版本记录）必须回滚，0001 的表不受影响。
func TestMigrateRollbackOnFailure(t *testing.T) {
	// 业务层面的 embed FS 在 store 包内是固定的，无法注入坏迁移；
	// 因此迁移逻辑抽出 migrateFS(ctx, fsys)，测试用 fstest.MapFS 注入。
	// 0001 用真实迁移文件内容，保证与生产路径一致。
	real0001, err := migrationsFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("读取内置 0001 迁移: %v", err)
	}
	badFS := fstest.MapFS{
		"migrations/0001_init.sql": {Data: real0001},
		"migrations/0003_bad.sql": {
			Data: []byte("CREATE TABLE should_rollback (id INTEGER);\n" + // 先成功
				"CREATE (语法错误);"), // 再失败
		},
	}

	dir := t.TempDir()
	conn, err := sqlite.Open(context.Background(), filepath.Join(dir, "trap.db"), sqlite.OpenOptions{})
	if err != nil {
		t.Fatalf("直接打开连接: %v", err)
	}
	defer conn.Close()

	db := &DB{conn: conn}
	err = db.migrateFS(context.Background(), badFS)
	if err == nil {
		t.Fatal("坏迁移应当报错")
	}

	// 本注入 FS 只含 0001 与坏 0003，故版本停在 1；坏迁移的表与其版本
	// 记录必须不存在（真实 0002 不参与本用例，其回滚语义由同机制保证）。
	v, err := db.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("读取版本: %v", err)
	}
	if v != 1 {
		t.Errorf("版本 = %d, 期望 1（坏迁移已回滚）", v)
	}
	if tableExists(t, db, "should_rollback") {
		t.Error("坏迁移中先执行的语句未回滚：should_rollback 表仍然存在")
	}
	// 0001 成功应用，它的四表应当存在。
	for _, table := range []string{"settings", "todos", "notes", "quick_links"} {
		if !tableExists(t, db, table) {
			t.Errorf("0001 的 %s 表应存在", table)
		}
	}
}
