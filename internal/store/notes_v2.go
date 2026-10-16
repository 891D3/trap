// notes_v2.go：信息记录 v2 的存取扩展（PRD v2.1 §3.5）。
//
// v2 概念：
//   - kind：'note'（笔记文档，沿用既有五种方法）| 'spark'（灵感速记卡片）；
//   - project_id：笔记/灵感皆可关联项目（fmt 删项目由 DeleteProject 一并置空）；
//   - archived_at：灵感自动归档时间戳；sweep 负责 30 天自动归档/90 天清除。
//
// 仅本文件 + notes.go 有 SQL（AGENTS.md §4.1）。
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// Note kind 取值（设计稿 35:1/39:41：笔记文档与灵感速记双态）。
const (
	NoteKindNote  = "note"
	NoteKindSpark = "spark"
)

// 自动归档口径（用户 2026-10-16 确认第 2 条）：未处理 30 天自动归档；
// 已归档 90 天后清除（不可恢复）。触发点 = 应用启动 sweep + 进入灵感 Tab。
const (
	SparkAutoArchiveDays = 30
	SparkAutoPurgeDays   = 90
)

// v2 列顺序：notes 既有列 + v2 三列（scanNoteV2 依赖）。
const noteColumnsV2 = "id, title, content, archived, created_at, updated_at, kind, project_id, archived_at"

// NoteV2 是 v2 视角下的完整实体（兼容 Note 的字段直接由 GetNote 读取）。
//
// 使用独立结构体而非扩展 Note，避免改变既有 JSON 形状破坏 v1 契约；前端
// 需要的字段以 json tag 导出。
type NoteV2 struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Archived   bool    `json:"archived"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	Kind       string  `json:"kind"`                    // note | spark
	ProjectID  *int64  `json:"projectId,omitempty"`      // 6.验证色点/组合标题
	ArchivedAt *string `json:"archivedAt,omitempty"`     // 灵感归档时间；笔记为 NULL
}

// NoteSummaryV2 是 v2 列表摘要（最近笔记卡/灵感流共用）。
type NoteSummaryV2 struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"` // 灵感卡直接展示全文；notes 摘要视图前端忽略
	Kind       string  `json:"kind"`
	Archived   bool    `json:"archived"`
	ArchivedAt *string `json:"archivedAt,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	ProjectID  *int64  `json:"projectId,omitempty"`
}

// CreateNoteV2 新建一条 v2 记录（kind 必填校验；标题允许空）。
func (db *DB) CreateNoteV2(ctx context.Context, kind, title, content string, projectID *int64) (NoteV2, error) {
	if kind != NoteKindNote && kind != NoteKindSpark {
		return NoteV2{}, fmt.Errorf("store: 非法记录类型 %q（合法值: %v）", kind, []string{NoteKindNote, NoteKindSpark})
	}
	ts := nowTime().Format(timeRFC3339)
	var proj any
	if projectID != nil {
		proj = *projectID
	}
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "INSERT INTO notes (title, content, archived, created_at, updated_at, kind, project_id) VALUES (?, ?, 0, ?, ?, ?, ?)",
			Args: []any{title, content, ts, ts, kind, proj},
		},
	})
	if err != nil {
		return NoteV2{}, fmt.Errorf("store: 新建 v2 记录失败: %w", err)
	}
	return db.GetNoteV2(ctx, res[len(res)-1].LastInsertID)
}

// GetNoteV2 按 id 查询 v2 实体（GET 全列）。
func (db *DB) GetNoteV2(ctx context.Context, id int64) (NoteV2, error) {
	rows, err := db.conn.Query(ctx, "SELECT "+noteColumnsV2+" FROM notes WHERE id = ?", id)
	if err != nil {
		return NoteV2{}, fmt.Errorf("store: 查询 v2 记录 %d 失败: %w", id, err)
	}
	if len(rows.Values) == 0 {
		return NoteV2{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return scanNoteV2(rows.Values[0]), nil
}

// sparkFilter 是 ListNotesV2 的过滤条件；零值 = 列出全部。
type NoteFilterV2 struct {
	Kind        string // '' = 不过滤；否则精确匹配（note/spark）
	ProjectID   *int64
	HasProject  bool // true = 仅该项目
	NoProject   bool // true = 仅未关联项目
	IncludeArchived bool
}

// ListNotesV2 按 v2 过滤条件列摘要（创建时间倒序）。
func (db *DB) ListNotesV2(ctx context.Context, f NoteFilterV2) ([]NoteSummaryV2, error) {
	conds := []string{"1=1"}
	args := []any{}
	if f.Kind != "" {
		conds = append(conds, "kind = ?")
		args = append(args, f.Kind)
	}
	if f.HasProject {
		conds = append(conds, "project_id = ?")
		args = append(args, *f.ProjectID)
	}
	if f.NoProject {
		conds = append(conds, "project_id IS NULL")
	}
	if !f.IncludeArchived {
		// 笔记沿用 archived 布尔；灵感看 archived_at 是否为空。
		conds = append(conds, "(archived = 0 AND kind = 'note') OR (kind = 'spark' AND archived_at IS NULL)")
	}
	sql := "SELECT id, title, content, kind, archived, archived_at, created_at, updated_at, project_id FROM notes WHERE " +
		whereJoin(conds)
	sql += " ORDER BY created_at DESC, id DESC"
	rows, err := db.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: 列出 v2 记录失败: %w", err)
	}
	out := make([]NoteSummaryV2, 0, len(rows.Values))
	for _, r := range rows.Values {
		out = append(out, scanNoteSummaryV2(r))
	}
	return out, nil
}

// whereJoin 把条件串接为 AND 子句（本包小工具，勿当通用 SQL builder）。
func whereJoin(conds []string) string {
	joined := conds[0]
	for _, c := range conds[1:] {
		joined += " AND " + c
	}
	return joined
}

// SaveNoteV2 保存标题与正文（防抖整篇覆盖；兼容 kind 不变）。
func (db *DB) SaveNoteV2(ctx context.Context, id int64, title, content string, projectID *int64) (NoteV2, error) {
	ts := nowTime().Format(timeRFC3339)
	var proj any
	if projectID != nil {
		proj = *projectID
	}
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE notes SET title = ?, content = ?, project_id = ?, updated_at = ? WHERE id = ?",
			Args: []any{title, content, proj, ts, id},
		},
	})
	if err != nil {
		return NoteV2{}, fmt.Errorf("store: 保存 v2 记录 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return NoteV2{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return db.GetNoteV2(ctx, id)
}

// ArchiveNote 灵感归档（archived_at=now；幂等：已归档不重复写）。
func (db *DB) ArchiveNote(ctx context.Context, id int64) (NoteV2, error) {
	cur, err := db.GetNoteV2(ctx, id)
	if err != nil {
		return NoteV2{}, err
	}
	if cur.ArchivedAt != nil {
		return cur, nil // 已归档幂等返回
	}
	ts := nowTime().Format(timeRFC3339)
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{SQL: "UPDATE notes SET archived_at = ?, archived = 1 WHERE id = ?", Args: []any{ts, id}},
	})
	if err != nil {
		return NoteV2{}, fmt.Errorf("store: 归档灵感 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return NoteV2{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return db.GetNoteV2(ctx, id)
}

// RestoreNote 恢复归档灵感（archived_at 置 NULL）。
func (db *DB) RestoreNote(ctx context.Context, id int64) (NoteV2, error) {
	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{SQL: "UPDATE notes SET archived_at = NULL, archived = 0 WHERE id = ?", Args: []any{id}},
	})
	if err != nil {
		return NoteV2{}, fmt.Errorf("store: 恢复灵感 %d 失败: %w", id, err)
	}
	if res[len(res)-1].Changes == 0 {
		return NoteV2{}, fmt.Errorf("store: 记录 %d: %w", id, ErrNotFound)
	}
	return db.GetNoteV2(ctx, id)
}

// SweepSparks 归档/清除清扫（PRD §3.5.1 口径）：
//   - kind=spark 且未归档且 updated_at < now-30d → 归档；
//   - kind=spark 且 archived_at < now-90d → 删除。
//
// now 注入便于测试（时区由调用方决定边界——统一 UTC 计算）。返回影响数。
func (db *DB) SweepSparks(ctx context.Context, now time.Time) (archived, purged int, err error) {
	archiveCutoff := now.AddDate(0, 0, -SparkAutoArchiveDays).UTC().Format(timeRFC3339)
	purgeCutoff := now.AddDate(0, 0, -SparkAutoPurgeDays).UTC().Format(timeRFC3339)

	res, err := db.conn.Transaction(ctx, []sqlite.Statement{
		{
			SQL:  "UPDATE notes SET archived_at = ?, archived = 1 WHERE kind = 'spark' AND archived_at IS NULL AND updated_at <= ?",  // 含满 30 天整（验收边界）
			Args: []any{archiveCutoff, archiveCutoff},
		},
		{
			SQL:  "DELETE FROM notes WHERE kind = 'spark' AND archived_at IS NOT NULL AND archived_at < ?",
			Args: []any{purgeCutoff},
		},
	})
	if err != nil {
		return 0, 0, fmt.Errorf("store: 灵感清扫失败: %w", err)
	}
	if len(res) >= 1 {
		archived = int(res[0].Changes)
	}
	if len(res) >= 2 {
		purged = int(res[1].Changes)
	}
	return archived, purged, nil
}

// PromoteSpark 把灵感转为笔记新行：返回新笔记实体；灵感行保留（用户确认口径：
// 「转笔记后原灵感保留，删除由用户执行」）。title/content 由调用方组装。
func (db *DB) PromoteSparkToNote(ctx context.Context, sparkID int64, title, content string, projectID *int64) (NoteV2, error) {
	spark, err := db.GetNoteV2(ctx, sparkID)
	if err != nil {
		return NoteV2{}, err
	}
	if spark.Kind != NoteKindSpark {
		return NoteV2{}, fmt.Errorf("store: 记录 %d 不是灵感（kind=%s），无法转笔记", sparkID, spark.Kind)
	}
	if projectID == nil {
		projectID = spark.ProjectID // 沿用灵感的关联项目
	}
	return db.CreateNoteV2(ctx, NoteKindNote, title, content, projectID)
}

// scanNoteV2 把行映射为 NoteV2（列序见 noteColumnsV2）。
func scanNoteV2(r []any) NoteV2 {
	var projectID *int64
	if r[7] != nil {
		id := columnInt(r[7])
		projectID = &id
	}
	return NoteV2{
		ID:         columnInt(r[0]),
		Title:      columnText(r[1]),
		Content:    columnText(r[2]),
		Archived:   columnBool(r[3]),
		CreatedAt:  columnText(r[4]),
		UpdatedAt:  columnText(r[5]),
		Kind:       columnText(r[6]),
		ProjectID:  projectID,
		ArchivedAt: columnTextOpt(r[8]),
	}
}

// scanNoteSummaryV2 把行映射为 NoteSummaryV2（列序见 ListNotesV2 SQL）。
func scanNoteSummaryV2(r []any) NoteSummaryV2 {
	var projectID *int64
	if r[8] != nil {
		id := columnInt(r[8])
		projectID = &id
	}
	return NoteSummaryV2{
		ID:         columnInt(r[0]),
		Title:      columnText(r[1]),
		Content:    columnText(r[2]),
		Kind:       columnText(r[3]),
		Archived:   columnBool(r[4]),
		ArchivedAt: columnTextOpt(r[5]),
		CreatedAt:  columnText(r[6]),
		UpdatedAt:  columnText(r[7]),
		ProjectID:  projectID,
	}
}
