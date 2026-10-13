// quicklinks.go：快捷入口表的存取（唯一 SQL 所在包）。
//
// kind 的判定（URL 还是本地路径）发生在录入校验（service 层，T6），
// store 只负责校验取值合法并持久化。
package store

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/plugins/sqlite"
)

// QuickLink 的 kind 取值（PRD §3：'url' | 'path'）。
const (
	KindURL  = "url"
	KindPath = "path"
)

// QuickLink 是一条快捷入口。
type QuickLink struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Target    string `json:"target"` // URL 或本地工具路径
	Kind      string `json:"kind"`   // 'url' | 'path'
	GroupName string `json:"group,omitempty"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// quickLinkColumns 是 quick_links 的固定查询列顺序。
const quickLinkColumns = "id, name, target, kind, group_name, sort_order, created_at, updated_at"

// AddQuickLink 新增快捷入口；kind 必须是 url/path 之一。
func (db *DB) AddQuickLink(ctx context.Context, name, target, kind, groupName string) (QuickLink, error) {
	if err := validKind(kind); err != nil {
		return QuickLink{}, err
	}
	if name == "" || target == "" {
		return QuickLink{}, fmt.Errorf("store: 快捷入口的 name/target 不能为空")
	}
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			// 新记录排在末尾：sort_order 取当前最大值 +1；空表则为 1。
			SQL: `INSERT INTO quick_links (name, target, kind, group_name, sort_order, created_at, updated_at)
			      VALUES (?, ?, ?, ?, COALESCE((SELECT MAX(sort_order) FROM quick_links), 0) + 1, ?, ?)`,
			Args: []any{name, target, kind, groupName, ts, ts},
		},
	})
	if err != nil {
		return QuickLink{}, fmt.Errorf("store: 新增快捷入口失败: %w", err)
	}
	return db.GetQuickLink(ctx, res[len(res)-1].LastInsertID)
}

// GetQuickLink 按 id 查询；不存在返回 ErrNotFound。
func (db *DB) GetQuickLink(ctx context.Context, id int64) (QuickLink, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+quickLinkColumns+" FROM quick_links WHERE id = ?", id)
	if err != nil {
		return QuickLink{}, fmt.Errorf("store: 查询快捷入口 %d 失败: %w", id, err)
	}
	if len(rows.Values) == 0 {
		return QuickLink{}, fmt.Errorf("store: 快捷入口 %d: %w", id, ErrNotFound)
	}
	return scanQuickLink(rows.Values[0]), nil
}

// ListQuickLinks 返回全部快捷入口，按 sort_order、id 升序。
func (db *DB) ListQuickLinks(ctx context.Context) ([]QuickLink, error) {
	rows, err := db.conn.Query(ctx,
		"SELECT "+quickLinkColumns+" FROM quick_links ORDER BY sort_order ASC, id ASC")
	if err != nil {
		return nil, fmt.Errorf("store: 列出快捷入口失败: %w", err)
	}
	links := make([]QuickLink, 0, len(rows.Values))
	for _, r := range rows.Values {
		links = append(links, scanQuickLink(r))
	}
	return links, nil
}

// UpdateQuickLink 更新名称/目标/分组/排序；kind 允许随 target 判定更新。
func (db *DB) UpdateQuickLink(ctx context.Context, id int64, name, target, kind, groupName string, sortOrder int) (QuickLink, error) {
	if err := validKind(kind); err != nil {
		return QuickLink{}, err
	}
	if name == "" || target == "" {
		return QuickLink{}, fmt.Errorf("store: 快捷入口的 name/target 不能为空")
	}
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL: `UPDATE quick_links
			      SET name = ?, target = ?, kind = ?, group_name = ?, sort_order = ?, updated_at = ?
			      WHERE id = ?`,
			Args: []any{name, target, kind, groupName, sortOrder, ts, id},
		},
	})
	if err != nil {
		return QuickLink{}, fmt.Errorf("store: 更新快捷入口 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return QuickLink{}, fmt.Errorf("store: 快捷入口 %d: %w", id, ErrNotFound)
	}
	return db.GetQuickLink(ctx, id)
}

// DeleteQuickLink 删除快捷入口；不存在返回 ErrNotFound。
func (db *DB) DeleteQuickLink(ctx context.Context, id int64) error {
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{SQL: "DELETE FROM quick_links WHERE id = ?", Args: []any{id}},
	})
	if err != nil {
		return fmt.Errorf("store: 删除快捷入口 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return fmt.Errorf("store: 快捷入口 %d: %w", id, ErrNotFound)
	}
	return nil
}

// validKind 校验 kind 取值。
func validKind(kind string) error {
	if kind != KindURL && kind != KindPath {
		return fmt.Errorf("store: 非法的快捷入口 kind %q（%q 或 %q）", kind, KindURL, KindPath)
	}
	return nil
}

// scanQuickLink 把一行查询结果映射为 QuickLink（列序见 quickLinkColumns）。
func scanQuickLink(r []any) QuickLink {
	sortOrder := columnInt(r[5])
	return QuickLink{
		ID:        columnInt(r[0]),
		Name:      columnText(r[1]),
		Target:    columnText(r[2]),
		Kind:      columnText(r[3]),
		GroupName: columnText(r[4]),
		SortOrder: int(sortOrder),
		CreatedAt: columnText(r[6]),
		UpdatedAt: columnText(r[7]),
	}
}
