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

	"github.com/891D3/trap/internal/service"
	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

func main() {
	app := &service.AppService{}
	todos := &service.TodoService{}
	notes := &service.NoteService{}
	projects := &service.ProjectService{} // v2.0：项目实体（待办关联/侧栏列表）
	mygo.Bind(app, todos, notes, projects)

	mygo.App.WhenReady(func() {
		// 数据库位置：userData 目录下的 trap.db（mygo.PathUserData 首次使用会创建目录）。
		userDir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Fatalf("解析用户数据目录失败: %v", err)
		}
		db, err := store.Open(context.Background(), filepath.Join(userDir, "trap.db"))
		if err != nil {
			log.Fatalf("打开数据库失败: %v", err)
		}
		service.AttachStore(app, db, userDir)
		service.AttachTodoStore(todos, db)
		service.AttachNoteStore(notes, db)
		service.AttachProjectStore(projects, db)

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
