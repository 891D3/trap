//go:build e2e

// onboarding_e2e.go —— 首次安装初始化 E2E（Onboarding 验收 2/4/7）。
//
// 与 m0_e2e.go 同一包，复用其 helper（waitFor/loadPageDocument/uniquePageURL/
// waitHTTP/setInputAndBlur 等）。流程：未初始化库 → 打开显示初始化页 →
// 填用户名 → 提交 → 进入主界面（数据看板）且姓名生效 → 重载后不再显示初始化页。
//
// 独立入口 `go run -tags e2e ./e2e -onboarding`（见 run-m0.sh 第二段）。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/891D3/trap/internal/service"
	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

// runOnboardingE2E 是初始化流程的独立入口。
func runOnboardingE2E() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("当前真实 WebView E2E 使用 macOS WKWebView")
	}
	url := os.Getenv("TRAP_E2E_URL")
	if url == "" {
		url = defaultE2EURL
	}
	if err := waitHTTP(url, 20*time.Second); err != nil {
		return fmt.Errorf("Vite 开发服务器未就绪: %w", err)
	}
	url = fmt.Sprintf("%s?e2e=%d", url, time.Now().UnixNano())

	dataDir, err := os.MkdirTemp("", "trap-onboarding-data-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)

	// 未初始化库（不预置 initialized/user.name）。
	db, err := store.Open(context.Background(), filepath.Join(dataDir, "trap.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	appService := &service.AppService{}
	todoService := &service.TodoService{}
	noteService := &service.NoteService{}
	projectService := &service.ProjectService{}
	quickLinkService := &service.QuickLinkService{}
	service.AttachStore(appService, db, dataDir)
	service.AttachTodoStore(todoService, db)
	service.AttachNoteStore(noteService, db)
	service.AttachTodoPeer(noteService, todoService)
	service.AttachProjectStore(projectService, db)
	service.AttachQuickLinkStore(quickLinkService, db)
	mygo.Bind(appService, todoService, noteService, projectService, quickLinkService)

	flowDone := make(chan error, 1)
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{
			Title: "Trap Onboarding E2E", Width: 1280, Height: 800, URL: url,
		})
		w.Page().OnDOMReady(func() {
			go func() {
				// 等 React 挂载（首个 DOM ready 后 vite 可能再 reload 一次）。
				time.Sleep(2 * time.Second)
				flowDone <- runOnboardingFlow(w.Page(), url, appService)
				mygo.App.Quit()
			}()
		})
	})
	timeout := time.AfterFunc(90*time.Second, func() {
		select {
		case flowDone <- fmt.Errorf("Onboarding E2E 超时"):
		default:
		}
		mygo.App.Quit()
	})
	defer timeout.Stop()

	if err := mygo.App.Run(); err != nil {
		return err
	}
	return <-flowDone
}

func runOnboardingFlow(page *mygo.Page, url string, app *service.AppService) error {
	// 1) 打开 → 显示初始化页（拦截主视图）。
	if err := loadPageDocument(page, uniquePageURL(url)); err != nil {
		return fmt.Errorf("加载首页: %w", err)
	}
	// Vite 首次访问会触发依赖预构建 reload：等页面稳定（onboarding 入口无预热）。
	if err := waitPageStable(page); err != nil {
		return fmt.Errorf("等待页面稳定: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.onboarding-card') !== null === true`, 15*time.Second); err != nil {
		dbg, _ := mygo.EvalAs[string](page, `JSON.stringify({body: document.body?.innerText?.slice(0,200), root: document.querySelector('#root')?.children?.length, cards: document.querySelectorAll('.onboarding-card').length, loading: document.querySelector('.app-loading') !== null})`)
		return fmt.Errorf("初始化页未出现: %w; 状态=%s", err, dbg)
	}
	// 服务端确认未初始化。
	if ok, err := app.IsInitialized(context.Background()); err != nil || ok {
		return fmt.Errorf("未初始化库应判定 false: ok=%v err=%v", ok, err)
	}

	// 2) 空用户名：提交按钮应处于禁用态（前端校验，无需点按）。
	if err := setInputAndBlur(page, `#ob-name`, ""); err != nil {
		return fmt.Errorf("填用户名失败: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.onboarding-submit')?.disabled === true`, 5*time.Second); err != nil {
		return fmt.Errorf("空用户名时提交按钮应禁用: %w", err)
	}

	// 3) 填用户名并提交。
	if err := setInputAndBlur(page, `#ob-name`, "E2E 新用户"); err != nil {
		return err
	}
	if _, err := page.Eval(`(() => {
		const b = document.querySelector('.onboarding-submit');
		b.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("提交初始化: %w", err)
	}

	// 4) 进入主界面（数据看板），姓名生效。
	if err := waitFor(page, `document.querySelector('.topbar')?.textContent?.includes('数据看板') === true`, 10*time.Second); err != nil {
		return fmt.Errorf("完成后未进入数据看板: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.side-user-name')?.textContent === 'E2E 新用户'`, 10*time.Second); err != nil {
		return fmt.Errorf("姓名未生效: %w", err)
	}
	// 服务端确认已初始化。
	if ok, err := app.IsInitialized(context.Background()); err != nil || !ok {
		return fmt.Errorf("初始化后服务端应判定 true: ok=%v err=%v", ok, err)
	}

	// 5) 重载 → 不再显示初始化页。
	if err := loadPageDocument(page, uniquePageURL(url)); err != nil {
		return fmt.Errorf("重载: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.onboarding-card') === null && document.querySelector('.topbar') !== null === true`, 15*time.Second); err != nil {
		return fmt.Errorf("重载后初始化页未消失: %w", err)
	}
	return nil
}

