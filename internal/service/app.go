// Package service 定义经 mygo typed IPC 暴露给前端的服务。
//
// 约定（PRD §2/§5）：前端不直接访问 SQLite，所有数据操作经这些服务的
// 方法完成；mygo generate 据此生成 TS 客户端，两端类型同步演进。
// 本文件 T2 只包含应用级设置能力；Todo/Note/QuickLink 服务随 T4–T6 增加。
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

// Settings 是设置页可见的应用级设置（map 到 settings 表的业务键）。
type Settings struct {
	Name       string `json:"name,omitempty"`       // user.name
	AvatarPath string `json:"avatarPath,omitempty"` // user.avatar_path
	Theme      string `json:"theme,omitempty"`      // app.theme：system/light/dark
	ExportDir  string `json:"exportDir,omitempty"`  // app.export_dir
	DataDir    string `json:"dataDir,omitempty"`    // 数据目录（只读展示）
}

// SettingsChanged 是设置变化事件（PRD §2：事件命名 <域>.<动作>）。
// Go 侧在每次设置成功写入后 Broadcast；前端据此刷新侧栏姓名等展示。
var SettingsChanged = mygo.NewEvent[Settings]("settings.changed")

// 主题的合法取值（M0 默认 system）。
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// appVersion 应用版本号：与 mygo.config.ts 的 version 保持一致
//（发版时两处同步改；前端侧栏底部与设置页展示）。
const appVersion = "0.3.1"

// AppService 提供应用级基础能力（设置读写、连通性自检）。
//
// 生命周期：main 在 Bind 时注册零值；数据库在 App.WhenReady 中打开后
// 经包级 AttachStore 注入。注入函数不是 AppService 的方法，因此不会被
// mygo 暴露给前端；服务方法仍显式判空，避免对空指针的隐式依赖。
type AppService struct {
	store   *store.DB
	dataDir string // 数据目录（userData），供 GetSettings 只读展示
}

// Ping 连通性自检：验证 mygo typed IPC 端到端链路。
func (s *AppService) Ping() string { return "pong" }

// Version 返回应用版本号（前端侧栏底部展示）。
func (s *AppService) Version() string { return appVersion }

// IsInitialized 读取首次安装初始化标记（Onboarding D2；读不到 = 未初始化）。
func (s *AppService) IsInitialized(ctx context.Context) (bool, error) {
	st, err := s.ready()
	if err != nil {
		return false, err
	}
	ok, e := st.IsInitialized(ctx)
	return ok, e
}

// BackfillInitialized 给存量安装静默补 initialized 标记（Onboarding 验收 5）：
// settings 已有 user.name（老用户）但缺标记 → 补 true；全新安装（无 user.name）
// 不动（留给初始化页判定）。
func BackfillInitialized(ctx context.Context, st *store.DB) error {
	kv, err := st.GetSettings(ctx)
	if err != nil {
		return err
	}
	if kv[string(store.KeyAppInitialized)] == "true" || kv[string(store.KeyUserName)] == "" {
		return nil
	}
	return st.SetSettings(ctx, map[string]string{string(store.KeyAppInitialized): "true"})
}

// OpenDataDir 用系统文件管理器打开数据目录（初始化页与设置页共用）。
// 平台白名单命令（同 QuickLinkService.Open 的安全边界）：单参数、无 shell。
func (s *AppService) OpenDataDir(ctx context.Context) error {
	if s.dataDir == "" {
		return fmt.Errorf("service: 数据目录尚未就绪")
	}
	var cmdName string
	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
	case "windows":
		cmdName = "explorer"
	case "linux":
		cmdName = "xdg-open"
	default:
		return fmt.Errorf("service: 平台 %q 暂不支持打开目录", runtime.GOOS)
	}
	cmd := exec.Command(cmdName, s.dataDir) // #nosec G204：固定目录单参数
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("service: 打开数据目录失败: %w", err)
	}
	return nil
}

// PickDirectory 弹出原生目录选择器（Onboarding D3/设置页导出目录共用）；
// 用户取消返回空串。Parent 绑主窗口（macOS sheet）。
func (s *AppService) PickDirectory(ctx context.Context) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             "选择目录",
		Directory:         true,
		CreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("service: 打开目录选择器失败: %w", err)
	}
	if len(paths) == 0 {
		return "", nil
	}
	return paths[0], nil
}

// InitializeInput 是初始化页提交的字段（用户名必填；其余可选有默认）。
type InitializeInput struct {
	Name      string `json:"name"`
	ExportDir string `json:"exportDir,omitempty"`
	Theme     string `json:"theme,omitempty"`
}

// Initialize 完成首次初始化：单事务原子写入 用户名/导出目录/主题/initialized
// 四键，并广播 settings.changed（Onboarding 验收 4）。
// 重复初始化幂等（再次写入覆盖同值）。
func (s *AppService) Initialize(ctx context.Context, in InitializeInput) (Settings, error) {
	st, err := s.ready()
	if err != nil {
		return Settings{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Settings{}, fmt.Errorf("service: 用户名不能为空")
	}
	theme := in.Theme
	if theme != ThemeLight && theme != ThemeDark {
		theme = ThemeSystem // 非法值归 system
	}
	kv := map[string]string{
		string(store.KeyUserName):        name,
		string(store.KeyAppTheme):        theme,
		string(store.KeyAppInitialized):  "true",
	}
	if dir := strings.TrimSpace(in.ExportDir); dir != "" {
		kv[string(store.KeyAppExportDir)] = dir
	}
	if err := st.SetSettings(ctx, kv); err != nil {
		return Settings{}, err
	}
	out := Settings{Name: name, Theme: theme, ExportDir: kv[string(store.KeyAppExportDir)], DataDir: s.dataDir}
	if err := SettingsChanged.Broadcast(out); err != nil {
		log.Printf("service: 广播 settings.changed 失败（数据已落库）: %v", err)
	}
	return out, nil
}

// AttachStore 注入数据层；由 main 在 WhenReady 打开数据库后调用。
// 这是包级函数而非 AppService 方法，避免被 mygo 自动绑定为 IPC 接口。
func AttachStore(s *AppService, st *store.DB, dataDir string) {
	s.setStore(st, dataDir)
}

// setStore 是真正的内部注入方法，保持非导出以避免进入 typed IPC 方法集。
func (s *AppService) setStore(st *store.DB, dataDir string) {
	s.store = st
	s.dataDir = dataDir
}

// ready 校验数据层已注入。
func (s *AppService) ready() (*store.DB, error) {
	if s.store == nil {
		return nil, errors.New("service: 数据层尚未初始化（应用尚未完成启动）")
	}
	return s.store, nil
}

// GetSettings 读取全部设置并映射为 Settings。
func (s *AppService) GetSettings(ctx context.Context) (Settings, error) {
	st, err := s.ready()
	if err != nil {
		return Settings{}, err
	}
	kv, err := st.GetSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	return settingsFrom(kv, s.dataDir), nil
}

// UpdateProfile 更新姓名与头像路径（头像文件复制等归后续任务，T2 只存路径）。
func (s *AppService) UpdateProfile(ctx context.Context, name, avatarPath string) (Settings, error) {
	st, err := s.ready()
	if err != nil {
		return Settings{}, err
	}
	if name == "" {
		return Settings{}, errors.New("姓名不能为空")
	}
	err = st.SetSettings(ctx, map[string]string{
		string(store.KeyUserName):   name,
		string(store.KeyUserAvatar): avatarPath,
	})
	if err != nil {
		return Settings{}, err
	}
	out, err := s.GetSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	// 事件广播失败不阻断业务（数据已落库），仅显式忽略（AGENTS.md §4.1 降级策略）。
	_ = SettingsChanged.Broadcast(out)
	return out, nil
}

// SetTheme 设置主题；主题必须是 system/light/dark 之一。
func (s *AppService) SetTheme(ctx context.Context, theme string) error {
	switch theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		return fmt.Errorf("不支持的主题 %q（应为 system/light/dark）", theme)
	}
	st, err := s.ready()
	if err != nil {
		return err
	}
	if err := st.SetSettings(ctx, map[string]string{string(store.KeyAppTheme): theme}); err != nil {
		return err
	}
	if out, err := s.GetSettings(ctx); err == nil {
		_ = SettingsChanged.Broadcast(out)
	}
	return nil
}

// SetExportDir 设置信息记录导出目录（T5 的导出功能使用该目录）。
func (s *AppService) SetExportDir(ctx context.Context, dir string) error {
	st, err := s.ready()
	if err != nil {
		return err
	}
	if err := st.SetSettings(ctx, map[string]string{string(store.KeyAppExportDir): dir}); err != nil {
		return err
	}
	if out, err := s.GetSettings(ctx); err == nil {
		_ = SettingsChanged.Broadcast(out)
	}
	return nil
}

// settingsFrom 把 settings 表的键值映射为 Settings。
func settingsFrom(kv map[string]string, dataDir string) Settings {
	return Settings{
		Name:       kv[string(store.KeyUserName)],
		AvatarPath: kv[string(store.KeyUserAvatar)],
		Theme:      kv[string(store.KeyAppTheme)],
		ExportDir:  kv[string(store.KeyAppExportDir)],
		DataDir:    dataDir,
	}
}
