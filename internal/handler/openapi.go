// internal/handler/openapi.go
// 接口开放控制器：目录查询与凭证管理（参数绑定与响应组装，业务在 service 层）。
// 路由：管理 /api/v1/admin/open-api/*（JWT + RBAC）；开放网关 /api/v1/open/* 由中间件鉴权，
// 直接复用现有公开 handler（Post/Social/Comment/Page/Site/User），本控制器不参与网关转发。
package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// OpenAPIHandler 接口开放控制器（连接器类）。
type OpenAPIHandler struct {
	openapi *service.OpenAPIService // 接口开放业务
	ai      *service.AiService      // AI 业务（开放网关 ai.models / ai.chat 复用统一对话链路）
	auth    *service.AuthService    // 认证业务（/open/me 凭 Key 返回绑定用户资料）
	posts   *service.PostService    // 帖子业务（/open/posts 凭 Key 绑定用户发文章）
	logger  *zap.Logger             // 错误日志（5xx 留痕）
}

// NewOpenAPIHandler 创建接口开放控制器。
func NewOpenAPIHandler(openapi *service.OpenAPIService, ai *service.AiService, auth *service.AuthService, posts *service.PostService, logger *zap.Logger) *OpenAPIHandler {
	return &OpenAPIHandler{openapi: openapi, ai: ai, auth: auth, posts: posts, logger: logger}
}

// failWithLog 失败响应：内部错误（6001）记录日志（含请求路径），其余直接返回。
func (h *OpenAPIHandler) failWithLog(c *gin.Context, err error) {
	if errs.From(err).Code == errs.CodeInternal {
		h.logger.Error("请求处理失败",
			zap.String("path", c.Request.URL.Path),
			zap.String("request_id", middleware.GetRequestID(c)),
			zap.Error(err),
		)
	}
	resp.FailFrom(c, err)
}

// Catalog 处理开放接口目录（GET /api/v1/admin/open-api/catalog）。
// 返回全部可开放的接口（标识/方法/路径/名称/描述/参数说明），供页面多选与手册生成。
func (h *OpenAPIHandler) Catalog(c *gin.Context) {
	resp.OK(c, gin.H{"items": model.OpenAPICatalog()})
}

// ListKeys 处理凭证列表（GET /api/v1/admin/open-api/keys，按创建时间倒序）。
func (h *OpenAPIHandler) ListKeys(c *gin.Context) {
	keys, err := h.openapi.ListKeys(c.Request.Context())
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"items": keys})
}

// CreateKey 处理生成凭证（POST /api/v1/admin/open-api/keys）。
// 请求体：name 备注名 / endpoints 勾选的接口标识 / expire_days 过期天数（空=永久）。
// 附加行为：Key 自动绑定当前操作管理员（JWT 身份），供 /open/me 返回资料。
func (h *OpenAPIHandler) CreateKey(c *gin.Context) {
	var req model.CreateOpenAPIKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	record, err := h.openapi.CreateKey(c.Request.Context(), req, middleware.GetUserID(c))
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, record)
}

// DeleteKey 处理删除凭证（DELETE /api/v1/admin/open-api/keys/:id）。
func (h *OpenAPIHandler) DeleteKey(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	if err := h.openapi.DeleteKey(c.Request.Context(), id); err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": id})
}

// UpdateKeyEndpoints 处理权限设置（PUT /api/v1/admin/open-api/keys/:id/endpoints）。
// 请求体：{endpoints: [...]}——增/减该 Key 可调用的接口（校验与创建同规）。
func (h *OpenAPIHandler) UpdateKeyEndpoints(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	var req struct {
		Endpoints []string `json:"endpoints"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	record, err := h.openapi.UpdateKeyEndpoints(c.Request.Context(), id, req.Endpoints)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, record)
}
