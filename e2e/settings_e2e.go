//go:build e2e

// settings_test.go 是真实 mygo WebView E2E，而非 store/service 单测。
// 测试进程会启动真实 macOS WebView，页面通过 generated client 调用绑定的
// AppService，Go 侧使用临时 SQLite 文件；每个 UI 操作都经过真实 IPC。
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/891D3/trap/internal/service"
	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

const defaultE2EURL = "http://127.0.0.1:5173/"

func main() {
	if err := runSettingsWebViewE2E(); err != nil {
		log.Fatal(err)
	}
}

func runSettingsWebViewE2E() error {
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

	dataDir, err := os.MkdirTemp("", "trap-e2e-data-")
	if err != nil {
		return fmt.Errorf("创建 E2E 数据目录: %w", err)
	}
	defer os.RemoveAll(dataDir)
	db, err := store.Open(context.Background(), filepath.Join(dataDir, "trap-e2e.db"))
	if err != nil {
		return fmt.Errorf("打开 E2E 临时数据库: %w", err)
	}
	defer db.Close()
	initialSettings, err := db.GetSettings(context.Background())
	if err != nil {
		return fmt.Errorf("读取 E2E 初始设置: %w", err)
	}
	log.Printf("E2E 初始 SQLite settings = %#v", initialSettings)

	appService := &service.AppService{}
	service.AttachStore(appService, db, dataDir)
	mygo.Bind(appService)

	flowDone := make(chan error, 1)
	secondPageReady := make(chan *mygo.Page, 1)
	mygo.App.WhenReady(func() {
		window := mygo.NewWindow(mygo.WindowOptions{
			Title:  "Trap E2E",
			Width:  1280,
			Height: 800,
			URL:    url,
		})
		secondWindow := mygo.NewWindow(mygo.WindowOptions{
			Title:  "Trap E2E Sync",
			Width:  1280,
			Height: 800,
			URL:    url,
		})
		secondWindow.Page().OnDOMReady(func() {
			secondPageReady <- secondWindow.Page()
		})
		window.Page().OnDOMReady(func() {
			go func() {
				secondPage := <-secondPageReady
				flowDone <- runSettingsFlow(window.Page(), secondPage, url)
				mygo.App.Quit()
			}()
		})
	})

	timeout := time.AfterFunc(60*time.Second, func() {
		select {
		case flowDone <- fmt.Errorf("E2E 超时：页面未完成设置流程"):
		default:
		}
		mygo.App.Quit()
	})
	defer timeout.Stop()

	if err := mygo.App.Run(); err != nil {
		return fmt.Errorf("mygo App.Run: %w", err)
	}
	select {
	case err := <-flowDone:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("mygo 已退出，但没有收到 E2E 流程结果")
	}
}

func runSettingsFlow(page, syncPage *mygo.Page, url string) error {
	if err := waitFor(page, `document.querySelector('button.side-item') !== null`, 15*time.Second); err != nil {
		return fmt.Errorf("等待主界面: %w", err)
	}
	if ok, err := mygo.EvalAs[bool](page, `(() => {
		const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '设置');
		if (!button) return false;
		button.click();
		return true;
	})()`); err != nil || !ok {
		return fmt.Errorf("点击设置导航失败: %v", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="数据目录"]') !== null`, 10*time.Second); err != nil {
		return fmt.Errorf("等待设置页: %w", err)
	}

	name, err := mygo.EvalAs[string](page, `document.querySelector('.side-user-name')?.textContent ?? ''`)
	if err != nil || name != "Eric" {
		return fmt.Errorf("首次设置姓名 = %q，期望 Eric，错误=%v", name, err)
	}
	dataDir, err := mygo.EvalAs[string](page, `document.querySelector('input[aria-label="数据目录"]')?.value ?? ''`)
	if err != nil || !strings.Contains(dataDir, "trap-e2e-data") {
		return fmt.Errorf("数据目录 = %q，期望包含 trap-e2e-data，错误=%v", dataDir, err)
	}

	if err := setInputAndBlur(page, `input`, "E2E 用户"); err != nil {
		return fmt.Errorf("填写姓名: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.side-user-name')?.textContent === 'E2E 用户'`, 10*time.Second); err != nil {
		current, _ := mygo.EvalAs[string](page, `JSON.stringify({name: document.querySelector('.side-user-name')?.textContent ?? '', input: document.querySelector('input')?.value ?? '', alert: document.querySelector('[role="alert"]')?.textContent ?? ''})`)
		return fmt.Errorf("姓名保存后 Sidebar 未同步: %w；当前状态=%s", err, current)
	}
	if err := waitFor(syncPage, `document.querySelector('.side-user-name')?.textContent === 'E2E 用户'`, 10*time.Second); err != nil {
		return fmt.Errorf("settings.changed 未同步第二个窗口姓名: %w", err)
	}

	if err := waitFor(page, `(() => {
		const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '深色');
		return button !== undefined && !(button instanceof HTMLButtonElement && button.disabled);
	})()`, 10*time.Second); err != nil {
		return fmt.Errorf("等待姓名保存完成: %w", err)
	}
	if ok, err := mygo.EvalAs[bool](page, `(() => {
		const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '深色');
		if (!button) return false;
		button.click();
		return true;
	})()`); err != nil || !ok {
		return fmt.Errorf("点击深色主题失败: %v", err)
	}
	if err := waitFor(page, `document.documentElement.dataset.theme === 'dark'`, 10*time.Second); err != nil {
		current, _ := mygo.EvalAs[string](page, `JSON.stringify({theme: document.documentElement.dataset.theme ?? '', buttons: [...document.querySelectorAll('button')].filter((item) => item.textContent?.trim() === '深色').map((item) => item.className), alert: document.querySelector('[role="alert"]')?.textContent ?? ''})`)
		return fmt.Errorf("深色主题未生效: %w；当前状态=%s", err, current)
	}
	if err := waitFor(syncPage, `document.documentElement.dataset.theme === 'dark'`, 10*time.Second); err != nil {
		return fmt.Errorf("settings.changed 未同步第二个窗口主题: %w", err)
	}

	if err := setInputAndBlur(page, `input[aria-label="导出目录"]`, "/tmp/trap-e2e-export"); err != nil {
		return fmt.Errorf("填写导出目录: %w", err)
	}
	if err := reloadAndWait(page, url); err != nil {
		return fmt.Errorf("首次持久化后重载: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="导出目录"]')?.value === '/tmp/trap-e2e-export'`, 10*time.Second); err != nil {
		return fmt.Errorf("导出目录未持久化: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.side-user-name')?.textContent === 'E2E 用户'`, 10*time.Second); err != nil {
		return fmt.Errorf("姓名重载后未持久化: %w", err)
	}
	if err := waitFor(page, `document.documentElement.dataset.theme === 'dark'`, 10*time.Second); err != nil {
		return fmt.Errorf("主题重载后未持久化: %w", err)
	}

	if err := setInputAndBlur(page, `input`, ""); err != nil {
		return fmt.Errorf("填写空姓名: %w", err)
	}
	if err := waitFor(page, `document.querySelector('[role="alert"]')?.textContent?.includes('姓名不能为空') === true`, 5*time.Second); err != nil {
		return fmt.Errorf("空姓名错误提示未出现: %w", err)
	}
	return nil
}

func setInputAndBlur(page *mygo.Page, selector, value string) error {
	quoted := strconv.Quote(value)
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const input = document.querySelector(%q);
		if (!input) throw new Error('input not found: ' + %q);
		const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
		if (!setter) throw new Error('input value setter not found');
		input.focus();
		setter.call(input, %s);
		input.dispatchEvent(new Event('input', { bubbles: true }));
		input.dispatchEvent(new Event('change', { bubbles: true }));
		input.blur();
		return true;
	})()`, selector, selector, quoted))
	return err
}

func reloadAndWait(page *mygo.Page, url string) error {
	if err := page.LoadURL(url); err != nil {
		return err
	}
	if err := waitFor(page, `document.querySelector('button.side-item') !== null`, 15*time.Second); err != nil {
		return err
	}
	if ok, err := mygo.EvalAs[bool](page, `(() => {
		const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '设置');
		if (!button) return false;
		button.click();
		return true;
	})()`); err != nil || !ok {
		return fmt.Errorf("重载后点击设置导航失败: %v", err)
	}
	return waitFor(page, `document.querySelector('input[aria-label="数据目录"]') !== null`, 10*time.Second)
}

func waitFor(page *mygo.Page, expression string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok, err := mygo.EvalAs[bool](page, "return "+expression)
		if err == nil && ok {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("条件未满足: %s", expression)
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, err := http.Get(url) //nolint:gosec // 仅访问本地 E2E Vite 服务。
		if err == nil {
			response.Body.Close()
			if response.StatusCode < http.StatusInternalServerError {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("本地服务 %s 在 %s 内未响应", url, timeout)
}
