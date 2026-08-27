// internal/handler/openapi_stream.go
// 开放网关「AI 流式对话」：POST /api/v1/open/ai/chat/stream（ai.chat.stream，SSE）。
//
// 与 ai.chat 同参（model/messages/max_tokens/web_search），响应为 SSE 事件流：
//   data: {"search_results":[...]}   —— web_search=true 时先行下发引用来源（可选事件）
//   data: {"text":"增量"}            —— 逐块下发正文增量
//   data: {"error":"..."}            —— 中途错误（可选事件，随后仍发 DONE）
//   data: [DONE]                     —— 流结束
// 供浏览器插件等外部应用逐字渲染；上游透传（内部流式链路复用）。
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// GatewayAIChatStream 处理流式对话（POST /api/v1/open/ai/chat/stream，SSE 透传）。
func (h *OpenAPIHandler) GatewayAIChatStream(c *gin.Context) {
	var req gatewayAIChatReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体需包含 model 与 messages"))
		return
	}
	if err := validateGatewayMessages(req.Messages); err != nil {
		resp.Fail(c, 400, err)
		return
	}
	messages := toAIMessages(req.Messages)

	// 联网增强：先检索并作为首事件下发引用来源，检索上下文注入消息头
	var cited []service.WebSearchResult
	if req.WebSearch {
		if query := lastUserQuery(messages); query != "" {
			if results, err := h.ai.SearchWeb(c.Request.Context(), query, 5); err == nil {
				cited = results
				var ctxBuilder strings.Builder
				ctxBuilder.WriteString("以下是关于用户问题的实时网络搜索结果（供参考，回答时可引用，注明来源）：\n")
				for i, item := range results {
					ctxBuilder.WriteString(fmt.Sprintf("%d. %s\n   %s\n   来源：%s\n", i+1, item.Title, item.Snippet, item.URL))
				}
				messages = append([]ai.Message{{Role: "system", Content: ctxBuilder.String()}}, messages...)
			}
			// 检索失败静默降级普通流式对话
		}
	}

	stream, err := h.ai.GenerateChatStream(c.Request.Context(), req.Model, messages, req.MaxTokens)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	defer stream.Close()

	// SSE 响应头（关闭代理/内核缓冲，逐块下发）
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		resp.Fail(c, 500, errs.New(errs.CodeInternal, "当前环境不支持流式响应"))
		return
	}

	// writeEvent 写单条 SSE 事件（JSON 编码规避增量文本含换行的解析歧义）。
	writeEvent := func(payload any) bool {
		raw, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := c.Writer.Write([]byte("data: " + string(raw) + "\n\n")); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// 引用来源先行下发（插件可在首个增量前渲染来源）
	if cited != nil {
		if !writeEvent(map[string]any{"search_results": cited}) {
			return
		}
	}

	// 逐 chunk 透传正文增量；中途错误以 error 事件告知后正常收尾
	// 逐 chunk 透传正文增量（推理模型思考段 <think>…</think> 跨块剥离）
	filter := ai.NewThinkFilter()
	enc := json.NewEncoder(c.Writer)
	writeText := func(text string) bool {
		if text == "" {
			return true
		}
		if _, err := c.Writer.Write([]byte("data: ")); err != nil {
			return false
		}
		if err := enc.Encode(map[string]string{"text": text}); err != nil {
			return false
		}
		if _, err := c.Writer.Write([]byte("\n")); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			break // io.EOF 或读取异常均视为结束
		}
		if !writeText(filter.Feed(chunk.Text)) {
			break
		}
	}
	writeText(filter.Flush())
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// lastUserQuery 取消息列表中最后一条用户消息文本（联网检索词；纯函数）。
func lastUserQuery(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}
