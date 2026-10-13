// notes.go：信息记录表的存取（唯一 SQL 所在包）。
//
// M0 的归档是单条简单标记（PRD §3 已确认决策 4）；正文即 GFM 文本，
// 零转换，导出逻辑在 service 层（后续任务）。
package store

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/plugins/sqlite"
)

// Note 是一条信息记录的完整实体。
type Note struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"` // GFM markdown 纯文本
	Archived  bool   `json:"archived"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// NoteSummary 是列表视图的摘要：不携带正文，避免大列表全量读 content。
type NoteSummary struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Archived  bool   `json:"archived"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// noteColumns 是 notes 的完整列顺序（scan 依赖）；summary 用其中五列。
const noteColumns = "id, title, content, archived, created_at, updated_at"

// CreateNote 新建一条空正文记录（标题允许为空，PRD §3 默认 ”）。
func (db *DB) CreateNote(ctx context.Context, title string) (Note, error) {
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "INSERT INTO notes (title, content, archived, created_at, updated_at) VALUES (?, '', 0, ?, ?)",
			Args: []any{title, ts, ts},
		},
	})
	if err != nil {
		return Note{}, fmt.Errorf("store: 新建记录失败: %w", err)
	}
	return db.GetNote(ctx, res[len(res)-1].LastInsertID)
}

// GetNote 按 id 查询完整记录；不存在返回 ErrNotFound。
func (db *DB) GetNote(ctx context.Context, id int64) (Note, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+noteColumns+" FROM notes WHERE id = ?", id)
	if err != nil {
		return Note{}, fmt.Errorf("store: 查询记录 %d 失败: %w", id, err)
	}
	if len(rows.Values) == 0 {
		return Note{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return scanNote(rows.Values[0]), nil
}

// ListNotes 返回记录摘要。
//
// includeArchived=false 时只列未归档记录；列表按创建时间倒序（最新在前）。
func (db *DB) ListNotes(ctx context.Context, includeArchived bool) ([]NoteSummary, error) {
	sql := "SELECT id, title, archived, created_at, updated_at FROM notes"
	if !includeArchived {
		sql += " WHERE archived = 0"
	}
	sql += " ORDER BY created_at DESC, id DESC"
	rows, err := db.conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("store: 列出记录失败: %w", err)
	}
	notes := make([]NoteSummary, 0, len(rows.Values))
	for _, r := range rows.Values {
		notes = append(notes, NoteSummary{
			ID:        columnInt(r[0]),
			Title:     columnText(r[1]),
			Archived:  columnBool(r[2]),
			CreatedAt: columnText(r[3]),
			UpdatedAt: columnText(r[4]),
		})
	}
	return notes, nil
}

// SaveNote 保存标题与正文（防抖整篇覆盖，见 F3）；返回更新后的实体。
func (db *DB) SaveNote(ctx context.Context, id int64, title, content string) (Note, error) {
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE notes SET title = ?, content = ?, updated_at = ? WHERE id = ?",
			Args: []any{title, content, ts, id},
		},
	})
	if err != nil {
		return Note{}, fmt.Errorf("store: 保存记录 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return Note{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return db.GetNote(ctx, id)
}

// SetNoteArchived 设置归档标记；返回更新后的完整实体。
func (db *DB) SetNoteArchived(ctx context.Context, id int64, archived bool) (Note, error) {
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE notes SET archived = ?, updated_at = ? WHERE id = ?",
			Args: []any{archived, ts, id},
		},
	})
	if err != nil {
		return Note{}, fmt.Errorf("store: 归档记录 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return Note{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return db.GetNote(ctx, id)
}

// DeleteNote 删除记录；不存在返回 ErrNotFound。
func (db *DB) DeleteNote(ctx context.Context, id int64) error {
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{SQL: "DELETE FROM notes WHERE id = ?", Args: []any{id}},
	})
	if err != nil {
		return fmt.Errorf("store: 删除记录 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return nil
}

// scanNote 把一行完整查询结果映射为 Note（列序见 noteColumns）。
func scanNote(r []any) Note {
	return Note{
		ID:        columnInt(r[0]),
		Title:     columnText(r[1]),
		Content:   columnText(r[2]),
		Archived:  columnBool(r[3]),
		CreatedAt: columnText(r[4]),
		UpdatedAt: columnText(r[5]),
	}
}
