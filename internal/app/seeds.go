package app

import (
	"strings"
	"time"
)

// seed 填入窗口打开时看到的对话。
func (a *App) seed(now time.Time) {
	win := &Session{Title: "Windows 上的原生聊天", Pinned: true, Updated: now.Add(-8 * time.Minute)}
	winUser1 := a.msg(RoleUser, now.Add(-22*time.Minute), "我想要一个在 Windows 上有原生手感的聊天应用，不要 webview。从哪儿开始？")
	winReply1 := a.msg(RoleAssistant, now.Add(-21*time.Minute), seedOpening())
	winUser2 := a.msg(RoleUser, now.Add(-14*time.Minute), "侧栏和对话区 Lobe 也提供吗？")
	winReply2 := a.msg(RoleAssistant, now.Add(-13*time.Minute), seedShell())
	winReply2.Tools = []*ToolCall{
		a.newTool("search_docs", `{"query":"Lobe 布局与流式渲染","scope":"docs"}`,
			"找到 3 条文档：`DraggablePanel` 可拖拽调整宽度；`display.List` 的行可带头像与副标题；`ui.List` 的 `FollowEnd` 跟随末尾。",
			480*time.Millisecond, true),
	}
	win.Messages = []*Message{winUser1, winReply1, winUser2, winReply2}

	design := &Session{Title: "Ada 的设计变量", Pinned: true, Updated: now.Add(-2 * time.Hour)}
	designReply := a.msg(RoleAssistant, now.Add(-2*time.Hour), seedTokens())
	designReply.Tools = []*ToolCall{
		a.newTool("read_file", `{"path":"tokens.go","lines":"12-48"}`,
			"读取 `tokens.go` 第 12–48 行：浅色与深色的 `BgLayout`、`Border`、`TextSecondary` 都在这里定义。",
			310*time.Millisecond, false),
	}
	design.Messages = []*Message{
		a.msg(RoleUser, now.Add(-2*time.Hour-10*time.Minute), "浅色和深色都要耐看，界面该用哪些颜色？"),
		designReply,
	}

	streaming := &Session{Title: "流式渲染 Markdown", Updated: now.Add(-25 * time.Minute)}
	streaming.Messages = []*Message{
		a.msg(RoleUser, now.Add(-26*time.Minute), "回答是一个字一个字来的，怎么渲染才不跳？"),
		a.msg(RoleAssistant, now.Add(-25*time.Minute), seedStreaming()),
	}

	shortcuts := &Session{Title: "键盘快捷键", Unread: 1, Updated: now.Add(-3 * time.Hour)}
	shortcuts.Messages = []*Message{
		a.msg(RoleUser, now.Add(-3*time.Hour-6*time.Minute), "第一天该定下哪些快捷键？"),
		a.msg(RoleAssistant, now.Add(-3*time.Hour), seedShortcuts()),
	}

	release := &Session{Title: "发布前清单", Updated: now.Add(-30 * time.Hour)}
	releaseUser := a.msg(RoleUser, now.Add(-30*time.Hour-4*time.Minute), "beta 发布前，给我一份检查清单。")
	releaseReply := a.msg(RoleAssistant, now.Add(-30*time.Hour), seedRelease())
	releaseReply.Tools = []*ToolCall{
		a.newTool("run_tests", `{"package":"./..."}`,
			"`go test ./...`：21 个用例通过，用时 13.9s。", 1200*time.Millisecond, true),
	}
	release.Messages = []*Message{releaseUser, releaseReply}

	review := &Session{Title: "周五前清完评审队列", Updated: now.Add(-3*24*time.Hour - 2*time.Hour)}
	reviewUser := a.msg(RoleUser, now.Add(-3*24*time.Hour-3*time.Hour), "帮我在周五前清掉评审队列。")
	reviewReply := a.msg(RoleAssistant, now.Add(-3*24*time.Hour-2*time.Hour), seedReview())
	reviewReply.Tools = []*ToolCall{
		a.failedTool("list_prs", `{"state":"open","remote":"origin"}`,
			"拉取失败：这个工作区没有配置远程仓库（origin）。", 640*time.Millisecond),
	}
	review.Messages = []*Message{reviewUser, reviewReply}

	a.Sessions = []*Session{win, design, streaming, shortcuts, release, review}
	for _, s := range a.Sessions {
		s.ID = a.id()
		if n := len(s.Messages); n > 0 {
			s.Preview = Clip(s.Messages[n-1].Text, 88)
		}
	}
	a.ActiveID = a.Sessions[0].ID
}

// ReplyText 是助手的固定回答：应用不带网络客户端，所以按问题挑一段回答，
// 再像模型输出一样逐字流式送出去。
func ReplyText(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case contains(lower, "主题", "配色", "颜色", "设计变量", "token", "theme", "color", "dark"):
		return seedTokens()
	case contains(lower, "快捷键", "键盘", "shortcut", "keyboard", "hotkey"):
		return seedShortcuts()
	case contains(lower, "流式", "markdown", "一个字", "stream"):
		return seedStreaming()
	case contains(lower, "评审", "队列", "review", "queue"):
		return seedReview()
	}
	return "关于 **" + Clip(prompt, 60) + "**，我会这么拆：\n\n" +
		"1. **收窄范围**，只留下能证明想法的那一个界面或流程。\n" +
		"2. **写成状态**：窗口显示什么，是状态的函数。\n" +
		"3. **先写无头测试**，再看效果。\n\n" +
		"想深入哪一步都可以告诉我，或者把代码贴过来，我帮你读。"
}

// chunksOf 把回答切成模型流式输出那样的几段：一次几个字符，空格保留。
func chunksOf(text string, size int) []string {
	if size <= 0 {
		size = 1
	}
	r := []rune(text)
	var out []string
	for i := 0; i < len(r); i += size {
		out = append(out, string(r[i:min(i+size, len(r))]))
	}
	return out
}

func mdCode(lang, src string) string {
	return "```" + lang + "\n" + src + "\n```"
}

func seedOpening() string {
	return "先从 **MyGo** 起窗口，再用 **Lobe MyGo** 定设计语言。两者都在纯 Go 里用 GPU 绘制，" +
		"整条链路里没有 HTML、JavaScript，也没有 webview。\n\n" +
		"- `mygo.App` 掌管进程；窗口只在 `WhenReady` 里创建一次。\n" +
		"- 视图是每帧都会调用的函数，由状态重建。\n" +
		"- Lobe 提供 Lobe UI 的 token 和组件：`pkg/theme`、`pkg/action`、`pkg/field` 等等。\n\n" +
		"第一帧大概是这样：\n\n" +
		mdCode("go", `func view(c *ui.Context) {
	theme.Apply(c)
	ui.Column(c).Fill().Background(theme.TokensOf(c).BgLayout).Children(func() {
		ui.Text(c, "你好，Ada")
	})
}`) +
		"\n\n接下来界面只有三块：标题栏、可拖动的侧栏和对话区。"
}

func seedShell() string {
	return "有。界面骨架用的是 Lobe 的布局组件，而不是手画：\n\n" +
		"1. `layout.DraggablePanel` 贴在左边缘：侧栏可以拖拽调整宽度，也能收成窄条。\n" +
		"2. `display.List` 负责每条对话：头像、预览、时间，并标出当前打开的那条。\n" +
		"3. `ui.List` 配 `ListState{FollowEnd: true}`，流式输出时对话区始终停在最新一条。\n\n" +
		"回复本身用 Lobe 的 Markdown 视图，文字会按节奏显现：\n\n" +
		mdCode("go", `markdown.Markdown(c, m.Text, markdown.MarkdownProps{
	Variant:  markdown.Chat,
	Animated: m.Streaming,
})`)
}

func seedTokens() string {
	return "让 token 决定颜色，浅色和深色都会自然成立：\n\n" +
		"- **画布**：窗口用 `BgLayout`，承载对话的卡片用 `BgContainer`，输入框卡片用 `BgElevated`。\n" +
		"- **文字**：正文用 `Text`，预览用 `TextSecondary`，时间和提示用 `TextTertiary`。\n" +
		"- **分隔**：静止时用 `Border`，卡片内部用 `BorderSecondary`，这样界面读起来是两层表面，而不是一堆盒子。\n" +
		"- **状态**：发送按钮用 `Primary`，只有危险操作才用 `Error`。\n\n" +
		"`theme.Apply(c)` 每帧读取系统深浅色，`theme.TokensOf(c)` 给出对应的一套。应用里没有写死任何颜色。"
}

func seedStreaming() string {
	return "要让流式回答稳住，关键有两点：\n\n" +
		"- `Animated: true` 会把字符按节奏送进来，写了一半的内容按最可能的结尾显示，文字不会每来一个 token 就重新折行。\n" +
		"- `Smoothing` 控制它落后多少：`Realtime` 紧跟 token，`Balanced` 是默认值，`Silky` 最平滑。\n\n" +
		mdCode("go", `markdown.Markdown(c, m.Text, markdown.MarkdownProps{
	Variant:     markdown.Chat,
	Animated:    m.Streaming,
	Smoothing:   markdown.Balanced,
	Granularity: markdown.ByWord,
})`) +
		"\n\n在回复完成的那一帧、同一个位置把 `Animated` 关掉。还在路上的内容会走完，下面的内容也不会跳。"
}

func seedShortcuts() string {
	return "第一天就把这些定下来，并且放进 tooltip，让人找得到：\n\n" +
		"| 按键 | 动作 |\n" +
		"| --- | --- |\n" +
		"| `mod+k` | 聚焦搜索框 |\n" +
		"| `mod+n` | 新建对话 |\n" +
		"| `Enter` | 发送 |\n" +
		"| `Shift+Enter` | 输入框内换行 |\n" +
		"| `Escape` | 关闭已打开菜单 |\n\n" +
		"`field.SearchBar` 通过 `ShortKey` 绑定了 `mod+k`；其余用 `c.Shortcut` 和 `OnTextKey`，后者能拦住输入框里的 Enter，不让它传到窗口。"
}

func seedRelease() string {
	return "beta 之前我会跑这几步：\n\n" +
		"1. `go build ./... && go vet ./...`——不留任何告警。\n" +
		"2. `go test ./...`——发送、搜索、设置都用无头测试驱动过。\n" +
		"3. `CGO_ENABLED=0 go test ./...`——发布构建不需要 cgo。\n" +
		"4. `go run .`——真正的帧，GPU 全开；缩到最小尺寸，再截一张图。\n" +
		"5. 系统深色模式启动一次，浅色再启动一次。\n\n" +
		"跳过最后两条，就会发出测试看不到的问题。"
}

func seedReview() string {
	return "分三遍做，比一次长看更有效：\n\n" +
		"- **今天**——最老的两个 PR，只处理你已经提过修改意见的。\n" +
		"- **周四**——会改变公开行为的两个，先看测试。\n" +
		"- **周五早上**——扫一遍还没关的，三十分钟，不开新讨论串。\n\n" +
		"每条都给出一个具体的请求，或者直接通过。没有决定的队列，等于没清。"
}
