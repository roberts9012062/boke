// internal/handler/comment.go
// 评论控制器：楼中楼列表、发表（登录/匿名）、回复、点赞、删除。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// CommentHandler 评论控制器（连接器类）。
type CommentHandler struct {
	comments *service.CommentService // 评论业务
}

// NewCommentHandler 创建评论控制器。
func NewCommentHandler(comments *service.CommentService) *CommentHandler {
	return &CommentHandler{comments: comments}
}

// List 处理评论列表（GET /api/v1/posts/:id/comments）。
func (h *CommentHandler) List(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	comments, err := h.comments.List(c.Request.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, comments)
}

// commentReq 发表/回复评论请求体。
type commentReq struct {
	Content    string `json:"content"`     // 评论内容
	GuestToken string `json:"guest_token"` // 匿名 token（未登录时必填）
}

// Create 处理发表评论（POST /api/v1/posts/:id/comments）。
func (h *CommentHandler) Create(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	commentID, err := h.comments.Create(c.Request.Context(), postID, middleware.GetUserID(c), service.CommentInput{
		Content:    req.Content,
		GuestToken: req.GuestToken,
	})
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"id": commentID})
}

// Reply 处理回复评论（POST /api/v1/comments/:id/reply）。
func (h *CommentHandler) Reply(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	commentID, err := h.comments.Reply(c.Request.Context(), targetID, middleware.GetUserID(c), service.CommentInput{
		Content:    req.Content,
		GuestToken: req.GuestToken,
	})
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"id": commentID})
}

// Like 处理评论点赞（POST /api/v1/comments/:id/like，需登录）。
func (h *CommentHandler) Like(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || commentID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	count, added, err := h.comments.Like(c.Request.Context(), commentID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"like_count": count, "added": added})
}

// Delete 处理删除评论（DELETE /api/v1/comments/:id，需登录）。
func (h *CommentHandler) Delete(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || commentID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.comments.Delete(c.Request.Context(), commentID, middleware.GetUserID(c)); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}
