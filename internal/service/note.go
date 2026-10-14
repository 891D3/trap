// note.go 定义信息记录 typed IPC 服务、导出能力与变更事件。
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

// NoteChangeAction 表示信息记录变更类型。
type NoteChangeAction string

const (
	NoteCreated  NoteChangeAction = "created"
	NoteSaved    NoteChangeAction = "saved"
	NoteArchived NoteChangeAction = "archived"
	NoteDeleted  NoteChangeAction = "deleted"
)

// NoteChangedPayload 是 note.changed 事件载荷。
type NoteChangedPayload struct {
	Action NoteChangeAction `json:"action"`
	Note   *store.Note      `json:"note,omitempty"`
	NoteID int64            `json:"noteId,omitempty"`
}

// NoteChanged 在记录成功写入、归档或删除后广播。
var NoteChanged = mygo.NewEvent[NoteChangedPayload]("note.changed")

// ExportResult 描述一次或多次 Markdown 导出的结果。
type ExportResult struct {
	Files []string `json:"files"`
}

// NoteService 暴露信息记录 CRUD 与 Markdown 导出 typed IPC。
type NoteService struct {
	store *store.DB
}

// AttachNoteStore 注入与 AppService/TodoService 相同的数据库；包级函数不会进入 IPC。
func AttachNoteStore(service *NoteService, db *store.DB) {
	service.setStore(db)
}

func (s *NoteService) setStore(db *store.DB) {
	s.store = db
}

func (s *NoteService) ready() (*store.DB, error) {
	if s.store == nil {
		return nil, errors.New("service: 信息记录数据层尚未初始化（应用尚未完成启动）")
	}
	return s.store, nil
}

// Create 创建一条空记录并广播 created。
func (s *NoteService) Create(ctx context.Context, title string) (store.Note, error) {
	db, err := s.ready()
	if err != nil {
		return store.Note{}, err
	}
	note, err := db.CreateNote(ctx, title)
	if err != nil {
		return store.Note{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteCreated, Note: &note, NoteID: note.ID})
	return note, nil
}

// List 返回记录摘要；includeArchived 控制是否包含已归档记录。
func (s *NoteService) List(ctx context.Context, includeArchived bool) ([]store.NoteSummary, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	return db.ListNotes(ctx, includeArchived)
}

// Get 读取一条记录的完整标题与 Markdown 正文。
func (s *NoteService) Get(ctx context.Context, id int64) (store.Note, error) {
	db, err := s.ready()
	if err != nil {
		return store.Note{}, err
	}
	return db.GetNote(ctx, id)
}

// Save 覆盖标题与 Markdown 正文并广播 saved。
func (s *NoteService) Save(ctx context.Context, id int64, title, content string) (store.Note, error) {
	db, err := s.ready()
	if err != nil {
		return store.Note{}, err
	}
	note, err := db.SaveNote(ctx, id, title, content)
	if err != nil {
		return store.Note{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteSaved, Note: &note, NoteID: note.ID})
	return note, nil
}

// SetArchived 设置归档状态并广播 archived。
func (s *NoteService) SetArchived(ctx context.Context, id int64, archived bool) (store.Note, error) {
	db, err := s.ready()
	if err != nil {
		return store.Note{}, err
	}
	note, err := db.SetNoteArchived(ctx, id, archived)
	if err != nil {
		return store.Note{}, err
	}
	s.broadcast(NoteChangedPayload{Action: NoteArchived, Note: &note, NoteID: note.ID})
	return note, nil
}

// Delete 删除记录并广播 deleted。
func (s *NoteService) Delete(ctx context.Context, id int64) error {
	db, err := s.ready()
	if err != nil {
		return err
	}
	if err := db.DeleteNote(ctx, id); err != nil {
		return err
	}
	s.broadcast(NoteChangedPayload{Action: NoteDeleted, NoteID: id})
	return nil
}

// Export 将指定记录以原文 Markdown 写入 settings.app.export_dir。
// 目录不存在时创建；文件名经过清理，禁止标题构造路径穿越。
func (s *NoteService) Export(ctx context.Context, ids []int64) (ExportResult, error) {
	db, err := s.ready()
	if err != nil {
		return ExportResult{}, err
	}
	if len(ids) == 0 {
		return ExportResult{}, errors.New("至少选择一条信息记录后再导出")
	}
	settings, err := db.GetSettings(ctx)
	if err != nil {
		return ExportResult{}, fmt.Errorf("读取导出目录失败: %w", err)
	}
	exportDir := strings.TrimSpace(settings[string(store.KeyAppExportDir)])
	if exportDir == "" {
		return ExportResult{}, errors.New("请先在设置中填写导出目录")
	}
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return ExportResult{}, fmt.Errorf("创建导出目录失败: %w", err)
	}
	info, err := os.Stat(exportDir)
	if err != nil || !info.IsDir() {
		return ExportResult{}, fmt.Errorf("导出目录不是有效目录: %s", exportDir)
	}

	result := ExportResult{Files: make([]string, 0, len(ids))}
	for _, id := range ids {
		note, err := db.GetNote(ctx, id)
		if err != nil {
			return ExportResult{}, fmt.Errorf("读取记录 %d 失败: %w", id, err)
		}
		path := filepath.Join(exportDir, safeNoteFilename(note.Title, note.ID))
		if err := os.WriteFile(path, []byte(note.Content), 0o644); err != nil {
			return ExportResult{}, fmt.Errorf("写入记录 %d 导出文件失败: %w", id, err)
		}
		result.Files = append(result.Files, path)
	}
	return result, nil
}

// safeNoteFilename 清理标题中的路径分隔符、控制字符和常见非法文件名字符。
func safeNoteFilename(title string, id int64) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "未命名记录"
	}
	var b strings.Builder
	for _, r := range title {
		if unicode.IsControl(r) || strings.ContainsRune(`/\\:*?"<>|`, r) {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.Trim(strings.TrimSpace(b.String()), ".")
	if cleaned == "" {
		cleaned = "未命名记录"
	}
	return fmt.Sprintf("%s-%d.md", cleaned, id)
}

func (s *NoteService) broadcast(payload NoteChangedPayload) {
	if err := NoteChanged.Broadcast(payload); err != nil {
		log.Printf("service: 广播 note.changed 失败（数据已落库）: %v", err)
	}
}
