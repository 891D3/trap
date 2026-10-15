// todos_v2_test.go：待办 v2 扩展字段（分类/日期/项目关联）与过滤条件的测试。
package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTodoExtendedFields(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	project, err := db.AddProject(ctx, "项目X", ColorGreen)
	if err != nil {
		t.Fatalf("新增项目: %v", err)
	}

	// v1 语义兼容：仅标题 = 未分类、未安排、无项目。
	todo, err := db.AddTodo(ctx, TodoInput{Title: "纯标题"})
	if err != nil {
		t.Fatalf("新增待办: %v", err)
	}
	if todo.Category != "" || todo.DueDate != nil || todo.ProjectID != nil {
		t.Errorf("默认字段应为零值: %+v", todo)
	}

	// 全字段。
	due := time.Date(2026, 10, 16, 7, 0, 0, 0, time.UTC).Format(timeRFC3339)
	full, err := db.AddTodo(ctx, TodoInput{
		Title:     "和律师确认条款",
		Category:  CatWork,
		DueDate:   &due,
		ProjectID: int64Ptr(project.ID),
	})
	if err != nil {
		t.Fatalf("新增全字段待办: %v", err)
	}
	if full.Category != CatWork || full.DueDate == nil || *full.DueDate != due || full.ProjectID == nil || *full.ProjectID != project.ID {
		t.Errorf("全字段结果 = %+v", full)
	}

	// 非法取值拒绝。
	if _, err := db.AddTodo(ctx, TodoInput{Title: "x", Category: "urgent"}); err == nil {
		t.Error("非法分类应被拒绝")
	}
	badDue := "2026-10-16 07:00:00"
	if _, err := db.AddTodo(ctx, TodoInput{Title: "x", DueDate: &badDue}); err == nil {
		t.Error("非 RFC3339 日期应被拒绝")
	}
	missID := int64(99999)
	if _, err := db.AddTodo(ctx, TodoInput{Title: "x", ProjectID: &missID}); err == nil {
		t.Error("不存在的项目应被拒绝")
	}

	// Update 全量更新字段；清空日期/项目（显式 nil）与更新为其他项目都有效。
	up, err := db.UpdateTodo(ctx, full.ID, TodoInput{Title: "更新后的标题", Category: CatPersonal})
	if err != nil {
		t.Fatalf("更新待办: %v", err)
	}
	if up.Title != "更新后的标题" || up.Category != CatPersonal || up.DueDate != nil || up.ProjectID != nil {
		t.Errorf("更新结果 = %+v", up)
	}
	if _, err := db.UpdateTodo(ctx, 424242, TodoInput{Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("更新不存在待办应 ErrNotFound, 实际 %v", err)
	}
}

func TestTodoFilters(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	p1, _ := db.AddProject(ctx, "P1", ColorSlate)
	p2, _ := db.AddProject(ctx, "P2", ColorAmber)

	// 种子：
	//   today: 今天 15:00 work / p1
	//   next:  明天 work / p1（已完成）
	//   none:  未安排 study / 无项目
	//   past:  昨天 simple / p2（已完成）
	today := time.Date(2026, 10, 15, 15, 0, 0, 0, time.UTC).Format(timeRFC3339)
	next := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC).Format(timeRFC3339)
	past := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC).Format(timeRFC3339)
	tToday, err := db.AddTodo(ctx, TodoInput{Title: "今天15点", Category: CatWork, DueDate: &today, ProjectID: int64Ptr(p1.ID)})
	if err != nil {
		t.Fatalf("种子今天15点: %v", err)
	}
	tNext, err := db.AddTodo(ctx, TodoInput{Title: "明天9点", Category: CatWork, DueDate: &next, ProjectID: int64Ptr(p1.ID)})
	if err != nil {
		t.Fatalf("种子明天9点: %v", err)
	}
	tNone, err := db.AddTodo(ctx, TodoInput{Title: "未安排", Category: CatStudy})
	if err != nil {
		t.Fatalf("种子未安排: %v", err)
	}
	tPast, err := db.AddTodo(ctx, TodoInput{Title: "昨天完成", Category: "", DueDate: &past, ProjectID: int64Ptr(p2.ID)})
	if err != nil {
		t.Fatalf("种子昨天完成: %v", err)
	}
	if _, err := db.SetTodoDone(ctx, tNext.ID, true); err != nil {
		t.Fatalf("完成任务: %v", err)
	}
	if _, err := db.SetTodoDone(ctx, tPast.ID, true); err != nil {
		t.Fatalf("完成任务: %v", err)
	}

	// 默认排序：未完成在前；区内已安排升序、未安排靠后；已完成沉底
	// （tPast 昨天/ tNext 明天同为已完成，按已安排升序）。
	ordered, err := db.ListTodos(ctx, TodoFilter{})
	if err != nil {
		t.Fatalf("全量列表: %v", err)
	}
	if len(ordered) != 4 || ordered[0].ID != tToday.ID || ordered[1].ID != tNone.ID || ordered[2].ID != tPast.ID || ordered[3].ID != tNext.ID {
		t.Errorf("默认排序 = %+v", ids(ordered))
	}

	list := func(f TodoFilter) map[int64]bool {
		t.Helper()
		got, err := db.ListTodos(ctx, f)
		if err != nil {
			t.Fatalf("过滤列表(%+v): %v", f, err)
		}
		out := map[int64]bool{}
		for _, td := range got {
			out[td.ID] = true
		}
		return out
	}

	// 已完成过滤：tNext（明天）与 tPast（昨天）。
	doneTrue := true
	if got := list(TodoFilter{Done: &doneTrue}); len(got) != 2 || got[tToday.ID] || !got[tNext.ID] || !got[tPast.ID] {
		t.Errorf("过滤已完成 = %v, 期望含明天/昨天不含今天", got)
	}
	if got := list(TodoFilter{Category: CatWork}); len(got) != 2 || !got[tToday.ID] || !got[tNext.ID] {
		t.Errorf("分类过滤 = %v", got)
	}
	if got := list(TodoFilter{HasProjectID: true, ProjectID: int64Ptr(p1.ID)}); len(got) != 2 {
		t.Errorf("项目过滤 = %v", got)
	}
	noDue := true
	if got := list(TodoFilter{NoDue: noDue}); len(got) != 1 || !got[tNone.ID] {
		t.Errorf("未安排过滤 = %v", got)
	}
	from := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC).Format(timeRFC3339)
	if got := list(TodoFilter{DueFrom: &from}); len(got) != 1 || !got[tNext.ID] {
		t.Errorf("日期下界过滤 = %v", got)
	}
	to := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC).Format(timeRFC3339)
	if got := list(TodoFilter{DueTo: &to}); len(got) != 1 || !got[tPast.ID] {
		t.Errorf("日期上界过滤 = %v", got)
	}
	// 无项目过滤。
	if got := list(TodoFilter{FilterNoProject: true}); len(got) != 1 || !got[tNone.ID] {
		t.Errorf("无项目过滤 = %v", got)
	}
}

// ids 取待办 id 列表，供断言信息输出。
func ids(todos []Todo) []int64 {
	out := make([]int64, 0, len(todos))
	for _, td := range todos {
		out = append(out, td.ID)
	}
	return out
}
