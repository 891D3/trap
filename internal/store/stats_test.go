// stats_test.go：数据看板聚合计数的测试（固定时间边界，v2.0 R1）。
package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// fixedLoc 是固定时区（UTC+8），保证测试对宿主时区无感知。
func fixedLoc() *time.Location {
	return time.FixedZone("FIXED+8", 8*3600)
}

// statsNow 固定「现在」：2026-10-15 20:00（UTC+8）。
func statsNow() time.Time {
	return time.Date(2026, 10, 15, 20, 0, 0, 0, fixedLoc())
}

// dueAt 返回 UTC+8 时区某日时刻的 UTC RFC3339 文本。
func dueAt(y int, m time.Month, d, hh, mm int) string {
	return time.Date(y, m, d, hh, mm, 0, 0, fixedLoc()).UTC().Format(timeRFC3339)
}

// statsTodo 构造带日期的待办。
func statsTodo(t *testing.T, db *DB, title, due string, projectID *int64) Todo {
	t.Helper()
	todo, err := db.AddTodo(context.Background(), TodoInput{Title: title, DueDate: &due, ProjectID: projectID})
	if err != nil {
		t.Fatalf("种子待办 %q: %v", title, err)
	}
	return todo
}

// setNoteCreatedAt 是 stats 测试专用的时间回填（ limiting：业务无「改创建时间」，
// 但看板近 7 日窗口需要确定性时间）。
func setNoteCreatedAt(ctx context.Context, db *DB, backfill map[int64]string) error {
	stmts := []sqlite.Statement{}
	for id, at := range backfill {
		stmts = append(stmts, sqlite.Statement{
			SQL:  "UPDATE notes SET created_at = ? WHERE id = ?",
			Args: []any{at, id},
		})
	}
	_, err := db.conn.Transaction(ctx, stmts)
	return err
}

// TestStatsAggregate 覆盖聚合口径（本地天边界 UTC+8 折算）：
//   - 今日未完成 KPI = 今日 due 未完成 + 过期未完成（并入今天应做）；
//   - 昨日口径 KPI = due <= 昨日末的未完成；
//   - 近 7 日记录窗口；
//   - 时间轴 = 当日 due 待办 + 当日新建记录，按 At 升序。
func TestStatsAggregate(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	proj := mustProject(t, db)

	// 待办种子。
	today23 := statsTodo(t, db, "今天23点", dueAt(2026, 10, 15, 23, 0), int64Ptr(proj.ID))
	_ = statsTodo(t, db, "昨天23点未完成", dueAt(2026, 10, 14, 23, 0), int64Ptr(proj.ID))
	yDone := statsTodo(t, db, "昨天23点已完成", dueAt(2026, 10, 14, 23, 0), int64Ptr(proj.ID))
	next := statsTodo(t, db, "明天9点", dueAt(2026, 10, 16, 9, 0), nil)
	noDue, err := db.AddTodo(ctx, TodoInput{Title: "未安排"})
	if err != nil {
		t.Fatalf("种子未安排: %v", err)
	}
	_ = noDue
	_ = next
	if _, err := db.SetTodoDone(ctx, yDone.ID, true); err != nil {
		t.Fatalf("完成种子待办: %v", err)
	}

	// 记录种子：CreateNote 后回填 created_at 以覆盖 7 日窗口与当日聚合。
	n1, err := db.CreateNote(ctx, "灵感A")
	if err != nil {
		t.Fatalf("种子记录A: %v", err)
	}
	n2, err := db.CreateNote(ctx, "灵感B")
	if err != nil {
		t.Fatalf("种子记录B: %v", err)
	}
	old1, err := db.CreateNote(ctx, "灵感C")
	if err != nil {
		t.Fatalf("种子记录C: %v", err)
	}
	ancient, err := db.CreateNote(ctx, "灵感D")
	if err != nil {
		t.Fatalf("种子记录D: %v", err)
	}
	err = setNoteCreatedAt(ctx, db, map[int64]string{
		n1.ID:    dueAt(2026, 10, 15, 9, 30),
		n2.ID:    dueAt(2026, 10, 15, 17, 0),
		old1.ID:  dueAt(2026, 10, 12, 10, 0),
		ancient.ID: dueAt(2026, 10, 5, 10, 0),
	})
	if err != nil {
		t.Fatalf("回填 created_at: %v", err)
	}

	st, err := db.Stats(ctx, statsNow(), fixedLoc())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	// 已完成待办的时间轴排除（yDone 不应在结果中）。
	if st.TodayTodoCount != 2 {
		t.Errorf("TodayTodoCount = %d, 期望 2（今天23点 + 过期的昨天23点）", st.TodayTodoCount)
	}
	if st.YesterdayTodoCount != 1 {
		t.Errorf("YesterdayTodoCount = %d, 期望 1（仅昨天23点未完成）", st.YesterdayTodoCount)
	}
	if st.ActiveProjects != 1 {
		t.Errorf("ActiveProjects = %d, 期望 1", st.ActiveProjects)
	}
	if st.NoteTotal != 4 {
		t.Errorf("NoteTotal = %d, 期望 4", st.NoteTotal)
	}
	if st.NoteWeekNew != 3 {
		t.Errorf("NoteWeekNew = %d, 期望 3（10 天前的灵感D 不计）", st.NoteWeekNew)
	}
	if len(st.ProjectProgress) != 1 || st.ProjectProgress[0].Open != 2 || st.ProjectProgress[0].Total != 3 {
		t.Errorf("项目进度 = %+v, 期望 open 2 total 3", st.ProjectProgress)
	}

	// 时间轴：today23、n1、n2（共 3 条；yDone 已完成不在当日待办里、nC/D 非当日）。
	if len(st.TodayAgenda) != 3 {
		t.Fatalf("TodayAgenda = [%d 条] %+v, 期望 3 条", len(st.TodayAgenda), st.TodayAgenda)
	}
	// 排序按 At 的 UTC 字面序：n1(01:30Z) → n2(09:00Z) → today23(15:00Z)。
	first := st.TodayAgenda[0]
	if first.Kind != "note" || first.Title != "灵感A" {
		t.Errorf("时间轴第1项 = %+v, 期望 灵感A", first)
	}
	if at := st.TodayAgenda[0].At; at == nil || *at != dueAt(2026, 10, 15, 9, 30) {
		t.Errorf("灵感A 的 At = %+v", at)
	}
	if second := st.TodayAgenda[1]; second.Kind != "note" || second.Title != "灵感B" {
		t.Errorf("时间轴第2项 = %+v, 期望 灵感B", second)
	}
	third := st.TodayAgenda[2]
	if third.Kind != "todo" || third.Title != "今天23点" || third.Status != "scheduled" {
		t.Errorf("时间轴第3项 = %+v", third)
	}
	if third.RefID != today23.ID {
		t.Errorf("时间轴待办 RefID = %d, 期望 %d", third.RefID, today23.ID)
	}
	// 今天23点（23:00 UTC+8 = 15:00Z）
	if at := st.TodayAgenda[2].At; at == nil || *at != dueAt(2026, 10, 15, 23, 0) {
		t.Errorf("今天23点 的 At = %+v", at)
	}
	// yOver/yDone 不在时间轴内（昨日数据）——由数量断言覆盖。
}

// mustProject 创建一个项目并返回（Stats 测试用）。
func mustProject(t *testing.T, db *DB) Project {
	t.Helper()
	p, err := db.AddProject(context.Background(), "聚合测试项目", ColorGreen)
	if err != nil {
		t.Fatalf("创建项目: %v", err)
	}
	return p
}

// pretty 仅在错误信息里输出可读 JSON（断言失败时辅助定位）。
func pretty(v any) string {
	return fmt.Sprintf("%+v", v)
}
