package ui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/form"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/nav"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

func (u *UI) settingsDialog(c *mui.Context) {
	ok := feedback.Modal(c, &u.settingsUp, feedback.ModalProps{
		Title: "设置", Width: 580, OkText: "保存", CancelText: "取消",
	}, func() { u.settingsForm(c) })
	if !ok {
		return
	}
	v := u.settings.Values
	v.Name = strings.TrimSpace(v.Name)
	v.RuntimePath = strings.TrimSpace(v.RuntimePath)
	v.AgentPreset = strings.TrimSpace(v.AgentPreset)
	v.Provider = strings.ToLower(strings.TrimSpace(v.Provider))
	v.Endpoint = strings.TrimSpace(v.Endpoint)
	v.Model = strings.TrimSpace(v.Model)
	v.APIKey = strings.TrimSpace(v.APIKey)
	switch v.Provider {
	case "", "openai", "replay":
	default:
		u.settingsTab = 1
		u.settings.SetError("provider", "未知的供应商")
		c.Invalidate()
		return
	}
	if v.RuntimePath != "" {
		abs, err := filepath.Abs(v.RuntimePath)
		if err != nil {
			u.settingsTab = 0
			u.settings.SetError("runtime", "路径非法")
			c.Invalidate()
			return
		}
		if info, err := os.Stat(abs); err != nil || info.IsDir() {
			u.settingsTab = 0
			u.settings.SetError("runtime", "文件不存在")
			c.Invalidate()
			return
		}
		v.RuntimePath = abs
	}
	if v.AgentPreset != "" {
		abs, err := filepath.Abs(v.AgentPreset)
		if err != nil {
			u.settingsTab = 0
			u.settings.SetError("preset", "路径非法")
			c.Invalidate()
			return
		}
		if info, err := os.Stat(abs); err != nil || info.IsDir() {
			u.settingsTab = 0
			u.settings.SetError("preset", "文件不存在")
			c.Invalidate()
			return
		}
		v.AgentPreset = abs
	}
	if v.Provider == "openai" && v.Endpoint != "" {
		parsed, err := url.Parse(v.Endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			u.settingsTab = 1
			u.settings.SetError("endpoint", "必须是 http(s) 地址")
			c.Invalidate()
			return
		}
	}
	u.App.Prefs = v
	u.settings.ResetTo(v)
	u.settingsUp = false
	if err := u.App.SavePrefs(); err != nil {
		feedback.ShowToast(c, feedback.ToastProps{
			Title: "设置已保存", Description: "但写入配置文件失败: " + err.Error(), Type: feedback.ToastError,
		})
		return
	}
	feedback.ShowToast(c, feedback.ToastProps{Title: "设置已保存", Type: feedback.ToastSuccess})
}

func (u *UI) settingsForm(c *mui.Context) {
	nav.Tabs(c, &u.settingsTab, nav.TabsProps{
		Variant: nav.TabsSquare,
		Size:    theme.Small,
		Items: []nav.TabItem{
			{Label: "Agent", Icon: lucide.Bot()},
			{Label: "供应商", Icon: lucide.Cpu()},
			{Label: "常规", Icon: lucide.Settings()},
		},
	}, func(i int) { u.settingsPane(c, i) })
}

func (u *UI) settingsPane(c *mui.Context, tab int) {
	f := u.settings
	switch tab {
	case 0:
		form.View(c, f, form.Props[app.Prefs]{
			Variant: form.Borderless, Layout: form.Vertical, Gap: 8,
		}, func() {
			form.Group(c, f, form.GroupProps{
				Title: "ada_go_v2 Agent",
				Desc:  "留空时回落到环境变量 ADA_BOT_RUNTIME / ADA_GO_AGENT_PRESET",
				Fixed: true,
			}, func() {
				form.Field(c, f, form.FieldProps{
					Name: "runtime", Label: "Agent 路径",
					Desc:  "ada-rpc-agent 可执行文件；新机器人的工作区进程使用它启动",
					Value: f.Values.RuntimePath,
				}, func() {
					u.settingsPath(c, &f.Values.RuntimePath, "runtime", "Agent 路径", "选择 ada-rpc-agent.exe")
				})
				form.Field(c, f, form.FieldProps{
					Name: "preset", Label: "Agent preset",
					Desc:  "传给 runtime 的 agent.preset.json",
					Value: f.Values.AgentPreset,
				}, func() {
					u.settingsPath(c, &f.Values.AgentPreset, "preset", "Agent preset", "选择 agent.preset.json")
				})
			})
		})
	case 1:
		form.View(c, f, form.Props[app.Prefs]{
			Variant: form.Borderless, Layout: form.Vertical, Gap: 8,
		}, func() {
			form.Group(c, f, form.GroupProps{
				Title: "模型供应商",
				Desc:  "选择新机器人进程使用的 LLM 供应商；已运行的进程不受影响",
				Fixed: true,
			}, func() {
				form.Field(c, f, form.FieldProps{
					Name: "provider", Label: "供应商",
					Desc:  "跟随 preset 时由 agent.preset.json 决定",
					Value: f.Values.Provider,
				}, func() {
					field.Select(c, &f.Values.Provider, field.SelectProps{
						Variant: field.InputOutlined,
						Options: []field.SelectOption{
							{Value: "", Label: "跟随 preset"},
							{Value: "openai", Label: "OpenAI 兼容", Icon: lucide.Cloud()},
							{Value: "replay", Label: "Replay（离线脚本）", Icon: lucide.Play()},
						},
					}).Label("供应商")
				})
				switch f.Values.Provider {
				case "openai":
					form.Field(c, f, form.FieldProps{
						Name: "endpoint", Label: "Endpoint",
						Desc:  "OpenAI 兼容的 /v1 地址；留空用官方端点",
						Value: f.Values.Endpoint,
					}, func() {
						field.Input(c, &f.Values.Endpoint, field.InputProps{
							Variant: field.InputOutlined, Placeholder: "https://api.openai.com/v1",
						}).Label("Endpoint")
					})
					form.Field(c, f, form.FieldProps{
						Name: "model", Label: "模型",
						Desc:  "请求时使用的模型 ID；留空用 gpt-4o-mini",
						Value: f.Values.Model,
					}, func() {
						field.Input(c, &f.Values.Model, field.InputProps{
							Variant: field.InputOutlined, Placeholder: "gpt-4o-mini",
						}).Label("模型")
					})
					form.Field(c, f, form.FieldProps{
						Name: "apiKey", Label: "API Key",
						Desc:  "保存在本机 config.json；留空时读 ADA_LLM_API_KEY",
						Value: f.Values.APIKey,
					}, func() { u.apiKeyInput(c, &f.Values.APIKey) })
				case "replay":
					display.Text(c, "Replay 使用 preset 里的离线脚本，不会访问网络。", display.TextProps{
						Type: display.TextSecondary, FontSize: 12,
					})
				}
			})
		})
	case 2:
		form.View(c, f, form.Props[app.Prefs]{
			Variant: form.Borderless, Layout: form.Vertical, Gap: 8,
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
			})
			form.Group(c, f, form.GroupProps{Title: "输入", Fixed: true}, func() {
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
}

// apiKeyInput 默认遮住密钥，并提供临时显示按钮。
func (u *UI) apiKeyInput(c *mui.Context, value *string) {
	mui.Row(c).Gap(4).AlignItems(mui.Center).Children(func() {
		mui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			input := field.Input(c, value, field.InputProps{
				Variant: field.InputOutlined, Placeholder: "sk-…",
			}).Label("API Key")
			if !u.showKey {
				input.Password()
			}
		})
		icon, title := lucide.Eye(), "显示 API Key"
		if u.showKey {
			icon, title = lucide.EyeOff(), "隐藏 API Key"
		}
		if action.ActionIcon(c, icon, small(title)).Clicked() {
			u.showKey = !u.showKey
		}
	})
}

// settingsPath 是设置页的一行路径：输入框 + 系统文件选择器。
func (u *UI) settingsPath(c *mui.Context, value *string, kind, label, placeholder string) {
	mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
		mui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			field.Input(c, value, field.InputProps{
				Variant: field.InputOutlined, Placeholder: placeholder,
			}).Label(label)
		})
		text := "选择文件"
		if u.pathPicking == kind {
			text = "选择中…"
		}
		if action.Button(c, text, action.ButtonProps{Size: theme.Small}).Clicked() && u.pathPicking == "" {
			u.pickSettingsFile(kind, value)
		}
	})
}

// pickSettingsFile opens the native file picker off the UI thread and writes
// the chosen path back into the form.
func (u *UI) pickSettingsFile(kind string, dst *string) {
	if u.pathPicking != "" {
		return
	}
	u.pathPicking = kind
	pick := u.pickFile
	if pick == nil {
		pick = pickAgentFile
	}
	apply := func(path string, err error) {
		u.pathPicking = ""
		if err == nil && path != "" {
			*dst = path
		}
	}
	dispatch := u.dispatch
	if dispatch == nil {
		path, err := pick(kind)
		apply(path, err)
		return
	}
	go func() {
		path, err := pick(kind)
		dispatch(func() { apply(path, err) })
	}()
}

func pickAgentFile(kind string) (string, error) {
	var filters []mygo.FileFilter
	switch kind {
	case "runtime":
		if runtime.GOOS == "windows" {
			filters = []mygo.FileFilter{{Name: "可执行文件", Extensions: []string{"exe"}}}
		}
	case "preset":
		filters = []mygo.FileFilter{{Name: "Agent preset", Extensions: []string{"json"}}}
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Filters: filters})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
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

// approvalDialog 显示 runtime 发来的升级审批请求：允许/拒绝都会把结果回给 runtime，
// 超时未答复时 runtime 自己按拒绝处理（fail-closed）。
func (u *UI) approvalDialog(c *mui.Context) {
	if len(u.App.Approvals) == 0 {
		u.approvalUp = false
		return
	}
	ap := u.App.Approvals[0]
	tk := theme.TokensOf(c)
	u.approvalUp = true
	ok := feedback.Modal(c, &u.approvalUp, feedback.ModalProps{
		Title: "需要审批", Width: 480, OkText: "允许一次", CancelText: "拒绝",
	}, func() {
		botName := "机器人"
		if b := u.App.BotByID(ap.BotID); b != nil {
			botName = b.Name
		}
		mui.Column(c).Gap(6).Children(func() {
			mui.Text(c, ap.Reason).FontSize(14).TextColor(tk.Text)
			detail := botName + " · " + ap.Source
			if ap.Target != "" {
				detail += " · " + ap.Target
			}
			mui.Text(c, detail).FontSize(12).TextColor(tk.TextTertiary)
			if ap.Timeout > 0 {
				mui.Text(c, fmt.Sprintf("超时未答复会按拒绝处理（%s）。", ap.Timeout.Round(time.Second))).
					FontSize(12).TextColor(tk.TextTertiary)
			}
		})
	})
	decide := func(allow bool) {
		reason := "用户允许"
		if !allow {
			reason = "用户拒绝"
		}
		if err := u.App.DecideApproval(ap.ID, allow, reason); err != nil {
			feedback.ShowToast(c, feedback.ToastProps{
				Title: "审批未送达", Description: err.Error(), Type: feedback.ToastError,
			})
		}
		u.approvalUp = false
		c.Invalidate()
	}
	if ok {
		decide(true)
		return
	}
	// 取消/关闭按钮会把开关置回 false；仍为 true 表示面板还开着。
	if !u.approvalUp {
		decide(false)
	}
}
