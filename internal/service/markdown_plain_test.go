// internal/service/markdown_plain_test.go
// Markdown 剥离单元测试：标题/列表/链接/代码块/加粗/图片等常见语法样例。
package service

import "testing"

// TestStripMarkdown 覆盖常见 Markdown 标记，校验剥离后的纯文本。
func TestStripMarkdown(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"纯文本原样", "今天天气很好", "今天天气很好"},
		{"标题", "# 标题\n正文", "标题\n正文"},
		{"多级标题", "## 二级\n### 三级", "二级\n三级"},
		{"无序列表", "- 项目一\n- 项目二", "项目一\n项目二"},
		{"有序列表", "1. 第一\n2. 第二", "第一\n第二"},
		{"引用", "> 引用的句子", "引用的句子"},
		{"加粗与斜体", "**加粗** 与 *斜体* 与 ~~删除~~", "加粗 与 斜体 与 删除"},
		{"行内代码", "用 `code` 包裹", "用 code 包裹"},
		{"链接保留文本", "访问[月言](https://example.com)", "访问月言"},
		{"图片保留 alt", "![封面图](https://x/y.png)", "封面图"},
		{"代码块整体剥离", "```go\nfmt.Println(1)\n```\n结尾", "\n结尾"},
		{"分割线", "上\n---\n下", "上\n\n下"},
		{"组合样例", "# 标题\n\n- 点一\n- 点二\n\n> 引用", "标题\n\n点一\n点二\n\n引用"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripMarkdown(c.in); got != c.want {
				t.Fatalf("stripMarkdown(%q)\n  得到 %q\n  期望 %q", c.in, got, c.want)
			}
		})
	}
}

// TestPlainText 覆盖 HTML 剥离 + 实体反转义（富文本正文 → 纯文本）。
func TestPlainText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"纯 HTML 段落", "<p>你好<strong>世界</strong></p>", "你好世界"},
		{"实体反转义", "<p>a &amp; b &lt;c&gt;</p>", "a & b <c>"},
		{"img 移除", "<p>前</p><img src=\"/x.png\"><p>后</p>", "前后"},
		{"iframe 移除", "<p>视频</p><iframe src=\"https://player.bilibili.com\"></iframe>", "视频"},
		{"HTML 内嵌 Markdown", "<p><strong>加粗</strong> 与 <em>斜体</em></p>", "加粗 与 斜体"},
		{"换行保留", "<p>第一行</p>\n<p>第二行</p>", "第一行\n第二行"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := plainText(c.in); got != c.want {
				t.Fatalf("plainText(%q)\n  得到 %q\n  期望 %q", c.in, got, c.want)
			}
		})
	}
}
