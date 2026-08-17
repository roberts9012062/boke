// internal/service/page_test.go
// 自定义页面输入校验单元测试（normalizePageInput 纯函数，覆盖 slug/标题/长度/格式归一化）。
package service

import (
	"strings"
	"testing"

	"github.com/roberts9012062/boke/internal/model"
)

// TestNormalizePageInput 覆盖合法输入归一化与非法输入拒绝两类场景。
func TestNormalizePageInput(t *testing.T) {
	// 合法输入：各字段按预期归一化（slug 转小写、格式/状态取默认值）
	t.Run("合法输入归一化", func(t *testing.T) {
		slug, title, _, format, _, status, err := normalizePageInput(
			"  About-Me2 ", " 关于我 ", "<p>内容</p>", "", "个人介绍", "")
		if err != nil {
			t.Fatalf("合法输入不应报错：%v", err)
		}
		if slug != "about-me2" || title != "关于我" {
			t.Fatalf("slug/title 归一化不符：%q / %q", slug, title)
		}
		if format != model.PageFormatHTML || status != model.PageStatusDraft {
			t.Fatalf("默认 format/status 应为 html/draft：%q / %q", format, status)
		}
	})

	// 非法输入：统一返回校验错误（大写 slug 会被归一化为小写，见上方合法用例，不属于非法）
	invalid := []struct {
		name  string
		slug  string
		title string
	}{
		{"slug 含中文", "关于", "标题"},
		{"slug 含下划线", "about_me", "标题"},
		{"slug 连字符开头", "-about", "标题"},
		{"slug 为空", "  ", "标题"},
		{"slug 超长", strings.Repeat("a", 101), "标题"},
		{"标题为空", "about", "   "},
		{"标题超长", "about", strings.Repeat("标", 201)},
	}
	for _, c := range invalid {
		t.Run(c.name, func(t *testing.T) {
			if _, _, _, _, _, _, err := normalizePageInput(
				c.slug, c.title, "", "", "", ""); err == nil {
				t.Fatalf("非法输入 %q/%q 应返回校验错误", c.slug, c.title)
			}
		})
	}

	// 内容超限：>200KB 拒绝
	t.Run("内容超限", func(t *testing.T) {
		if _, _, _, _, _, _, err := normalizePageInput(
			"about", "标题", strings.Repeat("a", maxPageContentByte+1), "", "", ""); err == nil {
			t.Fatal("超大内容应返回校验错误")
		}
	})

	// 格式/状态：未知值归一化为默认（html/draft），不报错
	t.Run("未知格式与状态归一化", func(t *testing.T) {
		_, _, _, format, _, status, err := normalizePageInput(
			"about", "标题", "", "rich-text", "x", "archived")
		if err != nil {
			t.Fatalf("未知格式/状态不应报错：%v", err)
		}
		if format != model.PageFormatHTML || status != model.PageStatusDraft {
			t.Fatalf("未知值应归一化为 html/draft：%q / %q", format, status)
		}
	})
}
