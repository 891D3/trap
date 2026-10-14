//go:build e2e

// m0_e2e.go 是真实 mygo WebView E2E，而非 store/service 单测。
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
	"sync"
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
	url = fmt.Sprintf("%s?e2e=%d", url, time.Now().UnixNano())

	dataDir, err := os.MkdirTemp("", "trap-e2e-data-")
	if err != nil {
		return fmt.Errorf("创建 E2E 数据目录: %w", err)
	}
	defer os.RemoveAll(dataDir)
	exportDir, err := os.MkdirTemp("", "trap-e2e-export-")
	if err != nil {
		return fmt.Errorf("创建 E2E 导出目录: %w", err)
	}
	defer os.RemoveAll(exportDir)
	db, err := store.Open(context.Background(), filepath.Join(dataDir, "trap-e2e.db"))
	if err != nil {
		return fmt.Errorf("打开 E2E 临时数据库: %w", err)
	}
	defer db.Close()
	initialSettings, err := db.GetSettings(context.Background())
	if err != nil {
		return fmt.Errorf("读取 E2E 初始设置: %w", err)
	}
	if len(initialSettings) != 0 {
		return fmt.Errorf("E2E 临时 SQLite 非空: %#v", initialSettings)
	}

	appService := &service.AppService{}
	todoService := &service.TodoService{}
	noteService := &service.NoteService{}
	service.AttachStore(appService, db, dataDir)
	service.AttachTodoStore(todoService, db)
	service.AttachNoteStore(noteService, db)
	mygo.Bind(appService, todoService, noteService)

	flowDone := make(chan error, 1)
	secondPageReady := make(chan *mygo.Page, 1)
	var firstDOMReady sync.Once
	var secondDOMReady sync.Once
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
			secondDOMReady.Do(func() {
				secondPageReady <- secondWindow.Page()
			})
		})
		window.Page().OnDOMReady(func() {
			firstDOMReady.Do(func() {
				go func() {
					secondPage := <-secondPageReady
					// Vite dev server 会在首次发现新依赖时整页 reload；先预热
					// Milkdown 依赖图并等页面稳定，避免真实流程被 reload 打断。
					if err := warmupEditorAndDeps(window.Page(), url, noteService); err != nil {
						flowDone <- err
						mygo.App.Quit()
						return
					}
					if err := runSettingsFlow(window.Page(), secondPage, url, exportDir); err != nil {
						flowDone <- err
						mygo.App.Quit()
						return
					}
					if err := runTodoFlow(window.Page(), secondPage, url); err != nil {
						flowDone <- err
						mygo.App.Quit()
						return
					}
					flowDone <- runNoteFlow(window.Page(), secondPage, url, exportDir, noteService)
					mygo.App.Quit()
				}()
			})
		})
	})

	timeout := time.AfterFunc(240*time.Second, func() {
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

func runSettingsFlow(page, syncPage *mygo.Page, url, exportDir string) error {
	if err := reloadAndWait(page, url); err != nil {
		return fmt.Errorf("初始化主窗口设置页: %w", err)
	}
	if err := reloadAndWait(syncPage, url); err != nil {
		return fmt.Errorf("初始化同步窗口设置页: %w", err)
	}
	if err := waitFor(page, `document.querySelector('button.side-item') !== null`, 15*time.Second); err != nil {
		return fmt.Errorf("等待主界面: %w", err)
	}
	if err := clickSettings(page); err != nil {
		return fmt.Errorf("点击设置导航失败: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="数据目录"]') !== null`, 10*time.Second); err != nil {
		return fmt.Errorf("等待设置页: %w", err)
	}

	name, err := mygo.EvalAs[string](page, `document.querySelector('.side-user-name')?.textContent ?? ''`)
	if err != nil || name != "Eric" {
		current, _ := mygo.EvalAs[string](page, `JSON.stringify({name: document.querySelector('.side-user-name')?.textContent ?? '', input: document.querySelector('input')?.value ?? '', dataDir: document.querySelector('input[aria-label="数据目录"]')?.value ?? ''})`)
		return fmt.Errorf("首次设置姓名 = %q，期望 Eric，错误=%v；当前状态=%s", name, err, current)
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

	if err := setInputAndBlur(page, `input[aria-label="导出目录"]`, exportDir); err != nil {
		return fmt.Errorf("填写导出目录: %w", err)
	}
	if err := reloadAndWait(page, url); err != nil {
		return fmt.Errorf("首次持久化后重载: %w", err)
	}
	if err := waitFor(page, fmt.Sprintf(`document.querySelector('input[aria-label="导出目录"]')?.value === %s`, strconv.Quote(exportDir)), 10*time.Second); err != nil {
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

func runTodoFlow(page, syncPage *mygo.Page, url string) error {
	if err := reloadToTodo(page, url); err != nil {
		return fmt.Errorf("重载主窗口到待办页: %w", err)
	}
	if err := reloadToTodo(syncPage, url); err != nil {
		return fmt.Errorf("重载同步窗口到待办页: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-input')?.disabled === false`, 10*time.Second); err != nil {
		return fmt.Errorf("待办页未加载完成: %w", err)
	}
	if err := waitFor(syncPage, `document.querySelector('.todo-input')?.disabled === false`, 10*time.Second); err != nil {
		return fmt.Errorf("同步窗口待办页未加载完成: %w", err)
	}

	if err := setInputAndPressEnter(page, "E2E 待办"); err != nil {
		return fmt.Errorf("添加待办: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-text')?.textContent === 'E2E 待办'`, 10*time.Second); err != nil {
		return fmt.Errorf("主窗口未显示新增待办: %w", err)
	}
	if err := waitFor(syncPage, `document.querySelector('.todo-text')?.textContent === 'E2E 待办'`, 10*time.Second); err != nil {
		return fmt.Errorf("todo.changed 未同步第二窗口新增待办: %w", err)
	}

	if err := reloadToTodo(page, url); err != nil {
		return fmt.Errorf("添加后重载待办: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-text')?.textContent === 'E2E 待办'`, 10*time.Second); err != nil {
		return fmt.Errorf("待办重载后未持久化: %w", err)
	}

	if err := clickTodoCheckbox(page, "完成 E2E 待办"); err != nil {
		return fmt.Errorf("完成待办: %w", err)
	}
	if err := waitFor(page, `document.querySelector('button[aria-label="恢复 E2E 待办"]')?.getAttribute('aria-checked') === 'true'`, 10*time.Second); err != nil {
		return fmt.Errorf("主窗口完成状态未更新: %w", err)
	}
	if err := waitFor(syncPage, `document.querySelector('button[aria-label="恢复 E2E 待办"]')?.getAttribute('aria-checked') === 'true'`, 10*time.Second); err != nil {
		return fmt.Errorf("todo.changed 未同步第二窗口完成状态: %w", err)
	}

	if err := clickTodoCheckbox(page, "恢复 E2E 待办"); err != nil {
		return fmt.Errorf("取消完成待办: %w", err)
	}
	if err := waitFor(page, `document.querySelector('button[aria-label="完成 E2E 待办"]')?.getAttribute('aria-checked') === 'false'`, 10*time.Second); err != nil {
		return fmt.Errorf("取消完成状态未更新: %w", err)
	}

	if _, err := page.Eval(`(() => {
		const button = document.querySelector('button[aria-label="删除 E2E 待办"]');
		if (!button) throw new Error('delete button not found');
		button.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("打开删除确认: %w", err)
	}
	if err := waitFor(page, `document.querySelector('[role="dialog"]')?.textContent?.includes('确定删除') === true`, 5*time.Second); err != nil {
		return fmt.Errorf("删除确认弹窗未出现: %w", err)
	}
	if _, err := page.Eval(`(() => {
		const button = [...document.querySelectorAll('[role="dialog"] button')].find((item) => item.textContent?.trim() === '删除');
		if (!button) throw new Error('confirm delete button not found');
		button.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("确认删除: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-text') === null`, 10*time.Second); err != nil {
		return fmt.Errorf("删除后主窗口仍有待办: %w", err)
	}
	if err := waitFor(syncPage, `document.querySelector('.todo-text') === null`, 10*time.Second); err != nil {
		return fmt.Errorf("删除事件未同步第二窗口: %w", err)
	}

	if err := setInputAndPressEnter(page, ""); err != nil {
		return fmt.Errorf("提交空待办: %w", err)
	}
	if err := waitFor(page, `document.querySelector('[role="alert"]')?.textContent?.includes('待办内容不能为空') === true`, 5*time.Second); err != nil {
		return fmt.Errorf("空待办中文错误提示未出现: %w", err)
	}
	return nil
}

func runNoteFlow(page, syncPage *mygo.Page, url, exportDir string, notes *service.NoteService) error {
	if err := reloadToNotes(page, url); err != nil {
		return fmt.Errorf("重载主窗口到信息记录: %w", err)
	}
	if err := reloadToNotes(syncPage, url); err != nil {
		return fmt.Errorf("重载同步窗口到信息记录: %w", err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := clickText(page, "新建记录"); err != nil {
		return fmt.Errorf("新建记录: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="记录标题"]') !== null && document.querySelector('[contenteditable="true"]') !== null`, 15*time.Second); err != nil {
		return fmt.Errorf("等待 Milkdown 编辑器: %w", err)
	}
	if err := setInputAndBlur(page, `input[aria-label="记录标题"]`, "E2E 记录"); err != nil {
		return fmt.Errorf("填写记录标题: %w", err)
	}
	markdown := "# E2E 记录\n\n| A | B |\n|---|---|\n| 1 | 2 |"
	if err := setMilkdownContent(page, markdown); err != nil {
		return fmt.Errorf("填写 Markdown 表格: %w", err)
	}
	if err := waitFor(page, `document.body.innerText.includes('已保存')`, 10*time.Second); err != nil {
		return fmt.Errorf("2 秒自动保存未完成: %w", err)
	}
	// 以数据库内容为基准：Milkdown 会对粘贴的 Markdown 做规范化，
	// 导出必须与库内内容完全一致（PRD：无损导出）。
	saved, err := noteContentByTitle(notes, "E2E 记录")
	if err != nil {
		return fmt.Errorf("读取保存后的记录: %w", err)
	}
	// Milkdown 会规范化表格对齐（如 `| :- |`），因此断言结构本质：
	// 无反斜杠转义（说明解析为 GFM 而非纯文本）、标题与两行单元格存在。
	if strings.Contains(saved, "\\#") || strings.Contains(saved, "\\|") ||
		!strings.Contains(saved, "# E2E 记录") || !strings.Contains(saved, "| 1") || !strings.Contains(saved, "| 2") {
		return fmt.Errorf("Markdown 未被解析为 GFM 结构: %q", saved)
	}
	if err := waitFor(syncPage, `document.querySelector('.note-card-title')?.textContent === 'E2E 记录'`, 10*time.Second); err != nil {
		return fmt.Errorf("note.changed 未同步第二窗口新记录: %w", err)
	}

	if err := reloadToNotes(page, url); err != nil {
		return fmt.Errorf("记录保存后重载: %w", err)
	}
	if err := clickNoteCard(page, "E2E 记录"); err != nil {
		return fmt.Errorf("打开保存后的记录: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="记录标题"]')?.value === 'E2E 记录'`, 10*time.Second); err != nil {
		return fmt.Errorf("记录标题未持久化: %w", err)
	}
	if err := waitFor(page, `document.querySelector('[contenteditable="true"]')?.innerText?.includes('1') === true`, 20*time.Second); err != nil {
		return fmt.Errorf("Markdown 表格正文未持久化: %w", err)
	}
	if err := clickAriaLabel(page, "归档"); err != nil {
		return fmt.Errorf("归档记录: %w", err)
	}
	if err := waitFor(syncPage, `document.querySelector('.note-card-title') === null`, 10*time.Second); err != nil {
		return fmt.Errorf("归档事件未同步第二窗口: %w", err)
	}
	if err := clickText(page, "已归档"); err != nil {
		return fmt.Errorf("打开已归档筛选: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.note-card-title')?.textContent === 'E2E 记录'`, 10*time.Second); err != nil {
		return fmt.Errorf("归档记录未出现在已归档列表: %w", err)
	}
	if err := clickText(page, "导出 Markdown"); err != nil {
		return fmt.Errorf("导出 Markdown: %w", err)
	}
	var exported string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(exportDir)
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".md" {
				exported = filepath.Join(exportDir, entry.Name())
				break
			}
		}
		if exported != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if exported == "" {
		return fmt.Errorf("导出目录未生成 Markdown 文件: %s", exportDir)
	}
	written, err := os.ReadFile(exported)
	if err != nil {
		return fmt.Errorf("读取导出文件: %w", err)
	}
	if string(written) != saved {
		return fmt.Errorf("导出内容与库内不一致: 导出=%q 库内=%q", string(written), saved)
	}

	if _, err := page.Eval(`(() => {
		const button = document.querySelector('button[aria-label="删除当前记录"]');
		if (!button) throw new Error('note delete button not found');
		button.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("打开记录删除确认: %w", err)
	}
	if err := waitFor(page, `document.querySelector('[role="dialog"]')?.textContent?.includes('确定删除') === true`, 5*time.Second); err != nil {
		return fmt.Errorf("记录删除确认未出现: %w", err)
	}
	if err := clickText(page, "删除"); err != nil {
		return fmt.Errorf("确认删除记录: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.note-card-title') === null`, 10*time.Second); err != nil {
		return fmt.Errorf("删除后记录仍存在: %w", err)
	}
	return nil
}

// warmupEditorAndDeps 预热 Vite 依赖图与 Milkdown 编辑器。
//
// Vite dev server 首次发现新依赖会整页 reload，真实流程不能被打断；
// 因此先等页面稳定，再实例化一次编辑器（重试以吸收期间的 reload），
// 经 Go 侧删除预热记录（UI 经 note.changed 刷新），最后再次确认稳定。
func warmupEditorAndDeps(page *mygo.Page, url string, notes *service.NoteService) error {
	if err := reloadToNotes(page, url); err != nil {
		return fmt.Errorf("预热信息记录页: %w", err)
	}
	if err := waitPageStable(page); err != nil {
		return fmt.Errorf("预热等待页面稳定: %w", err)
	}
	var editorWaitErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := clickText(page, "新建记录"); err != nil {
			return fmt.Errorf("预热创建记录: %w", err)
		}
		if err := waitFor(page, `document.querySelector('[contenteditable="true"]') !== null`, 20*time.Second); err != nil {
			editorWaitErr = err
			// 等待期间可能发生 Vite reload：重载后重试（重复记录由下方清理）。
			if err := reloadToNotes(page, url); err != nil {
				return fmt.Errorf("预热重载: %w", err)
			}
			if err := waitPageStable(page); err != nil {
				return fmt.Errorf("预热等待稳定: %w", err)
			}
			continue
		}
		editorWaitErr = nil
		break
	}
	if editorWaitErr != nil {
		return fmt.Errorf("预热等待编辑器: %w", editorWaitErr)
	}
	existing, err := notes.List(context.Background(), true)
	if err != nil {
		return fmt.Errorf("预热读取记录: %w", err)
	}
	for _, note := range existing {
		if err := notes.Delete(context.Background(), note.ID); err != nil {
			return fmt.Errorf("预热清理记录 %d: %w", note.ID, err)
		}
	}
	if err := waitFor(page, `document.querySelector('.note-card-title') === null`, 10*time.Second); err != nil {
		return fmt.Errorf("预热清理未同步: %w", err)
	}
	return waitPageStable(page)
}

// waitPageStable 反复写窗口标记并检查其是否存活，确认页面不再被 reload。
func waitPageStable(page *mygo.Page) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		_, _ = page.Eval(`(() => { window.__e2eMarker = "stable"; return true; })()`)
		time.Sleep(4 * time.Second)
		stable, err := mygo.EvalAs[bool](page, `window.__e2eMarker === "stable"`)
		if err == nil && stable {
			return nil
		}
	}
	return fmt.Errorf("页面在 60s 内未稳定（Vite 持续 reload）")
}

func reloadToNotes(page *mygo.Page, url string) error {
	if err := loadPageDocument(page, uniquePageURL(url)+"&e2eView=note"); err != nil {
		return err
	}
	if err := waitFor(page, `([...document.querySelectorAll('button')].some((item) => item.textContent?.trim() === '新建记录'))`, 15*time.Second); err != nil {
		return fmt.Errorf("等待信息记录页: %w", err)
	}
	return nil
}

// loadPageDocument 导航到 targetURL 并等待文档真正更换。
//
// WKWebView 的导航是异步提交的：LoadURL 返回时旧文档可能仍在展示；
// 若旧文档已包含等待条件（如信息记录页的“新建记录”按钮），后续断言会
// 在旧文档上假通过，导航提交后编辑器等状态被整体替换。因此以
// performance.timeOrigin 变化作为“新文档已生效”的判据。
func loadPageDocument(page *mygo.Page, targetURL string) error {
	oldOrigin, err := mygo.EvalAs[float64](page, "performance.timeOrigin")
	if err != nil {
		oldOrigin = 0
	}
	if err := page.LoadURL(targetURL); err != nil {
		return err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		newOrigin, err := mygo.EvalAs[float64](page, "performance.timeOrigin")
		if err == nil && newOrigin > 0 && newOrigin != oldOrigin {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("等待页面文档更换超时: %s", targetURL)
}

func clickText(page *mygo.Page, text string) error {
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === %q);
		if (!button) throw new Error('button not found: ' + %q);
		button.focus();
		button.click();
		return true;
	})()`, text, text))
	if err == nil {
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func clickAriaLabel(page *mygo.Page, label string) error {
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const button = document.querySelector('button[aria-label=%q]');
		if (!button) throw new Error('button not found by aria-label: ' + %q);
		button.focus();
		button.click();
		return true;
	})()`, label, label))
	if err == nil {
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

func clickNoteCard(page *mygo.Page, title string) error {
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const cards = [...document.querySelectorAll('.note-card')];
		const card = cards.find((item) => item.querySelector('.note-card-title')?.textContent?.trim() === %q);
		if (!card) throw new Error('note card not found: ' + %q);
		card.click();
		return true;
	})()`, title, title))
	return err
}

func setMilkdownContent(page *mygo.Page, markdown string) error {
	// Milkdown 的 clipboard 插件会把粘贴的 text/plain 解析为 GFM 文档；
	// 直接 execCommand('insertText') 只会插入纯文本并被转义。
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const editor = document.querySelector('[contenteditable="true"]');
		if (!editor) throw new Error('Milkdown contenteditable not found');
		editor.focus();
		document.execCommand('selectAll');
		const transfer = new DataTransfer();
		transfer.setData('text/plain', %s);
		const event = new ClipboardEvent('paste', { clipboardData: transfer, bubbles: true, cancelable: true });
		editor.dispatchEvent(event);
		return true;
	})()`, strconv.Quote(markdown)))
	return err
}

// noteContentByTitle 按标题读取记录正文（E2E 断言用）。
func noteContentByTitle(notes *service.NoteService, title string) (string, error) {
	list, err := notes.List(context.Background(), true)
	if err != nil {
		return "", err
	}
	for _, summary := range list {
		if summary.Title == title {
			note, err := notes.Get(context.Background(), summary.ID)
			if err != nil {
				return "", err
			}
			return note.Content, nil
		}
	}
	return "", fmt.Errorf("未找到标题为 %q 的记录", title)
}

func clickSettings(page *mygo.Page) error {
	time.Sleep(500 * time.Millisecond)
	_, err := page.Eval(`(() => {
		const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '设置');
		if (!button) throw new Error('settings button not found');
		button.focus();
		button.click();
		return true;
	})()`)
	return err
}

func reloadToTodo(page *mygo.Page, url string) error {
	if err := loadPageDocument(page, uniquePageURL(url)); err != nil {
		return err
	}
	return waitFor(page, `document.querySelector('.todo-input') !== null`, 15*time.Second)
}

func clickTodoCheckbox(page *mygo.Page, label string) error {
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const button = document.querySelector('button[aria-label=%q]');
		if (!button) throw new Error('checkbox not found: ' + %q);
		button.click();
		return true;
	})()`, label, label))
	return err
}

func setInputAndPressEnter(page *mygo.Page, value string) error {
	quoted := strconv.Quote(value)
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const input = document.querySelector('.todo-input');
		if (!input) throw new Error('todo input not found');
		const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
		if (!setter) throw new Error('input value setter not found');
		input.focus();
		setter.call(input, %s);
		input.dispatchEvent(new Event('input', { bubbles: true }));
		input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
		return true;
	})()`, quoted))
	return err
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
	if err := loadPageDocument(page, uniquePageURL(url)); err != nil {
		return err
	}
	if err := waitFor(page, `document.querySelector('.todo-input') !== null`, 15*time.Second); err != nil {
		return err
	}
	if err := clickSettings(page); err != nil {
		return fmt.Errorf("重载后点击设置导航失败: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="数据目录"]') !== null`, 10*time.Second); err != nil {
		current, _ := mygo.EvalAs[string](page, `JSON.stringify({body: document.body.innerText, buttons: [...document.querySelectorAll('button')].map((item) => ({text: item.textContent?.trim(), className: item.className})), inputs: [...document.querySelectorAll('input')].map((item) => item.getAttribute('aria-label'))})`)
		return fmt.Errorf("等待重载设置页: %w；当前状态=%s", err, current)
	}
	return nil
}

func uniquePageURL(url string) string {
	separator := "&"
	if !strings.Contains(url, "?") {
		separator = "?"
	}
	return fmt.Sprintf("%s%se2e_reload=%d", url, separator, time.Now().UnixNano())
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
