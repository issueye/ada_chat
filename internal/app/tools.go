package app

import (
	"strings"
	"time"
)

// 工具调用：助手在回答前调用的本地能力。卡片长什么样属于界面，这里只放状态和数据。

type ToolState int

const (
	ToolRunning ToolState = iota
	ToolDone
	ToolFailed
)

// ToolCall 是一次调用：参数、结果、状态和耗时。Open 是它在对话里是否展开。
type ToolCall struct {
	ID     int
	Key    string
	Name   string
	Args   string
	Result string
	State  ToolState
	Took   time.Duration
	Open   bool
}

// toolUse 是一次计划中的调用：运行前的参数、运行后的结果，以及演示用的耗时。
type toolUse struct {
	Key, Args, Result string
	Delay, Took       time.Duration
}

var toolNames = map[string]string{
	"search_docs":    "搜索文档",
	"read_file":      "读取文件",
	"run_tests":      "运行测试",
	"lookup_context": "检索上下文",
	"list_prs":       "拉取评审",
}

// ToolName 给工具键一个显示名。
func ToolName(key string) string {
	if n, ok := toolNames[key]; ok {
		return n
	}
	return key
}

// toolPlan 按问题选出这次要调用的工具；每个问题至少调用一个，卡片因此总能在对话里看到。
func toolPlan(prompt string) []toolUse {
	lower := strings.ToLower(prompt)
	switch {
	case contains(lower, "评审", "pr", "队列", "review"):
		return []toolUse{{
			Key: "list_prs", Args: `{"state":"open","remote":"origin"}`,
			Result: "2 个待评审 PR：#482 `theme: 深色边框`，#479 `chat: 跟随末尾滚动`。",
			Delay:  280 * time.Millisecond, Took: 720 * time.Millisecond,
		}}
	case contains(lower, "发布", "测试", "beta", "build", "test"):
		return []toolUse{{
			Key: "run_tests", Args: `{"package":"./..."}`,
			Result: "`go test ./...`：21 个用例通过，用时 13.9s。",
			Delay:  300 * time.Millisecond, Took: 1200 * time.Millisecond,
		}}
	case contains(lower, "文档", "搜索", "search", "docs", "token", "设计", "主题", "配色", "markdown", "流式", "快捷键"):
		return []toolUse{
			{
				Key: "search_docs", Args: `{"query":"Lobe 布局与流式渲染","scope":"docs"}`,
				Result: "找到 3 条文档：`DraggablePanel` 的 `MinSize` / `MaxSize`；`Markdown` 的 `Animated` 节奏；`ListState.FollowEnd` 的跟随末尾。",
				Delay:  260 * time.Millisecond, Took: 480 * time.Millisecond,
			},
			{
				Key: "read_file", Args: `{"path":"chat.go","lines":"140-180"}`,
				Result: "读取 `chat.go` 第 140–180 行：对话区用 `ui.List` 落位，消息行由状态每帧重建。",
				Delay:  240 * time.Millisecond, Took: 310 * time.Millisecond,
			},
		}
	}
	return []toolUse{{
		Key: "lookup_context", Args: `{"conversation":"当前对话","top_k":4}`,
		Result: "找到 4 条相关消息，摘要：界面骨架、设计变量、流式节奏与快捷键约定。",
		Delay:  240 * time.Millisecond, Took: 360 * time.Millisecond,
	}}
}

func contains(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// newTool 造一条已经跑完的工具调用，给内置对话用。
func (a *App) newTool(key, args, result string, took time.Duration, open bool) *ToolCall {
	return &ToolCall{
		ID: a.id(), Key: key, Name: ToolName(key), Args: args, Result: result,
		State: ToolDone, Took: took, Open: open,
	}
}

func (a *App) failedTool(key, args, result string, took time.Duration) *ToolCall {
	t := a.newTool(key, args, result, took, true)
	t.State = ToolFailed
	return t
}

func finishTools(m *Message, plan []toolUse) {
	for i, call := range m.Tools {
		if i >= len(plan) {
			break
		}
		call.State = ToolDone
		call.Result = plan[i].Result
		call.Took = plan[i].Took
	}
}

func completeTool(m *Message, id int, result string, took time.Duration) {
	for _, call := range m.Tools {
		if call.ID == id {
			call.State = ToolDone
			call.Result = result
			call.Took = took
			return
		}
	}
}
