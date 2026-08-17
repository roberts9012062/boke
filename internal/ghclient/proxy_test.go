// internal/ghclient/proxy_test.go
// GitHub 加速代理单元测试：元数据 API 恒直连、资产下载代理前缀拼接与直连回退。
package ghclient

import (
	"testing"
)

// TestWrapURL 代理包装规则：直连原样、代理前缀拼接、尾斜杠归一化、非法代理恢复直连。
func TestWrapURL(t *testing.T) {
	client := NewClient("")

	// 直连（默认）：原样返回
	if got := client.wrapURL("https://api.github.com/repos/o/r"); got != "https://api.github.com/repos/o/r" {
		t.Fatalf("直连应原样返回：%s", got)
	}
	// 代理模式：资产下载前缀拼接（输入尾斜杠归一化）；元数据 API 恒直连
	// （公共代理剥 Authorization 头，走代理的 API 请求按匿名 IP 限流——2026-08 实测 403）
	client.SetProxy("https://gh-proxy.com/")
	if got := client.wrapAssetURL("https://github.com/o/r/releases/download/v1/a.bpk"); got != "https://gh-proxy.com/https://github.com/o/r/releases/download/v1/a.bpk" {
		t.Fatalf("代理拼接结果不符：%s", got)
	}
	if got := client.wrapURL("https://api.github.com/repos/o/r"); got != "https://api.github.com/repos/o/r" {
		t.Fatalf("元数据 API 应恒直连：%s", got)
	}
	// 非 https 前缀代理：视为无效恢复直连
	client.SetProxy("http://insecure-proxy.com")
	if got := client.wrapURL("https://api.github.com/repos/o/r"); got != "https://api.github.com/repos/o/r" {
		t.Fatalf("非 https 代理应恢复直连：%s", got)
	}
	// 空串恢复直连
	client.SetProxy("https://gh-proxy.com")
	client.SetProxy("")
	if got := client.wrapURL("https://api.github.com/repos/o/r"); got != "https://api.github.com/repos/o/r" {
		t.Fatalf("空代理应恢复直连：%s", got)
	}
}
