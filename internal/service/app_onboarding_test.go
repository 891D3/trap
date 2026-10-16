// app_onboarding_test.go —— 初始化判定/Initialize/存量兜底的测试（Onboarding 验收 2/4/5）。
package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/891D3/trap/internal/store"
)

func newAppServiceV2(t *testing.T) *AppService {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("打开临时数据库: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := &AppService{}
	AttachStore(app, db, t.TempDir())
	return app
}

func TestIsInitializedFreshDB(t *testing.T) {
	app := newAppServiceV2(t)
	got, err := app.IsInitialized(context.Background())
	if err != nil {
		t.Fatalf("IsInitialized: %v", err)
	}
	if got {
		t.Error("空库应视为未初始化")
	}
}

func TestInitializeHappyPath(t *testing.T) {
	app := newAppServiceV2(t)
	ctx := context.Background()

	out, err := app.Initialize(ctx, InitializeInput{Name: "  小明  "})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if out.Name != "小明" {
		t.Errorf("姓名未去空白: %q", out.Name)
	}
	if out.Theme != ThemeSystem {
		t.Errorf("未传主题应默认 system: %q", out.Theme)
	}
	got, err := app.IsInitialized(ctx)
	if err != nil || !got {
		t.Errorf("初始化后标记应为 true: %v %v", got, err)
	}
	// 姓名/主题持久化。
	st, err := app.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "小明" || st.Theme != ThemeSystem {
		t.Errorf("回读 = %+v", st)
	}
}

func TestInitializeValidation(t *testing.T) {
	app := newAppServiceV2(t)
	ctx := context.Background()
	if _, err := app.Initialize(ctx, InitializeInput{Name: "   "}); err == nil {
		t.Error("空用户名应被拒绝")
	}
	// 非法主题归 system 而非报错。
	out, err := app.Initialize(ctx, InitializeInput{Name: "x", Theme: "blue"})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if out.Theme != ThemeSystem {
		t.Errorf("非法主题应归 system: %q", out.Theme)
	}
}

func TestBackfillInitialized(t *testing.T) {
	app := newAppServiceV2(t)
	ctx := context.Background()
	db := app.store

	// 存量用户（有 name 无标记）→ 补 true。
	if err := db.SetSettings(ctx, map[string]string{string(store.KeyUserName): "老用户"}); err != nil {
		t.Fatal(err)
	}
	if err := BackfillInitialized(ctx, db); err != nil {
		t.Fatalf("BackfillInitialized: %v", err)
	}
	got, _ := app.IsInitialized(ctx)
	if !got {
		t.Error("存量用户应被补标记")
	}

	// 全新安装（无 name）→ 不动（保持未初始化，走初始化页）。
	if err := BackfillInitialized(ctx, db); err != nil {
		t.Fatal(err)
	}
	// 用新库再验证。
	app2 := newAppService(t)
	if err := BackfillInitialized(context.Background(), app2.store); err != nil {
		t.Fatal(err)
	}
	got2, _ := app2.IsInitialized(context.Background())
	if got2 {
		t.Error("全新安装不应被兜底标记（留初始化页判定）")
	}
}
