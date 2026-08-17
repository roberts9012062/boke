// internal/handler/page.go
// 自定义页面控制器：参数绑定与响应组装（无业务判断，全部委托 service 层）。
// 路由：公开 GET /api/v1/pages/:slug（前台）；管理 /api/v1/admin/pages（CRUD）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// PageHandler 自定义页面控制器（连接器类）。
type PageHandler struct {
	pages  *service.PageService // 页面业务
	logger *zap.Logger          // 错误日志（5xx 留痕）
}

// NewPageHandler 创建自定义页面控制器。
func NewPageHandler(pages *service.PageService, logger *zap.Logger) *PageHandler {
	return &PageHandler{pages: pages, logger: logger}
}

// failWithLog 失败响应：内部错误（6001）记录日志（含请求路径），其余直接返回。
func (h *PageHandler) failWithLog(c *gin.Context, err error) {
	if errs.From(err).Code == errs.CodeInternal {
		h.logger.Error("请求处理失败",
			zap.String("path", c.Request.URL.Path),
			zap.String("request_id", middleware.GetRequestID(c)),
			zap.Error(err),
		)
	}
	resp.FailFrom(c, err)
}

// parseID 解析路径参数 ID（非法时返回 0 与参数错误）。
func parseID(c *gin.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errs.New(errs.CodeBadRequest, "参数不正确")
	}
	return id, nil
}

// GetBySlug 处理前台页面详情（GET /api/v1/pages/:slug，公开）。
// 仅已发布页面可见；草稿/不存在统一 404（不泄露存在性）。
func (h *PageHandler) GetBySlug(c *gin.Context) {
	detail, err := h.pages.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, detail)
}

// AdminList 处理后台页面列表（GET /api/v1/admin/pages，含草稿）。
func (h *PageHandler) AdminList(c *gin.Context) {
	items, err := h.pages.List(c.Request.Context())
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// AdminCreate 处理后台创建页面（POST /api/v1/admin/pages）。
func (h *PageHandler) AdminCreate(c *gin.Context) {
	var req model.CreatePageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	id, err := h.pages.Create(c.Request.Context(), req)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": id})
}

// AdminGet 处理后台页面详情（GET /api/v1/admin/pages/:id，编辑回显）。
func (h *PageHandler) AdminGet(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	page, err := h.pages.GetByID(c.Request.Context(), id)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, page)
}

// AdminUpdate 处理后台更新页面（PUT /api/v1/admin/pages/:id）。
func (h *PageHandler) AdminUpdate(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	var req model.UpdatePageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.pages.Update(c.Request.Context(), id, req); err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": id})
}

// AdminDelete 处理后台删除页面（DELETE /api/v1/admin/pages/:id）。
func (h *PageHandler) AdminDelete(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	if err := h.pages.Delete(c.Request.Context(), id); err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": id})
}
