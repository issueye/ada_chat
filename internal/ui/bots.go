package ui

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

// botGlyph is one icon choice; its color tints the avatar and the process
// status, so bots stay distinguishable without a color picker.
type botGlyph struct {
	key, label, color string
	svg               func() *mui.SVG
}

var botGlyphs = []botGlyph{
	{"bot", "机器人", "#7858f9", lucide.Bot},
	{"sparkles", "灵感", "#eb2f96", lucide.Sparkles},
	{"brain", "思考", "#1677ff", lucide.Brain},
	{"code", "代码", "#13c2c2", lucide.Code},
	{"palette", "设计", "#faad14", lucide.Palette},
	{"book", "知识", "#52c41a", lucide.BookOpen},
	{"rocket", "上手", "#f88c13", lucide.Rocket},
	{"wrench", "工具", "#383838", lucide.Wrench},
}

func botGlyphSVG(key string) *mui.SVG {
	for _, g := range botGlyphs {
		if g.key == key {
			return g.svg()
		}
	}
	return lucide.Bot()
}

func botGlyphColor(key string) string {
	for _, g := range botGlyphs {
		if g.key == key {
			return g.color
		}
	}
	return botGlyphs[0].color
}

// compactPath 让侧栏的机器人行只显示最后两级目录；完整路径在提示里。
func compactPath(path string) string {
	path = strings.TrimRight(path, `/\`)
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		return base
	}
	return parent + string(filepath.Separator) + base
}

// botAvatar 用图标的颜色画一枚小方块，侧栏的行和对话框的预览共用它。
func botAvatar(c *mui.Context, icon, color string, size float32) {
	if color == "" {
		color = botGlyphs[0].color
	}
	accent := mui.Hex(color)
	mui.Row(c).Size(size, size).Shrink(0).Radius(size / 4).Center().
		Background(accent.Alpha(0.14)).Children(func() {
		mui.Icon(c, botGlyphSVG(icon)).FontSize(size * 0.55).TextColor(accent)
	})
}

// botSection 列出已经添加的机器人；没有机器人时整段不占位置。
func (u *UI) botSection(c *mui.Context) {
	tk := theme.TokensOf(c)
	bots := u.App.Bots
	if len(bots) == 0 || strings.TrimSpace(u.query) != "" {
		return
	}
	active := 0
	if s := u.App.Current(); s != nil {
		active = s.BotID
	}
	mui.Column(c).Gap(2).Children(func() {
		mui.Row(c).Padding(10, 8, 2, 8).Gap(6).AlignItems(mui.Center).Children(func() {
			display.Text(c, "机器人", display.TextProps{Type: display.TextSecondary, FontSize: 11, Strong: true})
			mui.Spacer(c)
			display.Text(c, strconv.Itoa(len(bots)), display.TextProps{Type: display.TextSecondary, FontSize: 11})
		})
		for _, b := range bots {
			u.botRow(c, tk, b, active == b.ID)
		}
	})
}

func (u *UI) botRow(c *mui.Context, tk *theme.Tokens, b *app.Bot, active bool) {
	row := mui.ButtonBase(c).Label(b.Name).Height(44).PaddingX(8).Gap(8).AlignItems(mui.Center).
		Radius(tk.BorderRadiusSM).Cursor(mui.CursorPointer).AlignSelf(mui.Stretch)
	bg := mui.Transparent
	if active {
		bg = tk.FillTertiary
	}
	if row.Hovered() {
		bg = tk.FillSecondary
	}
	row.Background(bg)
	overlay.Tooltip(c, row, botTooltip(b), overlay.TooltipProps{})
	row.Children(func() {
		mui.Box(c).Shrink(0).Children(func() { botAvatar(c, b.Icon, b.Color, 24) })
		mui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			mui.Text(c, b.Name).FontSize(13).FontWeight(500).TextColor(tk.Text).SingleLine()
			path := b.Workspace
			if path == "" {
				path = "未选择工作目录"
			} else {
				path = compactPath(path)
			}
			display.Text(c, path, display.TextProps{Type: display.TextSecondary, FontSize: 11})
		})
		mui.Box(c).Size(7, 7).Shrink(0).Radius(4).Background(botStateColor(tk, b.State))
	})
	u.botContextMenu(c, row, b)
	if row.Clicked() {
		u.App.OpenBotChat(b.ID)
	}
}

// botContextMenu 让机器人的侧栏行可以右键：新建会话、编辑资料，或重启它的进程。
func (u *UI) botContextMenu(c *mui.Context, row mui.Element, b *app.Bot) {
	startLabel := "重启进程"
	if b.State == app.BotStopped || b.State == app.BotError {
		startLabel = "启动进程"
	}
	items := []overlay.MenuItem{
		{Key: "new", Label: "新建会话", Icon: lucide.MessageSquarePlus()},
		{Key: "edit", Label: "编辑信息", Icon: lucide.Pencil()},
		{Key: "restart", Label: startLabel, Icon: lucide.RefreshCw(), Disabled: b.State == app.BotStarting},
	}
	chosen := overlay.ContextMenu(c, row, overlay.DropdownMenuProps{Items: items})
	switch chosen {
	case "new":
		u.newBotSession(c, b)
	case "edit":
		u.openBotEdit(b)
	case "restart":
		u.restartBot(c, b)
	}
}

// newBotSession 新开一条对话并切过去；runtime 侧会为它建独立的 agent session。
func (u *UI) newBotSession(c *mui.Context, b *app.Bot) {
	if _, err := u.App.NewBotSession(b.ID); err != nil {
		feedback.ShowToast(c, feedback.ToastProps{
			Title: "新建会话失败", Description: err.Error(), Type: feedback.ToastError,
		})
		return
	}
	u.App.StartBot(b.ID)
	feedback.ShowToast(c, feedback.ToastProps{
		Title: "已新建会话", Description: b.Name, Type: feedback.ToastSuccess,
	})
	c.Invalidate()
}

func (u *UI) restartBot(c *mui.Context, b *app.Bot) {
	if b.State == app.BotStarting {
		return
	}
	if b.State == app.BotRunning {
		u.App.StopBot(b.ID)
	}
	u.App.StartBot(b.ID)
	feedback.ShowToast(c, feedback.ToastProps{
		Title: "机器人进程已重启", Description: b.Name, Type: feedback.ToastSuccess,
	})
}

func botTooltip(b *app.Bot) string {
	state := botStateLabel(b.State)
	if b.PID > 0 && b.State == app.BotRunning {
		state += " · pid " + strconv.Itoa(b.PID)
	}
	if b.Err != "" && b.State == app.BotError {
		state += " · " + b.Err
	}
	return b.Name + "\n" + b.Workspace + "\n" + state
}

func botStateLabel(s app.BotState) string {
	switch s {
	case app.BotStarting:
		return "启动中"
	case app.BotRunning:
		return "运行中"
	case app.BotError:
		return "启动失败"
	default:
		return "已停止"
	}
}

func botStateColor(tk *theme.Tokens, s app.BotState) mui.Color {
	switch s {
	case app.BotStarting:
		return tk.Warning
	case app.BotRunning:
		return tk.Success
	case app.BotError:
		return tk.Error
	default:
		return tk.TextQuaternary
	}
}

// openBotDialog 每次都从空白开始，只带上最近的工作目录和默认图标。
func (u *UI) openBotDialog() {
	icon := botGlyphs[0].key
	u.botEditID = 0
	u.botDraft = app.Bot{
		Workspace: u.App.Workspace,
		Icon:      icon,
		Color:     botGlyphColor(icon),
	}
	u.botErr = ""
	u.botDirErr = ""
	u.botUp = true
}

// openBotEdit 用现有机器人的资料填满同一个表单；保存走 UpdateBot。
func (u *UI) openBotEdit(b *app.Bot) {
	if b == nil {
		return
	}
	u.botEditID = b.ID
	u.botDraft = app.Bot{
		Name:        b.Name,
		Description: b.Description,
		Instruction: b.Instruction,
		Workspace:   b.Workspace,
		Icon:        b.Icon,
		Color:       b.Color,
	}
	u.botErr = ""
	u.botDirErr = ""
	u.botUp = true
}

func (u *UI) botDialog(c *mui.Context) {
	tk := theme.TokensOf(c)
	u.botDraft.Color = botGlyphColor(u.botDraft.Icon)
	title, okText := "添加机器人", "创建"
	if u.botEditID != 0 {
		title, okText = "编辑机器人", "保存"
	}
	ok := feedback.Modal(c, &u.botUp, feedback.ModalProps{
		Title: title, Width: 560, OkText: okText, CancelText: "取消",
	}, func() {
		u.botFormBody(c, tk)
	})
	if !ok {
		return
	}
	// 每次提交都从干净的错误状态开始，失败原因只保留到下一次提交。
	u.botErr, u.botDirErr = "", ""
	u.botDraft.Name = strings.TrimSpace(u.botDraft.Name)
	if u.botDraft.Name == "" {
		u.botErr = "给它起个名字，侧栏里才认得出来"
		c.Invalidate()
		return
	}
	if u.botEditID != 0 {
		bot, err := u.App.UpdateBot(u.botEditID, u.botDraft)
		if err != nil {
			u.botDirErr = err.Error()
			c.Invalidate()
			return
		}
		u.botUp = false
		u.botEditID = 0
		feedback.ShowToast(c, feedback.ToastProps{
			Title: "机器人信息已更新", Description: bot.Name, Type: feedback.ToastSuccess,
		})
		return
	}
	bot, err := u.App.AddBot(u.botDraft)
	if err != nil {
		u.botDirErr = err.Error()
		c.Invalidate()
		return
	}
	u.botUp = false
	u.App.OpenBotChat(bot.ID)
	feedback.ShowToast(c, feedback.ToastProps{
		Title: "机器人已添加", Description: bot.Name, Type: feedback.ToastSuccess,
	})
}

func (u *UI) botFormBody(c *mui.Context, tk *theme.Tokens) {
	d := &u.botDraft
	mui.Column(c).Gap(16).Children(func() {
		u.botPreview(c, tk)
		if u.botEditID != 0 {
			mui.Text(c, "改名与图标立即生效；工作目录改动会重启该机器人的进程；系统提示词在下一次进程启动时生效。").
				FontSize(11).TextColor(tk.TextTertiary)
		}
		mui.Column(c).Gap(12).Children(func() {
			botField(c, tk, "名称", true, "侧栏和对话里都用这个名字", func() {
				field.Input(c, &d.Name, field.InputProps{
					Variant: field.InputOutlined, Placeholder: "例如：代码助手", Invalid: u.botErr != "",
				}).Label("名称")
			})
			if u.botErr != "" {
				mui.Text(c, u.botErr).FontSize(11).TextColor(tk.Error).Margin(4, 0, 0, 0)
			}
			botField(c, tk, "工作目录", true, "机器人进程在这个目录里启动，也是它的工作区", func() {
				mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
					mui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						field.Input(c, &d.Workspace, field.InputProps{
							Variant: field.InputOutlined, Placeholder: "选择或输入一个目录",
							Invalid: u.botDirErr != "",
						}).Label("工作目录")
					})
					label := "选择目录"
					if u.dirPicking {
						label = "选择中…"
					}
					if action.Button(c, label, action.ButtonProps{Size: theme.Small}).Clicked() && !u.dirPicking {
						u.pickWorkspaceDir()
					}
				})
				if u.botDirErr != "" {
					mui.Text(c, u.botDirErr).FontSize(11).TextColor(tk.Error).Margin(4, 0, 0, 0)
				}
			})
			botField(c, tk, "介绍", false, "一句话说明它擅长什么", func() {
				field.TextArea(c, &d.Description, field.TextAreaProps{
					Variant: field.InputOutlined, Placeholder: "例如：帮我读代码，算清楚每一步",
					MinRows: 2, MaxRows: 3,
				}).Label("介绍")
			})
			botField(c, tk, "系统提示词", false, "每轮对话都会放在最前面，告诉它该怎么做", func() {
				field.TextArea(c, &d.Instruction, field.TextAreaProps{
					Variant: field.InputOutlined, Placeholder: "例如：回答前先读一遍相关代码，再给出具体改动",
					MinRows: 2, MaxRows: 4,
				}).Label("系统提示词")
			})
		})
		botField(c, tk, "图标", false, "挑一个一眼认得出的符号", func() { u.iconPicker(c, tk) })
	})
}

// botField 是对话框里紧凑的一栏：标题、说明在上，控件在下。
func botField(c *mui.Context, tk *theme.Tokens, label string, required bool, desc string, control func()) {
	mui.Column(c).Gap(4).Children(func() {
		mui.Row(c).Gap(4).AlignItems(mui.Center).Children(func() {
			mui.Text(c, label).FontSize(13).FontWeight(600).TextColor(tk.Text)
			if required {
				mui.Text(c, "*").FontSize(13).TextColor(tk.Error)
			}
		})
		if desc != "" {
			mui.Text(c, desc).FontSize(11).TextColor(tk.TextTertiary).SingleLine()
		}
		control()
	})
}

// botPreview 是对话框顶部的即时预览：名字、工作目录和图标还没保存就先画出来。
func (u *UI) botPreview(c *mui.Context, tk *theme.Tokens) {
	d := &u.botDraft
	name := strings.TrimSpace(d.Name)
	if name == "" {
		name = "新机器人"
	}
	where := "选择机器人进程的工作目录"
	if d.Workspace != "" {
		where = "将运行在 " + d.Workspace
	}
	mui.Row(c).Gap(12).AlignItems(mui.Center).Children(func() {
		mui.Box(c).Shrink(0).Children(func() { botAvatar(c, d.Icon, d.Color, 44) })
		mui.Column(c).Gap(2).MinWidth(0).Children(func() {
			mui.Text(c, name).FontSize(14).FontWeight(600).TextColor(tk.Text).SingleLine()
			mui.Text(c, where).FontSize(12).TextColor(tk.TextTertiary).SingleLine()
		})
	})
}

func (u *UI) iconPicker(c *mui.Context, tk *theme.Tokens) {
	f := &u.botDraft
	mui.RadioGroup(c, func() {
		for _, g := range botGlyphs {
			chosen := f.Icon == g.key
			b := mui.RadioBase(c.Key(g.key), &f.Icon, g.key).Size(38, 38).Radius(10).
				Center().Cursor(mui.CursorPointer)
			b.Label(g.label)
			overlay.Tooltip(c, b, g.label, overlay.TooltipProps{})
			bg, fg := tk.FillTertiary, tk.TextSecondary
			if chosen {
				bg, fg = tk.PrimaryBg, mui.Hex(g.color)
			}
			if b.Hovered() {
				bg = tk.FillSecondary
			}
			b.Background(bg).Children(func() {
				mui.Icon(c, g.svg()).FontSize(19).TextColor(fg)
			})
		}
	}).Label("图标").Row().Wrap().Gap(8)
}

// pickWorkspaceDir opens the native folder picker off the UI thread and puts
// the chosen directory into the draft. It is the only way to pick a workspace:
// there is no dropdown, because a workspace is a process working directory.
func (u *UI) pickWorkspaceDir() {
	if u.dirPicking {
		return
	}
	u.dirPicking = true
	pick := u.pickDir
	if pick == nil {
		pick = pickFolder
	}
	apply := func(dir string, err error) {
		u.dirPicking = false
		if err == nil && dir != "" {
			u.botDraft.Workspace = dir
			u.botDirErr = ""
		}
	}
	dispatch := u.dispatch
	if dispatch == nil {
		dir, err := pick()
		apply(dir, err)
		return
	}
	go func() {
		dir, err := pick()
		dispatch(func() { apply(dir, err) })
	}()
}

func pickFolder() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Directory: true})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}
