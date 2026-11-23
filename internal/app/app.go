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
	"sync"
	"time"
)

// Role 区分对话里的两种身份。
type Role int

const (
	RoleUser Role = iota
	RoleAssistant
)

// Message 是一轮消息，可以带着先跑完的工具调用。
// Message 是对话里的一条消息。Parts 记录一轮回答的真实顺序。
type Message struct {
	ID        int         `json:"id"`
	Role      Role        `json:"role"`
	Text      string      `json:"text"`
	At        time.Time   `json:"at"`
	Streaming bool        `json:"streaming,omitempty"`
	Tools     []*ToolCall `json:"tools,omitempty"`
	// Parts 保留一轮回答的真实顺序：模型先写的文字、工具调用、工具之后的文字。
	// 历史/非流式消息可能只有 Text 而没有 Parts。
	Parts []Part `json:"parts,omitempty"`
}

// Session 是一条对话。BotID 非零时，回答来自那个机器人的进程实例。
// Session 是一条对话：机器人、标题、预览和消息。
type Session struct {
	ID       int        `json:"id"`
	Title    string     `json:"title"`
	Preview  string     `json:"preview,omitempty"`
	Updated  time.Time  `json:"updated"`
	Unread   int        `json:"unread,omitempty"`
	Pinned   bool       `json:"pinned,omitempty"`
	BotID    int        `json:"botId"`
	Messages []*Message `json:"messages,omitempty"`
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
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Instruction string   `json:"instruction,omitempty"`
	Model       string   `json:"model,omitempty"`
	Workspace   string   `json:"workspace"`
	State       BotState `json:"-"` // 进程状态是瞬时的，重启后一律变成 BotStopped
	PID         int      `json:"-"`
	Err         string   `json:"-"`
	// Icon 是 ui 侧图标表里的键，Color 是 #rrggbb。
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

// BotEngine 管理机器人进程与 RPC 连接。它把状态回调到 App 的 Bot 方法。
type BotEngine interface {
	Start(b *Bot) error
	Stop(id int) error
	Send(b *Bot, turn int, text string, sessionID int) error
	// Cancel 终止该轮正在进行的 RPC 调用；已结束的轮次返回错误。
	Cancel(botID, turn int) error
	// Approve 回答一次升级审批请求；runtime 等不到答复时按拒绝处理。
	Approve(botID int, id string, allow bool, reason string) error
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
	// Approvals 是等待用户决定的升级审批，界面按队列显示；超时由 runtime 自己拒绝。
	Approvals []Approval
	// approvalOwner 是审批 id → 请求 的同步副本：窗口运行时 a.apply 是异步投递，
	// DecideApproval 不能只靠界面队列来判断请求属于哪个机器人。
	approvalMu    sync.Mutex
	approvalOwner map[string]Approval
}

// New 建出一个工作台：偏好从本机配置文件读取，机器人/对话从上一次的状态文件恢复。
// 没有保存过时，Agent 设置回落到环境变量，工作台为空。
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
	a := &App{Prefs: p}
	a.restore()
	return a
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
		a.persist()
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
	a.persist()
}

// Send 把草稿送进对话，并交给它绑定的机器人。上一轮还在生成时不再接受新消息。
func (a *App) Send() {
	if a.Busy() {
		return
	}
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

// Busy 报告当前会话是否还有一轮回答在生成（包括工具执行）；发送按钮据此显示运行态。
func (a *App) Busy() bool {
	s := a.Current()
	if s == nil || len(s.Messages) == 0 {
		return false
	}
	last := s.Messages[len(s.Messages)-1]
	return last.Role == RoleAssistant && last.Streaming
}

// StopTurn 终止当前会话正在生成的一轮回答；消息保留，并标注已停止。
func (a *App) StopTurn() bool {
	s := a.Current()
	if s == nil || len(s.Messages) == 0 {
		return false
	}
	last := s.Messages[len(s.Messages)-1]
	if last.Role != RoleAssistant || !last.Streaming {
		return false
	}
	if a.Engine != nil {
		_ = a.Engine.Cancel(s.BotID, last.ID)
	}
	const note = "（已停止生成）"
	if strings.TrimSpace(last.Text) == "" {
		last.Text = note
	} else {
		last.Text += "\n\n" + note
	}
	if len(last.Parts) > 0 {
		last.Parts = append(last.Parts, Part{Kind: PartText, Text: note})
	}
	for _, call := range last.Tools {
		if call.State == ToolRunning {
			call.State = ToolFailed
			if call.Result == "" {
				call.Result = "已停止生成"
			}
		}
	}
	last.Streaming = false
	s.Preview = Clip(last.Text, 88)
	s.Updated = a.now()
	a.removeApprovals(s.BotID, last.ID)
	a.persist()
	return true
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
		// StartBot 失败时进程实例还没有登记，Send 只会报“进程还没有运行”；
		// 这里把真正的原因（路径、工作目录、启动错误）带回对话。
		if bot.State != BotRunning && bot.State != BotStarting {
			reason := strings.TrimSpace(bot.Err)
			if reason == "" {
				reason = "机器人启动失败"
			}
			a.BotTurnFailed(bot.ID, m.ID, reason)
			return
		}
	}
	if err := a.Engine.Send(bot, m.ID, text, s.ID); err != nil {
		a.BotTurnFailed(bot.ID, m.ID, err.Error())
	}
	a.persist()
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
		a.persist()
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
			a.persist()
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
	a.persist()
	return bot, nil
}

// UpdateBot 修改机器人的资料。工作目录改变时，运行中的实例会停掉并在新目录重启；
// 其余字段立即生效，系统提示词要等该机器人的进程下一次启动才会重新装配。
func (a *App) UpdateBot(id int, patch Bot) (*Bot, error) {
	bot := a.BotByID(id)
	if bot == nil {
		return nil, fmt.Errorf("机器人不存在")
	}
	name := Clip(strings.TrimSpace(patch.Name), 40)
	if name == "" {
		name = "新机器人"
	}
	description := strings.TrimSpace(patch.Description)
	instruction := strings.TrimSpace(patch.Instruction)
	ws := strings.TrimSpace(patch.Workspace)
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

	moved := workspaceKey(bot.Workspace) != workspaceKey(abs)
	running := bot.State == BotRunning || bot.State == BotStarting
	if moved && running {
		a.StopBot(id)
	}
	bot.Name = name
	bot.Description = description
	bot.Instruction = instruction
	bot.Workspace = abs
	if strings.TrimSpace(patch.Icon) != "" {
		bot.Icon = patch.Icon
		bot.Color = patch.Color
	}
	a.Workspace = abs
	if moved && running {
		a.StartBot(id)
	}
	a.persist()
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

// NewBotSession 给机器人开一条新对话并切过去。
//
// 每个对话在 runtime 侧对应独立的 agent session（chat.send 带 sessionId），
// 所以新对话不会带着上一轮对话的模型上下文。
func (a *App) NewBotSession(botID int) (*Session, error) {
	bot := a.BotByID(botID)
	if bot == nil {
		return nil, fmt.Errorf("机器人不存在")
	}
	s := &Session{
		ID: a.id(), Title: "新对话", BotID: botID, Updated: a.now(),
		Preview: "新会话",
	}
	a.Sessions = append([]*Session{s}, a.Sessions...)
	a.ActiveID = s.ID
	a.persist()
	return s, nil
}

// OpenBotChat 打开这个机器人最近的一条对话；没有就新建一条，并确保它的进程在运行。
func (a *App) OpenBotChat(id int) bool {
	bot := a.BotByID(id)
	if bot == nil {
		return false
	}
	a.StartBot(id)
	for _, s := range a.Sessions {
		if s.BotID == id {
			a.ActiveID = s.ID
			a.persist()
			return true
		}
	}
	s := &Session{
		ID: a.id(), Title: bot.Name, BotID: id, Updated: a.now(),
		Preview: "工作目录 " + bot.Workspace,
	}
	a.Sessions = append([]*Session{s}, a.Sessions...)
	a.ActiveID = s.ID
	a.persist()
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
					// 工具调用会把一段文本截断：后续增量要开一个新的文本块，
					// 这样界面按真实顺序显示“文字 → 工具 → 文字”。
					if n := len(m.Parts); n == 0 || m.Parts[n-1].Kind != PartText {
						m.Parts = append(m.Parts, Part{Kind: PartText, Text: chunk})
					} else {
						m.Parts[n-1].Text += chunk
					}
					s.Preview = Clip(m.Text, 88)
					s.Updated = a.now()
					return
				}
			}
		}
	})
}

// BotReasoning 追加模型的一段思考内容；它和正文分开保存，界面显示成可折叠的“思考过程”。
func (a *App) BotReasoning(botID, turn int, chunk string) {
	a.apply(func() {
		for _, s := range a.Sessions {
			if s.BotID != botID {
				continue
			}
			for _, m := range s.Messages {
				if m.ID != turn || !m.Streaming {
					continue
				}
				if n := len(m.Parts); n == 0 || m.Parts[n-1].Kind != PartReasoning {
					m.Parts = append(m.Parts, Part{Kind: PartReasoning, Text: chunk, Open: true})
				} else {
					m.Parts[n-1].Text += chunk
				}
				s.Updated = a.now()
				return
			}
		}
	})
}

// BotTool 更新一轮回答里某次工具调用的状态；running 先建卡片，结果到达后再补内容。
func (a *App) BotTool(botID, turn int, ev ToolEvent) {
	a.apply(func() {
		m, _ := a.botTurn(botID, turn)
		if m == nil || ev.ID == "" {
			return
		}
		// 用户已经停止这一轮时，迟到的工具事件不应再改界面。
		if !m.Streaming {
			return
		}
		var call *ToolCall
		for _, c := range m.Tools {
			if c.ID == ev.ID {
				call = c
				break
			}
		}
		if call == nil {
			// 工具卡片默认折叠:标题行始终显示状态与耗时,参数和结果等用户点开再看。
			call = &ToolCall{ID: ev.ID}
			m.Tools = append(m.Tools, call)
			m.Parts = append(m.Parts, Part{Kind: PartTool, ToolID: ev.ID})
		} else if !hasToolPart(m.Parts, ev.ID) {
			// 只有结果事件、没有 running 事件：卡片仍要出现在它到达的位置。
			m.Parts = append(m.Parts, Part{Kind: PartTool, ToolID: ev.ID})
		}
		if ev.Name != "" {
			call.Name = ev.Name
		}
		if ev.Args != "" {
			call.Args = ev.Args
		}
		switch ev.Status {
		case "running":
			call.State = ToolRunning
		case "", "ok":
			call.State = ToolDone
		default:
			call.State = ToolFailed
		}
		if ev.Text != "" {
			call.Result = ev.Text
		}
		if ev.Duration > 0 {
			call.Took = ev.Duration
		}
		if call.State == ToolDone && call.Result == "" {
			call.Result = "(无输出)"
		}
	})
}

func hasToolPart(parts []Part, id string) bool {
	for _, p := range parts {
		if p.Kind == PartTool && p.ToolID == id {
			return true
		}
	}
	return false
}

// BotApproval 收到一次审批请求；同一 id 只入队一次。
func (a *App) BotApproval(botID, turn int, ap Approval) {
	ap.BotID, ap.Turn = botID, turn
	a.approvalMu.Lock()
	if a.approvalOwner == nil {
		a.approvalOwner = map[string]Approval{}
	}
	if _, ok := a.approvalOwner[ap.ID]; ok {
		a.approvalMu.Unlock()
		return
	}
	a.approvalOwner[ap.ID] = ap
	a.approvalMu.Unlock()

	a.apply(func() {
		for _, x := range a.Approvals {
			if x.ID == ap.ID {
				return
			}
		}
		a.Approvals = append(a.Approvals, ap)
	})
}

// DecideApproval 回答一次审批请求；allow=false 即拒绝。
//
// runtime 等不到答复会按超时拒绝，所以找不到 id 时只当作已经结束，不报错。
func (a *App) DecideApproval(id string, allow bool, reason string) error {
	a.approvalMu.Lock()
	ap, ok := a.approvalOwner[id]
	delete(a.approvalOwner, id)
	a.approvalMu.Unlock()
	if !ok {
		return nil
	}
	a.apply(func() {
		for i := range a.Approvals {
			if a.Approvals[i].ID == id {
				a.Approvals = append(a.Approvals[:i], a.Approvals[i+1:]...)
				return
			}
		}
	})
	if a.Engine == nil {
		return fmt.Errorf("机器人进程还没有接上")
	}
	return a.Engine.Approve(ap.BotID, id, allow, reason)
}

// removeApprovals 丢掉属于某一轮的审批；回合结束/失败/停止后用户不必再回答。
// 它只改内存里的队列与 id 映射，调用方决定是否在 UI 线程执行。
func (a *App) removeApprovals(botID, turn int) {
	a.approvalMu.Lock()
	for id, ap := range a.approvalOwner {
		if ap.BotID == botID && ap.Turn == turn {
			delete(a.approvalOwner, id)
		}
	}
	a.approvalMu.Unlock()

	kept := a.Approvals[:0]
	for _, ap := range a.Approvals {
		if ap.BotID == botID && ap.Turn == turn {
			continue
		}
		kept = append(kept, ap)
	}
	a.Approvals = kept
}

// BotTurnDone 结束一轮机器人回答；已经流式写入的文本保持原样，只在没有增量时用最终结果补齐。
func (a *App) BotTurnDone(botID, turn int, final string) {
	a.apply(func() {
		if m, s := a.botTurn(botID, turn); m != nil {
			if strings.TrimSpace(m.Text) == "" && strings.TrimSpace(final) != "" {
				m.Text = final
				// 只有工具、没有文本增量时，最终文本也要落到顺序里，否则界面只显示卡片。
				if len(m.Parts) > 0 {
					m.Parts = append(m.Parts, Part{Kind: PartText, Text: final})
				}
			}
			m.Streaming = false
			s.Preview = Clip(m.Text, 88)
			s.Updated = a.now()
			a.removeApprovals(botID, turn)
			a.persist()
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
			if len(m.Parts) > 0 {
				m.Parts = append(m.Parts, Part{Kind: PartText, Text: m.Text})
			}
		} else {
			m.Text += "\n\n（连接中断：" + reason + "）"
			m.Parts = append(m.Parts, Part{Kind: PartText, Text: "（连接中断：" + reason + "）"})
		}
		m.Streaming = false
		s.Preview = Clip(m.Text, 88)
		a.removeApprovals(botID, turn)
		a.persist()
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
