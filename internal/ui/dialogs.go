package ui

import (
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/form"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

func (u *UI) settingsDialog(c *mui.Context) {
	ok := feedback.Modal(c, &u.settingsUp, feedback.ModalProps{
		Title: "设置", Width: 560, OkText: "保存", CancelText: "取消",
	}, func() { u.settingsForm(c) })
	if !ok {
		return
	}
	v := u.settings.Values
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" {
		v.Name = "小杉"
	}
	u.App.Prefs = v
	u.settings.ResetTo(v)
	u.settingsUp = false
	feedback.ShowToast(c, feedback.ToastProps{Title: "设置已保存", Type: feedback.ToastSuccess})
}

func (u *UI) settingsForm(c *mui.Context) {
	f := u.settings
	form.View(c, f, form.Props[app.Prefs]{
		Variant: form.Borderless,
		Layout:  form.Vertical,
		Gap:     8,
	}, func() {
		form.Group(c, f, form.GroupProps{Title: "个人资料", Fixed: true}, func() {
			form.Field(c, f, form.FieldProps{
				Name: "name", Label: "显示名称",
				Desc:  "显示在你发送的消息旁",
				Value: f.Values.Name,
			}, func() {
				stretch(c, func() {
					field.Input(c, &f.Values.Name, field.InputProps{
						Variant: field.InputOutlined, Placeholder: "你的名字",
					}).Label("显示名称")
				})
			})
			form.Field(c, f, form.FieldProps{
				Name: "model", Label: "模型",
				Desc:  "这个窗口用来作答的助手",
				Value: f.Values.Model,
			}, func() {
				stretch(c, func() {
					field.Select(c, &f.Values.Model, field.SelectProps{
						Options: modelOptions(), Variant: field.InputOutlined,
					})
				})
			})
		})
		form.Group(c, f, form.GroupProps{Title: "输入与回复", Fixed: true}, func() {
			form.Field(c, f, form.FieldProps{
				Name: "streaming", Label: "流式输出回复",
				Desc:  "回答边生成边显示，一次几个字符",
				Value: f.Values.Streaming,
			}, func() {
				choiceSwitch(c, &f.Values.Streaming, "流式输出回复")
			})
			form.Field(c, f, form.FieldProps{
				Name: "enter", Label: "Enter 发送",
				Desc:  "关闭后 Enter 换行，用 Cmd 或 Ctrl + Enter 发送",
				Value: f.Values.EnterToSend,
			}, func() {
				choiceSwitch(c, &f.Values.EnterToSend, "Enter 发送")
			})
		})
	})
}

func modelOptions() []field.SelectOption {
	out := make([]field.SelectOption, 0, len(app.Models))
	for _, m := range app.Models {
		icon := lucide.Sparkles()
		switch m {
		case "Ada Flash":
			icon = lucide.Zap()
		case "Ada Local":
			icon = lucide.HardDrive()
		}
		out = append(out, field.SelectOption{Value: m, Label: m, Icon: icon})
	}
	return out
}

// stretch 让控件占满表单的右侧栏：field.Input 和 field.Select 返回的是内部元素，
// 不是包住它的那一行。
func stretch(c *mui.Context, control func()) {
	mui.Column(c).AlignSelf(mui.Stretch).AlignItems(mui.Stretch).Children(control)
}

func choiceSwitch(c *mui.Context, on *bool, label string) {
	choice.Switch(c, on, choice.SwitchProps{}).Label(label)
}

func (u *UI) renameDialog(c *mui.Context) {
	tk := theme.TokensOf(c)
	ok := feedback.Modal(c, &u.renameUp, feedback.ModalProps{
		Title: "重命名对话", Width: 440, OkText: "重命名",
	}, func() {
		mui.Column(c).Gap(8).Children(func() {
			mui.Text(c, "取一个在列表里好认的名字。").FontSize(13).TextColor(tk.TextSecondary)
			field.Input(c, &u.renameDraft, field.InputProps{
				Variant: field.InputOutlined, Placeholder: "对话名称",
			}).Label("对话名称")
		})
	})
	if !ok {
		return
	}
	title := strings.TrimSpace(u.renameDraft)
	s := u.App.Current()
	if title == "" || s == nil {
		return
	}
	u.App.Rename(s.ID, title)
	u.renameUp = false
	feedback.ShowToast(c, feedback.ToastProps{Title: "对话已重命名", Description: app.Clip(title, 48), Type: feedback.ToastSuccess})
}

func (u *UI) deleteDialog(c *mui.Context) {
	tk := theme.TokensOf(c)
	ok := feedback.Modal(c, &u.confirmDel, feedback.ModalProps{
		Title: "删除对话", Width: 440, OkText: "删除", CancelText: "保留",
	}, func() {
		mui.Column(c).Gap(8).Children(func() {
			s := u.App.Current()
			mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
				display.Avatar(c, s.Title, display.AvatarProps{Size: 26})
				mui.Text(c, s.Title).FontSize(14).FontWeight(500).TextColor(tk.Text).SingleLine()
			})
			mui.Text(c, "这条对话的消息会从这个窗口移除。撤销按钮会在提示里保留一会儿。").
				FontSize(13).TextColor(tk.TextTertiary)
		})
	})
	if !ok {
		return
	}
	s := u.App.Current()
	if s == nil {
		u.confirmDel = false
		return
	}
	at, deleted := u.App.DeleteSession(s.ID)
	u.confirmDel = false
	if !deleted {
		return
	}
	svc := c.Services()
	feedback.ShowToast(c, feedback.ToastProps{
		Title: "对话已删除", Description: app.Clip(s.Title, 48), Type: feedback.ToastInfo,
		Action: "撤销",
		OnAction: func() {
			u.App.RestoreSession(s, at)
			svc.Invalidate()
		},
	})
}
