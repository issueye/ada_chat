package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
)

// TestSnapshots 把应用打开时的几帧写到 testdata/preview，方便改样式后做视觉检查：
//
//	ADA_SNAPSHOT=1 go test ./internal/ui -run TestSnapshots
func TestSnapshots(t *testing.T) {
	if os.Getenv("ADA_SNAPSHOT") == "" {
		t.Skip("set ADA_SNAPSHOT=1 to write previews")
	}
	dir := filepath.Join("testdata", "preview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	shots := []struct {
		name  string
		dark  bool
		setup func(tt *mui.Tester)
	}{
		{name: "conversation", setup: func(tt *mui.Tester) {}},
		{name: "conversation-dark", dark: true},
		{name: "empty", setup: func(tt *mui.Tester) { press(t, tt, mui.RoleButton, "新建对话") }},
		{name: "settings", setup: func(tt *mui.Tester) { press(t, tt, mui.RoleButton, "设置") }},
		{name: "sidebar-hidden", setup: func(tt *mui.Tester) { press(t, tt, mui.RoleButton, "收起侧栏") }},
		{name: "failed-tool", setup: func(tt *mui.Tester) { press(t, tt, mui.RoleButton, "周五前清完评审队列") }},
	}
	for _, shot := range shots {
		tt, _ := newTester(t, 1180, 780)
		if shot.dark {
			tt.SetDark(true)
			settle(tt)
		}
		if shot.setup != nil {
			shot.setup(tt)
		}
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		path := filepath.Join(dir, shot.name+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}
