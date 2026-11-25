package ui

import "testing"

// TestRemoveThinkingTag 守住标签移除:从集合里删掉,若正是当前选中则一起清空。
func TestRemoveThinkingTag(t *testing.T) {
	u := &UI{providerTags: []string{"low", "medium", "high"}, modelThinking: "medium"}
	u.removeThinkingTag("low")
	if len(u.providerTags) != 2 || u.providerTags[0] != "medium" || u.providerTags[1] != "high" {
		t.Fatalf("移除后应剩 medium/high: %v", u.providerTags)
	}
	if u.modelThinking != "medium" {
		t.Fatalf("移除未选中的标签不应改当前选择: %q", u.modelThinking)
	}

	u.removeThinkingTag("medium")
	if u.modelThinking != "" {
		t.Fatalf("移除当前选中标签应清空选择: %q", u.modelThinking)
	}
	if len(u.providerTags) != 1 || u.providerTags[0] != "high" {
		t.Fatalf("应只剩 high: %v", u.providerTags)
	}
}

// TestAddThinkingTag 守住新增:去空白、去重,成功后清空输入。
func TestAddThinkingTag(t *testing.T) {
	u := &UI{providerTags: []string{"low"}, newTag: "  minimal  "}
	u.addThinkingTag()
	if len(u.providerTags) != 2 || u.providerTags[1] != "minimal" {
		t.Fatalf("新增标签失败: %v", u.providerTags)
	}
	if u.newTag != "" {
		t.Fatalf("新增后输入框应清空")
	}

	u.newTag = "low"
	u.addThinkingTag()
	if len(u.providerTags) != 2 {
		t.Fatalf("重复标签不应新增: %v", u.providerTags)
	}
}
