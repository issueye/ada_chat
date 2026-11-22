// Package app 持有聊天应用的状态与行为：会话、消息、偏好和回复流程。视图只读它。
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// Session 是一条对话。BotID 非零时，回答来自那个机器人的进程实例。
type Session struct {
	ID       int
	Title    string
	Preview  string
	Updated  time.Time
	Unread   int
	Pinned   bool
	BotID    int
	Messages []*Message
}

// Prefs 是用户在设置里保存的偏好。它同时写进本机配置文件。
type Prefs struct {
	Name        string `json:"name,omitempty"`
	EnterToSend bool   `json:"enterToSend"`
	// RuntimePath 是 ada_go_v2 的 RPC bot 可执行文件（ada-rpc-agent）路径。
	RuntimePath string `json:"runtimePath,omitempty"`
	// AgentPreset 是传给 runtime 的 agent preset 文件路径。
	AgentPreset string `json:"agentPreset,omitempty"`
	// Provider 是模型供应商覆盖："openai"/"replay"；空表示跟随 preset。
	Provider string `json:"provider,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Model    string `json:"model,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
}

// BotState 是一个机器人进程实例的连接状态。
type BotState int

const (
	BotStopped BotState = iota
	BotStarting
	BotRunning
	BotError
)

// Bot 是一个机器人。它对应一个进程实例：Workspace 是这个进程的工作目录。
type Bot struct {
	ID          int
	Name        string
	Description string
	Instruction string
	Model       string
	Workspace   string
	State       BotState
	PID         int
	Err         string
	// Icon 是 ui 侧图标表里的键，Color 是 #rrggbb。
	Icon  string
	Color string
}

// BotEngine 管理机器人进程与 RPC 连接。它把状态回调到 App 的 Bot 方法。
type BotEngine interface {
	Start(b *Bot) error
	Stop(id int) error
	Send(b *Bot, turn int, text string) error
	Close() error
}

// App 是窗口显示的唯一一份状态。事件处理改它，视图每帧读它。
type App struct {
	Sessions []*Session
	ActiveID int
	Draft    string
	Prefs    Prefs
	Bots     []*Bot
	// Workspace 是最近一个机器人使用的目录，添加下一个机器人时作为默认值。
	Workspace string
	Engine    BotEngine

	// dispatch 把改状态并重绘的闭包送到 UI 线程；为空时在当前帧同步执行，测试因此是确定的。
	dispatch func(func())
	nextID   int
}

// New 建出一个空的工作台；对话和机器人来自用户添加的机器人。
// 偏好从本机配置文件读取；没有保存过时，Agent 设置回落到环境变量。
func New() *App {
	p := Prefs{
		EnterToSend: true,
		RuntimePath: strings.TrimSpace(os.Getenv("ADA_BOT_RUNTIME")),
		AgentPreset: strings.TrimSpace(os.Getenv("ADA_GO_AGENT_PRESET")),
		Provider:    strings.TrimSpace(os.Getenv("ADA_LLM_PROVIDER")),
		Endpoint:    strings.TrimSpace(os.Getenv("ADA_LLM_ENDPOINT")),
		Model:       strings.TrimSpace(os.Getenv("ADA_LLM_MODEL")),
	}
	if path, err := configFile(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &p)
		}
	}
	// 环境变量只填充没有保存过的项；保存过的设置优先。
	if p.RuntimePath == "" {
		p.RuntimePath = strings.TrimSpace(os.Getenv("ADA_BOT_RUNTIME"))
	}
	if p.AgentPreset == "" {
		p.AgentPreset = strings.TrimSpace(os.Getenv("ADA_GO_AGENT_PRESET"))
	}
	if p.Provider == "" {
		p.Provider = strings.TrimSpace(os.Getenv("ADA_LLM_PROVIDER"))
	}
	if p.Endpoint == "" {
		p.Endpoint = strings.TrimSpace(os.Getenv("ADA_LLM_ENDPOINT"))
	}
	if p.Model == "" {
		p.Model = strings.TrimSpace(os.Getenv("ADA_LLM_MODEL"))
	}
	return &App{Prefs: p}
}

// ConfigFile 返回偏好文件的位置；目录不存在时也能返回路径。
func ConfigFile() (string, error) { return configFile() }

func configFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ada_chat", "config.json"), nil
}

// SavePrefs 把当前偏好原子写入配置文件。
func (a *App) SavePrefs() error {
	path, err := configFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.Prefs, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetDispatch 让应用在运行中的窗口里异步写回状态；不设时一切同步执行。
func (a *App) SetDispatch(fn func(func())) { a.dispatch = fn }

// UserName 是显示用户自己的名字；未设置时用「我」。
func (a *App) UserName() string {
	if name := strings.TrimSpace(a.Prefs.Name); name != "" {
		return name
	}
	return "我"
}

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

// DeleteSession 移除一条对话，返回它原来的位置；位置留给撤销用。
func (a *App) DeleteSession(id int) (int, bool) {
	for i, s := range a.Sessions {
		if s.ID != id {
			continue
		}
		a.Sessions = append(a.Sessions[:i], a.Sessions[i+1:]...)
		if a.ActiveID == id {
			if len(a.Sessions) == 0 {
				a.ActiveID = 0
			} else {
				a.ActiveID = a.Sessions[min(i, len(a.Sessions)-1)].ID
			}
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

// Send 把草稿送进对话，并交给它绑定的机器人。
func (a *App) Send() {
	s := a.Current()
	if s == nil || s.BotID == 0 {
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
	a.sendToBot(s, text)
}

// sendToBot 把一轮问题交给机器人进程；回答由 RPC 事件流式写回。
func (a *App) sendToBot(s *Session, text string) {
	bot := a.BotByID(s.BotID)
	m := &Message{ID: a.id(), Role: RoleAssistant, At: a.now(), Streaming: true}
	s.Messages = append(s.Messages, m)
	if bot == nil {
		a.BotTurnFailed(s.BotID, m.ID, "这个机器人已经不在了")
		return
	}
	if a.Engine == nil {
		a.BotTurnFailed(s.BotID, m.ID, "机器人进程还没有接上")
		return
	}
	if bot.State != BotRunning {
		a.StartBot(bot.ID)
	}
	if err := a.Engine.Send(bot, m.ID, text); err != nil {
		a.BotTurnFailed(bot.ID, m.ID, err.Error())
	}
}

// Regenerate 丢掉最后一条回复，按同一个问题再问一次机器人。
func (a *App) Regenerate(s *Session) {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role != RoleUser {
			continue
		}
		prompt := s.Messages[i].Text
		s.Messages = s.Messages[:i+1]
		a.sendToBot(s, prompt)
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

// SetEngine 接上管理机器人进程的实现；nil 时机器人只停留在状态里。
func (a *App) SetEngine(e BotEngine) { a.Engine = e }

// BotByID 返回一个机器人。
func (a *App) BotByID(id int) *Bot {
	for _, b := range a.Bots {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// BotIDsInWorkspace 返回共享同一个工作目录的全部机器人，供进程管理器
// 在重建工作区实例时把成员一起挂上。
func (a *App) BotIDsInWorkspace(workspace string) []int {
	key := workspaceKey(workspace)
	out := make([]int, 0, len(a.Bots))
	for _, b := range a.Bots {
		if workspaceKey(b.Workspace) == key {
			out = append(out, b.ID)
		}
	}
	return out
}

func workspaceKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

// AddBot 记下对话框里填好的机器人，放到侧栏列表的前面，并启动它的进程。
// 工作目录必须是已存在的目录：它是机器人进程的工作目录。
func (a *App) AddBot(b Bot) (*Bot, error) {
	b.Name = Clip(strings.TrimSpace(b.Name), 40)
	if b.Name == "" {
		b.Name = "新机器人"
	}
	b.Description = strings.TrimSpace(b.Description)
	b.Instruction = strings.TrimSpace(b.Instruction)
	ws := strings.TrimSpace(b.Workspace)
	if ws == "" {
		return nil, fmt.Errorf("请选择机器人进程的工作目录")
	}
	abs, err := filepath.Abs(ws)
	if err != nil {
		return nil, fmt.Errorf("工作目录非法: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("工作目录不存在或不是文件夹: %s", abs)
	}
	b.Workspace = abs
	b.State = BotStopped
	b.Err = ""
	b.ID = a.id()
	a.Workspace = abs
	a.Bots = append([]*Bot{&b}, a.Bots...)
	bot := a.Bots[0]
	a.StartBot(bot.ID)
	return bot, nil
}

// StartBot 启动机器人的进程实例；已经在运行时是空操作。
func (a *App) StartBot(id int) {
	bot := a.BotByID(id)
	if bot == nil {
		return
	}
	if bot.State == BotRunning || bot.State == BotStarting {
		return
	}
	bot.State = BotStarting
	bot.Err = ""
	bot.PID = 0
	if a.Engine == nil {
		bot.State = BotStopped
		return
	}
	if err := a.Engine.Start(bot); err != nil {
		bot.State = BotError
		bot.Err = err.Error()
	}
}

// StopBot 停掉机器人的进程实例。
func (a *App) StopBot(id int) {
	bot := a.BotByID(id)
	if bot == nil {
		return
	}
	if a.Engine != nil {
		_ = a.Engine.Stop(id)
	}
	bot.State = BotStopped
	bot.Err = ""
	bot.PID = 0
}

// SetBotState 由引擎在进程/RPC 状态变化时调用；它总是回到 UI 线程再改状态。
func (a *App) SetBotState(id int, state BotState, pid int, errText string) {
	a.apply(func() {
		bot := a.BotByID(id)
		if bot == nil {
			return
		}
		bot.State = state
		bot.PID = pid
		bot.Err = errText
	})
}

// SetBotInfo 用 bot.info 的结果修正模型与运行时报告的工作目录。
func (a *App) SetBotInfo(id int, model, workspace string) {
	a.apply(func() {
		bot := a.BotByID(id)
		if bot == nil {
			return
		}
		if model != "" {
			bot.Model = model
		}
		if workspace != "" {
			bot.Workspace = workspace
		}
	})
}

// OpenBotChat 打开这个机器人的对话；没有就新建一条，并确保它的进程在运行。
func (a *App) OpenBotChat(id int) bool {
	bot := a.BotByID(id)
	if bot == nil {
		return false
	}
	a.StartBot(id)
	for _, s := range a.Sessions {
		if s.BotID == id {
			a.ActiveID = s.ID
			return true
		}
	}
	s := &Session{
		ID: a.id(), Title: bot.Name, BotID: id, Updated: a.now(),
		Preview: "工作目录 " + bot.Workspace,
	}
	a.Sessions = append([]*Session{s}, a.Sessions...)
	a.ActiveID = s.ID
	return true
}

// BotDelta 追加机器人流式回答的一段文本。
func (a *App) BotDelta(botID, turn int, chunk string) {
	a.apply(func() {
		for _, s := range a.Sessions {
			if s.BotID != botID {
				continue
			}
			for _, m := range s.Messages {
				if m.ID == turn && m.Streaming {
					m.Text += chunk
					s.Preview = Clip(m.Text, 88)
					s.Updated = a.now()
					return
				}
			}
		}
	})
}

// BotTurnDone 结束一轮机器人回答；final 非空时作为最终文本。
func (a *App) BotTurnDone(botID, turn int, final string) {
	a.apply(func() {
		if m, s := a.botTurn(botID, turn); m != nil {
			if strings.TrimSpace(final) != "" {
				m.Text = final
			}
			m.Streaming = false
			s.Preview = Clip(m.Text, 88)
			s.Updated = a.now()
		}
	})
}

// BotTurnFailed 结束一轮机器人回答，并显示失败原因。
func (a *App) BotTurnFailed(botID, turn int, reason string) {
	a.apply(func() {
		m, s := a.botTurn(botID, turn)
		if m == nil {
			return
		}
		if strings.TrimSpace(m.Text) == "" {
			m.Text = "（机器人没有回答：" + reason + "）"
		} else {
			m.Text += "\n\n（连接中断：" + reason + "）"
		}
		m.Streaming = false
		s.Preview = Clip(m.Text, 88)
	})
}

func (a *App) botTurn(botID, turn int) (*Message, *Session) {
	for _, s := range a.Sessions {
		if s.BotID != botID {
			continue
		}
		for _, m := range s.Messages {
			if m.ID == turn {
				return m, s
			}
		}
	}
	return nil, nil
}

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
