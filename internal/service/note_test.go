package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/891D3/trap/internal/store"
)

func newNoteService(t *testing.T) (*NoteService, *store.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(root, "notes.db"))
	if err != nil {
		t.Fatalf("打开临时数据库: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := &NoteService{}
	AttachNoteStore(service, db)
	return service, db, root
}

func TestNoteServiceCRUDAndEvents(t *testing.T) {
	service, _, _ := newNoteService(t)
	ctx := context.Background()
	if NoteChanged.Name() != "note.changed" {
		t.Fatalf("事件名称 = %q, 期望 note.changed", NoteChanged.Name())
	}

	note, err := service.Create(ctx, "测试记录")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	saved, err := service.Save(ctx, note.ID, "测试记录", "| A | B |\n|---|---|\n| 1 | 2 |")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.Contains(saved.Content, "| 1 | 2 |") {
		t.Errorf("正文未保存: %q", saved.Content)
	}

	list, err := service.List(ctx, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != note.ID {
		t.Fatalf("List = %+v", list)
	}

	archived, err := service.SetArchived(ctx, note.ID, true)
	if err != nil {
		t.Fatalf("SetArchived: %v", err)
	}
	if !archived.Archived {
		t.Errorf("归档结果 = %+v", archived)
	}
	active, err := service.List(ctx, false)
	if err != nil {
		t.Fatalf("List active: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("归档后未归档列表 = %+v", active)
	}

	if err := service.Delete(ctx, note.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := service.Delete(ctx, note.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复删除错误 = %v, 期望 ErrNotFound", err)
	}
}

func TestNoteServiceExport(t *testing.T) {
	service, db, root := newNoteService(t)
	ctx := context.Background()
	exportDir := filepath.Join(root, "exports")
	if err := db.SetSettings(ctx, map[string]string{string(store.KeyAppExportDir): exportDir}); err != nil {
		t.Fatalf("写入导出设置: %v", err)
	}
	note, err := service.Create(ctx, `../表格:/记录?`)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	content := "# 标题\n\n| A | B |\n|---|---|\n| 1 | 2 |"
	if _, err := service.Save(ctx, note.ID, note.Title, content); err != nil {
		t.Fatalf("Save: %v", err)
	}

	result, err := service.Export(ctx, []int64{note.ID})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(result.Files) != 1 || strings.Contains(filepath.Base(result.Files[0]), "../") {
		t.Fatalf("导出文件 = %+v", result.Files)
	}
	written, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatalf("读取导出文件: %v", err)
	}
	if string(written) != content {
		t.Errorf("导出内容 = %q, 期望 %q", string(written), content)
	}

	if _, err := service.Export(ctx, nil); err == nil {
		t.Error("空 ids 应返回错误")
	}
}
