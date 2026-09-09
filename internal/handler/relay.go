// 中继站对接控制器：后台配置（测试/保存）与大世界前台列表。
package handler

import (
	"net/http"
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

// GetConfig GET /api/v1/admin/relay —— 对接配置回显（key 隐藏保管：只回 has_key，不回明文）。
func (h *RelayHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{
		"enabled":              cfg.Enabled,
		"url":                  cfg.URL,
		"mode":                 cfg.Mode,
		"default_category":     cfg.DefaultCategory,
		"local_retention_days": cfg.LocalRetentionDays,
		"has_key":              cfg.SiteKey != "",
		"claim_pending":        cfg.ClaimToken != "",
		"relay_meta_json":      cfg.RelayMetaJSON,
		"last_seq":             cfg.LastSeq,
		"updated_at":           cfg.UpdatedAt,
	})
}

// ApplyReq 自助申请请求体。
type ApplyReq struct {
	URL  string `json:"url"`
	Mode string `json:"mode"`
}

// Apply POST /api/v1/admin/relay/apply —— 向中继站申请对接许可（key 由后端隐藏保存）。
func (h *RelayHandler) Apply(c *gin.Context) {
	var req ApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	out, err := h.svc.ApplyForJoin(c.Request.Context(), req.URL, req.Mode)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
}

// Claim GET /api/v1/admin/relay/claim —— 轮询申请审批结果（审核中每 5 秒调用；通过即自动领 key）。
func (h *RelayHandler) Claim(c *gin.Context) {
	out, err := h.svc.PollClaim(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, out)
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

// Probe GET /api/v1/relay/probe?nonce= —— 申请质询探测端点（公开，协议 §4.12，v1.5）。
// 中继站申请时同步回调：nonce 为本站签发且未过期 → 回 boke 指纹与版本并原样回显；
// 否则 404（伪装成无此端点——探测者无从区分"不是 boke"与"nonce 不对"）。
func (h *RelayHandler) Probe(c *gin.Context) {
	nonce := c.Query("nonce")
	ok, version, echo := h.svc.ProbeAnswer(nonce)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found", "data": nil})
		return
	}
	resp.OK(c, gin.H{"boke": true, "version": version, "nonce": echo})
}
