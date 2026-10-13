package main

import (
	"log"

	"github.com/891D3/trap/internal/service"
	"github.com/egoist/mygo"
)

func main() {
	mygo.Bind(service.AppService{})

	mygo.App.WhenReady(func() {
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