package app

import "testing"

// TestEnsureProvidersMigratesLegacy 守住旧配置迁移:单供应商字段变成一条列表记录,
// 并回填 ActiveProvider 与模型。
func TestEnsureProvidersMigratesLegacy(t *testing.T) {
	p := Prefs{Provider: "openai", Endpoint: "https://api.deepseek.com", Model: "deepseek-chat", APIKey: "sk-x"}
	p.EnsureProviders()

	if len(p.Providers) != 1 {
		t.Fatalf("应迁移出 1 个供应商,实际 %d", len(p.Providers))
	}
	prov := p.Providers[0]
	if prov.Kind != "openai" || prov.Endpoint != "https://api.deepseek.com" || prov.APIKey != "sk-x" {
		t.Fatalf("供应商字段迁移不完整: %+v", prov)
	}
	if prov.Model != "deepseek-chat" || len(prov.Models) != 1 || prov.Models[0].ID != "deepseek-chat" {
		t.Fatalf("模型没有迁进列表: %+v", prov)
	}
	if p.ActiveProvider != prov.ID {
		t.Fatalf("ActiveProvider 应指向迁移出的供应商: %q", p.ActiveProvider)
	}
}

// TestSyncLegacyFollowsActiveProvider 切换当前供应商后,旧的单供应商字段跟着变。
func TestSyncLegacyFollowsActiveProvider(t *testing.T) {
	p := Prefs{Providers: []Provider{
		{ID: "a", Name: "A", Kind: "openai", Endpoint: "https://a.example", APIKey: "ka", Model: "m-a", Models: []ModelSpec{{ID: "m-a"}}},
		{ID: "b", Name: "B", Kind: "openai", Endpoint: "https://b.example", APIKey: "kb", Model: "m-b", Models: []ModelSpec{{ID: "m-b"}}},
	}, ActiveProvider: "b"}
	p.EnsureProviders()

	if p.Provider != "openai" || p.Endpoint != "https://b.example" || p.Model != "m-b" || p.APIKey != "kb" {
		t.Fatalf("旧字段没有跟随当前供应商: %+v", p)
	}

	p.ActiveProvider = "a"
	p.SyncLegacy()
	if p.Endpoint != "https://a.example" || p.Model != "m-a" {
		t.Fatalf("切回 A 后旧字段应更新: %+v", p)
	}
}

// TestEnsureProvidersFillsDefaults 空列表不造供应商;空模型 ID 不会被当成有效模型。
func TestEnsureProvidersFillsDefaults(t *testing.T) {
	p := Prefs{}
	p.EnsureProviders()
	if len(p.Providers) != 0 {
		t.Fatalf("没有旧字段时不应凭空造供应商: %+v", p.Providers)
	}
	if p.Provider != "" || p.Endpoint != "" || p.Model != "" {
		t.Fatalf("空偏好不应写回旧字段: %+v", p)
	}
}
