package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	app := NewApp()
	app.applyTheme()

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "LocalSever · 局域网文件传输",
			Width:           1024,
			Height:          768,
			MinWidth:        880,
			MinHeight:       620,
			BackgroundColor: "light-dark(#f6f7f9, #101820)",
			// 记住用户上次摆放窗口的位置。
			StateKey: "main",
			// 原生 UI：面板由 MyGo 自己绘制，不加载任何网页。
			Content: ui.View(app.view),
		})
		app.win = win

		if app.GetStatus().AutoStart {
			go func() {
				if _, err := app.Start(); err != nil {
					log.Printf("localserver: 自动启动共享失败: %v", err)
					return
				}
				win.Invalidate()
			}()
		}
		go app.watchStatus(win)
	})

	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { app.shutdown() })

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
