package ui

import (
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

func small(title string) action.ActionIconProps {
	return action.ActionIconProps{Size: theme.Small, Title: title}
}

// titleBar 在系统窗口按钮下方画出窗口自己的标题栏。
func (u *UI) titleBar(c *mui.Context) {
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

		if action.ActionIcon(c, lucide.SlidersHorizontal(), small("设置")).Clicked() {
			u.openSettings()
		}
	})
}

// openSettings 打开常规设置:先把已保存的偏好放回表单,并关掉其它弹窗/菜单。
func (u *UI) openSettings() {
	u.settings.ResetTo(u.App.Prefs)
	u.showKey = false
	u.userMenu = false
	u.agentUp, u.providerUp = false, false
	u.settingsUp = true
}
