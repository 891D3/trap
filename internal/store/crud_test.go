// crud_test.go：四表 CRUD 与设置读写、写操作事务行为的测试。
package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// mustParseTime 校验时间字段符合 UTC RFC3339（AGENTS.md §4.1 的可回放保证）。
func mustParseTime(t *testing.T, label, v string) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, v)
	if err != nil {
		t.Errorf("%s = %q 不是 RFC3339: %v", label, v, err)
		return
	}
	if parsed.Location() != time.UTC {
		t.Errorf("%s = %q 不是 UTC（时区 %v）", label, v, parsed.Location())
	}
}

// ---- settings ----

func TestSettingsRoundtrip(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	before, err := db.GetSettings(ctx)
	if err != nil {
		t.Fatalf("读空表: %v", err)
	}
	if len(before) != 0 {
		t.Errorf("空表应有 0 条键值, 实际 %d", len(before))
	}

	kv := map[string]string{
		string(KeyUserName):     "小明",
		string(KeyUserAvatar):   "/tmp/a.png",
		string(KeyAppTheme):     "dark",
		string(KeyAppExportDir): "/tmp/export",
	}
	if err := db.SetSettings(ctx, kv); err != nil {
		t.Fatalf("批量写入: %v", err)
	}
	got, err := db.GetSettings(ctx)
	if err != nil {
		t.Fatalf("回读: %v", err)
	}
	for k, want := range kv {
		if got[k] != want {
			t.Errorf("settings[%q] = %q, 期望 %q", k, got[k], want)
		}
	}
}

// TestSettingsUpsert：同名键重复写入应更新（不得因主键冲突报错）。
func TestSettingsUpsert(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	if err := db.SetSettings(ctx, map[string]string{string(KeyAppTheme): "dark"}); err != nil {
		t.Fatalf("第一次写入: %v", err)
	}
	if err := db.SetSettings(ctx, map[string]string{string(KeyAppTheme): "light"}); err != nil {
		t.Fatalf("upsert 写入: %v", err)
	}
	got, err := db.GetSettings(ctx)
	if err != nil {
		t.Fatalf("回读: %v", err)
	}
	if got[string(KeyAppTheme)] != "light" {
		t.Errorf("theme = %q, 期望 light", got[string(KeyAppTheme)])
	}
}

// TestSettingsRollback：事务内任一语句失败，整批设置不得落库。
// 通过向 setSettingsStmts 追加一条必然失败的语句来触发回滚。
func TestSettingsRollback(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	stmts := []sqlite.Statement{
		{
			SQL:  "INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)",
			Args: []any{"app.ok", "1", nowTime().Format(timeRFC3339)},
		},
		{SQL: "UPDATE settings SET 不存在的列 = 1"}, // 必然失败
	}
	if _, err := db.conn.Transaction(ctx, stmts); err == nil {
		t.Fatal("坏语句应当报错")
	}

	got, err := db.GetSettings(ctx)
	if err != nil {
		t.Fatalf("回读: %v", err)
	}
	if got["app.ok"] != "" {
		t.Errorf("事务回滚失效：app.ok = %q, 期望不存在（空）", got["app.ok"])
	}
}

// ---- todos ----

func TestTodoCRUDAndCompletedAt(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	// 新增。
	todo, err := db.AddTodo(ctx, TodoInput{Title: "  买牛奶  "})
	if err != nil {
		t.Fatalf("新增: %v", err)
	}
	if todo.Title != "买牛奶" {
		t.Errorf("标题应去首尾空白: %q", todo.Title)
	}
	if todo.Done || todo.CompletedAt != nil {
		t.Errorf("新待办应为未完成: %+v", todo)
	}
	mustParseTime(t, "createdAt", todo.CreatedAt)
	mustParseTime(t, "updatedAt", todo.UpdatedAt)

	// 勾选：completed_at 写入。
	done, err := db.SetTodoDone(ctx, todo.ID, true)
	if err != nil {
		t.Fatalf("勾选: %v", err)
	}
	if !done.Done || done.CompletedAt == nil {
		t.Errorf("勾选后应有 completed_at: %+v", done)
	}
	mustParseTime(t, "completedAt", *done.CompletedAt)

	// 取消：completed_at 置 NULL。
	undone, err := db.SetTodoDone(ctx, todo.ID, false)
	if err != nil {
		t.Fatalf("取消勾选: %v", err)
	}
	if undone.Done || undone.CompletedAt != nil {
		t.Errorf("取消后 completed_at 应为 NULL: %+v", undone)
	}

	// 列表。
	list, err := db.ListTodos(ctx, TodoFilter{})
	if err != nil {
		t.Fatalf("列表: %v", err)
	}
	if len(list) != 1 || list[0].ID != todo.ID {
		t.Errorf("列表 = %+v", list)
	}

	// 删除后再删除报 ErrNotFound。
	if err := db.DeleteTodo(ctx, todo.ID); err != nil {
		t.Fatalf("删除: %v", err)
	}
	if err := db.DeleteTodo(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复删除应返回 ErrNotFound, 实际 %v", err)
	}
	if _, err := db.GetTodo(ctx, todo.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("读已删除待办应返回 ErrNotFound, 实际 %v", err)
	}

	// 空标题拒绝。
	if _, err := db.AddTodo(ctx, TodoInput{Title: "   "}); err == nil {
		t.Error("空标题应被拒绝")
	}
}

// ---- notes ----

func TestNoteCRUDArchiveAndList(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	n1, err := db.CreateNote(ctx, "读书笔记")
	if err != nil {
		t.Fatalf("新建: %v", err)
	}
	n2, _ := db.CreateNote(ctx, "")
	if n2.Content != "" || n2.Archived {
		t.Errorf("新记录应为空正文的未归档记录: %+v", n2)
	}

	// 保存正文。
	saved, err := db.SaveNote(ctx, n1.ID, "读书笔记", "# 第一章\n\n内容")
	if err != nil {
		t.Fatalf("保存: %v", err)
	}
	if saved.Content != "# 第一章\n\n内容" {
		t.Errorf("正文未保存: %q", saved.Content)
	}
	mustParseTime(t, "updatedAt", saved.UpdatedAt)

	// 归档后默认列表不可见。
	if _, err := db.SetNoteArchived(ctx, n1.ID, true); err != nil {
		t.Fatalf("归档: %v", err)
	}
	active, err := db.ListNotes(ctx, false)
	if err != nil {
		t.Fatalf("列未归档: %v", err)
	}
	if len(active) != 1 || active[0].ID != n2.ID {
		t.Errorf("未归档列表 = %+v, 期望只剩 n2", active)
	}
	all, err := db.ListNotes(ctx, true)
	if err != nil {
		t.Fatalf("列全部: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("全部列表应有 2 条, 实际 %d", len(all))
	}

	// 摘要不携带正文（本行仅防字段膨胀——NoteSummary 无 Content 字段，编译期即保证）。
	// 删除。
	if err := db.DeleteNote(ctx, n1.ID); err != nil {
		t.Fatalf("删除: %v", err)
	}
	if err := db.DeleteNote(ctx, n1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复删除应返回 ErrNotFound, 实际 %v", err)
	}
	if _, err := db.GetNote(ctx, n1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("读已删除记录应返回 ErrNotFound, 实际 %v", err)
	}
}

// ---- quick_links ----

func TestQuickLinkCRUDAndOrder(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	l1, err := db.AddQuickLink(ctx, "GitHub", "https://github.com", KindURL, "")
	if err != nil {
		t.Fatalf("新增 url: %v", err)
	}
	l2, err := db.AddQuickLink(ctx, "终端", "iTerm2", KindPath, "工具")
	if err != nil {
		t.Fatalf("新增 path: %v", err)
	}
	if l1.SortOrder != 1 || l2.SortOrder != 2 {
		t.Errorf("sort_order 应自动递增: %d, %d", l1.SortOrder, l2.SortOrder)
	}
	mustParseTime(t, "l1.createdAt", l1.CreatedAt)

	// 非法 kind 拒绝。
	if _, err := db.AddQuickLink(ctx, "x", "y", "file", ""); err == nil {
		t.Error("非法 kind 应被拒绝")
	}

	// 更新（移动排序、改分组）。
	up, err := db.UpdateQuickLink(ctx, l2.ID, l2.Name, l2.Target, l2.Kind, "", 0)
	if err != nil {
		t.Fatalf("更新: %v", err)
	}
	if up.GroupName != "" || up.SortOrder != 0 {
		t.Errorf("更新结果 = %+v", up)
	}

	// 列表按 sort_order 排序：l2(0) 在 l1(1) 前。
	list, err := db.ListQuickLinks(ctx)
	if err != nil {
		t.Fatalf("列表: %v", err)
	}
	if len(list) != 2 || list[0].ID != l2.ID || list[1].ID != l1.ID {
		t.Errorf("列表顺序 = %+v", list)
	}

	// 删除与 NotFound。
	if err := db.DeleteQuickLink(ctx, l1.ID); err != nil {
		t.Fatalf("删除: %v", err)
	}
	if err := db.DeleteQuickLink(ctx, l1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复删除应返回 ErrNotFound, 实际 %v", err)
	}
}
