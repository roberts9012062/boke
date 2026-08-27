// internal/handler/openapi_gateway.go
// 开放网关 AI 接口：模型列表与对话（/api/v1/open/ai/*，X-Api-Key 鉴权后进入）。
//
// 说明：复用 AiService 通用对话链路（按模型名路由供应商 → 统一推理 → 用量落库），
//       模型清单脱敏（仅供应商名与模型名，API Key 不出站）；对话为非流式 JSON
//       （外部应用通用形态，流式 SSE 后续按需扩展）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// gatewayChatMessage 网关对话消息（OpenAI 兼容格式，直接映射 ai.Message）。
type gatewayChatMessage struct {
	Role    string `json:"role"`    // system / user / assistant
	Content string `json:"content"` // 消息内容
}

// gatewayAIChatReq AI 对话请求体。
type gatewayAIChatReq struct {
	Model     string               `json:"model"`      // 模型名（取自 ai.models 列表）
	Messages  []gatewayChatMessage `json:"messages"`   // 对话消息（含历史轮次）
	MaxTokens int                  `json:"max_tokens"` // 输出上限（缺省 300，上限 16000）
}

// 网关消息条数与单条长度上限（防滥用：单次请求约束在合理对话规模）。
const (
	gatewayChatMaxMessages = 50   // 消息条数上限（多轮对话足够）
	gatewayChatMaxContent  = 32 << 10 // 单条内容 32KB
)

// validateGatewayMessages 校验消息列表（角色合法、条数与长度受限；纯函数）。
func validateGatewayMessages(messages []gatewayChatMessage) *errs.Err {
	if len(messages) == 0 || len(messages) > gatewayChatMaxMessages {
		return errs.New(errs.CodeBadRequest, "对话消息需为 1-50 条")
	}
	for _, msg := range messages {
		switch msg.Role {
		case "system", "user", "assistant":
		default:
			return errs.New(errs.CodeBadRequest, "消息角色仅支持 system / user / assistant")
		}
		if msg.Content == "" || len(msg.Content) > gatewayChatMaxContent {
			return errs.New(errs.CodeBadRequest, "消息内容不能为空且不超过 32KB")
		}
	}
	return nil
}

// toAIMessages 转换为 AI 内核消息（纯函数，逐条拷贝不共享引用）。
func toAIMessages(messages []gatewayChatMessage) []ai.Message {
	out := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, ai.Message{Role: msg.Role, Content: msg.Content})
	}
	return out
}

// GatewayAIModels 处理 AI 模型列表（GET /api/v1/open/ai/models）。
// 返回启用的供应商与模型清单（脱敏；对话接口的 model 参数取值来源）。
func (h *OpenAPIHandler) GatewayAIModels(c *gin.Context) {
	providers, err := h.ai.AIModels(c.Request.Context())
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"providers": providers})
}

// GatewayAIChat 处理 AI 对话（POST /api/v1/open/ai/chat，body: model/messages/max_tokens）。
// 响应：{model, reply}；用量与费用计入站点 AI 统计（task=plugin.generate）。
func (h *OpenAPIHandler) GatewayAIChat(c *gin.Context) {
	var req gatewayAIChatReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体需包含 model 与 messages"))
		return
	}
	if err := validateGatewayMessages(req.Messages); err != nil {
		resp.Fail(c, 400, err)
		return
	}
	reply, err := h.ai.GenerateChat(c.Request.Context(), req.Model, toAIMessages(req.Messages), req.MaxTokens)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"model": req.Model, "reply": reply})
}

// GatewayAIAssist 处理 AI 辅助（POST /api/v1/open/ai/assist，X-Api-Key 鉴权）。
// 动作：expand/polish（文本）、image/music（生成物转存媒体库）、recognize（识图）；
// 复用后台同一条辅助管道（任务提示词与启停在后台 AI 设置配置）。
func (h *OpenAPIHandler) GatewayAIAssist(c *gin.Context) {
	var req struct {
		Action   string `json:"action" binding:"required"`
		Content  string `json:"content"`
		ImageURL string `json:"image_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体需包含 action（expand/polish/image/music/recognize）"))
		return
	}
	result, err := h.ai.Assist(c.Request.Context(), req.Action, service.AssistInput{
		Content:  req.Content,
		ImageURL: req.ImageURL,
	})
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, result)
}
