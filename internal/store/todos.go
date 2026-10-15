// todos.go：待办表的存取（唯一 SQL 所在包）。
//
// v2.0 扩展（PRD §3）：待办增加 category（个人/工作/学习）、due_date（UTC
// RFC3339，NULL=未安排）、project_id（关联项目）。增删改全部走 TodoInput。
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// Todo 分类取值（设计稿：个人/工作/学习；空串=未分类，兼容 v1 数据）。
const (
	CatPersonal = "personal"
	CatWork     = "work"
	CatStudy    = "study"
)

// categories 是合法分类集合。
var categories = map[string]bool{
	"":           true,
	CatPersonal:  true,
	CatWork:      true,
	CatStudy:     true,
}

// Todo 是一条待办的领域模型。时间是 UTC RFC3339 文本（AGENTS.md §4.1）。
type Todo struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Done      bool    `json:"done"`
	Category  string  `json:"category,omitempty"`
	DueDate   *string `json:"dueDate,omitempty"`  // UTC RFC3339；NULL=未安排
	ProjectID *int64  `json:"projectId,omitempty"`
	CreatedAt string  `json:"createdAt"`
	CompletedAt *string `json:"completedAt,omitempty"` // NULL = 未完成
	UpdatedAt string  `json:"updatedAt"`
}

// todoColumns 是 todos 表的固定查询列顺序，scan 与之对应。
const todoColumns = "id, title, done, category, due_date, project_id, created_at, completed_at, updated_at"

// TodoInput 是新增/更新待办的可编辑字段。
//
// dueDate 传 UTC RFC3339 文本（由前端完成「明天 15:00」等短语解析后转 UTC
// 传入，PRD v2.0 §10 拟定决策 2）；无声存 Date-only 时前端统一补当地当日
// 00:00 再转 UTC，Go 只做格式校验。
type TodoInput struct {
	Title     string  `json:"title"`
	Category  string  `json:"category,omitempty"`
	DueDate   *string `json:"dueDate,omitempty"`
	ProjectID *int64  `json:"projectId,omitempty"`
}

// TodoFilter 是 List 的过滤条件；零值 = 不过滤（等价 v1 全量列表）。
type TodoFilter struct {
	Done *bool // nil=不过滤；true=仅已完成；false=仅未完成
	// Category 精确匹配；空串 = 不过滤分类（M0 的「按属性看」只有三个
	// 具名分类，不存在「仅未分类」的过滤需求）。
	Category string
	// ProjectID 与 FilterNoProject 组合使用：
	//   - HasProjectID=false：不过滤项目；
	//   - HasProjectID=true 且 ProjectID=id：仅该项目；
	//   - 含 ProjectID=NULL 的语义用 FilterNoProject=true 表达（保留 Entry 以防歧义）。
	HasProjectID   bool
	ProjectID      *int64
	FilterNoProject bool
	// Due 从 / Due 到：due_date 范围过滤（含边界，UTC RFC3339 文本比较）。
	DueFrom, DueTo *string
	NoDue          bool // true = 仅未安排（due_date IS NULL）
}

// validateInput 校验 TodoInput 的取值合法性（title/category/dueDate）。
func validateInput(in TodoInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("store: 待办标题不能为空")
	}
	if !categories[in.Category] {
		return fmt.Errorf("store: 非法待办分类 %q（合法值: %v）", in.Category, []string{CatPersonal, CatWork, CatStudy})
	}
	if in.DueDate != nil {
		if _, err := time.Parse(time.RFC3339, *in.DueDate); err != nil {
			return fmt.Errorf("store: dueDate 不是合法的 UTC RFC3339: %w", err)
		}
	}
	return nil
}

// validateProject 校验 projectId 指向的项目存在（不依赖 FK pragma）。
func (db *DB) validateProject(ctx context.Context, projectID *int64) error {
	if projectID == nil {
		return nil
	}
	if _, err := db.GetProject(ctx, *projectID); err != nil {
		return fmt.Errorf("store: 关联项目 %d 不存在（或删除失败）: %w", *projectID, err)
	}
	return nil
}

// whereCond 由 TodoFilter 生成 WHERE 子句与参数；供 List/Count 复用。
func whereCond(f TodoFilter) (string, []any) {
	var conds []string
	var args []any
	if f.Done != nil {
		conds = append(conds, "done = ?")
		if *f.Done {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if f.Category != "" {
		conds = append(conds, "category = ?")
		args = append(args, f.Category)
	}
	if f.HasProjectID {
		conds = append(conds, "project_id = ?")
		if f.ProjectID != nil {
			args = append(args, *f.ProjectID)
		} else {
			args = append(args, nil)
		}
	}
	if f.FilterNoProject {
		conds = append(conds, "project_id IS NULL")
	}
	if f.DueFrom != nil {
		conds = append(conds, "due_date >= ?")
		args = append(args, *f.DueFrom)
	}
	if f.DueTo != nil {
		conds = append(conds, "due_date <= ?")
		args = append(args, *f.DueTo)
	}
	if f.NoDue {
		conds = append(conds, "due_date IS NULL")
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// AddTodo 新增一条待办，返回完整实体。
func (db *DB) AddTodo(ctx context.Context, in TodoInput) (Todo, error) {
	if err := validateInput(in); err != nil {
		return Todo{}, err
	}
	if err := db.validateProject(ctx, in.ProjectID); err != nil {
		return Todo{}, err
	}
	title := strings.TrimSpace(in.Title)
	ts := nowTime().Format(timeRFC3339)
	var due any
	if in.DueDate != nil {
		due = *in.DueDate
	}
	var projectID any
	if in.ProjectID != nil {
		projectID = *in.ProjectID
	}
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL: `INSERT INTO todos (title, done, category, due_date, project_id, created_at, updated_at)
			      VALUES (?, 0, ?, ?, ?, ?, ?)`,
			Args: []any{title, in.Category, due, projectID, ts, ts},
		},
	})
	if err != nil {
		return Todo{}, fmt.Errorf("store: 新增待办失败: %w", err)
	}
	return db.GetTodo(ctx, res[len(res)-1].LastInsertID)
}

// UpdateTodo 更新待办的可编辑字段（标题/分类/日期/项目）；done 走 SetTodoDone。
// 不存在返回 ErrNotFound。
func (db *DB) UpdateTodo(ctx context.Context, id int64, in TodoInput) (Todo, error) {
	if err := validateInput(in); err != nil {
		return Todo{}, err
	}
	if err := db.validateProject(ctx, in.ProjectID); err != nil {
		return Todo{}, err
	}
	title := strings.TrimSpace(in.Title)
	ts := nowTime().Format(timeRFC3339)
	var due any
	if in.DueDate != nil {
		due = *in.DueDate
	}
	var projectID any
	if in.ProjectID != nil {
		projectID = *in.ProjectID
	}
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE todos SET title = ?, category = ?, due_date = ?, project_id = ?, updated_at = ? WHERE id = ?",
			Args: []any{title, in.Category, due, projectID, ts, id},
		},
	})
	if err != nil {
		return Todo{}, fmt.Errorf("store: 更新待办 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return Todo{}, fmt.Errorf("store: 待办 %d: %w", id, ErrNotFound)
	}
	return db.GetTodo(ctx, id)
}

// GetTodo 按 id 查询单条待办；不存在返回 ErrNotFound 包装错误。
func (db *DB) GetTodo(ctx context.Context, id int64) (Todo, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+todoColumns+" FROM todos WHERE id = ?", id)
	if err != nil {
		return Todo{}, fmt.Errorf("store: 查询待办 %d 失败: %w", id, err)
	}
	if len(rows.Values) == 0 {
		return Todo{}, fmt.Errorf("store: 待办 %d: %w", id, ErrNotFound)
	}
	return scanTodo(rows.Values[0]), nil
}

// ListTodos 按过滤条件返回待办。
//
// 排序：未完成在前；同区内已安排（due_date 升序）先于未安排；同日期按 id
// 倒序（数字排序题）。分组逻辑在前端日期分组完成，这里只给一个稳定顺序。
func (db *DB) ListTodos(ctx context.Context, f TodoFilter) ([]Todo, error) {
	cond, args := whereCond(f)
	rows, err := db.conn.Query(ctx,
		"SELECT "+todoColumns+" FROM todos"+cond+
			" ORDER BY done ASC, (due_date IS NULL) ASC, due_date ASC, id DESC", args...)
	if err != nil {
		return nil, fmt.Errorf("store: 列出待办失败: %w", err)
	}
	todos := make([]Todo, 0, len(rows.Values))
	for _, r := range rows.Values {
		todos = append(todos, scanTodo(r))
	}
	return todos, nil
}

// SetTodoDone 不变（完成/取消完成，completed_at 跟随变化）。
func (db *DB) SetTodoDone(ctx context.Context, id int64, done bool) (Todo, error) {
	ts := nowTime().Format(timeRFC3339)
	var completedAt any
	if done {
		completedAt = ts
	}
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE todos SET done = ?, completed_at = ?, updated_at = ? WHERE id = ?",
			Args: []any{done, completedAt, ts, id},
		},
	})
	if err != nil {
		return Todo{}, fmt.Errorf("store: 更新待办 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return Todo{}, fmt.Errorf("store: 待办 %d: %w", id, ErrNotFound)
	}
	return db.GetTodo(ctx, id)
}

// DeleteTodo 删除待办；不存在返回 ErrNotFound 包装错误。
func (db *DB) DeleteTodo(ctx context.Context, id int64) error {
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{SQL: "DELETE FROM todos WHERE id = ?", Args: []any{id}},
	})
	if err != nil {
		return fmt.Errorf("store: 删除待办 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return fmt.Errorf("store: 待办 %d: %w", id, ErrNotFound)
	}
	return nil
}

// scanTodo 把一行查询结果映射为 Todo（列序见 todoColumns）。
func scanTodo(r []any) Todo {
	var projectID *int64
	if r[5] != nil {
		id := columnInt(r[5])
		projectID = &id
	}
	return Todo{
		ID:          columnInt(r[0]),
		Title:       columnText(r[1]),
		Done:        columnBool(r[2]),
		Category:    columnText(r[3]),
		DueDate:     columnTextOpt(r[4]),
		ProjectID:   projectID,
		CreatedAt:   columnText(r[6]),
		CompletedAt: columnTextOpt(r[7]),
		UpdatedAt:   columnText(r[8]),
	}
}
