package app

import (
	"strings"
	"testing"
	"time"
)

func TestSendAddsTurnAndRunsATool(t *testing.T) {
	a := New(time.Now())
	s := a.Current()
	before := len(s.Messages)
	a.Draft = "你好"
	a.Send()

	if len(s.Messages) != before+2 {
		t.Fatalf("%d messages, want the user turn and the reply", len(s.Messages))
	}
	user := s.Messages[before]
	if user.Role != RoleUser || user.Text != "你好" {
		t.Errorf("user turn %+v", user)
	}
	reply := s.Messages[before+1]
	if reply.Streaming || reply.Text == "" || len(reply.Tools) == 0 {
		t.Fatalf("reply %+v, want a finished answer with a tool call", reply)
	}
	for _, call := range reply.Tools {
		if call.State != ToolDone || call.Result == "" || call.Took <= 0 {
			t.Errorf("tool %+v, want it finished with a result and duration", call)
		}
	}
	if a.Draft != "" {
		t.Errorf("draft %q, want it cleared", a.Draft)
	}
	if s.Preview == "" || s.Updated.IsZero() {
		t.Errorf("session %+v, want its preview and time updated", s)
	}
}

func TestSendTitlesAConversationOnItsFirstTurn(t *testing.T) {
	a := New(time.Now())
	a.NewChat()
	a.Draft = "帮我规划这个原生应用的界面"
	a.Send()
	if s := a.Current(); !strings.HasPrefix(s.Title, "帮我规划") {
		t.Errorf("title %q, want the first turn to name the conversation", s.Title)
	}
}

func TestRegenerateKeepsThePromptAndReruns(t *testing.T) {
	a := New(time.Now())
	s := a.Current()
	last := s.Messages[len(s.Messages)-1]
	before := len(s.Messages)
	a.Regenerate(s)
	if len(s.Messages) != before {
		t.Errorf("%d messages, want the reply replaced", len(s.Messages))
	}
	reply := s.Messages[len(s.Messages)-1]
	if reply.ID == last.ID || reply.Streaming || reply.Text == "" || len(reply.Tools) == 0 {
		t.Errorf("reply %+v, want a fresh answer with tools", reply)
	}
}

func TestDeleteAndRestoreKeepThePosition(t *testing.T) {
	a := New(time.Now())
	target := a.Sessions[1]
	id := target.ID
	was := len(a.Sessions)
	at, ok := a.DeleteSession(id)
	if !ok || at != 1 || len(a.Sessions) != was-1 {
		t.Fatalf("deleted at %d (%v) with %d conversations, want one fewer at index 1", at, ok, len(a.Sessions))
	}
	a.RestoreSession(target, at)
	if len(a.Sessions) != was {
		t.Errorf("%d conversations, want the deleted one back", len(a.Sessions))
	}
	if a.Sessions[1].ID != id || a.Current() == nil {
		t.Errorf("conversation at index 1 is %+v, want the restored one", a.Sessions[1])
	}
}

func TestNewChatSelectsAnEmptyConversation(t *testing.T) {
	a := New(time.Now())
	a.NewChat()
	s := a.Current()
	if s == nil || s.Title != "新对话" || len(s.Messages) != 0 {
		t.Errorf("current %+v, want a new empty conversation", s)
	}
}

func TestSelectSessionClearsUnread(t *testing.T) {
	a := New(time.Now())
	var id int
	for _, s := range a.Sessions {
		if s.Unread > 0 {
			id = s.ID
		}
	}
	if id == 0 {
		t.Fatal("no unread conversation in the seed")
	}
	a.SelectSession(id)
	if a.Current().Unread != 0 {
		t.Error("opening a conversation keeps it unread")
	}
}

func TestReplyTextFollowsThePrompt(t *testing.T) {
	cases := map[string]string{
		"浅色和深色的聊天窗口该用哪些颜色？": "BgLayout",
		"第一天该定下哪些快捷键？":      "mod+k",
		"回答是一个字一个字来的，怎么渲染？": "Animated",
		"帮我在周五前清掉评审队列。":     "三遍",
	}
	for prompt, want := range cases {
		if got := ReplyText(prompt); !strings.Contains(got, want) {
			t.Errorf("reply to %q lacks %q", prompt, want)
		}
	}
	if got := ReplyText("随便聊聊"); !strings.Contains(got, "收窄范围") {
		t.Errorf("default reply %q", got)
	}
}

func TestToolPlanCoversEveryPrompt(t *testing.T) {
	cases := []struct {
		prompt string
		first  string
		count  int
	}{
		{"帮我搜索文档", "search_docs", 2},
		{"beta 发布前要做什么", "run_tests", 1},
		{"帮我清评审队列", "list_prs", 1},
		{"随便聊聊", "lookup_context", 1},
	}
	for _, tc := range cases {
		plan := toolPlan(tc.prompt)
		if len(plan) != tc.count || plan[0].Key != tc.first {
			t.Errorf("plan for %q = %+v, want %d starting with %q", tc.prompt, plan, tc.count, tc.first)
		}
		for _, use := range plan {
			if use.Result == "" || use.Took <= 0 {
				t.Errorf("use %+v, want a result and duration", use)
			}
		}
	}
}

func TestRenameIgnoresAnEmptyTitle(t *testing.T) {
	a := New(time.Now())
	title := a.Current().Title
	a.Rename(a.Current().ID, "   ")
	if a.Current().Title != title {
		t.Errorf("title %q, want it unchanged", a.Current().Title)
	}
	a.Rename(a.Current().ID, "重命名后的对话")
	if got := a.Current().Title; got != "重命名后的对话" {
		t.Errorf("title %q", got)
	}
}

func TestClipKeepsOneLineAndEllipsis(t *testing.T) {
	if got := Clip("a\n b\tc", 100); got != "a b c" {
		t.Errorf("clip %q", got)
	}
	r := []rune("这是一段很长的文字，需要被截断")
	if got := Clip(string(r), 6); got != string(r[:6])+"…" {
		t.Errorf("clip %q", got)
	}
}
