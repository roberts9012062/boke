// internal/service/markdown_plain.go
// 富文本/Markdown 文本剥离（M5 富文本）：从正文生成「纯文本摘要」与做字数统计时，
// 移除 HTML 标签与 Markdown 标记，避免列表卡片 / 草稿箱 / 后台列表出现脏字符，
// 且字数按纯文本计（HTML 标签不占字数）。仅用于摘要/字数，不改变正文存储。
package service

import (
	"html"
	"regexp"
)

// 预编译正则（包级一次性编译，避免每次摘要生成重复编译）。
// 行首匹配统一用 [ \t]（不含 \n），避免 \s 跨行吃掉换行符导致空行丢失。
var (
	// HTML 标签：<tag ...>（跨行，非贪婪；含自闭合）
	htmlTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// 代码块围栏：```lang ... ```（跨行）
	mdFenceRe = regexp.MustCompile("(?s)```.*?```")
	// 图片：![alt](url)，保留 alt 文本
	mdImageRe = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	// 链接：[text](url)，保留 text
	mdLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	// 标题：# / ## / ### ...（行首）
	mdHeadingRe = regexp.MustCompile(`(?m)^#{1,6}[ \t]+`)
	// 加粗 / 斜体 / 删除线
	mdBoldItalicRe = regexp.MustCompile(`(\*\*|__|\*|_|~~)`)
	// 行内代码与代码标记
	mdCodeRe = regexp.MustCompile("`")
	// 引用：行首 > 
	mdQuoteRe = regexp.MustCompile(`(?m)^>[ \t]?`)
	// 列表：行首 - / * / + / 数字.
	mdListRe = regexp.MustCompile(`(?m)^[ \t]*([-*+]|\d+\.)[ \t]+`)
	// 分割线：--- / *** / ___
	mdHrRe = regexp.MustCompile(`(?m)^[ \t]*([-*_]){3,}[ \t]*$`)
)

// stripMarkdown 移除常见 Markdown 标记，返回纯文本（仅摘要用，非精确渲染）。
// 顺序：先整体剥离代码块与图片/链接（保留说明文字），再逐行去标题/引用/列表/分割线，
//       最后去加粗/斜体/行内代码等行内标记。
func stripMarkdown(content string) string {
	text := mdFenceRe.ReplaceAllString(content, "")
	text = mdImageRe.ReplaceAllString(text, "$1")
	text = mdLinkRe.ReplaceAllString(text, "$1")
	text = mdHeadingRe.ReplaceAllString(text, "")
	text = mdQuoteRe.ReplaceAllString(text, "")
	text = mdListRe.ReplaceAllString(text, "")
	text = mdHrRe.ReplaceAllString(text, "")
	text = mdBoldItalicRe.ReplaceAllString(text, "")
	text = mdCodeRe.ReplaceAllString(text, "")
	return text
}

// stripHTML 移除 HTML 标签并反转义实体，返回纯文本（iframe/img 等整体移除）。
// 说明：富文本正文的 img/iframe 不产生可见文本，直接去掉标签即可；
//       &amp; / &lt; 等实体经 html.UnescapeString 还原为可见字符。
func stripHTML(content string) string {
	text := htmlTagRe.ReplaceAllString(content, "")
	return html.UnescapeString(text)
}

// plainText 将正文（Markdown 或 HTML）转为纯文本：先剥 HTML，再剥 Markdown 标记。
// 用于摘要生成与字数统计——两种格式统一得到干净纯文本。
func plainText(content string) string {
	return stripMarkdown(stripHTML(content))
}
