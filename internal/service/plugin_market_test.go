// internal/service/plugin_market_test.go
// 商城文件夹结构解析单元测试：pluginFolders 提取、splitSource 解析。
package service

import "testing"

// TestPluginFolders 从文件树提取插件文件夹（顶层且含 plugin.json）。
func TestPluginFolders(t *testing.T) {
	paths := []string{
		"README.md",
		"market.json",
		"seo-optimizer/plugin.json",
		"seo-optimizer/README.md",
		"demo-plugin/plugin.json",
		"nested/sub/plugin.json", // 非顶层，应忽略
		"assets/logo.png",
	}
	folders := pluginFolders(paths)
	if len(folders) != 2 || folders[0] != "demo-plugin" || folders[1] != "seo-optimizer" {
		t.Fatalf("文件夹列表不符：%v", folders)
	}
}

// TestSplitSource owner/repo 解析与非法格式拒绝。
func TestSplitSource(t *testing.T) {
	owner, repo, err := splitSource("roberts9012062/yueyan-plugins")
	if err != nil || owner != "roberts9012062" || repo != "yueyan-plugins" {
		t.Fatalf("解析不符：%s %s %v", owner, repo, err)
	}
	for _, bad := range []string{"", "no-slash", "/", "owner/", "/repo"} {
		if _, _, err := splitSource(bad); err == nil {
			t.Fatalf("非法源应拒绝：%q", bad)
		}
	}
}
