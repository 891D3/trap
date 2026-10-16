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
	projectService := &service.ProjectService{}
	quickLinkService := &service.QuickLinkService{}
	service.AttachStore(appService, db, dataDir)
	service.AttachTodoStore(todoService, db)
	service.AttachNoteStore(noteService, db)
	service.AttachProjectStore(projectService, db)
	service.AttachQuickLinkStore(quickLinkService, db)
	mygo.Bind(appService, todoService, noteService, projectService, quickLinkService)

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
					// 布局约定：辅助区仅看板可见（用户确认的 UI 调整）。
					if err := runLayoutFlow(window.Page(), url); err != nil {
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
		current, _ := mygo.EvalAs[string](page, `JSON.stringify({name: document.querySelector('.side-user-name')?.textContent ?? '', input: document.querySelector('input[aria-label="姓名"]')?.value ?? '', dataDir: document.querySelector('input[aria-label="数据目录"]')?.value ?? ''})`)
		return fmt.Errorf("首次设置姓名 = %q，期望 Eric，错误=%v；当前状态=%s", name, err, current)
	}
	dataDir, err := mygo.EvalAs[string](page, `document.querySelector('input[aria-label="数据目录"]')?.value ?? ''`)
	if err != nil || !strings.Contains(dataDir, "trap-e2e-data") {
		return fmt.Errorf("数据目录 = %q，期望包含 trap-e2e-data，错误=%v", dataDir, err)
	}

	if err := setInputAndBlur(page, `input[aria-label="姓名"]`, "E2E 用户"); err != nil {
		return fmt.Errorf("填写姓名: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.side-user-name')?.textContent === 'E2E 用户'`, 10*time.Second); err != nil {
		current, _ := mygo.EvalAs[string](page, `JSON.stringify({name: document.querySelector('.side-user-name')?.textContent ?? '', input: document.querySelector('input[aria-label="姓名"]')?.value ?? '', alert: document.querySelector('[role="alert"]')?.textContent ?? ''})`)
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

	if err := setInputAndBlur(page, `input[aria-label="姓名"]`, ""); err != nil {
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

	// v3：未完成列表不渲染已完成项（改为分组分区），完成状态改为在
	// 「已完成」Tab 中断言（同步窗口同口径）。
	clickTab := func(p *mygo.Page, label string) error {
		_, err := p.Eval(fmt.Sprintf(`(() => {
			const tab = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim().startsWith(%q));
			if (!tab) throw new Error('tab not found: ' + %q);
			tab.click();
			return true;
		})()`, label, label))
		return err
	}
	if err := clickTodoCheckbox(page, "完成 E2E 待办"); err != nil {
		return fmt.Errorf("完成待办: %w", err)
	}
	if err := clickTab(page, "已完成"); err != nil {
		return fmt.Errorf("切已完成 Tab: %w", err)
	}
	if err := waitFor(page, `document.querySelector('button[aria-label="恢复 E2E 待办"]')?.getAttribute('aria-checked') === 'true'`, 10*time.Second); err != nil {
		return fmt.Errorf("主窗口完成状态未更新: %w", err)
	}
	if err := clickTab(syncPage, "已完成"); err != nil {
		return fmt.Errorf("切同步窗口已完成 Tab: %w", err)
	}
	if err := waitFor(syncPage, `document.querySelector('button[aria-label="恢复 E2E 待办"]')?.getAttribute('aria-checked') === 'true'`, 10*time.Second); err != nil {
		return fmt.Errorf("todo.changed 未同步第二窗口完成状态: %w", err)
	}

	// 取消完成：在已完成列表点击恢复，随后切回全部 Tab 验证重新进入未完成分组。
	if err := clickTodoCheckbox(page, "恢复 E2E 待办"); err != nil {
		return fmt.Errorf("取消完成待办: %w", err)
	}
	if err := clickTab(page, "全部"); err != nil {
		return fmt.Errorf("切回全部 Tab: %w", err)
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

// runLayoutFlow 验证 v3 壳布局约定（PRD v2.0 + ui-baseline v3 §八）：
//   - 辅助区进入看板与待办均可见（内容随视图映射，R4/R5 充实）；
//   - 设置页不渲染辅助区；
//   - 看板中间不再有「看板 · M2 开放」占位；
//   - 默认进入信息记录页（深链接）后顶栏呈现页面标题与面包屑。
func runLayoutFlow(page *mygo.Page, url string) error {
	// 看板：辅助区可见，且中间占位已移除；顶栏标题为「数据看板」。宽屏默认视图也是看板。
	if err := loadPageDocument(page, uniquePageURL(url)); err != nil {
		return fmt.Errorf("布局检查: 加载默认视图: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.aux') !== null && document.querySelector('.topbar')?.textContent.includes('数据看板') === true`, 10*time.Second); err != nil {
		return fmt.Errorf("默认视图应为数据看板且含辅助区: %w", err)
	}
	if ok, err := mygo.EvalAs[bool](page, `document.body.innerText.includes('看板 · M2 开放') === false`); err != nil || !ok {
		return fmt.Errorf("看板视图仍残留中间占位（err=%v, ok=%v）", err, ok)
	}
	// 待办：辅助区同样可见（v3：随视图映射为月历/按属性等卡片，R5 充实）。
	if err := loadPageDocument(page, uniquePageURL(url)+"&e2eView=todo"); err != nil {
		return fmt.Errorf("布局检查: 加载待办: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-input') !== null && document.querySelector('.aux') !== null`, 10*time.Second); err != nil {
		return fmt.Errorf("待办视图应保留辅助区: %w", err)
	}
	// 设置：无辅助区。
	if err := loadPageDocument(page, uniquePageURL(url)+"&e2eView=settings"); err != nil {
		return fmt.Errorf("布局检查: 加载设置页: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="数据目录"]') !== null`, 10*time.Second); err != nil {
		return fmt.Errorf("布局检查: 等待设置页: %w", err)
	}
	if ok, err := mygo.EvalAs[bool](page, `document.querySelector('.aux') === null`); err != nil || !ok {
		return fmt.Errorf("设置页不应显示辅助区（err=%v, ok=%v）", err, ok)
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
	if err := clickText(page, "新建笔记"); err != nil {
		return fmt.Errorf("新建记录: %w", err)
	}
	if err := waitFor(page, `document.querySelector('input[aria-label="笔记标题"]') !== null && document.querySelector('[contenteditable="true"]') !== null`, 15*time.Second); err != nil {
		return fmt.Errorf("等待 Milkdown 编辑器: %w", err)
	}
	// v2 布局约定：笔记文档态的代码/预览切换（整编辑器只读渲染）。
	if err := clickAriaLabel(page, "切换到预览"); err != nil {
		return fmt.Errorf("切换预览态: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.note-doc-editor.is-preview') !== null`, 5*time.Second); err != nil {
		return fmt.Errorf("预览态未生效: %w", err)
	}
	if err := clickAriaLabel(page, "切换到编辑"); err != nil {
		return fmt.Errorf("切回编辑态: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.note-doc-editor.is-preview') === null`, 5*time.Second); err != nil {
		return fmt.Errorf("编辑态未恢复: %w", err)
	}
	if err := setInputAndBlur(page, `input[aria-label="笔记标题"]`, "E2E 记录"); err != nil {
		return fmt.Errorf("填写记录标题: %w", err)
	}
	markdown := "# E2E 记录\n\n| A | B |\n|---|---|\n| 1 | 2 |"
	if err := setMilkdownContent(page, markdown); err != nil {
		return fmt.Errorf("填写 Markdown 表格: %w", err)
	}
	// 自动保存以数据库为断言基准（轮询，吸收防抖窗口与保存耗时）。
	waitNoteContent(notes, "E2E 记录", "| 1", 10*time.Second)
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
	if err := waitFor(syncPage, `[...document.querySelectorAll('.aux-note-title')].some((item) => item.textContent === 'E2E 记录')`, 10*time.Second); err != nil {
		return fmt.Errorf("note.changed 未同步第二窗口新记录: %w", err)
	}

	if err := reloadToNotes(page, url); err != nil {
		return fmt.Errorf("记录保存后重载: %w", err)
	}
	// 辅助区行点击 → 打开保存后的记录（v2 联动通道）。
	clickAuxRow(page, "E2E 记录")
	if err := waitFor(page, `document.querySelector('input[aria-label="笔记标题"]')?.value === 'E2E 记录'`, 10*time.Second); err != nil {
		return fmt.Errorf("记录标题未持久化: %w", err)
	}
	if err := waitFor(page, `document.querySelector('[contenteditable="true"]')?.innerText?.includes('1') === true`, 20*time.Second); err != nil {
		return fmt.Errorf("Markdown 表格正文未持久化: %w", err)
	}
	// v2：经辅助区导出卡导出当前笔记（Markdown 主链路保真断言沿用）。
	if err := waitFor(page, `document.querySelector('input[aria-label="笔记标题"]')?.value === 'E2E 记录'`, 6*time.Second); err != nil {
		return fmt.Errorf("导出前编辑态丢失: %w", err)
	}
	_, err2 := page.Eval(`(() => {
		const row = [...document.querySelectorAll('.aux-export-row')].find((item) => item.title?.includes('Markdown'));
		if (!row) throw new Error('export row not found');
		row.click();
		return true;
	})()`)
	if err2 != nil {
		return fmt.Errorf("点击导出 Markdown: %w", err2)
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
	if err := waitFor(page, `[...document.querySelectorAll('.aux-note-title')].every((item) => item.textContent !== 'E2E 记录')`, 10*time.Second); err != nil {
		return fmt.Errorf("删除后记录仍存在: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.note-empty') !== null`, 8*time.Second); err != nil {
		dbg, _ := mygo.EvalAs[string](page, `(() => {
			const rows = [...document.querySelectorAll('.aux-note-row')].map((r) => r.querySelector('.aux-note-title')?.textContent);
			const editor = document.querySelector('.note-doc-title')?.value;
			const errBox = document.querySelector('.notev2 [role="alert"]')?.textContent;
			return JSON.stringify({rows, editor, errBox, body: document.body.innerText.slice(0, 300)});
		})()`)
		return fmt.Errorf("删除后未回到空态: %w；状态=%s", err, dbg)
	}
	return nil
}

// clickAuxRow 点击辅助区最近笔记行（v2 联动通道）。
func clickAuxRow(page *mygo.Page, title string) {
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const row = [...document.querySelectorAll('.aux-note-row')].find((item) => item.querySelector('.aux-note-title')?.textContent === %q);
		if (!row) throw new Error('aux note row not found: ' + %q);
		row.click();
		return true;
	})()`, title, title))
	if err != nil {
		return
	}
	time.Sleep(500 * time.Millisecond)
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
		if err := clickText(page, "新建笔记"); err != nil {
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
	if err := waitFor(page, `[...document.querySelectorAll('.aux-note-title')].length === 0`, 10*time.Second); err != nil {
		return fmt.Errorf("预热清理未同步: %w", err)
	}
	return waitPageStable(page)
}

// waitNoteContent 轮询数据库直到指定标题笔记的正文包含子串（自动保存同步）。
func waitNoteContent(notes *service.NoteService, title, substr string, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if content, err := noteContentByTitle(notes, title); err == nil && strings.Contains(content, substr) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
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
	// v2 信息记录默认灵感速记态；E2E 流程进入「笔记文档」态。
	if err := waitFor(page, `document.querySelector('.notev2 .view-tab-row') !== null`, 15*time.Second); err != nil {
		return fmt.Errorf("等待信息记录页视图 Tab: %w", err)
	}
	_, _ = page.Eval(`(() => {
		const tab = [...document.querySelectorAll('.view-tab')].find((item) => item.textContent?.includes('笔记文档'));
		if (tab) tab.click();
		return true;
	})()`)
	if err := waitFor(page, `([...document.querySelectorAll('button')].some((item) => item.textContent?.trim() === '新建笔记'))`, 15*time.Second); err != nil {
		return fmt.Errorf("等待笔记文档态: %w", err)
	}
	return nil
}

// loadPageDocument 导航到 targetURL 并等待文档真正更换。
//
// WKWebView 的导航是异步提交的：LoadURL 返回时旧文档可能仍在展示；
// 若旧文档已包含等待条件（如信息记录页的“新建笔记”按钮），后续断言会
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
	// 记录列表异步加载；先等任意卡片出现再定位标题，避免与首次列表请求赛跑。
	if err := waitFor(page, `document.querySelector('.note-card') !== null`, 10*time.Second); err != nil {
		return fmt.Errorf("等待记录列表加载: %w", err)
	}
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const cards = [...document.querySelectorAll('.note-card')];
		const card = cards.find((item) => item.querySelector('.note-card-title')?.textContent?.trim() === %q);
		if (!card) {
			const titles = cards.map((item) => item.querySelector('.note-card-title')?.textContent);
			const tabs = document.body.innerText.slice(0, 600);
			throw new Error('note card not found: ' + %q + ' | cards=' + cards.length + ' titles=' + JSON.stringify(titles) + ' | page:' + tabs);
		}
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
	if err := loadPageDocument(page, uniquePageURL(url)+"&e2eView=todo"); err != nil {
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
	// v3：待办快速创建是多行 textarea，Enter 仅换行，提交用 ⌘/Ctrl+Enter
	// （PRD v2.0 拟定决策 7）。同步 setter 走 HTMLTextAreaElement 原型。
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const input = document.querySelector('.todo-input');
		if (!input) throw new Error('todo input not found');
		const TagProto = input instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement;
		const setter = Object.getOwnPropertyDescriptor(TagProto.prototype, 'value')?.set;
		if (!setter) throw new Error('input value setter not found');
		input.focus();
		setter.call(input, %s);
		input.dispatchEvent(new Event('input', { bubbles: true }));
		input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true }));
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
	if err := waitFor(page, `document.querySelector('button.side-item') !== null`, 15*time.Second); err != nil {
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

// runTodoV2Flow 覆盖 v2.0 待办扩展能力（PRD v2.0 验收 3）：
//   - 时间短语「明天 15:00」→ 落库 due_date 且入「明天」分组；
//   - # 项目联想 → project_id 持久化；行内编辑改标题保存；月历日期过滤。
func runTodoV2Flow(page *mygo.Page, url string, todos *service.TodoService, projects *service.ProjectService) error {
	if err := reloadToTodo(page, url); err != nil {
		return fmt.Errorf("v2 流程: 重载待办页: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-input')?.disabled === false`, 10*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 待办页未加载: %w", err)
	}

	// 种子项目（服务直建，验证联想与关联）。
	project, err := projects.Add(context.Background(), "前端测试项目", "green")
	if err != nil {
		return fmt.Errorf("v2 流程: 种子项目: %w", err)
	}

	// 1) 时间短语解析：「明天 15:00 测试解析」。
	page2 := page
	if _, err := callSetter(page2, "明天 15:00 测试解析"); err != nil {
		return fmt.Errorf("v2 流程: 输入时间短语: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.quick-chips')?.textContent?.includes('明天 15:00') === true`, 5*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 时间 chip 未出现: %w", err)
	}
	if err := pressMetaEnter(page2); err != nil {
		return fmt.Errorf("v2 流程: 提交时间短语: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.todo-group-label')?.textContent?.includes('明天') === true`, 6*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 未进入明天分组: %w", err)
	}
	// 库内断言 due_date（按本机时区折算 UTC）。
	loc := time.Local
	parsed := time.Now().In(loc).AddDate(0, 0, 1)
	expectedDue := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 15, 0, 0, 0, loc).UTC()
	list, err := todos.List(context.Background(), store.TodoFilter{})
	if err != nil {
		return fmt.Errorf("v2 流程: 读库: %w", err)
	}
	var dueTodo *store.Todo
	for i := range list {
		if list[i].Title == "测试解析" {
			dueTodo = &list[i]
		}
	}
	if dueTodo == nil || dueTodo.DueDate == nil || *dueTodo.DueDate != expectedDue.Format(time.RFC3339) ||
		dueTodo.Category != "work" {
		return fmt.Errorf("v2 流程: due_date 断言失败, 实际 %+v", dueTodo)
	}

	// 2) # 项目联想：输入 #前端 → 点击浮层项 → ⌘Enter 提交。
	if _, err := callSetter(page, "#前端 解析联想待办"); err != nil {
		return fmt.Errorf("v2 流程: 输入 # 联想: %w", err)
	}
	if err := waitFor(page, `[...document.querySelectorAll('.project-pop-item')].some((item) => item.textContent?.includes('#前端测试项目'))`, 5*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 项目联想浮层未出现: %w", err)
	}
	if _, err := page.Eval(`(() => {
		const item = [...document.querySelectorAll('.project-pop-item')].find((item) => item.textContent?.includes('#前端测试项目'));
		if (!item) throw new Error('popover item not found');
		item.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("v2 流程: 点击联想项: %w", err)
	}
	if err := pressMetaEnter(page); err != nil {
		return fmt.Errorf("v2 流程: 提交联想: %w", err)
	}
	list2, err := todos.List(context.Background(), store.TodoFilter{})
	if err != nil {
		return fmt.Errorf("v2 流程: 读库二: %w", err)
	}
	var linked *store.Todo
	for i := range list2 {
		if list2[i].Title == "解析联想待办" && list2[i].ProjectID != nil && *list2[i].ProjectID == project.ID {
			linked = &list2[i]
		}
	}
	if linked == nil {
		return fmt.Errorf("v2 流程: 项目关联断言失败: %+v", list2)
	}
	// 待办行上出现 #项目 文本。
	if err := waitFor(page, `[...document.querySelectorAll('.todo-project')].some((item) => item.textContent?.includes('#前端测试项目'))`, 6*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 条目项目名未显示: %w", err)
	}

	// 3) 行内编辑：点击「测试解析」标题 → 修改为「测试解析·改」→ 保存 → 库内断言。
	clicked := false
	for _, candidate := range []string{"测试解析", "解析联想待办"} {
		_, err := page.Eval(fmt.Sprintf(`(() => {
			const cell = [...document.querySelectorAll('.todo-text')].find((item) => item.textContent?.trim() === %q);
			if (!cell) return false;
			cell.click();
			return true;
		})()`, candidate))
		if err == nil {
			if ok, _ := mygo.EvalAs[bool](page, `document.querySelector('.todo-edit-input') !== null`); ok {
				clicked = true
				break
			}
		}
	}
	if !clicked {
		return fmt.Errorf("v2 流程: 进入行内编辑失败")
	}
	if _, err := page.Eval(`(() => {
		const input = document.querySelector('.todo-edit-input');
		const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
		setter.call(input, '测试解析·改');
		input.dispatchEvent(new Event('input', { bubbles: true }));
		return true;
	})()`); err != nil {
		return fmt.Errorf("v2 流程: 编辑标题: %w", err)
	}
	if _, err := page.Eval(`(() => {
		const btn = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '保存');
		if (!btn) throw new Error('save button not found');
		btn.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("v2 流程: 保存编辑: %w", err)
	}
	if err := waitFor(page, `[...document.querySelectorAll('.todo-text')].some((item) => item.textContent?.includes('测试解析·改'))`, 6*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 编辑未生效: %w", err)
	}
	list3, err := todos.List(context.Background(), store.TodoFilter{})
	if err != nil {
		return fmt.Errorf("v2 流程: 读库三: %w", err)
	}
	editedOK := false
	for _, t := range list3 {
		if t.Title == "测试解析·改" && t.Category == "work" {
			editedOK = true
		}
	}
	if !editedOK {
		return fmt.Errorf("v2 流程: 编辑落库断言失败: %+v", list3)
	}

	// 4) 月历日期过滤：点击「明天」格（明天有 1 条带日期待办）→ 过滤 chip 出现。
	if err := clickText(page, "明天"); err != nil {
		return fmt.Errorf("v2 流程: 明天分组日不存在: %w", err)
	}
	// 通过月历格（值 = 明天日期号）点击。
	tomorrowNum := time.Now().In(loc).AddDate(0, 0, 1).Format("2")
	_, err = page.Eval(fmt.Sprintf(`(() => {
		const cells = [...document.querySelectorAll('.aux-month-day')].filter((el) => !el.className.includes('is-out') && el.querySelector('.aux-month-num')?.textContent === %q && el.querySelector('.aux-month-dot') !== null);
		const cell = cells.find((el) => !el.className.includes('is-today'));
		if (!cell) throw new Error('tomorrow cell not found');
		cell.click();
		return true;
	})()`, tomorrowNum))
	if err != nil {
		return fmt.Errorf("v2 流程: 点击月历明天格: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.list-filter-chips')?.querySelectorAll('.chip').length >= 2`, 6*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 日期过滤 chip 未出现: %w", err)
	}
	if _, err := page.Eval(`(() => {
		const btn = document.querySelector('button[aria-label="清除日期过滤"]');
		if (!btn) throw new Error('clear chip not found');
		btn.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("v2 流程: 清除日期过滤: %w", err)
	}
	if err := waitFor(page, `document.querySelector('.list-filter-chips') === null || document.querySelector('.list-filter-chips')?.querySelector('[aria-label="清除日期过滤"]') === null`, 5*time.Second); err != nil {
		return fmt.Errorf("v2 流程: 日期过滤未清除: %w", err)
	}
	return nil
}

// callSetter 向快速创建 textarea 写入文本（不提交）。
func callSetter(page *mygo.Page, text string) (bool, error) {
	return mygo.EvalAs[bool](page, fmt.Sprintf(`(() => {
		const input = document.querySelector('.todo-input');
		if (!input) throw new Error('todo input not found');
		const TagProto = input instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement;
		const setter = Object.getOwnPropertyDescriptor(TagProto.prototype, 'value')?.set;
		if (!setter) throw new Error('value setter not found');
		input.focus();
		setter.call(input, %s);
		input.dispatchEvent(new Event('input', { bubbles: true }));
		return true;
	})()`, strconv.Quote(text)))
}

// pressMetaEnter 在快速创建输入上以 Ctrl+Enter 提交（同 UI 行为）。
func pressMetaEnter(page *mygo.Page) error {
	_, err := page.Eval(`(() => {
		const input = document.querySelector('.todo-input');
		if (!input) throw new Error('todo input not found');
		input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true }));
		return true;
	})()`)
	if err == nil {
		time.Sleep(600 * time.Millisecond)
	}
	return err
}

// runLinkFlow 覆盖快捷入口主链路（PRD 验收：URL 添加→点击跳转→持久化→分组）。
// 说明：点击卡片会真实调用系统 open（打开一个浏览器标签），为用户可见行为的
// 真实验证；断言不依赖浏览器窗口，仅断言服务无错与列表状态。
func runLinkFlow(page *mygo.Page, url string, links *service.QuickLinkService) error {
	if err := loadPageDocument(page, uniquePageURL(url)+"&e2eView=link"); err != nil {
		return fmt.Errorf("link 流程: 加载快捷入口: %w", err)
	}
	if err := waitFor(page, `[...document.querySelectorAll('button')].some((item) => item.textContent?.trim() === '添加入口')`, 15*time.Second); err != nil {
		return fmt.Errorf("link 流程: 等待视图: %w", err)
	}

	// 1) 通过 UI 添加 URL 入口（InputDialog 输入 + 添加按钮）。
	if _, err := page.Eval(`(() => {
		const btn = [...document.querySelectorAll('button')].find((item) => item.textContent?.trim() === '添加入口');
		if (!btn) throw new Error('add button not found');
		btn.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("link 流程: 打开添加弹层: %w", err)
	}
		if err := setDialogFields(page); err != nil {
		return fmt.Errorf("link 流程: 填写弹层: %w", err)
	}
	if _, err := page.Eval(`(() => {
		const btn = [...document.querySelectorAll('.dialog button')].find((item) => item.textContent?.trim() === '添加');
		if (!btn) throw new Error('submit not found');
		btn.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("link 流程: 提交: %w", err)
	}
	if err := waitFor(page, `[...document.querySelectorAll('.link-name')].some((item) => item.textContent === 'GitHub Trap')`, 8*time.Second); err != nil {
		return fmt.Errorf("link 流程: 新入口未显示: %w", err)
	}

	// 2) 服务层断言持久化 + kind 判定。
	list, err := links.List(context.Background())
	if err != nil {
		return fmt.Errorf("link 流程: 读库: %w", err)
	}
	found := false
	for _, l := range list {
		if l.Name == "GitHub Trap" && l.Kind == store.KindURL && l.GroupName == "开发" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("link 流程: 持久化断言失败: %+v", list)
	}

	// 3) 点击卡片真实打开（open 调系统浏览器；断言点击无 JS 错误 + toast 无错误）。
	if _, err := page.Eval(`(() => {
		const card = [...document.querySelectorAll('.link-card-main')].find((item) => item.textContent?.includes('GitHub Trap'));
		if (!card) throw new Error('card not found');
		card.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("link 流程: 点击卡片: %w", err)
	}
	if err := waitFor(page, `document.querySelector('div.linkv2 [role="alert"]') === null`, 6*time.Second); err != nil {
		return fmt.Errorf("link 流程: 打开报错: %w", err)
	}

	// 4) 删除（二次确认）→ 列表消失 + 库内消失。
	if _, err := page.Eval(`(() => {
		const del = document.querySelector('button[aria-label="删除 GitHub Trap"]');
		if (!del) throw new Error('delete button not found');
		del.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("link 流程: 打开删除确认: %w", err)
	}
	if _, err := page.Eval(`(() => {
		const btn = [...document.querySelectorAll('[role="dialog"] button')].find((item) => item.textContent?.trim() === '删除');
		if (!btn) throw new Error('confirm delete not found');
		btn.click();
		return true;
	})()`); err != nil {
		return fmt.Errorf("link 流程: 确认删除: %w", err)
	}
	if err := waitFor(page, `[...document.querySelectorAll('.link-name')].every((item) => item.textContent !== 'GitHub Trap')`, 8*time.Second); err != nil {
		return fmt.Errorf("link 流程: 删除后仍在: %w", err)
	}
	return nil
}

// setDialogFields 在 InputDialog 中填写三个字段（名称/URL/分组）。
func setDialogFields(page *mygo.Page) error {
	_, err := page.Eval(fmt.Sprintf(`(() => {
		const setv = (label, value) => {
			const input = [...document.querySelectorAll('.dialog input')].find((el) => el.closest('.field')?.querySelector('label')?.textContent === label);
			if (!input) throw new Error('field not found: ' + label);
			const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
			setter.call(input, value);
			input.dispatchEvent(new Event('input', { bubbles: true }));
		};
		setv(" + strconv.Quote("名称") + ", " + strconv.Quote("GitHub Trap") + ");
		setv(" + strconv.Quote("URL 或本地路径") + ", " + strconv.Quote("https://github.com/891D3/trap") + ");
		setv(" + strconv.Quote("分组（可选）") + ", " + strconv.Quote("开发") + ");
		return true;
	})()`))
	return err
}
