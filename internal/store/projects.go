// projects.go：项目表的存取（唯一 SQL 所在包）。
//
// v2.0 的项目是最小实体：仅做待办关联与统计（PRD §1 范围外明确「项目工作
// 视图仍留 M1」）。PROJECT 颜色存前端色令牌名（ui-baseline v3 §三：slate 主
// 蓝灰梯度与语义色梯度），store 校验取值合法性。
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
)

// 项目颜色令牌（ui-baseline v3：主色蓝灰 + 语义色梯度中的四色）；前端按此
// 顺序循环分配新项目颜色（PRD v2.0 §10 拟定决策 6）。
const (
	ColorSlate  = "slate"
	ColorAmber  = "amber"
	ColorGreen  = "green"
	ColorPurple = "purple"
)

// colorTokens 是合法颜色令牌集合。
var colorTokens = map[string]bool{
	ColorSlate:  true,
	ColorAmber:  true,
	ColorGreen:  true,
	ColorPurple: true,
}

// validColor 校验颜色令牌取值。
func validColor(color string) error {
	if !colorTokens[color] {
		return fmt.Errorf("store: 非法项目颜色 %q（合法值: %v）", color, []string{ColorSlate, ColorAmber, ColorGreen, ColorPurple})
	}
	return nil
}

// Project 是一个项目（含列表页所需的计数）。
//
// TotalTodos/OpenTodos 仅在 List/Get 时填充（聚合列）；保证零值安全。
type Project struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	// 统计：该项目的全部待办数与未完成数。
	TotalTodos int `json:"totalTodos"`
	OpenTodos  int `json:"openTodos"`
}

// ProjectColumns 是 projects 聚合查询的固定列顺序（scanProject 依赖此顺序）。
const projectColumns = `p.id, p.name, p.color, p.sort_order, p.created_at, p.updated_at,
 COALESCE(COUNT(t.id), 0), COALESCE(SUM(CASE WHEN t.done = 0 THEN 1 ELSE 0 END), 0)`

// projectFrom 是项目聚合查询语句主体：LEFT JOIN 出计数，供 List/Get 复用。
const projectFrom = `
 FROM projects p
 LEFT JOIN todos t ON t.project_id = p.id
`

// AddProject 新增项目；color 缺省为 ColorSlate。
// 新记录 sort_order 取当前最大值 +1（末尾追加，与 quick_link 同策略）。
func (db *DB) AddProject(ctx context.Context, name, color string) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, errors.New("store: 项目名称不能为空")
	}
	if color == "" {
		color = ColorSlate
	}
	if err := validColor(color); err != nil {
		return Project{}, err
	}
	ts := nowTime().Format(timeRFC3339)
	// 单条语句 无法返回 LastInsertID 时不能 Transaction 内拼装——这里 INSERT 只有一条。
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL: `INSERT INTO projects (name, color, sort_order, created_at, updated_at)
			      VALUES (?, ?, COALESCE((SELECT MAX(sort_order) FROM projects), 0) + 1, ?, ?)`,
			Args: []any{name, color, ts, ts},
		},
	})
	if err != nil {
		return Project{}, fmt.Errorf("store: 新增项目失败: %w", err)
	}
	return db.GetProject(ctx, res[len(res)-1].LastInsertID)
}

// GetProject 按 id 查询（聚合列含计数）；不存在返回 ErrNotFound。
func (db *DB) GetProject(ctx context.Context, id int64) (Project, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+projectColumns+projectFrom+" WHERE p.id = ? GROUP BY p.id", id)
	if err != nil {
		return Project{}, fmt.Errorf("store: 查询项目 %d 失败: %w", id, err)
	}
	if len(rows.Values) == 0 {
		return Project{}, fmt.Errorf("store: 项目 %d: %w", id, ErrNotFound)
	}
	return scanProject(rows.Values[0]), nil
}

// ListProjects 返回全部项目（sort_order 升序，末尾按 id 稳定排序）。
func (db *DB) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+projectColumns+projectFrom+
			" GROUP BY p.id ORDER BY p.sort_order ASC, p.id ASC")
	if err != nil {
		return nil, fmt.Errorf("store: 列出项目失败: %w", err)
	}
	projects := make([]Project, 0, len(rows.Values))
	for _, r := range rows.Values {
		projects = append(projects, scanProject(r))
	}
	return projects, nil
}

// UpdateProject 更新项目（名称/颜色/排序全量）；不存在返回 ErrNotFound。
func (db *DB) UpdateProject(ctx context.Context, id int64, name, color string, sortOrder int) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, errors.New("store: 项目名称不能为空")
	}
	if err := validColor(color); err != nil {
		return Project{}, err
	}
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE projects SET name = ?, color = ?, sort_order = ?, updated_at = ? WHERE id = ?",
			Args: []any{name, color, sortOrder, ts, id},
		},
	})
	if err != nil {
		return Project{}, fmt.Errorf("store: 更新项目 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return Project{}, fmt.Errorf("store: 项目 %d: %w", id, ErrNotFound)
	}
	return db.GetProject(ctx, id)
}

// DeleteProject 删除项目，并把关联待办的 project_id 显式置 NULL（时间戳同步
// 更新，让前端能感知到关联变化）。
//
// 不依赖 SQLite FK 级联：mygo sqlite 不保证 foreign_keys pragma 开启，行为
// 必须由本方法显式保证（PRD v2.0 验收 4：删除项目后待办保留且关联置空）。
func (db *DB) DeleteProject(ctx context.Context, id int64) error {
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE todos SET project_id = NULL, updated_at = ? WHERE project_id = ?",
			Args: []any{ts, id},
		},
		{
			// v2：笔记/灵感的项目关联一并置空（PRD v2.1 §3.5；验收 4 口径）。
			SQL:  "UPDATE notes SET project_id = NULL, updated_at = ? WHERE project_id = ?",
			Args: []any{ts, id},
		},
		{
			SQL:  "DELETE FROM projects WHERE id = ?",
			Args: []any{id},
		},
	})
	if err != nil {
		return fmt.Errorf("store: 删除项目 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return fmt.Errorf("store: 项目 %d: %w", id, ErrNotFound)
	}
	return nil
}

// scanProject 把一行聚合结果映射为 Project（列序见 projectColumns）。
func scanProject(r []any) Project {
	return Project{
		ID:         columnInt(r[0]),
		Name:       columnText(r[1]),
		Color:      columnText(r[2]),
		SortOrder:  int(columnInt(r[3])),
		CreatedAt:  columnText(r[4]),
		UpdatedAt:  columnText(r[5]),
		TotalTodos: int(columnInt(r[6])),
		OpenTodos:  int(columnInt(r[7])),
	}
}
