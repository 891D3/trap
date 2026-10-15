// stats.go：数据看板聚合查询（唯一 SQL 所在包）。
//
// 口径（PRD v2.0 §4.2 + §10 拟定决策）：
//   - 今日待办 KPI：未完成且 due_date <= 今日 24:00（本地时时区折算 UTC 比较，
//     含过期未完成）；
//   - 昨日同口径计数用于「较昨日」差值；
//   - 今日信息时间轴 = 当日 due 的待办 + 当日新建的记录；
//   - 项目进度 = 每项目未完成/全部（百分比由前端计算，便于 Edge case 处理）。
//
// 时间参数由调用方传入（service 用 time.Now()；测试可注入固定时间），
// day 边界按调用方本地时区计算后转 UTC RFC3339 字符串（文本序与时间序一致）。
package store

import (
	"context"
	"fmt"
	"time"
)

// AgendaItem 是「今日信息」时间轴的一项。
type AgendaItem struct {
	Kind   string  `json:"kind"` // "todo" | "note"
	RefID  int64   `json:"refId"`
	Title  string  `json:"title"`
	At     *string `json:"at,omitempty"` // UTC RFC3339（todo.due_date 或 note.created_at）
	Status string  `json:"status"`       // todo: done|scheduled；note: logged
}

// ProjectProgress 是单个项目的进度聚合。
type ProjectProgress struct {
	ProjectID int64  `json:"projectId"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Open      int    `json:"open"`
	Total     int    `json:"total"`
}

// DashboardStats 是数据看板的全部聚合结果（PRD v2.0 §4.2）。
type DashboardStats struct {
	TodayTodoCount     int  `json:"todayTodoCount"`
	YesterdayTodoCount int  `json:"yesterdayTodoCount"`
	ActiveProjects     int  `json:"activeProjects"`
	NoteTotal          int  `json:"noteTotal"`
	NoteWeekNew        int  `json:"noteWeekNew"`
	TodayAgenda        []AgendaItem `json:"todayAgenda"`
	ProjectProgress    []ProjectProgress `json:"projectProgress"`
}

// dayRangeUTC 返回 [当日 00:00, 次日 00:00) 的 UTC RFC3339 文本（loc 为
// 本地时区，见时间参数约定）。
func dayRangeUTC(t time.Time, loc *time.Location) (start, end string) {
	y, m, d := t.In(loc).Date()
	s := time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
	return s.Format(timeRFC3339), s.AddDate(0, 0, 1).Format(timeRFC3339)
}

// Stats 计算看板聚合。now 供 day 边界与「近 7 天」窗口；loc 通常为
// time.Local（测试注入自定义时区以覆盖边界）。
func (db *DB) Stats(ctx context.Context, now time.Time, loc *time.Location) (DashboardStats, error) {
	st := DashboardStats{
		TodayAgenda:     []AgendaItem{},
		ProjectProgress: []ProjectProgress{},
	}
	todayStart, todayEnd := dayRangeUTC(now, loc)
	_, yEnd := dayRangeUTC(now.AddDate(0, 0, -1), loc)

	scanInt := func(sql string, args ...any) (int, error) {
		rows, err := db.conn.Query(ctx, sql, args...)
		if err != nil {
			return 0, err
		}
		if len(rows.Values) == 0 {
			return 0, nil
		}
		return int(columnInt(rows.Values[0][0])), nil
	}

	// KPI 今日待办：未完成且 due <= 今日末（含过期）。
	today, err := scanInt(
		"SELECT COUNT(*) FROM todos WHERE done = 0 AND due_date IS NOT NULL AND due_date < ?", todayEnd)
	if err != nil {
		return st, fmt.Errorf("store: 统计今日待办失败: %w", err)
	}
	st.TodayTodoCount = today

	// 昨日同口径：未完成且 due <= 昨日末（历史快照口径，PRD v2.0 §4.2）。
	yesterday, err := scanInt(
		"SELECT COUNT(*) FROM todos WHERE done = 0 AND due_date IS NOT NULL AND due_date < ?", yEnd)
	if err != nil {
		return st, fmt.Errorf("store: 统计昨日待办失败: %w", err)
	}
	st.YesterdayTodoCount = yesterday

	// 进行中项目：有未完成待办的项目数。
	active, err := scanInt(
		`SELECT COUNT(DISTINCT project_id) FROM todos
		  WHERE done = 0 AND project_id IS NOT NULL`)
	if err != nil {
		return st, fmt.Errorf("store: 统计进行中项目失败: %w", err)
	}
	st.ActiveProjects = active

	// KPI 灵感记录：总数 + 近 7 天新增。
	total, err := scanInt("SELECT COUNT(*) FROM notes")
	if err != nil {
		return st, fmt.Errorf("store: 统计记录总数失败: %w", err)
	}
	st.NoteTotal = total
	weekAgo := now.Add(-7 * 24 * time.Hour).UTC().Format(timeRFC3339)
	weekNew, err := scanInt("SELECT COUNT(*) FROM notes WHERE created_at >= ?", weekAgo)
	if err != nil {
		return st, fmt.Errorf("store: 统计近7日记录失败: %w", err)
	}
	st.NoteWeekNew = weekNew

	// 今日信息：当日 due 待办（UTC 排序）。
	todoRows, err := db.conn.Query(ctx,
		"SELECT id, title, done, due_date FROM todos WHERE due_date >= ? AND due_date < ? ORDER BY due_date ASC",
		todayStart, todayEnd)
	if err != nil {
		return st, fmt.Errorf("store: 查询今日时间轴待办失败: %w", err)
	}
	for _, r := range todoRows.Values {
		status := "scheduled"
		if columnBool(r[2]) {
			status = "done"
		}
		due := columnText(r[3])
		st.TodayAgenda = append(st.TodayAgenda, AgendaItem{
			Kind: "todo", RefID: columnInt(r[0]), Title: columnText(r[1]),
			At: &due, Status: status,
		})
	}

	// 今日信息：当日新建的记录。
	noteRows, err := db.conn.Query(ctx,
		"SELECT id, title, created_at FROM notes WHERE created_at >= ? ORDER BY created_at ASC",
		todayStart)
	if err != nil {
		return st, fmt.Errorf("store: 查询今日新建记录失败: %w", err)
	}
	for _, r := range noteRows.Values {
		at := columnText(r[2])
		st.TodayAgenda = append(st.TodayAgenda, AgendaItem{
			Kind: "note", RefID: columnInt(r[0]), Title: columnText(r[1]),
			At: &at, Status: "logged",
		})
	}

	// 时间轴按 At 升序（同为 nil 不会出现：两类都有 At）。
	sortAgenda(st.TodayAgenda)

	// 项目进度：全部项目（含 0 待办，前端按需过滤/排序）。
	ppRows, err := db.conn.Query(ctx, "SELECT "+projectColumns+projectFrom+
		" GROUP BY p.id ORDER BY p.sort_order ASC, p.id ASC")
	if err != nil {
		return st, fmt.Errorf("store: 查询项目进度失败: %w", err)
	}
	for _, r := range ppRows.Values {
		p := scanProject(r)
		st.ProjectProgress = append(st.ProjectProgress, ProjectProgress{
			ProjectID: p.ID, Name: p.Name, Color: p.Color,
			Open: p.OpenTodos, Total: p.TotalTodos,
		})
	}
	return st, nil
}

// sortAgenda 直接插入排序（条目量小，避免引入依赖）。
func sortAgenda(items []AgendaItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			a, b := items[j], items[j-1]
			if a.At != nil && (b.At == nil || *a.At < *b.At) {
				items[j], items[j-1] = items[j-1], items[j]
				continue
			}
			break
		}
	}
}
