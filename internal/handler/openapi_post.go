// internal/handler/openapi_post.go
// 开放网关「发布文章」：POST /api/v1/open/posts（posts.create，X-Api-Key 鉴权后进入）。
//
// 说明：浏览器插件「AI 生成文章」链路的发布端点——以 Key 绑定用户（迁移 021）的
// 身份创建文章，复用主站 PostService 完整校验与落库；正文格式与前台一致支持 html。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// CreatePost 处理发布文章（POST /api/v1/open/posts）。
// 请求体与主站发帖一致（model.CreatePostReq）；Key 未绑定用户时拒绝（无归属）。
func (h *OpenAPIHandler) CreatePost(c *gin.Context) {
	userID := middleware.GetAPIKeyUserID(c)
	if userID == 0 {
		resp.Fail(c, 403, errs.New(errs.CodeForbidden,
			"该 API Key 未绑定用户：请在后台重新生成 Key 后再发布"))
		return
	}

	var req model.CreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	// 开放通道缺省归一（插件端可显式传参覆盖）：
	// 文章形态 + 文字媒体 + 公开 + 发布；标签/媒体空集合防 nil
	if req.PostKind == "" {
		req.PostKind = "article"
	}
	if req.ContentType == "" {
		req.ContentType = "text"
	}
	if req.Visibility == "" {
		req.Visibility = "public"
	}
	if req.Status == "" {
		req.Status = "published"
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	if req.MediaIDs == nil {
		req.MediaIDs = []int64{}
	}

	postID, err := h.posts.Create(c.Request.Context(), userID, req)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, gin.H{"id": postID})
}
