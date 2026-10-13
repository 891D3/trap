// todos.go：待办表的存取（唯一 SQL 所在包）。
//
// M0 的待办只有 title + done 最小属性集（PRD §3）；优先级/截止日等留 M1。
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/plugins/sqlite"
)

// ErrNotFound 是「记录不存在」的统一错误；可用 errors.Is 判断。
var ErrNotFound = errors.New("store: 记录不存在")

// Todo 是一条待办的领域模型。时间是 UTC RFC3339 文本（AGENTS.md §4.1）。
type Todo struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Done        bool    `json:"done"`
	CreatedAt   string  `json:"createdAt"`
	CompletedAt *string `json:"completedAt,omitempty"` // NULL = 未完成
	UpdatedAt   string  `json:"updatedAt"`
}

// todoColumns 是 todos 表的固定查询列顺序，scan 与之对应。
const todoColumns = "id, title, done, created_at, completed_at, updated_at"

// AddTodo 新增一条待办，返回完整实体。
func (db *DB) AddTodo(ctx context.Context, title string) (Todo, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Todo{}, errors.New("store: 待办标题不能为空")
	}
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "INSERT INTO todos (title, done, created_at, updated_at) VALUES (?, 0, ?, ?)",
			Args: []any{title, ts, ts},
		},
	})
	if err != nil {
		return Todo{}, fmt.Errorf("store: 新增待办失败: %w", err)
	}
	return db.GetTodo(ctx, res[len(res)-1].LastInsertID)
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

// ListTodos 返回全部待办，按创建顺序（id 升序）排列。
func (db *DB) ListTodos(ctx context.Context) ([]Todo, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+todoColumns+" FROM todos ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("store: 列出待办失败: %w", err)
	}
	todos := make([]Todo, 0, len(rows.Values))
	for _, r := range rows.Values {
		todos = append(todos, scanTodo(r))
	}
	return todos, nil
}

// SetTodoDone 勾选/取消勾选待办。
//
// 勾选时写 completed_at（UTC RFC3339），取消时置 NULL（PRD §3 语义）。
func (db *DB) SetTodoDone(ctx context.Context, id int64, done bool) (Todo, error) {
	ts := nowTime().Format(timeRFC3339)
	// done INTEGER 0/1；completed_at 完成时写入、取消时置 NULL。
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
	return Todo{
		ID:          columnInt(r[0]),
		Title:       columnText(r[1]),
		Done:        columnBool(r[2]),
		CreatedAt:   columnText(r[3]),
		CompletedAt: columnTextOpt(r[4]),
		UpdatedAt:   columnText(r[5]),
	}
}
