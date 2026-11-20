package main

import (
	"log"
	"time"

	"github.com/egoist/mygo"

	"ada_chat/internal/app"
	"ada_chat/internal/ui"
)

func main() {
	mygo.App.SetLocale("zh-Hans")

	state := app.New(time.Now())
	view := ui.New(state)

	var win *mygo.Window
	state.SetDispatch(func(fn func()) {
		if win != nil {
			win.Update(fn)
			return
		}
		fn()
	})

	mygo.App.WhenReady(func() {
		win = mygo.NewWindow(ui.WindowOptions(view))
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
