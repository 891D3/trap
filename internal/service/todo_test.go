package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/891D3/trap/internal/store"
)

func newTodoService(t *testing.T) *TodoService {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "todo.db"))
	if err != nil {
		t.Fatalf("打开临时数据库: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := &TodoService{}
	AttachTodoStore(service, db)
	return service
}

func TestTodoServiceReadyGuard(t *testing.T) {
	service := &TodoService{}
	if _, err := service.List(context.Background()); err == nil {
		t.Fatal("未注入 store 时 List 应返回错误")
	}
}

func TestTodoServiceCRUDAndEvents(t *testing.T) {
	service := newTodoService(t)
	ctx := context.Background()

	if TodoChanged.Name() != "todo.changed" {
		t.Fatalf("事件名称 = %q, 期望 todo.changed", TodoChanged.Name())
	}
	if _, err := service.Add(ctx, "   "); err == nil {
		t.Fatal("空标题应返回错误")
	}

	// Add 会走真实 store 写入并触发 added 广播路径；无窗口时 Broadcast 安全返回。
	todo, err := service.Add(ctx, "  写服务测试  ")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if todo.Title != "写服务测试" || todo.Done {
		t.Errorf("Add 结果 = %+v", todo)
	}

	list, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != todo.ID {
		t.Fatalf("List = %+v", list)
	}

	done, err := service.SetDone(ctx, todo.ID, true)
	if err != nil {
		t.Fatalf("SetDone(true): %v", err)
	}
	if !done.Done || done.CompletedAt == nil {
		t.Errorf("完成结果 = %+v", done)
	}

	undone, err := service.SetDone(ctx, todo.ID, false)
	if err != nil {
		t.Fatalf("SetDone(false): %v", err)
	}
	if undone.Done || undone.CompletedAt != nil {
		t.Errorf("恢复结果 = %+v", undone)
	}

	if err := service.Delete(ctx, todo.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := service.Delete(ctx, todo.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复 Delete 错误 = %v, 期望 ErrNotFound", err)
	}
}
