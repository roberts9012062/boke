// internal/service/plugin_openapi_test.go
// 插件声明式开放端点测试：命名空间校验、路径推导、包清单解析、目录聚合
// （Entries/RouteIndex/FindRoute）、违规声明防御性跳过。
package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roberts9012062/boke/internal/plugin/bpkg"
)

// TestValidateOpenEndpoints 验证声明的命名空间与格式约束（前缀=插件 ID 原样）。
func TestValidateOpenEndpoints(t *testing.T) {
	ok := []bpkg.OpenEndpointDecl{
		{Endpoint: "nav-links.private-list", Method: "GET", Path: "/api/v1/open/plugins/nav-links/private/links", Name: "私有导航列表"},
		{Endpoint: "nav-links.private-save", Method: "POST", Path: "/api/v1/open/plugins/nav-links/private/config", Name: "私有设置写入"},
	}
	if reason := bpkg.ValidateOpenEndpoints("nav-links", ok); reason != "" {
		t.Fatalf("合法声明被拒绝：%s", reason)
	}
	bad := map[string][]bpkg.OpenEndpointDecl{
		"标识未用插件前缀": {{Endpoint: "posts.create", Method: "GET", Path: "/api/v1/open/plugins/nav-links/x", Name: "n"}},
		"方法非法":      {{Endpoint: "navlinks.a", Method: "DELETE", Path: "/api/v1/open/plugins/nav-links/x", Name: "n"}},
		"路径越出命名空间":  {{Endpoint: "navlinks.a", Method: "GET", Path: "/api/v1/open/nav/links", Name: "n"}},
		"路径伪造他插件":   {{Endpoint: "navlinks.a", Method: "GET", Path: "/api/v1/open/plugins/other-plug/x", Name: "n"}},
		"缺展示名":      {{Endpoint: "navlinks.a", Method: "GET", Path: "/api/v1/open/plugins/nav-links/x", Name: ""}},
		"缺子路径":      {{Endpoint: "navlinks.a", Method: "GET", Path: "/api/v1/open/plugins/nav-links/", Name: "n"}},
	}
	for name, decls := range bad {
		if reason := bpkg.ValidateOpenEndpoints("nav-links", decls); reason == "" {
			t.Fatalf("%s：违规声明未被拦截", name)
		}
	}
}

// TestPluginPathFromOpenPath 验证开放路径到插件端路径的推导。
func TestPluginPathFromOpenPath(t *testing.T) {
	got := bpkg.PluginPathFromOpenPath("nav-links", "/api/v1/open/plugins/nav-links/private/links")
	if got != "/private/links" {
		t.Fatalf("推导结果 %q，期望 /private/links", got)
	}
	if got := bpkg.PluginPathFromOpenPath("nav-links", "/api/v1/open/plugins/other/x"); got != "" {
		t.Fatalf("跨命名空间应返回空串，实际 %q", got)
	}
}

// TestParseManifestOpenEndpoints 验证包清单解析携带声明并执行校验。
func TestParseManifestOpenEndpoints(t *testing.T) {
	raw := []byte(`{
		"id":"demo","name":"演示","version":"1.0.0",
		"open_endpoints":[{"endpoint":"demo.ping","method":"GET","path":"/api/v1/open/plugins/demo/ping","name":"探活"}]
	}`)
	m, err := bpkg.ParseManifest(raw)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if len(m.OpenEndpoints) != 1 || m.OpenEndpoints[0].Endpoint != "demo.ping" {
		t.Fatal("声明未解析出来")
	}
	badRaw := []byte(`{
		"id":"demo","name":"演示","version":"1.0.0",
		"open_endpoints":[{"endpoint":"evil.posts","method":"GET","path":"/api/v1/open/plugins/demo/x","name":"伪装"}]
	}`)
	if _, err := bpkg.ParseManifest(badRaw); err == nil {
		t.Fatal("伪装宿主标识的声明应被解析期拒绝")
	}
}

// TestPluginOpenCatalogScan 验证聚合器扫描：合法条目入目录/索引，违规条目跳过。
func TestPluginOpenCatalogScan(t *testing.T) {
	dataDir := t.TempDir()
	plugDir := filepath.Join(dataDir, "plugins", "demo")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"id":"demo","name":"演示插件","version":"1.0.0",
		"open_endpoints":[
			{"endpoint":"demo.ping","method":"GET","path":"/api/v1/open/plugins/demo/ping","name":"探活","description":"测试探活"},
			{"endpoint":"hack.list","method":"GET","path":"/api/v1/open/plugins/demo/hack","name":"违规"}
		]
	}`
	if err := os.WriteFile(filepath.Join(plugDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	// 无声明插件（老版本）：不应出现在目录
	otherDir := filepath.Join(dataDir, "plugins", "legacy")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "manifest.json"), []byte(`{"id":"legacy","name":"老插件","version":"0.9.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog := NewPluginOpenCatalog(dataDir, nil)
	entries := catalog.Entries()
	if len(entries) != 1 {
		t.Fatalf("目录应只含 1 条合法声明（违规跳过、老插件无声明），实际 %d", len(entries))
	}
	e := entries[0]
	if e.Endpoint != "demo.ping" || e.Source != "plugin" || e.PluginName != "演示插件" || e.Description != "测试探活" {
		t.Fatalf("目录条目字段不正确：%+v", e)
	}
	idx := catalog.RouteIndex()
	if idx["GET /api/v1/open/plugins/demo/ping"] != "demo.ping" {
		t.Fatal("网关索引未命中合法声明")
	}
	if _, ok := idx["GET /api/v1/open/plugins/demo/hack"]; ok {
		t.Fatal("违规声明不应进入网关索引")
	}
	target, ok := catalog.FindRoute("GET", "/api/v1/open/plugins/demo/ping")
	if !ok || target.PluginID != "demo" || target.PluginPath != "/ping" {
		t.Fatalf("转发目标不正确：%+v ok=%v", target, ok)
	}
	if _, ok := catalog.FindRoute("POST", "/api/v1/open/plugins/demo/ping"); ok {
		t.Fatal("方法不匹配（声明 GET 请求 POST）不应命中")
	}
}
