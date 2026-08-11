// internal/handler/ai.go
// AI 控制器（M4）：供应商管理 / 任务配置 / 用量统计 / 内置场景触发（摘要/标签/评论审核）。
// 说明：全部挂 /admin 组（统一角色鉴权）；场景接口供后台编辑页与审核队列调用。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

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
