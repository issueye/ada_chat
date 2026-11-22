package ui

import (
	"strconv"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/markdown"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

// 对话区是一条固定宽度的列，和 lobe-chat 在大窗口里保持一致。
const (
	conversationWidth = 880
	chatHeaderHeight  = 54
)

// chatPanel 是放对话的卡片，位于侧栏右侧。
func (u *UI) chatPanel(c *mui.Context) {
	tk := theme.TokensOf(c)
	inset := float32(0)
	if !u.sidebarOpen {
		inset = 8
	}
	edge := tk.Border
	if tk.Dark {
		edge = tk.BorderSecondary
	}
	mui.Column(c).Grow(1).Basis(0).MinWidth(0).Margin(0, 8, 8, inset).
		Radius(tk.BorderRadiusLG).Background(tk.BgContainer).Border(1, edge).Clip().
		Children(func() {
			if u.App.Current() == nil {
				u.welcomePanel(c, tk)
				return
			}
			u.chatHeader(c, tk)
			mui.Box(c).Height(1).Shrink(0).Background(tk.BorderSecondary)
			mui.Column(c).Grow(1).Basis(0).MinHeight(0).AlignItems(mui.Center).Children(func() {
				u.transcript(c, tk)
			})
			u.composerArea(c, tk)
		})
}

// welcomePanel 是没有任何对话时的空工作台：这里没有预置聊天，只引导添加机器人。
func (u *UI) welcomePanel(c *mui.Context, tk *theme.Tokens) {
	mui.Column(c).Fill().Center().Gap(18).Padding(32).Children(func() {
		display.Empty(c, display.EmptyProps{
			Icon:        lucide.Bot(),
			Title:       "还没有对话",
			Description: "添加一个机器人，选择它的工作目录。消息会交给那个工作区里的进程。",
		})
		if action.Button(c, "添加机器人", action.ButtonProps{Variant: action.Primary}).Clicked() {
			u.openBotDialog()
		}
	})
}

func (u *UI) chatHeader(c *mui.Context, tk *theme.Tokens) {
	s := u.App.Current()
	if s == nil {
		return
	}
	bot := u.botForSession(s)
	mui.Row(c).Height(chatHeaderHeight).Shrink(0).Padding(8, 16).Gap(8).AlignItems(mui.Center).Justify(mui.SpaceBetween).Children(func() {
		mui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3).Children(func() {
			mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
				mui.Text(c, s.Title).FontSize(15).FontWeight(600).TextColor(tk.Text).SingleLine().MaxWidth(560)
				if bot != nil {
					display.Tag(c, bot.Name, display.TagProps{
						Variant: display.TagBorderless, Color: display.TagGray, Size: theme.Small, Icon: botGlyphSVG(bot.Icon),
					})
				}
				if s.Pinned {
					display.Tag(c, "已置顶", display.TagProps{Variant: display.TagBorderless, Color: display.TagGold, Size: theme.Small, Icon: lucide.Pin()})
				}
			})
			mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
				if bot != nil && bot.Model != "" {
					display.Tag(c, bot.Model, display.TagProps{Variant: display.TagBorderless, Color: display.TagGray, Size: theme.Small})
				}
				if bot != nil {
					tag := display.Tag(c, compactPath(bot.Workspace), display.TagProps{
						Variant: display.TagBorderless, Color: display.TagGray, Size: theme.Small, Icon: lucide.FolderClosed(),
					})
					overlay.Tooltip(c, tag, "工作目录: "+bot.Workspace, overlay.TooltipProps{})
				}
				display.Text(c, "·", display.TextProps{Type: display.TextSecondary, FontSize: 12})
				display.Text(c, messageCount(len(s.Messages)), display.TextProps{Type: display.TextSecondary, FontSize: 12})
			})
		})
		mui.Row(c).Gap(4).AlignItems(mui.Center).Children(func() {
			u.conversationMenu(c, s)
		})
	})
}

// botForSession 返回这条对话绑定的机器人。
func (u *UI) botForSession(s *app.Session) *app.Bot {
	if s == nil || s.BotID == 0 {
		return nil
	}
	return u.App.BotByID(s.BotID)
}

// assistantName 是当前对话里回答者的名字。
func (u *UI) assistantName() string {
	if b := u.botForSession(u.App.Current()); b != nil {
		return b.Name
	}
	return "机器人"
}

func messageCount(n int) string {
	switch {
	case n == 0:
		return "暂无消息"
	case n == 1:
		return "1 条消息"
	}
	return strconv.Itoa(n) + " 条消息"
}

func (u *UI) conversationText(s *app.Session) string {
	var b strings.Builder
	for i, m := range s.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		if m.Role == app.RoleUser {
			b.WriteString("**" + u.App.UserName() + "：** ")
		} else {
			name := "机器人"
			if bot := u.botForSession(s); bot != nil {
				name = bot.Name
			}
			b.WriteString("**" + name + "：** ")
		}
		b.WriteString(m.Text)
	}
	return b.String()
}

func (u *UI) conversationMenu(c *mui.Context, s *app.Session) {
	pinLabel, pinIcon := "置顶对话", lucide.Pin()
	if s.Pinned {
		pinLabel, pinIcon = "取消置顶", lucide.PinOff()
	}
	items := []overlay.MenuItem{
		{Key: "pin", Label: pinLabel, Icon: pinIcon},
		{Key: "rename", Label: "重命名", Icon: lucide.Pencil()},
		{Key: "copy", Label: "复制对话", Icon: lucide.Copy()},
		{Key: "divider", Divider: true},
		{Key: "delete", Label: "删除对话", Icon: lucide.Trash2(), Danger: true},
	}
	chosen := overlay.DropdownMenu(c, &u.chatMenu, overlay.DropdownMenuProps{Items: items}, func(t mui.Element) {
		action.StyleActionIcon(c, t, lucide.Ellipsis(), action.ActionIconProps{Size: theme.Small, Title: "对话操作", Active: u.chatMenu})
	})
	switch chosen {
	case "pin":
		s.Pinned = !s.Pinned
	case "rename":
		u.renameDraft = s.Title
		u.renameUp = true
	case "copy":
		c.WriteClipboard(u.conversationText(s))
		feedback.ShowToast(c, feedback.ToastProps{Title: "已复制对话", Type: feedback.ToastSuccess})
	case "delete":
		u.confirmDel = true
	}
}

func (u *UI) transcript(c *mui.Context, tk *theme.Tokens) {
	s := u.App.Current()
	if s == nil {
		return
	}
	u.resetListIfSwitched()
	msgs := s.Messages
	u.chatList.Key = func(i int) any { return msgs[i].ID }
	u.chatList.Label = func(i int) string {
		name := u.assistantName()
		if msgs[i].Role == app.RoleUser {
			name = u.App.UserName()
		}
		label := name + ": " + app.Clip(msgs[i].Text, 80)
		if n := len(msgs[i].Tools); n > 0 {
			label = name + "（" + strconv.Itoa(n) + " 个工具）：" + app.Clip(msgs[i].Text, 80)
		}
		return label
	}
	u.chatList.FollowEnd = true
	u.chatList.FollowAnimation = &mui.Smooth

	list := mui.List(c, &u.chatList, len(msgs), func(i int) {
		u.messageRow(c, tk, msgs[i])
	}).Grow(1).Basis(0).Width(conversationWidth).MaxWidthPercent(100).
		Padding(16, 24, 8, 24).Gap(4)
	if len(msgs) == 0 {
		list.Justify(mui.Center).Children(func() { u.emptyChat(c, tk) })
	} else {
		list.Justify(mui.End)
	}
	// 列表行可能要求边构建边改动列表：先记下来，等列表建完再执行。
	if fn := u.pending; fn != nil {
		u.pending = nil
		fn()
	}
}

func (u *UI) emptyChat(c *mui.Context, tk *theme.Tokens) {
	bot := u.botForSession(u.App.Current())
	title, desc := "开始一次对话", "消息会交给这个工作目录里的机器人进程。"
	if bot != nil {
		title = "和 " + bot.Name + " 对话"
		desc = "它运行在 " + bot.Workspace + "，第一句话会发到那个进程。"
	}
	mui.Column(c).Gap(20).Padding(24, 0, 8, 0).AlignItems(mui.Center).Children(func() {
		display.Empty(c, display.EmptyProps{
			Icon:        lucide.MessageSquare(),
			Title:       title,
			Description: desc,
		})
	})
}

func (u *UI) messageRow(c *mui.Context, tk *theme.Tokens, m *app.Message) {
	if m.Role == app.RoleUser {
		u.userMessage(c, tk, m)
		return
	}
	u.assistantMessage(c, tk, m)
}

func (u *UI) userMessage(c *mui.Context, tk *theme.Tokens, m *app.Message) {
	mui.Row(c).Justify(mui.End).PaddingY(10).Children(func() {
		mui.Column(c).MaxWidthPercent(80).Gap(3).AlignItems(mui.End).Children(func() {
			b := layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(10, 14).Radius(tk.BorderRadiusLG)
			b.Children(func() {
				mui.Text(c, m.Text).FontSize(14).LineHeight(1.5).TextColor(tk.Text)
			})
			display.Text(c, app.RelativeTime(c.Now(), m.At)+" · "+u.App.UserName(), display.TextProps{
				Type: display.TextSecondary, FontSize: 11,
			})
		})
	})
}

func (u *UI) assistantMessage(c *mui.Context, tk *theme.Tokens, m *app.Message) {
	bot := u.botForSession(u.App.Current())
	name := "机器人"
	if bot != nil {
		name = bot.Name
	}
	mui.Row(c).Gap(12).AlignItems(mui.Start).PaddingY(10).Children(func() {
		mui.Box(c).PaddingY(4).Shrink(0).Children(func() {
			if bot != nil {
				botAvatar(c, bot.Icon, bot.Color, 30)
				return
			}
			mui.Icon(c, lucide.Bot()).FontSize(24).TextColor(tk.TextTertiary)
		})
		mui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(6).Children(func() {
			mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
				display.Text(c, name, display.TextProps{FontSize: 13, Strong: true})
				display.Text(c, app.RelativeTime(c.Now(), m.At), display.TextProps{Type: display.TextSecondary, FontSize: 11})
				if m.Streaming {
					display.Text(c, "思考中", display.TextProps{Type: display.TextSecondary, FontSize: 11, Shiny: true})
				}
			})
			if len(m.Tools) > 0 {
				mui.Column(c).Gap(8).Children(func() {
					for _, call := range m.Tools {
						toolCard(c, tk, call)
					}
				})
			}
			switch {
			case m.Text == "" && m.Streaming && len(m.Tools) == 0:
				feedback.Spin(c, feedback.SpinProps{Size: theme.Small, Variant: feedback.SpinNetwork}, nil)
			case m.Text != "":
				markdown.Markdown(c, m.Text, markdown.MarkdownProps{
					Variant:  markdown.Chat,
					Animated: m.Streaming,
				})
			}
			if !m.Streaming {
				u.messageActions(c, m)
			}
		})
	})
}

func (u *UI) messageActions(c *mui.Context, m *app.Message) {
	s := u.App.Current()
	if s == nil {
		return
	}
	chosen := action.ActionIconGroup(c, action.ActionIconGroupProps{
		Variant: action.GroupBorderless,
		Size:    theme.Small,
		Items: []action.ActionGroupItem{
			{Key: "copy", Icon: lucide.Copy(), Label: "复制回复"},
			{Key: "regenerate", Icon: lucide.RefreshCw(), Label: "重新生成"},
		},
		Menu: []overlay.MenuItem{
			{Key: "delete", Label: "删除消息", Icon: lucide.Trash2(), Danger: true},
		},
	})
	switch chosen {
	case "copy":
		c.WriteClipboard(m.Text)
		feedback.ShowToast(c, feedback.ToastProps{Title: "已复制到剪贴板", Type: feedback.ToastSuccess})
	case "regenerate", "delete":
		// 列表正在构建：等它建完再改。
		u.pending = func() {
			if chosen == "regenerate" {
				u.App.Regenerate(s)
				return
			}
			u.App.DeleteMessage(s, m.ID)
			feedback.ShowToast(c, feedback.ToastProps{Title: "消息已删除", Type: feedback.ToastInfo})
		}
	}
}
