// note_v2_test.go：信息记录 v2 服务层测试（PRD v2.1 §3.5 验收 1/5/7）。
//
// sweep 天数边界测试在 store 包（notes_v2_test.go），因为回填 updated_at
// 需要直连事务（service 层无该入口）。
package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo/transfer"
)

func newNoteServiceV2(t *testing.T) (*NoteService, *TodoService) {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "note-v2.db"))
	if err != nil {
		t.Fatalf("打开临时数据库: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	notes := &NoteService{}
	todos := &TodoService{}
	AttachNoteStore(notes, db)
	AttachTodoStore(todos, db)
	AttachTodoPeer(notes, todos)
	return notes, todos
}

func TestCreateSparkAndList(t *testing.T) {
	notes, _ := newNoteServiceV2(t)
	ctx := context.Background()

	if _, err := notes.CreateSpark(ctx, "   ", nil); err == nil {
		t.Fatal("空内容灵感应被拒绝")
	}
	sp, err := notes.CreateSpark(ctx, "竞品 A 的 bundle 定价策略值得抄", nil)
	if err != nil {
		t.Fatalf("CreateSpark: %v", err)
	}
	if sp.Kind != store.NoteKindSpark {
		t.Errorf("kind = %q, 期望 spark", sp.Kind)
	}
	sparks, err := notes.ListSparks(ctx, false)
	if err != nil || len(sparks) != 1 {
		t.Fatalf("ListSparks = %v err=%v, 期望 1 条", sparks, err)
	}
}

func TestSparkTitleTruncates(t *testing.T) {
	long := "这是一条非常长的灵感内容用于验证标题截断到二十个字符的边界行为"
	if got := sparkTitle(long); len([]rune(got)) != 20 {
		t.Errorf("标题应为 20 字，实际 %d: %q", len([]rune(got)), got)
	}
}

func TestPromoteSparkToNoteKeepsSpark(t *testing.T) {
	notes, _ := newNoteServiceV2(t)
	ctx := context.Background()
	sp, _ := notes.CreateSpark(ctx, "侧边栏图标 hover 加 tooltip 提示", nil)

	res, err := notes.PromoteSpark(ctx, sp.ID, "note")
	note := res.Note
	if err != nil {
		t.Fatalf("PromoteSpark(note): %v", err)
	}
	if note.Kind != store.NoteKindNote {
		t.Errorf("新实体 kind = %q, 期望 note", note.Kind)
	}
	// 正文以引用格式承接灵感内容（契约口径）。
	if !strings.HasPrefix(note.Content, "> ") {
		t.Errorf("正文应以引用开始: %q", note.Content)
	}
	// 原灵感保留（用户 2026-10-16 裁定：转笔记不删除，删除由用户执行）。
	sparks, _ := notes.ListSparks(ctx, false)
	if len(sparks) != 1 || sparks[0].ID != sp.ID {
		t.Errorf("原灵感应保留，实际 %v", sparks)
	}
}

func TestPromoteSparkToTodo(t *testing.T) {
	notes, todos := newNoteServiceV2(t)
	ctx := context.Background()
	if _, err := notes.CreateSpark(ctx, "比价表要加付款条件列", nil); err != nil {
		t.Fatal(err)
	}
	sparks, _ := notes.ListSparks(ctx, false)
	res, err := notes.PromoteSpark(ctx, sparks[0].ID, "todo")
	if err != nil {
		t.Fatalf("PromoteSpark(todo): %v", err)
	}
	if res.Todo == nil || res.Todo.ID == 0 {
		t.Fatalf("转待办应返回新待办: %+v", res)
	}
	// 待办已创建（title = 灵感内容）；灵感保留。
	td, err := todos.List(ctx, store.TodoFilter{})
	if err != nil || len(td) != 1 || td[0].Title != "比价表要加付款条件列" {
		t.Fatalf("转待办未生效: %+v err=%v", td, err)
	}
	if _, err := notes.PromoteSpark(ctx, sparks[0].ID, "unknown"); err == nil {
		t.Error("未知转换目标应报错")
	}
}

func TestArchiveAndRestoreSpark(t *testing.T) {
	notes, _ := newNoteServiceV2(t)
	ctx := context.Background()
	sp, _ := notes.CreateSpark(ctx, "周报模板改成结论先行", nil)

	arch, err := notes.ArchiveSpark(ctx, sp.ID)
	if err != nil {
		t.Fatalf("ArchiveSpark: %v", err)
	}
	if arch.ArchivedAt == nil || !arch.Archived {
		t.Errorf("归档结果 = %+v", arch)
	}
	open, _ := notes.ListSparks(ctx, false)
	if len(open) != 0 {
		t.Errorf("归档后未处理流应空")
	}
	archiveList, _ := notes.ListSparks(ctx, true)
	if len(archiveList) != 1 {
		t.Errorf("归档列表应 1 条")
	}
	// 幂等：再归档不报错。
	if _, err := notes.ArchiveSpark(ctx, sp.ID); err != nil {
		t.Errorf("重复归档应幂等: %v", err)
	}
	back, err := notes.RestoreSpark(ctx, sp.ID)
	if err != nil {
		t.Fatalf("RestoreSpark: %v", err)
	}
	if back.ArchivedAt != nil {
		t.Errorf("恢复后 archived_at 应为 NULL: %+v", back)
	}
}

func TestLinkProjectValidates(t *testing.T) {
	notes, _ := newNoteServiceV2(t)
	ctx := context.Background()
	sp, _ := notes.CreateSpark(ctx, "灵感", nil)
	missID := int64(99999)
	if _, err := notes.LinkProject(ctx, sp.ID, &missID); err == nil {
		t.Error("关联不存在的项目应报错")
	}
}

func TestListNotesV2TagFilter(t *testing.T) {
	notes, _ := newNoteServiceV2(t)
	ctx := context.Background()
	n1, _ := notes.Create(ctx, "标签甲")
	n2, _ := notes.Create(ctx, "标签乙")
	if _, err := notes.SaveNoteV2(ctx, n1.ID, "标签甲", "#选品 讨论纪要\n#周报", nil); err != nil {
		t.Fatalf("SaveNoteV2Service: %v", err)
	}
	if _, err := notes.SaveNoteV2(ctx, n2.ID, "标签乙", "没有标签的笔记", nil); err != nil {
		t.Fatal(err)
	}
	got, err := notes.ListNotesV2(ctx, nil, "选品")
	if err != nil {
		t.Fatalf("ListNotesV2(tag): %v", err)
	}
	if len(got) != 1 || got[0].ID != n1.ID {
		t.Errorf("标签过滤结果 = %+v, 期望仅 n1", got)
	}
}

// 测试注入剪贴板写入（不触真实系统剪贴板）。
func TestCopyRichTextGeneratesHTML(t *testing.T) {
	notes, _ := newNoteServiceV2(t)
	ctx := context.Background()
	n1, _ := notes.Create(ctx, "文档标题")
	if _, err := notes.SaveNoteV2(ctx, n1.ID, "文档标题", "# 大标题\n\n- 列表甲\n- [x] 任务乙", nil); err != nil {
		t.Fatal(err)
	}

	var gotHTML, gotPlain string
	clipboardWriter = func(data transfer.Data) error {
		if b, err := data.Read(transfer.HTML); err == nil {
			gotHTML = string(b)
		}
		if b, err := data.Read(transfer.Text); err == nil {
			gotPlain = string(b)
		}
		return nil
	}
	t.Cleanup(func() { clipboardWriter = writeClipboardData })

	if err := notes.CopyRichText(ctx, n1.ID); err != nil {
		t.Fatalf("CopyRichText: %v", err)
	}
	for _, want := range []string{"<li>列表甲</li>", "checked", "<h1>文档标题</h1>"} {
		if !strings.Contains(gotHTML, want) {
			t.Errorf("HTML 缺 %q: %s", want, gotHTML)
		}
	}
	// 纯文本 flavor = 原始 markdown（契约：GFM 存取零转换）。
	for _, want := range []string{"- [x] 任务乙", "- 列表甲"} {
		if !strings.Contains(gotPlain, want) {
			t.Errorf("纯文本缺 %q: %s", want, gotPlain)
		}
	}
}
