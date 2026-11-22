package app

import "time"

// 工具调用是助手在回答前调用的本地能力。卡片长什么样属于界面，这里只放状态和数据。

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
