package ui

import (
	"net/url"
	"strconv"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/nav"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"ada_chat/internal/app"
)

// thinkingTags 是思考级别的默认标签;输入框仍可写自定义值(动态 tag)。
var thinkingTags = []string{"low", "medium", "high"}

// —— 供应商管理:左侧列表 + 右侧[供应商信息][模型列表] ——

func (u *UI) providerManager(c *mui.Context) {
	tk := theme.TokensOf(c)
	mui.Column(c).Gap(8).Children(func() {
		mui.Row(c).Gap(0).MinHeight(430).AlignItems(mui.Stretch).Children(func() {
			u.providerSidebar(c, tk)
			mui.Column(c).Grow(1).Basis(0).Padding(0, 0, 0, 4).Children(func() {
				u.providerDetail(c, tk)
			})
		})
		if u.providerErr != "" {
			mui.Text(c, u.providerErr).FontSize(11).TextColor(tk.Error)
		}
	})
}

func (u *UI) providerSidebar(c *mui.Context, tk *theme.Tokens) {
	mui.Column(c).Width(236).Shrink(0).Padding(0, 12, 0, 0).Gap(6).Children(func() {
		mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "供应商").FontSize(13).FontWeight(600).TextColor(tk.Text)
			mui.Spacer(c)
			if action.ActionIcon(c, lucide.Plus(), small("添加供应商")).Clicked() {
				u.addProviderDraft()
			}
		})
		mui.Column(c).Gap(2).Children(func() {
			for i := range u.providerList {
				i := i
				p := u.providerList[i]
				mui.Row(c).Gap(4).AlignItems(mui.Center).Children(func() {
					name := p.Name
					if name == "" {
						name = "未命名供应商"
					}
					row := mui.ButtonBase(c).Label(name).Row().Gap(6).AlignItems(mui.Center).
						Grow(1).Basis(0).MinWidth(0).Padding(6, 8).Radius(tk.BorderRadiusSM).
						Cursor(mui.CursorPointer)
					if i == u.providerIdx {
						row.Background(tk.FillSecondary)
					} else if row.Hovered() {
						row.Background(tk.FillTertiary)
					}
					row.Children(func() {
						mui.Column(c).Gap(1).MinWidth(0).Children(func() {
							mui.Text(c, name).FontSize(13).TextColor(tk.Text).SingleLine()
							display.Text(c, providerSubtitle(p), display.TextProps{Type: display.TextSecondary, FontSize: 11}).SingleLine()
						})
					})
					if row.Clicked() {
						u.providerIdx = i
						u.providerTab = 0
						u.modelEditor = false
					}
					activeIcon := lucide.Star()
					fg := tk.TextTertiary
					if p.ID == u.providerActive {
						fg = tk.Primary
					}
					_ = fg
					star := action.ActionIcon(c, activeIcon, small("设为当前供应商"))
					if star.Clicked() {
						u.providerActive = p.ID
					}
				})
			}
			if len(u.providerList) == 0 {
				mui.Text(c, "还没有供应商,点右上角 + 添加").FontSize(12).TextColor(tk.TextTertiary)
			}
		})
	})
}

func providerSubtitle(p app.Provider) string {
	kind := p.Kind
	if kind == "" {
		kind = "跟随 preset"
	}
	n := len(p.Models)
	return kind + " · " + strconv.Itoa(n) + " 个模型"
}

func (u *UI) providerDetail(c *mui.Context, tk *theme.Tokens) {
	if len(u.providerList) == 0 || u.providerIdx < 0 || u.providerIdx >= len(u.providerList) {
		mui.Column(c).Grow(1).Center().Children(func() {
			display.Text(c, "先添加一个供应商", display.TextProps{Type: display.TextSecondary, FontSize: 13})
		})
		return
	}
	p := &u.providerList[u.providerIdx]
	nav.Tabs(c, &u.providerTab, nav.TabsProps{
		Variant: nav.TabsSquare,
		Size:    theme.Small,
		Items: []nav.TabItem{
			{Label: "供应商信息", Icon: lucide.Info()},
			{Label: "模型列表", Icon: lucide.Cpu()},
		},
	}, func(tab int) {
		if tab == 1 {
			u.providerModelsPane(c, tk, p)
		} else {
			u.providerInfoPane(c, tk, p)
		}
	})
}

func (u *UI) providerInfoPane(c *mui.Context, tk *theme.Tokens, p *app.Provider) {
	mui.Column(c).Gap(10).Padding(2, 2, 0, 0).Children(func() {
		mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "编辑供应商信息;保存后,新启动的机器人进程才使用它").FontSize(11).TextColor(tk.TextTertiary)
			mui.Spacer(c)
			if action.Button(c, "删除供应商", action.ButtonProps{Size: theme.Small}).Clicked() {
				u.providerList = append(u.providerList[:u.providerIdx], u.providerList[u.providerIdx+1:]...)
				if u.providerIdx >= len(u.providerList) && u.providerIdx > 0 {
					u.providerIdx = len(u.providerList) - 1
				}
				if u.providerActive == p.ID {
					u.providerActive = ""
					if len(u.providerList) > 0 {
						u.providerActive = u.providerList[0].ID
					}
				}
			}
		})
		botField(c, tk, "名称", true, "在供应商列表里显示的名字", func() {
			field.Input(c, &p.Name, field.InputProps{
				Variant: field.InputOutlined, Placeholder: "例如：DeepSeek",
			}).Label("名称")
		})
		botField(c, tk, "类型", false, "跟随 preset 时由 agent.preset.json 决定", func() {
			field.Select(c, &p.Kind, field.SelectProps{
				Variant: field.InputOutlined,
				Options: []field.SelectOption{
					{Value: "", Label: "跟随 preset"},
					{Value: "openai", Label: "OpenAI 兼容", Icon: lucide.Cloud()},
					{Value: "replay", Label: "Replay(离线脚本)", Icon: lucide.Play()},
				},
			}).Label("类型")
		})
		if p.Kind == "openai" || p.Kind == "" {
			botField(c, tk, "Endpoint", false, "OpenAI 兼容的 /v1 地址;留空用官方端点", func() {
				field.Input(c, &p.Endpoint, field.InputProps{
					Variant: field.InputOutlined, Placeholder: "https://api.deepseek.com",
				}).Label("Endpoint")
			})
			botField(c, tk, "API Key", false, "保存在本机 config.json;留空时读 ADA_LLM_API_KEY", func() {
				u.apiKeyInput(c, &p.APIKey)
			})
		} else {
			display.Text(c, "Replay 使用 preset 里的离线脚本,不会访问网络。", display.TextProps{
				Type: display.TextSecondary, FontSize: 12,
			})
		}
		mui.Row(c).Gap(8).Children(func() {
			if p.ID == u.providerActive {
				display.Tag(c, "当前供应商", display.TagProps{Variant: display.TagOutlined, Color: display.TagGreen, Size: theme.Small})
			} else if action.Button(c, "设为当前供应商", action.ButtonProps{Size: theme.Small}).Clicked() {
				u.providerActive = p.ID
			}
		})
	})
}

func (u *UI) providerModelsPane(c *mui.Context, tk *theme.Tokens, p *app.Provider) {
	remove := -1
	edit := -1
	mui.Column(c).Gap(8).Padding(2, 2, 0, 0).Children(func() {
		mui.Row(c).Gap(8).AlignItems(mui.Center).Children(func() {
			mui.Text(c, "模型列表:每个模型可以带上下文长度、最大输出、摘要阈值与思考级别").FontSize(11).TextColor(tk.TextTertiary)
			mui.Spacer(c)
			if action.Button(c, "添加模型", action.ButtonProps{Size: theme.Small}).Clicked() {
				u.openModelEditor(-1)
			}
		})
		mui.Column(c).Gap(2).Children(func() {
			for i := range p.Models {
				i := i
				m := p.Models[i]
				mui.Row(c).Gap(4).AlignItems(mui.Center).Children(func() {
					row := mui.ButtonBase(c).Label(m.ID).Row().Gap(8).AlignItems(mui.Center).
						Grow(1).Basis(0).MinWidth(0).Padding(6, 8).Radius(tk.BorderRadiusSM).Cursor(mui.CursorPointer)
					if row.Hovered() {
						row.Background(tk.FillTertiary)
					}
					row.Children(func() {
						mui.Column(c).Gap(1).MinWidth(0).Children(func() {
							title := m.ID
							if title == "" {
								title = "未命名模型"
							}
							mui.Text(c, title).FontSize(13).TextColor(tk.Text).SingleLine()
							display.Text(c, modelSummary(m), display.TextProps{Type: display.TextSecondary, FontSize: 11}).SingleLine()
						})
					})
					if row.Clicked() {
						p.Model = m.ID
					}
					fg := tk.TextTertiary
					if p.Model == m.ID {
						fg = tk.Primary
					}
					_ = fg
					if action.ActionIcon(c, lucide.Star(), small("设为当前模型")).Clicked() {
						p.Model = m.ID
					}
					if action.ActionIcon(c, lucide.Pencil(), small("编辑模型")).Clicked() {
						edit = i
					}
					if action.ActionIcon(c, lucide.Trash2(), small("删除模型")).Clicked() {
						remove = i
					}
				})
			}
			if len(p.Models) == 0 {
				mui.Text(c, "还没有模型,点「添加模型」添加第一个").FontSize(12).TextColor(tk.TextTertiary)
			}
		})
		if u.modelEditor {
			u.modelEditorCard(c, tk, p)
		}
	})
	if remove >= 0 {
		p.Models = append(p.Models[:remove], p.Models[remove+1:]...)
		if p.Model == "" && len(p.Models) > 0 {
			p.Model = p.Models[0].ID
		}
		if u.modelEdit == remove {
			u.modelEditor = false
		}
	}
	if edit >= 0 {
		u.openModelEditor(edit)
	}
}

func modelSummary(m app.ModelSpec) string {
	parts := make([]string, 0, 4)
	if m.ContextWindow > 0 {
		parts = append(parts, "上下文 "+strconv.Itoa(m.ContextWindow))
	}
	if m.MaxOutput > 0 {
		parts = append(parts, "最大输出 "+strconv.Itoa(m.MaxOutput))
	}
	if m.SummaryThreshold > 0 {
		parts = append(parts, "摘要阈值 "+strconv.Itoa(m.SummaryThreshold)+"%")
	}
	if m.Thinking != "" {
		parts = append(parts, "思考 "+m.Thinking)
	}
	if len(parts) == 0 {
		return "未设置参数"
	}
	return strings.Join(parts, " · ")
}

func (u *UI) openModelEditor(index int) {
	u.modelEditor = true
	u.modelEdit = index
	u.modelErr = ""
	p := &u.providerList[u.providerIdx]
	m := app.ModelSpec{}
	if index >= 0 && index < len(p.Models) {
		m = p.Models[index]
	}
	u.modelID = m.ID
	u.modelThinking = m.Thinking
	u.modelCtx = intText(m.ContextWindow)
	u.modelMax = intText(m.MaxOutput)
	u.modelSummary = intText(m.SummaryThreshold)
}

func intText(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func (u *UI) modelEditorCard(c *mui.Context, tk *theme.Tokens, p *app.Provider) {
	mui.Column(c).Gap(8).Padding(8).Radius(tk.BorderRadiusSM).Background(tk.FillTertiary).Children(func() {
		label := func(text string) {
			mui.Text(c, text).FontSize(11).TextColor(tk.TextTertiary)
		}
		mui.Row(c).Gap(8).AlignItems(mui.Stretch).Children(func() {
			mui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				label("模型 ID")
				field.Input(c, &u.modelID, field.InputProps{
					Variant: field.InputOutlined, Placeholder: "deepseek-chat",
				}).Label("模型 ID")
			})
			mui.Column(c).Width(118).Children(func() {
				label("上下文长度")
				field.Input(c, &u.modelCtx, field.InputProps{
					Variant: field.InputOutlined, Placeholder: "131072",
				}).Label("上下文长度")
			})
			mui.Column(c).Width(108).Children(func() {
				label("最大输出")
				field.Input(c, &u.modelMax, field.InputProps{
					Variant: field.InputOutlined, Placeholder: "8192",
				}).Label("最大输出")
			})
			mui.Column(c).Width(108).Children(func() {
				label("摘要阈值 %")
				field.Input(c, &u.modelSummary, field.InputProps{
					Variant: field.InputOutlined, Placeholder: "80",
				}).Label("摘要阈值")
			})
		})
		mui.Column(c).Gap(4).Children(func() {
			label("思考级别(可直接输入自定义标签)")
			mui.Row(c).Gap(6).AlignItems(mui.Center).Children(func() {
				for _, tag := range thinkingTags {
					tag := tag
					b := mui.ButtonBase(c).Label(tag).Padding(3, 10).Radius(999).FontSize(11).
						Cursor(mui.CursorPointer)
					bg, fg := tk.FillSecondary, tk.TextSecondary
					if u.modelThinking == tag {
						bg, fg = tk.PrimaryBg, tk.Primary
					}
					b.Background(bg).TextColor(fg).Children(func() {
						mui.Text(c, tag).FontSize(11).TextColor(fg)
					})
					if b.Clicked() {
						u.modelThinking = tag
					}
				}
				mui.Column(c).Width(150).Children(func() {
					field.Input(c, &u.modelThinking, field.InputProps{
						Variant: field.InputOutlined, Placeholder: "自定义",
					}).Label("自定义思考级别")
				})
			})
		})
		if u.modelErr != "" {
			mui.Text(c, u.modelErr).FontSize(11).TextColor(tk.Error)
		}
		mui.Row(c).Gap(8).Children(func() {
			if action.Button(c, "保存模型", action.ButtonProps{Size: theme.Small}).Clicked() {
				u.commitModel(p)
			}
			if action.Button(c, "取消", action.ButtonProps{Size: theme.Small}).Clicked() {
				u.modelEditor = false
			}
		})
	})
}

func (u *UI) commitModel(p *app.Provider) {
	id := strings.TrimSpace(u.modelID)
	if id == "" {
		u.modelErr = "模型 ID 不能为空"
		return
	}
	for i := range p.Models {
		if p.Models[i].ID == id && i != u.modelEdit {
			u.modelErr = "已有同名模型: " + id
			return
		}
	}
	ctx, err := parseOptional(u.modelCtx)
	if err != "" {
		u.modelErr = "上下文长度" + err
		return
	}
	maxOut, err := parseOptional(u.modelMax)
	if err != "" {
		u.modelErr = "最大输出" + err
		return
	}
	summary, err := parseOptional(u.modelSummary)
	if err != "" {
		u.modelErr = "摘要阈值" + err
		return
	}
	m := app.ModelSpec{
		ID: id, ContextWindow: ctx, MaxOutput: maxOut,
		SummaryThreshold: summary, Thinking: strings.TrimSpace(u.modelThinking),
	}
	if u.modelEdit >= 0 && u.modelEdit < len(p.Models) {
		p.Models[u.modelEdit] = m
	} else {
		p.Models = append(p.Models, m)
	}
	if p.Model == "" {
		p.Model = id
	}
	u.modelEditor = false
	u.modelErr = ""
}

// parseOptional 接受空串(0);负数与非数字给出可读错误。
func parseOptional(s string) (int, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ""
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, "必须是非负整数"
	}
	return n, ""
}

func (u *UI) addProviderDraft() {
	id := "provider-" + strconv.Itoa(len(u.providerList)+1)
	for u.providerHasID(id) {
		id += "-x"
	}
	u.providerList = append(u.providerList, app.Provider{
		ID: id, Name: "供应商 " + strconv.Itoa(len(u.providerList)+1), Kind: "openai",
	})
	u.providerIdx = len(u.providerList) - 1
	u.providerActive = id
	u.providerTab = 0
	u.modelEditor = false
}

func (u *UI) providerHasID(id string) bool {
	for _, p := range u.providerList {
		if p.ID == id {
			return true
		}
	}
	return false
}

// saveProviders 校验整份供应商列表,写回偏好并同步旧的单供应商字段。
func (u *UI) saveProviders(c *mui.Context) {
	tk := theme.TokensOf(c)
	_ = tk
	if len(u.providerList) == 0 {
		u.providerErr = "至少保留一个供应商"
		c.Invalidate()
		return
	}
	for i := range u.providerList {
		p := &u.providerList[i]
		p.Name = strings.TrimSpace(p.Name)
		p.Endpoint = strings.TrimSpace(p.Endpoint)
		p.APIKey = strings.TrimSpace(p.APIKey)
		if p.Name == "" {
			u.providerErr = "第 " + strconv.Itoa(i+1) + " 个供应商缺少名称"
			c.Invalidate()
			return
		}
		switch p.Kind {
		case "", "openai", "replay":
		default:
			u.providerErr = "供应商 " + p.Name + " 的类型未知"
			c.Invalidate()
			return
		}
		if p.Kind == "openai" && p.Endpoint != "" {
			if parsed, err := url.Parse(p.Endpoint); err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				u.providerErr = "供应商 " + p.Name + " 的 Endpoint 必须是 http(s) 地址"
				c.Invalidate()
				return
			}
		}
		for j := range p.Models {
			p.Models[j].ID = strings.TrimSpace(p.Models[j].ID)
			if p.Models[j].ID == "" {
				u.providerErr = "供应商 " + p.Name + " 有空模型 ID"
				c.Invalidate()
				return
			}
		}
		if p.ID == "" {
			p.ID = "provider-" + strconv.Itoa(i+1)
		}
	}
	if !u.providerHasID(u.providerActive) {
		u.providerActive = u.providerList[0].ID
	}
	u.App.Prefs.Providers = u.providerList
	u.App.Prefs.ActiveProvider = u.providerActive
	u.App.Prefs.SyncLegacy()
	u.providerUp = false
	u.providerErr = ""
	if err := u.App.SavePrefs(); err != nil {
		feedback.ShowToast(c, feedback.ToastProps{Title: "供应商已保存", Description: "但写入配置文件失败: " + err.Error(), Type: feedback.ToastError})
		return
	}
	feedback.ShowToast(c, feedback.ToastProps{Title: "供应商已保存", Type: feedback.ToastSuccess})
}

// cloneProviders 深拷贝模型列表;取消编辑时不影响已保存的偏好。
func cloneProviders(in []app.Provider) []app.Provider {
	out := make([]app.Provider, 0, len(in))
	for _, p := range in {
		cp := p
		cp.Models = append([]app.ModelSpec(nil), p.Models...)
		out = append(out, cp)
	}
	return out
}
