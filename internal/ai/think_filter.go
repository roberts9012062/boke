// internal/ai/think_filter.go
// 流式输出的思考段过滤器：<think>…</think> 跨 chunk 剥离。
//
// 背景：推理模型（MiniMax-M3 等）流式输出的首批增量是 <think>推理过程</think>，
// 非流式路径在响应解析后统一剥离；流式路径增量逐块透传，开/闭标签可能被
// 拆在相邻 chunk（如 "<thi" + "nk>"），需带状态的过滤器逐块滤除。
package ai

import "strings"

// thinkFilter 流式思考段过滤器（有状态，单流单实例，非并发安全）。
type thinkFilter struct {
	inThink bool  // 当前是否处于思考段内
	pending string // 尾部暂存：可能是被拆断的标签前缀（如 "</th"）
}

// NewThinkFilter 创建流式思考段过滤器。
func NewThinkFilter() *thinkFilter {
	return &thinkFilter{}
}

// openTag / closeTag 思考段标签（与小写化比较配合，容错大写变体）。
const (
	thinkTagOpen  = "<think>"
	thinkTagClose = "</think>"
)

// feed 输入一个增量，返回应向下游展示的文本（思考段返回空串；可能为空）。
// 标签被拆断的部分暂存在过滤器内，待后续增量拼齐后处理。
func (f *thinkFilter) Feed(chunk string) string {
	buf := f.pending + chunk
	f.pending = ""
	var out strings.Builder
	for {
		if f.inThink {
			idx := indexCaseInsensitive(buf, thinkTagClose)
			if idx < 0 {
				// 未闭合：尾部可能是被拆断的 </thi… 前缀，暂存防漏判
				f.pending = trimDanglingPrefix(buf, thinkTagClose)
				return out.String()
			}
			buf = buf[idx+len(thinkTagClose):]
			f.inThink = false
			continue
		}
		idx := indexCaseInsensitive(buf, thinkTagOpen)
		if idx < 0 {
			// 尾部可能是被拆断的 <thi… 前缀，暂存（其余正常下发）
			f.pending = trimDanglingPrefix(buf, thinkTagOpen)
			out.WriteString(buf[:len(buf)-len(f.pending)])
			return out.String()
		}
		out.WriteString(buf[:idx])
		buf = buf[idx+len(thinkTagOpen):]
		f.inThink = true
	}
}

// Flush 流结束时清空暂存（被拆断且未拼齐的尾巴按原文下发，防丢字）。
func (f *thinkFilter) Flush() string {
	rest := f.pending
	f.pending = ""
	return rest
}

// indexCaseInsensitive 大小写不敏感查找子串位置（纯函数；未找到返回 -1）。
func indexCaseInsensitive(s string, sub string) int {
	return strings.Index(strings.ToLower(s), sub)
}

// trimDanglingPrefix 返回 s 尾部「是 sub 前缀」的最长片段（如 "</th"；纯函数）。
// 用于暂存可能被拆断的标签开头，待下一增量拼齐判定。
func trimDanglingPrefix(s string, sub string) string {
	maxCheck := len(sub) - 1
	if maxCheck > len(s) {
		maxCheck = len(s)
	}
	for n := maxCheck; n > 0; n-- {
		if strings.HasPrefix(sub, s[len(s)-n:]) {
			return s[len(s)-n:]
		}
	}
	return ""
}
