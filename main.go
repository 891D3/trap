// Trap — 个人工作台（mygo 应用本体）。
//
// 启动流程（PRD §2/T1）：
//  1. Bind 服务（typed IPC：AppService；Todo/Note/QuickLink 随 T4–T6 注册）；
//  2. App.WhenReady 中打开 SQLite 并执行迁移（必须晚于 ready：mygo generate
//     会以 MYGO_GENERATE 运行 main 到 App.Run 即退出，过早迁移会在生成期跑库）；
//  3. 打开主窗口。
package main

import (
	"context"
	"log"
	"path/filepath"

	"github.com/891D3/trap/internal/onboarding"
	"github.com/891D3/trap/internal/service"
	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

func main() {
	app := &service.AppService{}
	todos := &service.TodoService{}
	notes := &service.NoteService{}
	projects := &service.ProjectService{} // v2.0：项目实体（待办关联/侧栏列表）
	links := &service.QuickLinkService{}  // v2.0 R6：快捷入口（CRUD/分组/打开）
	news := &service.NewsService{}        // 热点资讯（微博等渠道，定时+手动刷新）
	mygo.Bind(app, todos, notes, projects, links, news)

	mygo.App.WhenReady(func() {
		// 数据目录策略（Onboarding D1）：统一 ~/.trap；存量 userData 自动迁移
		//（失败回退旧路径，不阻塞启动）。
		ob, err := onboarding.Resolve(context.Background())
		if err != nil {
			log.Printf("数据目录解析警告: %v", err)
		}
		if ob.Migrated {
			log.Printf("数据已迁移至 %s（旧库备份为 %s.bak-migrated）", ob.Dir, ob.LegacyPath)
		} else if ob.Fallback {
			log.Printf("数据目录回退为 %s（迁移失败，请检查权限）", ob.Dir)
		}
		userDir := ob.Dir

		db, err := store.Open(context.Background(), filepath.Join(userDir, "trap.db"))
		if err != nil {
			log.Fatalf("打开数据库失败: %v", err)
		}
		service.AttachStore(app, db, userDir)
		service.AttachTodoStore(todos, db)
		service.AttachNoteStore(notes, db)
		service.AttachProjectStore(projects, db)
		service.AttachQuickLinkStore(links, db)
		service.AttachTodoPeer(notes, todos)
		// 热点资讯定时刷新（启动即预热一次，之后每 10 分钟后台拉取；
		// goroutine 用可取消 ctx，符合 AGENTS.md goroutine 退出约束）。
		newsCtx, newsCancel := context.WithCancel(context.Background())
		defer newsCancel()
		news.StartNewsScheduler(newsCtx)

		// M0.5：灵感自动归档清扫（30 天自动归档/90 天清除），启动时点；
		// 有变动时广播 note.changed，前端刷新灵感流。
		if _, err := notes.SweepSparks(context.Background()); err != nil {
			log.Printf("灵感清扫失败（不阻塞启动）: %v", err)
		}

		// Onboarding D2 存量兜底：旧安装（有 user.name 但无 initialized 标记）
		// 静默补标记，避免升级后误显示初始化页。
		if legacyFillErr := service.BackfillInitialized(context.Background(), db); legacyFillErr != nil {
			log.Printf("初始化标记兜底失败（不阻塞启动）: %v", legacyFillErr)
		}

		mygo.NewWindow(mygo.WindowOptions{
			Title:           "Trap",
			Width:           1280,
			Height:          800,
			MinWidth:        1024,
			MinHeight:       640,
			BackgroundColor: "light-dark(#f6f7f9, #0f1115)", // 对应 web/src/style.css 的 --bg
			// 记住上次窗口位置/大小。
			StateKey: "main",
			// 前端：dev 期间用 devUrl，打包后用嵌入的 web/dist（见 mygo.config.ts）。
			URL: "/",
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
