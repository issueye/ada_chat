// Package app 持有聊天应用的状态与行为：会话、消息、偏好和回复流程。视图只读它。
package app

import (
	"strconv"
	"strings"
	"time"
)

// Role 区分对话里的两种身份。
type Role int

const (
	RoleUser Role = iota
	RoleAssistant
)

// Message 是一轮消息，可以带着先跑完的工具调用。
type Message struct {
	ID        int
	Role      Role
	Text      string
	At        time.Time
	Streaming bool
	Tools     []*ToolCall
}

// Session 是一条对话。
type Session struct {
	ID       int
	Title    string
	Preview  string
	Updated  time.Time
	Unread   int
	Pinned   bool
	Messages []*Message
}

// Prefs 是用户在设置里保存的偏好。
type Prefs struct {
	Name        string
	Model       string
	Streaming   bool
	EnterToSend bool
}

// Models 是模型菜单里的选项。
var Models = []string{"Ada Pro", "Ada Flash", "Ada Local"}

// App 是窗口显示的唯一一份状态。事件处理改它，视图每帧读它。
type App struct {
	Sessions  []*Session
	ActiveID  int
	Draft     string
	Prefs     Prefs
	Workspace string

	// dispatch 把改状态并重绘的闭包送到 UI 线程；为空时在当前帧同步执行，测试因此是确定的。
	dispatch func(func())
	nextID   int
}

// New 建出打开时看到的那些对话。
func New(now time.Time) *App {
	a := &App{
		Workspace: "Ada 工作区",
		Prefs:     Prefs{Name: "小杉", Model: "Ada Pro", Streaming: true, EnterToSend: true},
	}
	a.seed(now)
	return a
}

// SetDispatch 让应用在运行中的窗口里异步写回状态；不设时一切同步执行。
func (a *App) SetDispatch(fn func(func())) { a.dispatch = fn }

func (a *App) apply(fn func()) {
	if a.dispatch != nil {
		a.dispatch(fn)
		return
	}
	fn()
}

func (a *App) id() int {
	a.nextID++
	return a.nextID
}

func (a *App) now() time.Time { return time.Now() }

func (a *App) msg(r Role, at time.Time, text string) *Message {
	return &Message{ID: a.id(), Role: r, Text: text, At: at}
}

// Current 返回窗口当前显示的会话。窗口总会有会话可显示。
func (a *App) Current() *Session {
	for _, s := range a.Sessions {
		if s.ID == a.ActiveID {
			return s
		}
	}
	if len(a.Sessions) > 0 {
		a.ActiveID = a.Sessions[0].ID
		return a.Sessions[0]
	}
	return nil
}

// SelectSession 打开一条对话，并清掉它的未读标记。
func (a *App) SelectSession(id int) {
	if id == a.ActiveID {
		return
	}
	a.ActiveID = id
	for _, s := range a.Sessions {
		if s.ID == id {
			s.Unread = 0
		}
	}
}

// NewChat 在最前面放一条空对话并选中它。
func (a *App) NewChat() {
	s := &Session{ID: a.id(), Title: "新对话", Updated: a.now()}
	a.Sessions = append([]*Session{s}, a.Sessions...)
	a.ActiveID = s.ID
}

// DeleteSession 移除一条对话，返回它原来的位置；位置留给撤销用。
func (a *App) DeleteSession(id int) (int, bool) {
	for i, s := range a.Sessions {
		if s.ID != id {
			continue
		}
		a.Sessions = append(a.Sessions[:i], a.Sessions[i+1:]...)
		if len(a.Sessions) == 0 {
			a.NewChat()
		}
		if a.ActiveID == id {
			a.ActiveID = a.Sessions[min(i, len(a.Sessions)-1)].ID
		}
		return i, true
	}
	return 0, false
}

// RestoreSession 把撤销回来的对话放回原位。
func (a *App) RestoreSession(s *Session, at int) {
	if at < 0 || at > len(a.Sessions) {
		at = 0
	}
	a.Sessions = append(a.Sessions, &Session{})
	copy(a.Sessions[at+1:], a.Sessions[at:])
	a.Sessions[at] = s
	a.ActiveID = s.ID
}

// Send 把草稿送进对话，并开始生成回复。
func (a *App) Send() {
	s := a.Current()
	if s == nil {
		return
	}
	text := strings.TrimSpace(a.Draft)
	if text == "" {
		return
	}
	if len(s.Messages) == 0 {
		s.Title = Clip(text, 48)
	}
	s.Messages = append(s.Messages, &Message{ID: a.id(), Role: RoleUser, Text: text, At: a.now()})
	s.Preview = Clip(text, 88)
	s.Updated = a.now()
	s.Unread = 0
	a.Draft = ""
	a.reply(s, text)
}

// reply 追加助手的一轮回答：先调用工具，再流式写出文字；在运行中的窗口里它离开 UI 线程执行。
func (a *App) reply(s *Session, prompt string) {
	id := a.id()
	m := &Message{ID: id, Role: RoleAssistant, At: a.now(), Streaming: true}
	plan := toolPlan(prompt)
	for i := range plan {
		m.Tools = append(m.Tools, &ToolCall{
			ID: a.id(), Key: plan[i].Key, Name: ToolName(plan[i].Key),
			Args: plan[i].Args, State: ToolRunning, Open: true,
		})
	}
	s.Messages = append(s.Messages, m)
	text := ReplyText(prompt)
	if !a.Prefs.Streaming || a.dispatch == nil {
		a.apply(func() {
			finishTools(m, plan)
			finishReply(s, id, text)
		})
		return
	}
	chunks := chunksOf(text, 18)
	go func() {
		for i, use := range plan {
			time.Sleep(use.Delay)
			a.apply(func() { completeTool(m, m.Tools[i].ID, use.Result, use.Took) })
		}
		for _, chunk := range chunks {
			time.Sleep(6 * time.Millisecond)
			a.apply(func() { appendChunk(s, id, chunk) })
		}
		a.apply(func() { finishReply(s, id, "") })
	}()
}

func appendChunk(s *Session, id int, chunk string) {
	for _, m := range s.Messages {
		if m.ID == id && m.Streaming {
			m.Text += chunk
			s.Preview = Clip(m.Text, 88)
			return
		}
	}
}

func finishReply(s *Session, id int, text string) {
	for _, m := range s.Messages {
		if m.ID == id && m.Streaming {
			if text != "" {
				m.Text = text
			}
			m.Streaming = false
			s.Preview = Clip(m.Text, 88)
			return
		}
	}
}

// Regenerate 丢掉最后一条回复，按同一个问题重跑工具与回答。
func (a *App) Regenerate(s *Session) {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role != RoleUser {
			continue
		}
		prompt := s.Messages[i].Text
		s.Messages = s.Messages[:i+1]
		a.reply(s, prompt)
		return
	}
}

// DeleteMessage 移除一条消息。
func (a *App) DeleteMessage(s *Session, id int) {
	for i, m := range s.Messages {
		if m.ID != id {
			continue
		}
		s.Messages = append(s.Messages[:i], s.Messages[i+1:]...)
		return
	}
}

// Rename 给一条对话换名字。
func (a *App) Rename(id int, title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	for _, s := range a.Sessions {
		if s.ID == id {
			s.Title = Clip(title, 60)
			return
		}
	}
}

// SetModel 选一个模型。
func (a *App) SetModel(model string) { a.Prefs.Model = model }

// Clip 把文字裁到控件合适的长度：单行，末尾省略号。
func Clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// RelativeTime 是列表行和消息旁显示的短时间。
func RelativeTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "分钟"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "小时"
	case d < 7*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "天"
	}
	return strconv.Itoa(int(t.Month())) + "月" + strconv.Itoa(t.Day()) + "日"
}
