// project_test.go：ProjectService CRUD、事件命名与前置注入守卫的测试（v2.0 R1）。
package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/891D3/trap/internal/store"
)

func newProjectService(t *testing.T) *ProjectService {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "project.db"))
	if err != nil {
		t.Fatalf("打开临时数据库: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := &ProjectService{}
	AttachProjectStore(svc, db)
	return svc
}

func TestProjectServiceReadyGuard(t *testing.T) {
	svc := &ProjectService{}
	if _, err := svc.List(context.Background()); err == nil {
		t.Fatal("未注入 store 时 List 应返回错误")
	}
}

func TestProjectServiceCRUD(t *testing.T) {
	svc := newProjectService(t)
	ctx := context.Background()

	if ProjectChanged.Name() != "project.changed" {
		t.Fatalf("事件名称 = %q, 期望 project.changed", ProjectChanged.Name())
	}
	if _, err := svc.Add(ctx, "   ", ""); err == nil {
		t.Fatal("空名称应返回错误")
	}

	p, err := svc.Add(ctx, "  工作台重构  ", "")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if p.Name != "工作台重构" || p.Color != store.ColorSlate {
		t.Errorf("Add 结果 = %+v", p)
	}

	projects, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != p.ID {
		t.Fatalf("List = %+v", projects)
	}

	up, err := svc.Update(ctx, p.ID, "改名", store.ColorAmber, 0)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if up.Name != "改名" || up.Color != store.ColorAmber || up.SortOrder != 0 {
		t.Errorf("Update 结果 = %+v", up)
	}

	// 备注：项目删除联动「关联待办置空 + 前端重新拉取」在 store 层与前端
	// realtime 模块验证；这里验证服务边界与 ErrNotFound 语义。
	if _, err := svc.Add(ctx, "被关联项目", store.ColorGreen); err != nil {
		t.Fatalf("Add 被关联项目: %v", err)
	}

	if err := svc.Delete(ctx, p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(ctx, p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("重复 Delete 错误 = %v, 期望 ErrNotFound", err)
	}
}
