// 中继站发布出口的内容纯文本化（从 relay.go 拆出；测试见 relay_plain_test.go）：
// 大世界是纯文本广场——html 内容先保留媒体引用（防丢媒体），再剥标签还原实体。
package service

import (
	"html"
	"regexp"
	"strings"
)

// htmlTagPattern HTML 标签匹配（发布出口剥离；大世界为纯文本广场，渲染层另有转义兜底）。
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// 内嵌媒体提取（剥离前转 markdown 引用，防 html 内容里的媒体被丢弃）。
var (
	htmlImgPattern   = regexp.MustCompile(`<img[^>]*src=["']([^"']+)["'][^>]*>`)
	htmlVideoPattern = regexp.MustCompile(`<video[^>]*src=["']([^"']+)["'][^>]*>`)
	htmlAudioPattern = regexp.MustCompile(`<audio[^>]*src=["']([^"']+)["'][^>]*>`)
)

// plainForWorld 大世界纯文本化：html 内容先保留媒体引用（图转 markdown、音视频转链接文本），
// 再剥标签并还原实体；markdown 原样（成员站按受限渲染器呈现，纯文本层自动转义）。
func plainForWorld(content string) string {
	if !strings.Contains(content, "<") {
		return content
	}
	kept := htmlImgPattern.ReplaceAllString(content, "\n![图片]($1)\n")
	kept = htmlVideoPattern.ReplaceAllString(kept, "[视频]($1)")
	kept = htmlAudioPattern.ReplaceAllString(kept, "[音频]($1)")
	plain := htmlTagPattern.ReplaceAllString(kept, "")
	plain = html.UnescapeString(plain)
	return strings.TrimSpace(strings.Join(strings.Fields(plain), " "))
}
