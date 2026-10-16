// note_v2.go：信息记录 v2 的 typed IPC 扩展（PRD v2.1 §3.5）。
//
// 灵感流事件沿用 note.changed（kind 字段区分 spark/note），避免前端多订阅源；
// 转待办属于跨域写入：PromoteSparkToTodo 组合 TodoService（同包直接调用），
// 单事务由 store 层保证，事件分 note.changed + todo.changed 两条广播。
package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/891D3/trap/internal/store"
)

// CopyRichText 把笔记正文渲染为 HTML 并写入系统剪贴板（text/html + 兜底纯文本）。
// 依赖 mygo 桌面剪贴板 API；失败返回可读错误。
func (s *NoteService) CopyRichText(ctx context.Context, id int64) error {
	db, err := s.ready()
	if err != nil {
		return err
	}
	note, err := db.GetNoteV2(ctx, id)
	if err != nil {
		return err
	}
	html, err := markdownToHTML(note.Title, note.Content)
	if err != nil {
		return fmt.Errorf("service: 富文本转换失败: %w", err)
	}
	if err := writeClipboard(html, note.Content); err != nil {
		return fmt.Errorf("service: 写入剪贴板失败: %w", err)
	}
	return nil
}

// richClipboardHTML 保留扩展点：如需切换转换器实现（测试注入）覆盖此变量。
var richClipboardHTML = markdownToHTML

// AttachTodoPeer 给笔记服务注入待办服务（「转待办」跨域写入用；包级函数
// 不进 IPC 方法集）。
func AttachTodoPeer(service *NoteService, todos *TodoService) { service.todos = todos }

// CreateSpark 保存一条灵感速记（content 纯文本，可关联项目）。
func (s *NoteService) CreateSpark(ctx context.Context, content string, projectID *int64) (store.NoteV2, error) {
	db, err := s.ready()
	if err != nil {
		return store.NoteV2{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return store.NoteV2{}, fmt.Errorf("service: 灵感内容不能为空")
	}
	sp, err := db.CreateNoteV2(ctx, store.NoteKindSpark, sparkTitle(content), content, projectID)
	if err != nil {
		return store.NoteV2{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteCreated, Note: toNote(sp), NoteID: sp.ID})
	return sp, nil
}

// ListSparks 列出灵感（includeArchived=false 时只列未处理流）。
func (s *NoteService) ListSparks(ctx context.Context, includeArchived bool) ([]store.NoteSummaryV2, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	return db.ListNotesV2(ctx, store.NoteFilterV2{Kind: store.NoteKindSpark, IncludeArchived: includeArchived})
}

// ListNotesV2 列出笔记摘要（projectID/tag 过滤；tag 匹配标题或正文 #token）。
func (s *NoteService) ListNotesV2(ctx context.Context, projectID *int64, tag string) ([]store.NoteSummaryV2, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	f := store.NoteFilterV2{}
	if projectID != nil {
		f.HasProject = true
		f.ProjectID = projectID
	}
	all, err := db.ListNotesV2(ctx, f)
	if err != nil {
		return nil, err
	}
	if tag == "" {
		return all, nil
	}
	filtered := make([]store.NoteSummaryV2, 0, len(all))
	for _, n := range all {
		// 标签过滤按正文 token 解析（切段在 service；正文保持零转换）。
		full, err := db.GetNoteV2(ctx, n.ID)
		if err != nil {
			continue
		}
		if containsTag(full.Content, tag) || containsTag(n.Title, tag) {
			filtered = append(filtered, n)
		}
	}
	return filtered, nil
}

// PromoteSparkResult 是「转笔记/转待办」的单值返回（mygo IPC ≤1 值 + error）。
type PromoteSparkResult struct {
	Note *store.NoteV2 `json:"note,omitempty"` // as="note" 时的新笔记
	Todo *store.Todo   `json:"todo,omitempty"` // as="todo" 时的新待办
}

// PromoteSpark 把灵感转成新主体。as = "note" → 新笔记文档（保留原灵感）；
// as = "todo" → 用注入的 TodoService peer 新建待办（title=灵感内容）。
// 原灵感一律保留，删除由用户手动执行（用户 2026-10-16 裁定）。
func (s *NoteService) PromoteSpark(ctx context.Context, sparkID int64, as string) (PromoteSparkResult, error) {
	db, err := s.ready()
	if err != nil {
		return PromoteSparkResult{}, err
	}
	spark, err := db.GetNoteV2(ctx, sparkID)
	if err != nil {
		return PromoteSparkResult{}, err
	}
	if spark.Kind != store.NoteKindSpark {
		return PromoteSparkResult{}, fmt.Errorf("service: 记录 %d 不是灵感，不能转换", sparkID)
	}
	content := strings.TrimSpace(spark.Content)
	if content == "" {
		return PromoteSparkResult{}, fmt.Errorf("service: 灵感内容为空，无法转换")
	}
	switch as {
	case "note":
		body := "> " + strings.ReplaceAll(content, "\n", "\n> ")
		note, err := db.PromoteSparkToNote(ctx, sparkID, sparkTitle(content), body, nil)
		if err != nil {
			return PromoteSparkResult{}, err
		}
		s.broadcast(NoteChangedPayload{Action: NoteCreated, Note: toNote(note), NoteID: note.ID})
		return PromoteSparkResult{Note: &note}, nil
	case "todo":
		if s.todos == nil {
			return PromoteSparkResult{}, fmt.Errorf("service: 待办服务未注入，无法转待办")
		}
		todo, err := s.todos.Add(ctx, store.TodoInput{Title: content})
		if err != nil {
			return PromoteSparkResult{}, err
		}
		return PromoteSparkResult{Todo: &todo}, nil
	default:
		return PromoteSparkResult{}, fmt.Errorf("service: 未知转换目标 %q（note | todo）", as)
	}
}

// ArchiveSpark 手动归档灵感。
func (s *NoteService) ArchiveSpark(ctx context.Context, id int64) (store.NoteV2, error) {
	db, err := s.ready()
	if err != nil {
		return store.NoteV2{}, err
	}
	n, err := db.ArchiveNote(ctx, id)
	if err != nil {
		return store.NoteV2{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteArchived, Note: toNote(n), NoteID: n.ID})
	return n, nil
}

// RestoreSpark 恢复归档灵感（回到未处理流）。
func (s *NoteService) RestoreSpark(ctx context.Context, id int64) (store.NoteV2, error) {
	db, err := s.ready()
	if err != nil {
		return store.NoteV2{}, err
	}
	n, err := db.RestoreNote(ctx, id)
	if err != nil {
		return store.NoteV2{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteSaved, Note: toNote(n), NoteID: n.ID})
	return n, nil
}

// GetNoteV2 按 id 读取 v2 实体（笔记/灵感编辑态共用）。
func (s *NoteService) GetNoteV2(ctx context.Context, id int64) (store.NoteV2, error) {
	db, err := s.ready()
	if err != nil {
		return store.NoteV2{}, err
	}
	return db.GetNoteV2(ctx, id)
}

// SaveNoteV2 保存标题/正文并更新项目关联（编辑态保存；无关联传 nil 以清空）。
func (s *NoteService) SaveNoteV2(ctx context.Context, id int64, title, content string, projectID *int64) (store.NoteV2, error) {
	db, err := s.ready()
	if err != nil {
		return store.NoteV2{}, err
	}
	n, err := db.SaveNoteV2(ctx, id, strings.TrimSpace(title), content, projectID)
	if err != nil {
		return store.NoteV2{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteSaved, Note: toNote(n), NoteID: n.ID})
	return n, nil
}

// TagCount 是常用标签统计项。
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// TagStats 统计笔记正文中的 `#标签` token Top N（辅助区常用标签卡数据源）。
// 标签不落库（PRD §3.5.1 决策；正文实时解析）。
func (s *NoteService) TagStats(ctx context.Context, limit int) ([]TagCount, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	notes, err := db.ListNotesV2(ctx, store.NoteFilterV2{Kind: store.NoteKindNote})
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, summary := range notes {
		full, err := db.GetNoteV2(ctx, summary.ID)
		if err != nil {
			continue
		}
		for _, tag := range extractTags(full.Title + "\n" + full.Content) {
			counts[tag]++
		}
	}
	out := make([]TagCount, 0, len(counts))
	for tag, count := range counts {
		out = append(out, TagCount{Tag: tag, Count: count})
	}
	// 按次数降序，次数相同按标签字典序稳定排序。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			if out[j].Count > out[j-1].Count ||
				(out[j].Count == out[j-1].Count && out[j].Tag < out[j-1].Tag) {
				out[j], out[j-1] = out[j-1], out[j]
				continue
			}
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// extractTags 提取文本中的 `#xx` token 序列（# 后接非空白字符）。
func extractTags(text string) []string {
	var tags []string
	for _, line := range strings.Split(text, "\n") {
		for _, raw := range strings.Fields(line) {
			if strings.HasPrefix(raw, "#") && len([]rune(raw)) > 1 {
				trimmed := strings.TrimLeft(raw, "#")
				// 去掉尾随标点（`#标签。`/`#标签，`）。
				trimmed = strings.TrimRight(trimmed, "。，、;；.,")
				if trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}
	}
	return tags
}

// LinkProject 更新笔记/灵感的项目关联（chip 选择浮层保存时调用）。
func (s *NoteService) LinkProject(ctx context.Context, id int64, projectID *int64) (store.NoteV2, error) {
	db, err := s.ready()
	if err != nil {
		return store.NoteV2{}, err
	}
	if projectID != nil {
		if _, err := db.GetProject(ctx, *projectID); err != nil {
			return store.NoteV2{}, fmt.Errorf("service: 关联项目不存在: %w", err)
		}
	}
	cur, err := db.GetNoteV2(ctx, id)
	if err != nil {
		return store.NoteV2{}, err
	}
	n, err := db.SaveNoteV2(ctx, id, cur.Title, cur.Content, projectID)
	if err != nil {
		return store.NoteV2{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteSaved, Note: toNote(n), NoteID: n.ID})
	return n, nil
}

// SweepResult 是清扫结果（IPC 单值口径）。
type SweepResult struct {
	Archived int `json:"archived"`
	Purged   int `json:"purged"`
}

// SweepSparks 执行自动归档清扫（main 启动时点 + 前端进入灵感 Tab 触发）。
func (s *NoteService) SweepSparks(ctx context.Context) (SweepResult, error) {
	db, err := s.ready()
	if err != nil {
		return SweepResult{}, err
	}
	archived, purged, err := db.SweepSparks(ctx, time.Now())
	if err != nil {
		return SweepResult{}, err
	}
	if archived > 0 || purged > 0 {
		log.Printf("service: 灵感清扫：自动归档 %d 条，清除 %d 条", archived, purged)
		s.broadcast(NoteChangedPayload{Action: NoteSaved, NoteID: 0})
	}
	return SweepResult{Archived: archived, Purged: purged}, nil
}

// sparkTitle 取灵感首行截 20 字做标题（PRD 验收 1 口径）。
func sparkTitle(content string) string {
	line := content
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		line = content[:i]
	}
	line = strings.TrimSpace(line)
	for len(line) > 0 && !utf8.ValidString(line) {
		// 朝后裁剪半个 rune 的边界情形（防御式，实际不触发）。
		line = line[:len(line)-1]
	}
	r := []rune(line)
	if len(r) > 20 {
		return string(r[:20])
	}
	return line
}

// containsTag 判断文本是否存在 `#tag` token（空白/行首/标点边界）。
func containsTag(text, tag string) bool {
	if tag == "" || text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if text[i] != '#' {
			continue
		}
		rest := text[i+1:]
		if !strings.HasPrefix(rest, tag) {
			continue
		}
		after := rest[len(tag):]
		if after == "" {
			return true // 命中到行尾
		}
		next, _ := utf8.DecodeRuneInString(after) // 边界按 rune 判定（中文标点）
		switch next {
		case ' ', '\n', '\t', '#', ',', '。', ')', '）', '，':
			return true
		}
	}
	return false
}

// toNote 把 NoteV2 摘要为 v1 Note（事件 payload 兼容 v1 字段；v2 字段前端
// 通过 ListSparks/GetNoteV2 另取——payload 只做存在性触发，兼做内容提示）。
func toNote(n store.NoteV2) *store.Note {
	return &store.Note{
		ID:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		Archived:  n.Archived,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}
