// internal/handler/post.go
// 帖子控制器：参数绑定与响应组装（无业务判断，全部委托 service 层）。
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

// PostHandler 帖子控制器（连接器类）。
type PostHandler struct {
	posts  *service.PostService // 帖子业务
	logger *zap.Logger          // 错误日志（5xx 留痕）
}

// NewPostHandler 创建帖子控制器。
func NewPostHandler(posts *service.PostService, logger *zap.Logger) *PostHandler {
	return &PostHandler{posts: posts, logger: logger}
}

// failWithLog 失败响应：内部错误（6001）记录日志（含请求路径），其余直接返回。
func (h *PostHandler) failWithLog(c *gin.Context, err error) {
	if errs.From(err).Code == errs.CodeInternal {
		h.logger.Error("请求处理失败",
			zap.String("path", c.Request.URL.Path),
			zap.String("request_id", middleware.GetRequestID(c)),
			zap.Error(err),
		)
	}
	resp.FailFrom(c, err)
}

// Create 处理发帖/存草稿（POST /api/v1/posts，需登录）。
func (h *PostHandler) Create(c *gin.Context) {
	var req model.CreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	postID, err := h.posts.Create(c.Request.Context(), middleware.GetUserID(c), req)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": postID})
}

// List 处理时间线列表（GET /api/v1/posts）。
func (h *PostHandler) List(c *gin.Context) {
	// 类型过滤：全部（空）/text/image/audio/video
	contentType := c.Query("type")
	// 分页：page 从 1 起，page_size 默认 20 上限 100
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.posts.ListTimeline(c.Request.Context(), contentType, page, pageSize, middleware.GetUserID(c))
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"items":     items,
	})
}

// Get 处理帖子详情（GET /api/v1/posts/:id）。
func (h *PostHandler) Get(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	detail, err := h.posts.GetDetail(c.Request.Context(), postID, middleware.GetUserID(c), c.Query("guest_token"))
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, detail)
}

// Update 处理更新帖子（PUT /api/v1/posts/:id，需登录）。
func (h *PostHandler) Update(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req model.UpdatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.posts.Update(c.Request.Context(), middleware.GetUserID(c), postID, req); err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": postID})
}

// Publish 处理发布草稿（POST /api/v1/posts/:id/publish，需登录）。
func (h *PostHandler) Publish(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.posts.Publish(c.Request.Context(), middleware.GetUserID(c), postID); err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": postID})
}

// Delete 处理删除帖子（DELETE /api/v1/posts/:id，需登录）。
func (h *PostHandler) Delete(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.posts.Delete(c.Request.Context(), middleware.GetUserID(c), postID); err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// ListDrafts 处理草稿箱（GET /api/v1/me/drafts，需登录）。
func (h *PostHandler) ListDrafts(c *gin.Context) {
	drafts, err := h.posts.ListDrafts(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, drafts)
}
