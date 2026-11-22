package main

import (
	"log"

	"github.com/egoist/mygo"

	"ada_chat/internal/app"
	"ada_chat/internal/bot"
	"ada_chat/internal/ui"
)

func main() {
	mygo.App.SetLocale("zh-Hans")

	state := app.New()
	supervisor, err := bot.NewSupervisor(state)
	if err != nil {
		log.Printf("机器人进程管理不可用: %v", err)
	} else {
		state.SetEngine(supervisor)
	}
	view := ui.New(state)

	var win *mygo.Window
	dispatch := func(fn func()) {
		if win != nil {
			win.Update(fn)
			return
		}
		fn()
	}
	state.SetDispatch(dispatch)
	view.SetDispatch(dispatch)

	mygo.App.WhenReady(func() {
		win = mygo.NewWindow(ui.WindowOptions(view))
	})
	runErr := mygo.App.Run()
	if supervisor != nil {
		_ = supervisor.Close()
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
}
