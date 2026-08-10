// internal/handler/site.go
// 站点信息控制器（公开接口，无业务判断，仅透传 service 结果）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/yueyan/boke/internal/service"
	"github.com/yueyan/boke/pkg/resp"
)

// SiteHandler 站点信息控制器（连接器类）。
type SiteHandler struct {
	site *service.SiteService // 站点信息服务
}

// NewSiteHandler 创建站点信息控制器。
func NewSiteHandler(site *service.SiteService) *SiteHandler {
	return &SiteHandler{site: site}
}

// GetMeta 站点元信息（GET /api/v1/meta）：站点名/描述/默认主题。
func (h *SiteHandler) GetMeta(c *gin.Context) {
	resp.OK(c, h.site.Meta(c.Request.Context()))
}
