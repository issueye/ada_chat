// Package ui 画出窗口：标题栏、侧栏、对话、输入框和对话框。它只读 app 的状态。
package ui

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/form"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

// TitleBarHeight 是窗口自绘标题栏的高度，与 cmd 里的窗口参数共用。
const TitleBarHeight = 44

// UI 是视图自己的状态：面板开合、搜索词、当前展开的菜单、表单和滚动位置。
type UI struct {
	App *app.App

	sidebarWidth float32
	sidebarOpen  bool

	query string

	chatMenu bool

	settings    *form.Form[app.Prefs]
	settingsUp  bool
	settingsTab int
	showKey     bool
	renameUp    bool
	renameDraft string
	confirmDel  bool
	approvalUp  bool

	botDraft  app.Bot
	botEditID int
	botErr    string
	botDirErr string
	botUp     bool

	dirPicking bool
	pickDir    func() (string, error)

	pathPicking string
	pickFile    func(kind string) (string, error)
	dispatch    func(func())

	chatList mui.ListState
	activeID int
	// pending 是列表行发起的改动，等列表建完再执行。
	pending func()
}

// New 把视图接到一份应用状态上。
func New(a *app.App) *UI {
	return &UI{
		App:          a,
		sidebarWidth: sidebarDefault,
		sidebarOpen:  true,
		settings:     form.New(a.Prefs),
		botDraft:     app.Bot{Workspace: a.Workspace},
		activeID:     a.ActiveID,
	}
}

// SetDispatch 让目录选择这类异步 UI 改动回到 UI 线程。
func (u *UI) SetDispatch(fn func(func())) { u.dispatch = fn }

// Root 是窗口的内容：每帧由 MyGo 调用，按状态重建。
func (u *UI) Root(c *mui.Context) {
	theme.Apply(c)
	tk := theme.TokensOf(c)
	page := mui.Column(c).Fill()
	if !c.Vibrancy() {
		page.Background(tk.BgLayout)
	}
	page.Children(func() {
		u.titleBar(c)
		mui.Row(c).Grow(1).Basis(0).MinHeight(0).AlignItems(mui.Stretch).Children(func() {
			u.sidebar(c)
			u.chatPanel(c)
		})
		u.settingsDialog(c)
		u.renameDialog(c)
		u.deleteDialog(c)
		u.botDialog(c)
		u.approvalDialog(c)
		feedback.ToastHost(c, feedback.ToastHostProps{})
	})
}

// resetListIfSwitched 让列表在切换会话后从新的一头开始，不带着上一条对话的滚动位置。
func (u *UI) resetListIfSwitched() {
	if u.activeID == u.App.ActiveID {
		return
	}
	u.activeID = u.App.ActiveID
	u.chatList = mui.ListState{}
}
