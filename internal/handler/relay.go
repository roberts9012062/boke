// 中继站对接控制器：后台配置（测试/保存）与大世界前台列表。
package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
	"github.com/roberts9012062/boke/internal/service"
)

// RelayHandler 中继站控制器。
type RelayHandler struct {
	svc *service.RelayService
}

// NewRelayHandler 构造中继站控制器。
func NewRelayHandler(svc *service.RelayService) *RelayHandler {
	return &RelayHandler{svc: svc}
}

// GetConfig GET /api/v1/admin/relay —— 对接配置回显。
func (h *RelayHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, cfg)
}

// TestConnectionReq 连接测试请求体。
type TestConnectionReq struct {
	URL     string `json:"url"`
	SiteKey string `json:"site_key"`
	Mode    string `json:"mode"`
}

// TestConnection POST /api/v1/admin/relay/test —— 实时握手回显元信息与配额。
func (h *RelayHandler) TestConnection(c *gin.Context) {
	var req TestConnectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	out, err := h.svc.TestConnection(c.Request.Context(), req.URL, req.SiteKey, req.Mode)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

// SaveConfigReq 配置保存请求体（全量显式字段）。
type SaveConfigReq struct {
	Enabled            bool   `json:"enabled"`
	URL                string `json:"url"`
	SiteKey            string `json:"site_key"`
	Mode               string `json:"mode"`
	DefaultCategory    string `json:"default_category"`
	LocalRetentionDays int    `json:"local_retention_days"`
}

// SaveConfig PUT /api/v1/admin/relay —— 保存并即时重启订阅任务。
func (h *RelayHandler) SaveConfig(c *gin.Context) {
	var req SaveConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	err := h.svc.SaveConfig(c.Request.Context(), service.SaveConfigParams{
		Enabled: req.Enabled, URL: req.URL, SiteKey: req.SiteKey, Mode: req.Mode,
		DefaultCategory: req.DefaultCategory, LocalRetentionDays: req.LocalRetentionDays,
	})
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}

// ListWorld GET /api/v1/relay/contents —— 大世界前台列表（本地缓存分页）。
func (h *RelayHandler) ListWorld(c *gin.Context) {
	before := time.Now()
	if raw := c.Query("before"); raw != "" {
		if ts, err := strconv.ParseInt(raw, 10, 64); err == nil && ts > 0 {
			before = time.Unix(ts, 0)
		}
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	items, err := h.svc.ListWorld(c.Request.Context(), c.Query("category"), before, limit)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// WorldStatus GET /api/v1/relay/status —— 前台判断大世界板块是否可见。
func (h *RelayHandler) WorldStatus(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"enabled": cfg.Enabled})
}
