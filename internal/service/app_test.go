// app_test.go：AppService 设置读写的单测。
//
// 依赖真实的 store（临时目录文件库，T2 已验证迁移幂等），验证：
//   - 数据层未注入时的显式报错；
//   - GetSettings 的键值映射（含 DataDir 透传）；
//   - UpdateProfile 的姓名校验与落库；
//   - SetTheme 的取值校验与落库；
//   - SetExportDir 落库。
//
// Broadcast 在无窗口时安全（mygo 实现为空循环），事件断言不做 UI 级验证。
package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/891D3/trap/internal/store"
)

// newAppService 构造注入了临时库的 AppService。
func newAppService(t *testing.T) *AppService {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(context.Background(), filepath.Join(dir, "trap.db"))
	if err != nil {
		t.Fatalf("打开临时库: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	svc := &AppService{dataDir: dir}
	AttachStore(svc, db, dir)
	return svc
}

func TestReadyGuard(t *testing.T) {
	svc := &AppService{}
	if _, err := svc.GetSettings(context.Background()); err == nil || !strings.Contains(err.Error(), "尚未初始化") {
		t.Errorf("未注入数据层应显式报错, 实际 %v", err)
	}
}

func TestGetSettingsMapping(t *testing.T) {
	svc := newAppService(t)
	ctx := context.Background()

	// 空库：Settings 各业务字段为空。
	got, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Name != "" || got.Theme != "" || got.ExportDir != "" {
		t.Errorf("空库默认值 = %+v", got)
	}
	if got.DataDir != svc.dataDir {
		t.Errorf("DataDir = %q, 期望 %q", got.DataDir, svc.dataDir)
	}

	// 写入后再读，逐字段校验映射。
	if _, err := svc.UpdateProfile(ctx, "小明", "/tmp/a.png"); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if err := svc.SetTheme(ctx, ThemeDark); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	if err := svc.SetExportDir(ctx, "/tmp/export"); err != nil {
		t.Fatalf("SetExportDir: %v", err)
	}
	got, err = svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Name != "小明" || got.AvatarPath != "/tmp/a.png" || got.Theme != "dark" || got.ExportDir != "/tmp/export" {
		t.Errorf("映射结果 = %+v", got)
	}
}

func TestUpdateProfileValidation(t *testing.T) {
	svc := newAppService(t)
	ctx := context.Background()
	if _, err := svc.UpdateProfile(ctx, "", ""); err == nil || !strings.Contains(err.Error(), "姓名") {
		t.Errorf("空姓名应报错, 实际 %v", err)
	}
}

func TestSetThemeValidation(t *testing.T) {
	svc := newAppService(t)
	ctx := context.Background()
	if err := svc.SetTheme(ctx, "purple"); err == nil || !strings.Contains(err.Error(), "主题") {
		t.Errorf("非法主题应报错, 实际 %v", err)
	}
}
