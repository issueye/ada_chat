package ui_test

import (
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"ada_chat/internal/ui"
)

// menuOn 在侧栏会话行上右键,返回打开菜单的 tester。
func menuFrame(t *testing.T, tt *mui.Tester, title string) {
	t.Helper()
	if err := tt.RightClick(title); err != nil {
		t.Fatalf("右键 %q 失败: %v", title, err)
	}
	tt.Frame()
}

// TestSessionContextMenuCopyAndArchive 守住会话右键菜单:
// 复制会话 ID 进剪贴板;归档后这一行从「今天」移到「归档」分组。
func TestSessionContextMenuCopyAndArchive(t *testing.T) {
	a := baseApp(t)
	u := ui.New(a)
	tt := mui.NewTester(u.Root, 980, 760)
	tt.Frame()

	menuFrame(t, tt, "你好")
	if err := tt.ChooseMenuItem("复制会话 ID"); err != nil {
		t.Fatalf("选复制会话 ID 失败: %v", err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got != "1" {
		t.Fatalf("剪贴板应为会话 ID,实际 %q", got)
	}

	menuFrame(t, tt, "你好")
	if err := tt.ChooseMenuItem("归档"); err != nil {
		t.Fatalf("选归档失败: %v", err)
	}
	tt.Frame()
	if _, ok := tt.Find("归档"); !ok {
		t.Fatalf("归档后应出现「归档」分组")
	}
	if !a.Sessions[0].Archived {
		t.Fatalf("会话没有被标记为归档")
	}
	shot(t, tt, "session-archived.png")

	// 取消归档回到主列表。
	menuFrame(t, tt, "你好")
	if err := tt.ChooseMenuItem("取消归档"); err != nil {
		t.Fatalf("选取消归档失败: %v", err)
	}
	tt.Frame()
	if a.Sessions[0].Archived {
		t.Fatalf("取消归档后标记应清除")
	}
}

// TestSessionContextMenuRemove 守住「移除」:先开确认弹窗,确认后对话消失。
func TestSessionContextMenuRemove(t *testing.T) {
	a := baseApp(t)
	u := ui.New(a)
	tt := mui.NewTester(u.Root, 980, 760)
	tt.Frame()

	menuFrame(t, tt, "你好")
	if err := tt.ChooseMenuItem("移除"); err != nil {
		t.Fatalf("选移除失败: %v", err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("删除对话"); !ok {
		t.Fatalf("移除应先打开确认弹窗")
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatalf("确认删除失败: %v", err)
	}
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if len(a.Sessions) != 0 {
		t.Fatalf("确认后应删除对话,实际 %d", len(a.Sessions))
	}
	// 删除后没有会话分组;标题仍可能出现在「已删除」提示里,所以按分组断言。
	if _, ok := tt.Find("今天"); ok {
		t.Fatalf("删除后侧栏不应再有会话分组")
	}
}
