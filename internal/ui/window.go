package ui

import (
	"github.com/egoist/mygo"

	mui "github.com/egoist/mygo/ui"
)

// WindowOptions 是打开窗口时的参数：尺寸、自绘标题栏和画这一页的视图。
func WindowOptions(v *UI) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:           "Ada Chat",
		Width:           1180,
		Height:          780,
		MinWidth:        920,
		MinHeight:       560,
		StateKey:        "ada-chat-main",
		TitleBarStyle:   mygo.TitleBarHidden,
		TitleBarHeight:  TitleBarHeight,
		BackgroundColor: "light-dark(#f8f8f8, #0b0b0c)",
		Content:         mui.View(v.Root),
	}
}
