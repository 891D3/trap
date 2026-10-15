// quicklink.go 定义快捷入口 typed IPC 服务与变更事件（v2.0 R6，吸收原 T6）。
//
// 安全边界（PRD §4.4 v1.1 继承）：仅打开用户显式录入的 target，禁止任何动
// 态拼接命令；Go 侧打开实现固定为平台标准白名单命令（open/xdg-open/explorer），
// target 只作为单个参数传入（exec.Command 不经 shell，无注入面）。
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

// QuickLinkChangeAction 表示快捷入口变更类型。
type QuickLinkChangeAction string

const (
	QuickLinkAdded   QuickLinkChangeAction = "added"
	QuickLinkUpdated QuickLinkChangeAction = "updated"
	QuickLinkDeleted QuickLinkChangeAction = "deleted"
)

// QuickLinkChangedPayload 是 quick_link.changed 事件载荷。
type QuickLinkChangedPayload struct {
	Action      QuickLinkChangeAction `json:"action"`
	QuickLink   *store.QuickLink      `json:"quickLink,omitempty"`
	QuickLinkID int64                 `json:"quickLinkId,omitempty"`
}

// QuickLinkChanged 在快捷入口成功写入后广播到所有 WebView。
var QuickLinkChanged = mygo.NewEvent[QuickLinkChangedPayload]("quick_link.changed")

// QuickLinkService 暴露快捷入口 CRUD 与打开 typed IPC。
type QuickLinkService struct {
	store *store.DB
}

// AttachQuickLinkStore 注入数据层；由 main 在 App.WhenReady 中调用。
// 包级函数，不会被 mygo 绑定为 IPC 方法。
func AttachQuickLinkStore(service *QuickLinkService, db *store.DB) {
	service.setStore(db)
}

func (s *QuickLinkService) setStore(db *store.DB) { s.store = db }

func (s *QuickLinkService) ready() (*store.DB, error) {
	if s.store == nil {
		return nil, errors.New("service: 快捷入口数据层尚未初始化（应用尚未完成启动）")
	}
	return s.store, nil
}

// classifyTarget 判定 target 类型（录入时判定，PRD §4.4）：
//   - http/https 前缀 → url；其余 → path（本地工具路径）。
//   - 非法（仅空白）返回错误。
func classifyTarget(target string) (string, string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", "", errors.New("service: 快捷入口目标不能为空")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "http://") || strings.HasPrefix(strings.ToLower(trimmed), "https://") {
		return trimmed, store.KindURL, nil
	}
	return trimmed, store.KindPath, nil
}

// Add 新增快捷入口（kind 由 target 形态判定）。
func (s *QuickLinkService) Add(ctx context.Context, name, target, groupName string) (store.QuickLink, error) {
	db, err := s.ready()
	if err != nil {
		return store.QuickLink{}, err
	}
	normalized, kind, err := classifyTarget(target)
	if err != nil {
		return store.QuickLink{}, err
	}
	link, err := db.AddQuickLink(ctx, strings.TrimSpace(name), normalized, kind, groupName)
	if err != nil {
		return store.QuickLink{}, err
	}
	s.broadcast(QuickLinkChangedPayload{Action: QuickLinkAdded, QuickLink: &link, QuickLinkID: link.ID})
	return link, nil
}

// List 列出全部快捷入口（sort_order 升序）。
func (s *QuickLinkService) List(ctx context.Context) ([]store.QuickLink, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	return db.ListQuickLinks(ctx)
}

// Update 全量更新（name/target/kind 重判定/group/sortOrder）。
func (s *QuickLinkService) Update(ctx context.Context, id int64, name, target, groupName string, sortOrder int) (store.QuickLink, error) {
	db, err := s.ready()
	if err != nil {
		return store.QuickLink{}, err
	}
	normalized, kind, err := classifyTarget(target)
	if err != nil {
		return store.QuickLink{}, err
	}
	link, err := db.UpdateQuickLink(ctx, id, strings.TrimSpace(name), normalized, kind, groupName, sortOrder)
	if err != nil {
		return store.QuickLink{}, err
	}
	s.broadcast(QuickLinkChangedPayload{Action: QuickLinkUpdated, QuickLink: &link, QuickLinkID: link.ID})
	return link, nil
}

// Delete 删除并广播（前端二次确认由 UI 承担，服务层不做确认）。
func (s *QuickLinkService) Delete(ctx context.Context, id int64) error {
	db, err := s.ready()
	if err != nil {
		return err
	}
	if err := db.DeleteQuickLink(ctx, id); err != nil {
		return err
	}
	s.broadcast(QuickLinkChangedPayload{Action: QuickLinkDeleted, QuickLinkID: id})
	return nil
}

// openRunner 是打开外部目标的平台实现（白名单命令 + 单参数；见文件头安全边界）。
var openRunner = func(target string) error {
	var cmdName string
	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
	case "windows":
		cmdName = "explorer"
	case "linux":
		cmdName = "xdg-open"
	default:
		return fmt.Errorf("service: 平台 %q 暂不支持打开外部目标", runtime.GOOS)
	}
	cmd := exec.Command(cmdName, target) // #nosec G204：目标来自用户录入并以单参数传递，无 shell 参与
	return cmd.Run()
}

// Open 用系统默认方式打开目标；store 只有 kind 校验，真正打开在这里执行。
func (s *QuickLinkService) Open(ctx context.Context, id int64) error {
	db, err := s.ready()
	if err != nil {
		return err
	}
	link, err := db.GetQuickLink(ctx, id)
	if err != nil {
		return err
	}
	if err := openRunner(link.Target); err != nil {
		return fmt.Errorf("service: 打开 %q 失败（目标 %s）: %w", link.Name, link.Target, err)
	}
	return nil
}

// broadcast 记录广播失败但不回滚已落库的数据；前端可通过 List 重新同步。
func (s *QuickLinkService) broadcast(payload QuickLinkChangedPayload) {
	if err := QuickLinkChanged.Broadcast(payload); err != nil {
		log.Printf("service: 广播 quick_link.changed 失败（数据已落库）: %v", err)
	}
}
