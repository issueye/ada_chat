package ui

import (
	"strconv"
	"strings"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

// 侧栏留出足够宽度，让较长的对话标题也不至于挤到对话区。
const (
	sidebarMin     = 240
	sidebarMax     = 360
	sidebarDefault = 276
)

var workspaces = []string{"Ada 工作区", "个人"}

type sessionGroup struct {
	label string
	items []*app.Session
}

// sidebar 是对话来源列表：工作区切换、搜索框和对话，新的在前。
func (u *UI) sidebar(c *mui.Context) {
	layout.DraggablePanel(c, &u.sidebarWidth, &u.sidebarOpen, layout.DraggablePanelProps{
		Placement: layout.Left,
		MinSize:   sidebarMin,
		MaxSize:   sidebarMax,
		NoToggle:  true,
		NoBorder:  true,
	}, func() {
		tk := theme.TokensOf(c)
		mui.Column(c).Fill().TextColor(tk.TextSecondary).Children(func() {
			u.sidebarHeader(c)
			mui.Box(c).Padding(0, 10, 6, 10).Children(func() {
				field.SearchBar(c, &u.query, field.SearchBarProps{
					Placeholder: "搜索对话",
					ShortKey:    "mod+k",
					Size:        theme.Small,
					Variant:     field.InputFilled,
				}).Label("搜索对话")
			})
			mui.Scroll(c).Grow(1).Basis(0).Children(func() {
				mui.Column(c).PaddingX(4).PaddingY(4).Gap(14).Children(func() { u.sessionSections(c) })
			})
			u.sidebarFooter(c)
		})
	})
}

func (u *UI) sidebarHeader(c *mui.Context) {
	mui.Row(c).Shrink(0).Padding(8, 6).Gap(4).AlignItems(mui.Center).Justify(mui.SpaceBetween).Children(func() {
		u.workspaceSwitcher(c)
		mui.Row(c).Gap(2).AlignItems(mui.Center).Children(func() {
			if action.ActionIcon(c, lucide.SquarePen(), small("新建对话")).Clicked() || isShortcut(c, "mod+n") {
				u.App.NewChat()
				u.query = ""
			}
		})
	})
}

func (u *UI) workspaceSwitcher(c *mui.Context) {
	tk := theme.TokensOf(c)
	items := make([]overlay.MenuItem, 0, len(workspaces))
	for _, w := range workspaces {
		it := overlay.MenuItem{Key: w, Label: w}
		if w == u.App.Workspace {
			it.Extra = "当前"
		}
		items = append(items, it)
	}
	chosen := overlay.DropdownMenu(c, &u.workspaceOpen, overlay.DropdownMenuProps{Items: items}, func(t mui.Element) {
		t.Row().Height(34).PaddingX(6).Gap(6).AlignItems(mui.Center).Radius(tk.BorderRadiusSM).
			TextColor(tk.Text).Cursor(mui.CursorPointer).Label("工作区")
		if t.Hovered() || u.workspaceOpen {
			t.Background(tk.FillTertiary)
		}
		t.Children(func() {
			display.Avatar(c, u.App.Workspace, display.AvatarProps{Size: 22})
			mui.Column(c).MinWidth(0).Children(func() {
				mui.Text(c, u.App.Workspace).FontSize(13).FontWeight(600).SingleLine()
				mui.Text(c, "免费版").FontSize(11).TextColor(tk.TextTertiary).SingleLine()
			})
			mui.Icon(c, lucide.ChevronDown()).FontSize(12).TextColor(tk.TextTertiary)
		})
	})
	if chosen != "" {
		u.App.Workspace = chosen
	}
}

func (u *UI) sessionSections(c *mui.Context) {
	now := c.Now()
	active := ""
	if s := u.App.Current(); s != nil {
		active = keyOf(s.ID)
	}
	for _, g := range groupSessions(u.App.Sessions, u.query, now) {
		if len(g.items) == 0 {
			continue
		}
		mui.Row(c).Padding(10, 8, 2, 8).Gap(6).AlignItems(mui.Center).Children(func() {
			display.Text(c, strings.ToUpper(g.label), display.TextProps{Type: display.TextSecondary, FontSize: 11, Strong: true})
			mui.Spacer(c)
			display.Text(c, strconv.Itoa(len(g.items)), display.TextProps{Type: display.TextSecondary, FontSize: 11})
		})
		items := make([]display.ListItem, 0, len(g.items))
		for _, s := range g.items {
			it := display.ListItem{
				Key:         keyOf(s.ID),
				Label:       s.Title,
				Description: s.Preview,
				Extra:       app.RelativeTime(now, s.Updated),
				Avatar:      func() { display.Avatar(c, s.Title, display.AvatarProps{Size: 22}) },
			}
			if s.Unread > 0 {
				it.Extra = "新"
			}
			items = append(items, it)
		}
		clicked := display.List(c, display.ListProps{
			Items: items, Active: active, Variant: display.ListBorderless, Compact: true,
		})
		if id, err := strconv.Atoi(strings.TrimPrefix(clicked, "s-")); clicked != "" && err == nil {
			u.App.SelectSession(id)
		}
	}
}

func keyOf(id int) string { return "s-" + strconv.Itoa(id) }

func groupSessions(all []*app.Session, query string, now time.Time) []sessionGroup {
	q := strings.ToLower(strings.TrimSpace(query))
	if q != "" {
		var hits []*app.Session
		for _, s := range all {
			if strings.Contains(strings.ToLower(s.Title), q) || strings.Contains(strings.ToLower(s.Preview), q) {
				hits = append(hits, s)
			}
		}
		return []sessionGroup{{label: "搜索结果", items: hits}}
	}
	var pinned, today, earlier []*app.Session
	for _, s := range all {
		switch {
		case s.Pinned:
			pinned = append(pinned, s)
		case now.Sub(s.Updated) < 24*time.Hour:
			today = append(today, s)
		default:
			earlier = append(earlier, s)
		}
	}
	return []sessionGroup{
		{label: "置顶", items: pinned},
		{label: "今天", items: today},
		{label: "更早", items: earlier},
	}
}

func (u *UI) sidebarFooter(c *mui.Context) {
	tk := theme.TokensOf(c)
	layout.Divider(c, layout.DividerProps{})
	mui.Row(c).Shrink(0).Padding(8).Gap(4).AlignItems(mui.Center).Children(func() {
		display.Avatar(c, u.App.Prefs.Name, display.AvatarProps{Size: 26})
		mui.Column(c).Grow(1).Basis(0).MinWidth(0).Padding(0, 0, 0, 4).Children(func() {
			mui.Text(c, u.App.Prefs.Name).FontSize(13).FontWeight(500).TextColor(tk.Text).SingleLine()
			mui.Text(c, "本地会话").FontSize(11).TextColor(tk.TextTertiary).SingleLine()
		})
		if action.ActionIcon(c, lucide.CircleHelp(), small("帮助")).Clicked() {
			c.OpenURL("https://mygo.lobehub.com")
		}
	})
}
