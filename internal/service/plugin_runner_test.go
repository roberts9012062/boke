// internal/service/plugin_runner_test.go
// 前台插件扩展解析单元测试：manifest 解析（siteNav/site 页面路由提取）+ 导航项白名单过滤。
package service

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseFrontendManifest 覆盖清单读取：正常解析 / 文件缺失零值 / 非法 JSON 零值。
func TestParseFrontendManifest(t *testing.T) {
	dir := t.TempDir()

	// 正常清单：含 siteNav 与 site 页面
	raw := `{
  "extensionPoints": [{"slot": "theme.header", "entry": "index.js"}],
  "pages": [
    {"route": "login", "entry": "login.js"},
    {"route": "radio", "entry": "radio.html", "sandbox": true, "scope": "site"}
  ],
  "siteNav": [{"label": "电台", "path": "/plugins/demo/radio"}]
}`
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "manifest.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	decl := parseFrontendManifest(dir)
	if len(decl.Pages) != 2 || len(decl.SiteNav) != 1 {
		t.Fatalf("清单解析不符：pages=%d siteNav=%d", len(decl.Pages), len(decl.SiteNav))
	}
	if routes := sitePageRoutes(decl.Pages); len(routes) != 1 || routes[0] != "radio" {
		t.Fatalf("site 页面路由提取不符：%v", routes)
	}

	// 文件缺失：零值不报错
	if decl := parseFrontendManifest(t.TempDir()); decl.Pages != nil || decl.SiteNav != nil {
		t.Fatal("清单缺失应返回零值")
	}

	// 非法 JSON：零值不报错
	bad := t.TempDir()
	_ = os.MkdirAll(filepath.Join(bad, "frontend"), 0o755)
	_ = os.WriteFile(filepath.Join(bad, "frontend", "manifest.json"), []byte("{not-json"), 0o644)
	if decl := parseFrontendManifest(bad); decl.Pages != nil {
		t.Fatal("非法 JSON 应返回零值")
	}
}

// TestSanitizeSiteNav 导航项白名单：合法保留，空 label / 外链 / 危险协议 / 相对协议丢弃。
func TestSanitizeSiteNav(t *testing.T) {
	items := []struct {
		Label string `json:"label"`
		Path  string `json:"path"`
		Icon  string `json:"icon"`
	}{
		{Label: "电台", Path: "/plugins/demo/radio"},  // 合法站内
		{Label: "外站", Path: "https://example.com"},  // 外链：拒绝（仅允许站内）
		{Label: "", Path: "/x"},                      // 空 label：拒绝
		{Label: "危险", Path: "javascript:alert(1)"},  // 危险协议：拒绝
		{Label: "相对", Path: "//evil.com"},           // 协议相对：拒绝
	}
	cleaned := sanitizeSiteNav(items)
	if len(cleaned) != 1 || cleaned[0].Label != "电台" || cleaned[0].Path != "/plugins/demo/radio" {
		t.Fatalf("导航项过滤不符：%+v", cleaned)
	}
}

// TestSanitizeSiteNavLimit 每插件导航项上限 5 个。
func TestSanitizeSiteNavLimit(t *testing.T) {
	items := make([]struct {
		Label string `json:"label"`
		Path  string `json:"path"`
		Icon  string `json:"icon"`
	}, 0)
	for i := 0; i < 8; i++ {
		items = append(items, struct {
			Label string `json:"label"`
			Path  string `json:"path"`
			Icon  string `json:"icon"`
		}{Label: "项", Path: "/ok"})
	}
	if cleaned := sanitizeSiteNav(items); len(cleaned) != 5 {
		t.Fatalf("超过 5 项应截断为 5，得到 %d", len(cleaned))
	}
}
