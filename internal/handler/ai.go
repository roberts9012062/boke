// internal/handler/ai.go
// AI 控制器（M4）：供应商管理 / 任务配置 / 用量统计 / 内置场景触发（摘要/标签/评论审核）。
// 说明：全部挂 /admin 组（统一角色鉴权）；场景接口供后台编辑页与审核队列调用。
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// AiHandler AI 控制器（连接器类）。
type AiHandler struct {
	ai     *service.AiService // AI 服务
	logger *zap.Logger        // 错误日志（5xx 留痕）
}

// NewAiHandler 创建 AI 控制器。
func NewAiHandler(ai *service.AiService, logger *zap.Logger) *AiHandler {
	return &AiHandler{ai: ai, logger: logger}
}

// ---------- 供应商管理 ----------

// ListProviders 供应商列表（GET /api/v1/admin/ai/providers）。
func (h *AiHandler) ListProviders(c *gin.Context) {
	items, err := h.ai.ListProviders(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// CreateProvider 新增供应商（POST /api/v1/admin/ai/providers）。
func (h *AiHandler) CreateProvider(c *gin.Context) {
	var input service.AiProviderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	id, err := h.ai.CreateProvider(c.Request.Context(), input)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"id": id})
}

// UpdateProvider 更新供应商（PUT /api/v1/admin/ai/providers/:id）。
func (h *AiHandler) UpdateProvider(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var input service.AiProviderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.ai.UpdateProvider(c.Request.Context(), id, input); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// DeleteProvider 删除供应商（DELETE /api/v1/admin/ai/providers/:id）。
func (h *AiHandler) DeleteProvider(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.ai.DeleteProvider(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// TestProvider 测试供应商连通性（POST /api/v1/admin/ai/providers/:id/test）。
func (h *AiHandler) TestProvider(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.ai.TestProvider(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"ok": true, "message": "连通正常"})
}

// FetchModels 拉取供应商模型清单（POST /api/v1/admin/ai/providers/fetch-models，
// body: {base_url, api_key}；以表单当前值直连，不落库）。
func (h *AiHandler) FetchModels(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"` // 接口地址
		APIKey  string `json:"api_key"`  // API Key（仅本次拉取用，不落库）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	models, err := h.ai.FetchModels(c.Request.Context(), req.BaseURL, req.APIKey)
	if err != nil {
		h.logger.Error("拉取模型失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"models": models})
}

// ---------- 任务配置 ----------

// ListTasks 任务列表（GET /api/v1/admin/ai/tasks）。
func (h *AiHandler) ListTasks(c *gin.Context) {
	items, err := h.ai.ListTasks(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// UpdateTask 更新任务配置（PUT /api/v1/admin/ai/tasks/:name）。
func (h *AiHandler) UpdateTask(c *gin.Context) {
	taskName := c.Param("name")
	if taskName == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var input service.AiTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.ai.UpdateTask(c.Request.Context(), taskName, input); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// ToggleTask 启停任务（POST /api/v1/admin/ai/tasks/:name/toggle，body: {enabled}）。
func (h *AiHandler) ToggleTask(c *gin.Context) {
	taskName := c.Param("name")
	if taskName == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Enabled bool `json:"enabled"` // true 启用 / false 停用
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.ai.SetTaskEnabled(c.Request.Context(), taskName, req.Enabled); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"enabled": req.Enabled})
}

// ---------- 用量统计 ----------

// UsageStats 用量统计（GET /api/v1/admin/ai/usage：汇总 + 近 7 日）。
func (h *AiHandler) UsageStats(c *gin.Context) {
	stats, err := h.ai.UsageStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// ---------- 内置场景（后台触发） ----------

// GenSummary 生成帖子摘要（POST /api/v1/admin/ai/gen/summary?post_id=，
// 写入 seo_meta.summary；后台编辑页 SEO 面板入口）。
func (h *AiHandler) GenSummary(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Query("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	summary, err := h.ai.GenSummary(c.Request.Context(), postID)
	if err != nil {
		h.logger.Error("AI 生成摘要失败", zap.Int64("post_id", postID), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"summary": summary})
}

// GenTags 生成标签建议（POST /api/v1/admin/ai/gen/tags?post_id=；
// 返回建议数组，前端确认后经帖子更新接口写入）。
func (h *AiHandler) GenTags(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Query("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	tags, err := h.ai.GenTags(c.Request.Context(), postID)
	if err != nil {
		h.logger.Error("AI 生成标签失败", zap.Int64("post_id", postID), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"tags": tags})
}

// ReviewComments 批量 AI 审核评论（POST /api/v1/admin/ai/review/comments，
// body: {comment_ids}；后台评论管理「AI 审核」手动兜底入口）。
func (h *AiHandler) ReviewComments(c *gin.Context) {
	var req struct {
		CommentIDs []int64 `json:"comment_ids"` // 评论 ID 列表（≤50 条/次）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if len(req.CommentIDs) == 0 || len(req.CommentIDs) > 50 {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "评论 ID 列表需为 1-50 条"))
		return
	}
	result, err := h.ai.ReviewComments(c.Request.Context(), req.CommentIDs)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, result)
}

// ---------- 统一推理接口（通用入口，插件/未来功能直调） ----------

// Generate 通用非流式生成（POST /api/v1/admin/ai/generate）。
// 请求体：{model, prompt, content} 单轮（旧调用方不变）；
// 或 {model, messages: [{role, content}], max_tokens} 多轮（自定义页面构建器等对话式调用方，
// 携带 messages 时优先于 prompt/content）。
func (h *AiHandler) Generate(c *gin.Context) {
	var req struct {
		Model     string       `json:"model"`     // 模型名
		Prompt    string       `json:"prompt"`    // 系统提示词（单轮形态）
		Content   string       `json:"content"`   // 用户输入（单轮形态）
		Messages  []ai.Message `json:"messages"`  // 多轮对话（可选，携带时优先）
		MaxTokens int          `json:"max_tokens"` // 输出上限（可选，默认 300，上限 16000）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var text string
	var err error
	if len(req.Messages) > 0 {
		text, err = h.ai.GenerateChat(c.Request.Context(), req.Model, req.Messages, req.MaxTokens)
	} else {
		text, err = h.ai.Generate(c.Request.Context(), req.Model, req.Prompt, req.Content)
	}
	if err != nil {
		h.logger.Error("AI 通用生成失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"text": text})
}

// GenerateStream 通用流式生成（POST /api/v1/admin/ai/generate/stream，SSE）。
// 请求体与 Generate 一致（单轮 {model,prompt,content} 或多轮 {model,messages,max_tokens}）。
// 事件格式：data: {"text":"增量"} 直至 data: [DONE]。
func (h *AiHandler) GenerateStream(c *gin.Context) {
	var req struct {
		Model     string       `json:"model"`     // 模型名
		Prompt    string       `json:"prompt"`    // 系统提示词（单轮形态）
		Content   string       `json:"content"`   // 用户输入（单轮形态）
		Messages  []ai.Message `json:"messages"`  // 多轮对话（可选，携带时优先）
		MaxTokens int          `json:"max_tokens"` // 输出上限（可选，默认 1024，上限 16000）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var stream ai.ChatStream
	var err error
	if len(req.Messages) > 0 {
		stream, err = h.ai.GenerateChatStream(c.Request.Context(), req.Model, req.Messages, req.MaxTokens)
	} else {
		stream, err = h.ai.GenerateStream(c.Request.Context(), req.Model, req.Prompt, req.Content)
	}
	if err != nil {
		h.logger.Error("AI 流式生成失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	defer stream.Close()

	// SSE 响应头（关闭缓冲，逐块下发）
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
	// 逐 chunk 写出增量文本（JSON 编码，规避增量文本含换行导致的 SSE 解析歧义）
	enc := json.NewEncoder(c.Writer)
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			break // io.EOF 或读取异常均视为结束
		}
		if _, err := c.Writer.Write([]byte("data: ")); err != nil {
			break
		}
		if err := enc.Encode(map[string]string{"text": chunk.Text}); err != nil {
			break
		}
		if _, err := c.Writer.Write([]byte("\n")); err != nil {
			break
		}
		flusher.Flush()
	}
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// Embedding 向量嵌入（POST /api/v1/admin/ai/embedding，body: {model, text}）。
func (h *AiHandler) Embedding(c *gin.Context) {
	var req struct {
		Model string `json:"model"` // 模型名
		Text  string `json:"text"`  // 待嵌入文本
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	vector, err := h.ai.Embedding(c.Request.Context(), req.Model, req.Text)
	if err != nil {
		h.logger.Error("AI 嵌入失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"embedding": vector})
}

// ---------- 内置场景：智能回复助手 / SEO 建议 ----------

// GenReply 智能回复助手（POST /api/v1/admin/ai/gen/reply?post_id=&action=，
// action ∈ continue / polish / translate；body {content} 为当前编辑正文（缺省回退查库）。
// 返回处理后的文本）。
func (h *AiHandler) GenReply(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Query("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	action := c.Query("action")
	// 请求体：{content} 编辑框当前正文（可选——为空时服务端按 post_id 查库）
	var req struct {
		Content string `json:"content"`
	}
	// 空 body 允许（旧调用兼容），忽略解析错误
	_ = c.ShouldBindJSON(&req)
	text, err := h.ai.GenReplyAssistant(c.Request.Context(), postID, action, req.Content)
	if err != nil {
		h.logger.Error("AI 智能回复助手失败", zap.Int64("post_id", postID), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"text": text})
}

// GenSeoAdvice 生成 SEO 建议（POST /api/v1/admin/ai/gen/seo-advice?post_id=，
// 返回 {title, description, keywords} 结构化建议，前端回填 SEO 面板）。
func (h *AiHandler) GenSeoAdvice(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Query("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	advice, err := h.ai.GenSeoAdvice(c.Request.Context(), postID)
	if err != nil {
		h.logger.Error("AI 生成 SEO 建议失败", zap.Int64("post_id", postID), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"title": advice.Title, "description": advice.Description, "keywords": advice.Keywords})
}
