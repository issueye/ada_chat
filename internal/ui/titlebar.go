package ui

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/overlay"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

func small(title string) action.ActionIconProps {
	return action.ActionIconProps{Size: theme.Small, Title: title}
}

func hotkeyBinding(keys string) (mui.Modifiers, mui.Key, bool) {
	return display.HotkeyBinding(keys, display.HotkeyAuto)
}

func styleChip(t mui.Element, tk *theme.Tokens, open bool, height, radius float32) {
	t.Row().Height(height).PaddingX(8).Gap(6).AlignItems(mui.Center).Radius(radius).FontSize(12).
		TextColor(tk.TextSecondary).Cursor(mui.CursorPointer)
	if t.Hovered() || open {
		t.Background(tk.FillSecondary).TextColor(tk.Text)
	}
}

// titleBar 在系统窗口按钮下方画出窗口自己的标题栏。
func (u *UI) titleBar(c *mui.Context) {
	tk := theme.TokensOf(c)
	bar := c.TitleBar()
	height := float32(TitleBarHeight)
	if bar.Height > 0 {
		height = bar.Height
	}
	inset := float32(10)
	if bar.Left > 0 {
		inset = bar.Left + 8
	}
	right := float32(10)
	if bar.Right > 0 {
		right = bar.Right + 8
	}

	mui.Row(c).Height(height).Shrink(0).AlignItems(mui.Center).
		Padding(0, right, 0, inset).DragWindow().Children(func() {
		icon := lucide.PanelLeftClose()
		title := "收起侧栏"
		if !u.sidebarOpen {
			icon, title = lucide.PanelLeftOpen(), "展开侧栏"
		}
		if action.ActionIcon(c, icon, small(title)).Clicked() {
			u.sidebarOpen = !u.sidebarOpen
		}

		mui.Row(c).Margin(0, 0, 0, 10).Gap(8).AlignItems(mui.Center).Children(func() {
			mui.Box(c).Size(24, 24).Children(func() {
				display.AgentAvatar(c, display.AgentAvatarProps{Seed: "ada", Name: "Ada", Size: 24})
			})
			mui.Text(c, "Ada Chat").FontSize(13).FontWeight(600).SingleLine()
			display.Tag(c, "本地", display.TagProps{Variant: display.TagBorderless, Color: display.TagGray, Size: theme.Small})
		})

		mui.Spacer(c)

		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			u.modelChip(c, tk)
			if action.ActionIcon(c, lucide.SlidersHorizontal(), small("设置")).Clicked() {
				u.openSettings()
			}
		})
	})
}

func (u *UI) modelChip(c *mui.Context, tk *theme.Tokens) {
	items := make([]overlay.MenuItem, 0, len(app.Models))
	for _, m := range app.Models {
		it := overlay.MenuItem{Key: m, Label: m, Icon: lucide.Sparkles()}
		if m == u.App.Prefs.Model {
			it.Extra = "当前"
		}
		items = append(items, it)
	}
	chosen := overlay.DropdownMenu(c, &u.modelOpen, overlay.DropdownMenuProps{Items: items}, func(t mui.Element) {
		styleChip(t.Label("模型"), tk, u.modelOpen, 28, tk.BorderRadiusSM)
		t.Children(func() {
			mui.Icon(c, lucide.Sparkles()).FontSize(13)
			mui.Text(c, u.App.Prefs.Model).MaxWidth(160).SingleLine()
			mui.Icon(c, lucide.ChevronDown()).FontSize(12)
		})
	})
	if chosen != "" {
		u.App.SetModel(chosen)
	}
}

// openSettings 打开前先把已保存的偏好放回表单。
func (u *UI) openSettings() {
	u.settings.ResetTo(u.App.Prefs)
	u.settingsUp = true
}
