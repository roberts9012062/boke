// internal/handler/seo.go
// SEO 控制器（M4）：全局设置 / 帖子元数据 / 健康度 / 批量修复 / SERP 预览 / sitemap / robots。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// SeoHandler SEO 控制器（连接器类）。
type SeoHandler struct {
	seo *service.SeoService // SEO 业务
}

// NewSeoHandler 创建 SEO 控制器。
func NewSeoHandler(seo *service.SeoService) *SeoHandler {
	return &SeoHandler{seo: seo}
}

// GetSettings 读取全局 SEO 设置（GET /api/v1/admin/seo/settings）。
func (h *SeoHandler) GetSettings(c *gin.Context) {
	settings, err := h.seo.Settings(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, settings)
}

// SaveSettings 保存全局 SEO 设置（PUT /api/v1/admin/seo/settings）。
func (h *SeoHandler) SaveSettings(c *gin.Context) {
	var req repository.SeoSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.seo.SaveSettings(c.Request.Context(), req); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}

// GetMeta 读取帖子 SEO 元数据（GET /api/v1/admin/seo/meta/:postId）。
func (h *SeoHandler) GetMeta(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("postId"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	meta, err := h.seo.Meta(c.Request.Context(), postID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, meta)
}

// SaveMeta 保存帖子 SEO 元数据（PUT /api/v1/admin/seo/meta/:postId）。
func (h *SeoHandler) SaveMeta(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("postId"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req repository.SeoMeta
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.seo.SaveMeta(c.Request.Context(), postID, req); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}

// ScanHealth 全量健康扫描（POST /api/v1/admin/seo/health/scan）。
func (h *SeoHandler) ScanHealth(c *gin.Context) {
	summary, err := h.seo.ScanHealth(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, summary)
}

// Health 健康度汇总（GET /api/v1/admin/seo/health）。
func (h *SeoHandler) Health(c *gin.Context) {
	summary, err := h.seo.ScanHealth(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, summary)
}

// BatchFix 批量修复（POST /api/v1/admin/seo/batch-fix，自动补齐缺省 SEO 字段）。
func (h *SeoHandler) BatchFix(c *gin.Context) {
	fixed, err := h.seo.BatchFix(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"fixed": fixed})
}

// SerpPreview SERP 预览（GET /api/v1/admin/seo/serp-preview?post_id=）。
func (h *SeoHandler) SerpPreview(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Query("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	preview, err := h.seo.SerpPreview(c.Request.Context(), postID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, preview)
}

// Sitemap sitemap.xml（GET /sitemap.xml，公开）。
func (h *SeoHandler) Sitemap(c *gin.Context) {
	xml, err := h.seo.SitemapXML(c.Request.Context())
	if err != nil {
		c.String(500, "sitemap 生成失败")
		return
	}
	c.Header("Content-Type", "application/xml")
	c.String(200, xml)
}

// Robots robots.txt（GET /robots.txt，公开）。
func (h *SeoHandler) Robots(c *gin.Context) {
	text, err := h.seo.RobotsTxt(c.Request.Context())
	if err != nil {
		c.String(500, "robots 生成失败")
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(200, text)
}
