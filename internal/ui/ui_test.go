package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"

	"ada_chat/internal/app"
)

func newTester(t *testing.T, width, height int) (*mui.Tester, *UI) {
	t.Helper()
	state := app.New(time.Now())
	view := New(state)
	tt := mui.NewTester(view.Root, width, height)
	tt.SetLocale("zh-Hans")
	settle(tt)
	return tt, view
}

func settle(tt *mui.Tester) {
	for range 3 {
		tt.Advance(time.Second)
		tt.Frame()
	}
}

func press(t *testing.T, tt *mui.Tester, role mui.Role, name string) {
	t.Helper()
	info, ok := tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("no %q (%v); texts %q", name, role, tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	settle(tt)
}

// pressLowest 点同一名字里最靠下的那一个，用于每条消息都带同名操作的情况。
func pressLowest(t *testing.T, tt *mui.Tester, role mui.Role, name string) {
	t.Helper()
	var bounds mui.Rect
	found := false
	for _, a := range tt.AccessAll() {
		if a.Role != role || a.Name != name {
			continue
		}
		if !found || a.Bounds.Y > bounds.Y {
			bounds, found = a.Bounds, true
		}
	}
	if !found {
		t.Fatalf("no %q (%v); texts %q", name, role, tt.Texts())
	}
	tt.ClickAt(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
	settle(tt)
}

func TestWindowShowsTheFirstConversation(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	if _, ok := tt.AccessByRole(mui.RoleButton, "Windows 上的原生聊天"); !ok {
		t.Errorf("texts %q, want the first conversation listed", tt.Texts())
	}
	if view.App.ActiveID != view.App.Sessions[0].ID {
		t.Errorf("active %d, want %d", view.App.ActiveID, view.App.Sessions[0].ID)
	}
	if !tt.HasText("Ada Chat") || !tt.HasText("置顶") || !tt.HasText("今天") {
		t.Errorf("texts %q, want the shell with its sections", tt.Texts())
	}
	if _, ok := tt.AccessByRole(mui.RoleTextField, "给 Ada 发消息"); !ok {
		t.Error("no composer")
	}
}

func TestTypeEnterSendsAndTheReplyArrives(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	focus(t, tt, "给 Ada 发消息")
	tt.Type("hello there")
	settle(tt)
	press(t, tt, mui.RoleButton, "发送")

	s := view.App.Current()
	if s == nil || len(s.Messages) != 6 {
		t.Fatalf("messages %d, want the two seeded turns plus the new pair", len(s.Messages))
	}
	if got := s.Messages[4].Text; got != "hello there" {
		t.Errorf("sent %q", got)
	}
	if s.Messages[5].Streaming || s.Messages[5].Text == "" {
		t.Errorf("reply %+v, want it finished", s.Messages[5])
	}
	if view.App.Draft != "" {
		t.Errorf("draft %q, want it cleared", view.App.Draft)
	}
	if !tt.HasText("hello there") || !tt.HasText("6 条消息") {
		t.Errorf("texts %q, want the sent turn and the new count", tt.Texts())
	}
}

func TestEnterSendsFromTheComposer(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	focus(t, tt, "给 Ada 发消息")
	tt.Type("a quick note")
	settle(tt)
	tt.Key(0, mui.KeyEnter)
	settle(tt)
	if s := view.App.Current(); len(s.Messages) != 6 {
		t.Fatalf("messages %d, want the pair from Enter", len(s.Messages))
	}
	if !tt.HasText("a quick note") {
		t.Errorf("texts %q, want the sent turn", tt.Texts())
	}
}

func TestSendIsDisabledWhileTheDraftIsEmpty(t *testing.T) {
	tt, _ := newTester(t, 1180, 780)
	if b, ok := tt.AccessByRole(mui.RoleButton, "发送"); !ok || !b.Disabled {
		t.Errorf("send %+v (%v), want it disabled with an empty draft", b, ok)
	}
	focus(t, tt, "给 Ada 发消息")
	tt.Type("x")
	settle(tt)
	if b, _ := tt.AccessByRole(mui.RoleButton, "发送"); b.Disabled {
		t.Error("send still disabled with text in the draft")
	}
}

func TestNewChatShowsTheEmptyState(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "新建对话")
	if !tt.HasText("今天想做点什么？") || !tt.HasText("搭起界面骨架") {
		t.Errorf("texts %q, want the empty state and its suggestions", tt.Texts())
	}
	if s := view.App.Current(); len(s.Messages) != 0 || s.Title != "新对话" {
		t.Errorf("conversation %+v, want a new empty one", s)
	}
}

func TestSuggestionFillsTheDraft(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "新建对话")
	press(t, tt, mui.RoleButton, "搭起界面骨架")
	if !strings.Contains(view.App.Draft, "帮我搭出") {
		t.Errorf("draft %q, want the suggestion in it", view.App.Draft)
	}
}

func TestSearchFiltersTheConversationList(t *testing.T) {
	tt, _ := newTester(t, 1180, 780)
	mods, key, ok := display.HotkeyBinding("mod+k", display.HotkeyAuto)
	if !ok {
		t.Fatal("mod+k does not parse")
	}
	tt.Key(mods, key)
	settle(tt)
	if !tt.Focused("搜索对话") {
		t.Fatalf("search is not focused; texts %q", tt.Texts())
	}
	tt.Type("设计")
	settle(tt)
	if !tt.HasText("搜索结果") {
		t.Fatalf("texts %q, want the search results", tt.Texts())
	}
	if tt.HasText("键盘快捷键") || !tt.HasText("Ada 的设计变量") {
		t.Errorf("texts %q, want only the matching conversation", tt.Texts())
	}
}

func TestSelectingAConversationShowsItsTurns(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "Ada 的设计变量")
	if !tt.HasText("浅色和深色都要耐看，界面该用哪些颜色？") {
		t.Errorf("texts %q, want that conversation's prompt", tt.Texts())
	}
	if view.App.Current().Title != "Ada 的设计变量" {
		t.Errorf("current %q", view.App.Current().Title)
	}
}

func TestTheModelMenuChangesTheModel(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleMenuButton, "模型")
	press(t, tt, mui.RoleMenuItem, "Ada Flash")
	if view.App.Prefs.Model != "Ada Flash" {
		t.Fatalf("model %q", view.App.Prefs.Model)
	}
	if !tt.HasText("Ada Flash") {
		t.Errorf("texts %q, want the chip and header on the new model", tt.Texts())
	}
}

func TestSettingsKeepsTheNewName(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "设置")
	if !tt.HasText("显示名称") {
		t.Fatalf("texts %q, want the settings form", tt.Texts())
	}
	view.settings.Values.Name = "Bo"
	settle(tt)
	press(t, tt, mui.RoleButton, "保存")
	if view.App.Prefs.Name != "Bo" {
		t.Errorf("name %q, want Bo", view.App.Prefs.Name)
	}
	if !tt.HasText("设置已保存") {
		t.Errorf("texts %q, want the save toast", tt.Texts())
	}
	if view.settingsUp {
		t.Error("the modal is still open after Save")
	}
}

func TestSwitchesChangeTheWritingPreferences(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "设置")
	press(t, tt, mui.RoleSwitch, "Enter 发送")
	if view.settings.Values.EnterToSend {
		t.Fatal("the switch did not change the setting")
	}
	press(t, tt, mui.RoleButton, "保存")
	if view.App.Prefs.EnterToSend {
		t.Error("the preference was not saved")
	}
}

func TestDeleteAsksThenOffersUndo(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleMenuButton, "对话操作")
	press(t, tt, mui.RoleMenuItem, "删除对话")
	if !tt.HasText("这条对话的消息会从这个窗口移除。") {
		t.Fatalf("texts %q, want the confirmation", tt.Texts())
	}
	was := len(view.App.Sessions)
	press(t, tt, mui.RoleButton, "删除")
	if len(view.App.Sessions) != was-1 {
		t.Errorf("%d conversations, want one fewer", len(view.App.Sessions))
	}
	if !tt.HasText("对话已删除") {
		t.Errorf("texts %q, want the deletion toast", tt.Texts())
	}
	press(t, tt, mui.RoleButton, "撤销")
	if len(view.App.Sessions) != was {
		t.Errorf("%d conversations, want the deleted one back", len(view.App.Sessions))
	}
}

func TestRegenerateReplacesTheLastReply(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	before := len(view.App.Current().Messages)
	press(t, tt, mui.RoleButton, "重新生成")
	after := view.App.Current().Messages
	if len(after) != before {
		t.Errorf("%d messages, want %d with the last reply regenerated", len(after), before)
	}
	final := after[len(after)-1]
	if final.Role != app.RoleAssistant || final.Streaming || final.Text == "" {
		t.Errorf("last turn %+v, want a finished reply", final)
	}
}

func TestCopyPutsTheMessageOnTheClipboard(t *testing.T) {
	tt, _ := newTester(t, 1180, 780)
	pressLowest(t, tt, mui.RoleButton, "复制回复")
	if !strings.Contains(tt.Clipboard(), "有。界面骨架用的是 Lobe 的布局组件") {
		t.Errorf("clipboard %q, want the reply", tt.Clipboard())
	}
	if !tt.HasText("已复制到剪贴板") {
		t.Errorf("texts %q, want the copy toast", tt.Texts())
	}
}

func TestSidebarToggleHidesTheList(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "收起侧栏")
	if view.sidebarOpen {
		t.Fatal("the sidebar is still open")
	}
	if b, ok := tt.AccessByRole(mui.RoleButton, "键盘快捷键"); ok && b.Bounds.X+b.Bounds.W > 0 {
		t.Errorf("row at %+v, want it out of view", b.Bounds)
	}
	press(t, tt, mui.RoleButton, "展开侧栏")
	if !view.sidebarOpen {
		t.Error("the sidebar did not come back")
	}
}

func TestNewChatShortcutOpensAConversation(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	mods, key, ok := display.HotkeyBinding("mod+n", display.HotkeyAuto)
	if !ok {
		t.Fatal("mod+n does not parse")
	}
	tt.Key(mods, key)
	settle(tt)
	if view.App.Current().Title != "新对话" {
		t.Errorf("current %q, want a new conversation", view.App.Current().Title)
	}
	if !tt.HasText("今天想做点什么？") {
		t.Errorf("texts %q, want the empty state", tt.Texts())
	}
}

func TestTheMinimumWindowKeepsTheComposerInView(t *testing.T) {
	tt, _ := newTester(t, 920, 560)
	if r, ok := tt.Find("发送"); !ok || r.Y+r.H > 560-8 {
		t.Errorf("send at %+v, want it inside the minimum window", r)
	}
	if b, ok := tt.AccessByRole(mui.RoleTextField, "给 Ada 发消息"); !ok || b.Bounds.H < 20 {
		t.Errorf("composer %+v (%v), want a usable field", b, ok)
	}
}

func TestAnIdleWindowSettles(t *testing.T) {
	tt, _ := newTester(t, 1180, 780)
	if tt.FramePending() {
		t.Error("the idle window keeps asking for frames")
	}
}

func TestDarkModeFollowsTheSystem(t *testing.T) {
	tt, _ := newTester(t, 1180, 780)
	tt.SetDark(true)
	settle(tt)
	img := tt.Image()
	if px := img.RGBAAt(10, 400); px.R > 60 || px.G > 60 || px.B > 60 {
		t.Errorf("sidebar background %+v, want the dark layout color", px)
	}
	if px := img.RGBAAt(600, 700); px.R > 60 || px.G > 60 || px.B > 60 {
		t.Errorf("card background %+v, want the dark container color", px)
	}
}

func TestWindowKeepsTheTitleBarAndItsControls(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	o := WindowOptions(view)
	if o.TitleBarStyle != mygo.TitleBarHidden || o.TitleBarHeight != TitleBarHeight {
		t.Fatalf("title bar %q height %d, want a hidden one of %d", o.TitleBarStyle, o.TitleBarHeight, TitleBarHeight)
	}
	tt.SetTitleBar(mui.TitleBar{Height: TitleBarHeight, Right: 138})
	settle(tt)
	b, ok := tt.AccessByRole(mui.RoleButton, "设置")
	if !ok || b.Bounds.X+b.Bounds.W > 1180-130 {
		t.Errorf("settings at %+v, want it left of the window buttons", b.Bounds)
	}
	if r, ok := tt.Find("Ada Chat"); !ok || r.Y > TitleBarHeight {
		t.Errorf("brand at %+v, want it in the title bar", r)
	}
}

func TestTitleBarClickClearsItsContentOnMinWidth(t *testing.T) {
	tt, _ := newTester(t, 920, 560)
	header, ok := tt.AccessByRole(mui.RoleButton, "收起侧栏")
	if !ok {
		t.Fatal("no sidebar toggle")
	}
	composer, _ := tt.AccessByRole(mui.RoleTextField, "给 Ada 发消息")
	if header.Bounds.X > composer.Bounds.X {
		t.Errorf("toggle at %+v, composer at %+v, want the toggle left of the card", header.Bounds, composer.Bounds)
	}
}

func TestReplyRunsAToolBeforeAnswering(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	focus(t, tt, "给 Ada 发消息")
	tt.Type("你好")
	settle(tt)
	press(t, tt, mui.RoleButton, "发送")

	last := view.App.Current().Messages[len(view.App.Current().Messages)-1]
	if len(last.Tools) != 1 {
		t.Fatalf("%d tool calls, want one", len(last.Tools))
	}
	call := last.Tools[0]
	if call.State != app.ToolDone || call.Result == "" || call.Took <= 0 {
		t.Errorf("tool %+v, want it finished with a result and duration", call)
	}
	if !tt.HasText("检索上下文") || !tt.HasText("已完成") || !tt.HasText("找到 4 条相关消息") {
		t.Errorf("texts %q, want the tool card with its result", tt.Texts())
	}
}

func TestToolCardOpensAndCloses(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	last := view.App.Current().Messages[len(view.App.Current().Messages)-1]
	if len(last.Tools) != 1 {
		t.Fatalf("%d tool calls, want the seeded one", len(last.Tools))
	}
	docs := last.Tools[0]
	if !docs.Open || !tt.HasText("找到 3 条文档") {
		t.Fatalf("tool %q open %v, want its result shown", docs.Name, docs.Open)
	}
	press(t, tt, mui.RoleDisclosure, "搜索文档")
	if docs.Open || tt.HasText("找到 3 条文档") {
		t.Error("the card is still open after its header was clicked")
	}
	press(t, tt, mui.RoleDisclosure, "搜索文档")
	if !docs.Open || !tt.HasText("找到 3 条文档") {
		t.Error("the card did not open again")
	}
}

func TestAFailedToolShowsItsReason(t *testing.T) {
	tt, view := newTester(t, 1180, 780)
	press(t, tt, mui.RoleButton, "周五前清完评审队列")
	last := view.App.Current().Messages[len(view.App.Current().Messages)-1]
	if len(last.Tools) != 1 || last.Tools[0].State != app.ToolFailed {
		t.Fatalf("tools %+v, want one failed call", last.Tools)
	}
	if !tt.HasText("失败") || !tt.HasText("工具没有跑完") || !tt.HasText("拉取失败：这个工作区没有配置远程仓库") {
		t.Errorf("texts %q, want the failed tool with its reason", tt.Texts())
	}
}

func focus(t *testing.T, tt *mui.Tester, name string) {
	t.Helper()
	info, ok := tt.AccessByRole(mui.RoleTextField, name)
	if !ok {
		t.Fatalf("no text field %q; texts %q", name, tt.Texts())
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	settle(tt)
	if !tt.Focused(name) {
		t.Fatalf("%q is not focused", name)
	}
}
