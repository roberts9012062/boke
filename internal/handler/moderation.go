// internal/handler/moderation.go
// 内容治理控制器（M2）：前台举报提交 + 后台审核队列/敏感词/封禁管理。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// ModerationHandler 内容治理控制器（连接器类）。
type ModerationHandler struct {
	moderation *service.ModerationService // 内容治理服务
	logger     *zap.Logger                // 错误日志（5xx 留痕）
}

// NewModerationHandler 创建内容治理控制器。
func NewModerationHandler(moderation *service.ModerationService, logger *zap.Logger) *ModerationHandler {
	return &ModerationHandler{moderation: moderation, logger: logger}
}

// ---------- 前台：举报 ----------

// SubmitReport 提交举报（POST /api/v1/reports，需登录）。
// body：{target_type, target_id, reason, detail}。
func (h *ModerationHandler) SubmitReport(c *gin.Context) {
	var req struct {
		TargetType string `json:"target_type"` // post / comment / user
		TargetID   int64  `json:"target_id"`   // 对象 ID
		Reason     string `json:"reason"`      // 原因（预置选项）
		Detail     string `json:"detail"`      // 补充说明（可选）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.moderation.SubmitReport(c.Request.Context(), middleware.GetUserID(c), req.TargetType, req.TargetID, req.Reason, req.Detail); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"submitted": true})
}

// ---------- 后台：审核队列（举报工单） ----------

// ListReports 工单列表（GET /api/v1/admin/reports?status=&page=）。
func (h *ModerationHandler) ListReports(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.moderation.ListReports(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		h.logger.Error("查询举报工单失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// SetReportStatus 处理工单（PUT /api/v1/admin/reports/:id/status，body: {status}）。
func (h *ModerationHandler) SetReportStatus(c *gin.Context) {
	reportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || reportID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Status string `json:"status"` // resolved / rejected
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.moderation.SetReportStatus(c.Request.Context(), reportID, req.Status); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// ReportStats 审核队列统计（GET /api/v1/admin/reports/stats）。
func (h *ModerationHandler) ReportStats(c *gin.Context) {
	stats, err := h.moderation.ReportStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// ---------- 后台：敏感词 ----------

// ListSensitiveWords 词库列表（GET /api/v1/admin/sensitive-words?q=&page=）。
func (h *ModerationHandler) ListSensitiveWords(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.moderation.ListSensitiveWords(c.Request.Context(), c.Query("q"), page, pageSize)
	if err != nil {
		h.logger.Error("查询敏感词失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// AddSensitiveWord 添加敏感词（POST /api/v1/admin/sensitive-words，body: {word, level}）。
func (h *ModerationHandler) AddSensitiveWord(c *gin.Context) {
	var req struct {
		Word  string `json:"word"`  // 词内容
		Level string `json:"level"` // forbidden / review
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.moderation.AddSensitiveWord(c.Request.Context(), req.Word, req.Level); err != nil {
		resp.FailFrom(c, err)
		return
	}
	// 词表变更后刷新内存匹配表
	_ = h.moderation.ReloadForbidden(c.Request.Context())
	resp.OK(c, gin.H{"added": true})
}

// DeleteSensitiveWord 删除敏感词（DELETE /api/v1/admin/sensitive-words/:word）。
func (h *ModerationHandler) DeleteSensitiveWord(c *gin.Context) {
	word := c.Param("word")
	if word == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.moderation.DeleteSensitiveWord(c.Request.Context(), word); err != nil {
		resp.FailFrom(c, err)
		return
	}
	_ = h.moderation.ReloadForbidden(c.Request.Context())
	resp.OK(c, gin.H{"deleted": true})
}

// ---------- 后台：封禁记录 ----------

// SensitiveStats 敏感词统计（GET /api/v1/admin/sensitive-words/stats）。
func (h *ModerationHandler) SensitiveStats(c *gin.Context) {
	stats, err := h.moderation.SensitiveStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// ListBans 封禁记录（GET /api/v1/admin/bans?page=）。
func (h *ModerationHandler) ListBans(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.moderation.ListBans(c.Request.Context(), page, pageSize)
	if err != nil {
		h.logger.Error("查询封禁记录失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}
