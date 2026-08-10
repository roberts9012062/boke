// internal/handler/reaction.go
// 互动控制器：帖子点赞/收藏 + 匿名身份签发。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yueyan/boke/internal/auth"
	"github.com/yueyan/boke/internal/middleware"
	"github.com/yueyan/boke/internal/service"
	"github.com/yueyan/boke/pkg/errs"
	"github.com/yueyan/boke/pkg/resp"
)

// ReactionHandler 互动控制器（连接器类）。
type ReactionHandler struct {
	reactions *service.ReactionService // 互动业务
	guests    *auth.GuestManager       // 匿名身份管理器
}

// NewReactionHandler 创建互动控制器。
func NewReactionHandler(reactions *service.ReactionService, guests *auth.GuestManager) *ReactionHandler {
	return &ReactionHandler{reactions: reactions, guests: guests}
}

// LikePost 处理点赞（POST /api/v1/posts/:id/like，需登录）。
func (h *ReactionHandler) LikePost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	count, added, err := h.reactions.LikePost(c.Request.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"like_count": count, "added": added})
}

// UnlikePost 处理取消点赞（DELETE /api/v1/posts/:id/like，需登录）。
func (h *ReactionHandler) UnlikePost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	count, err := h.reactions.UnlikePost(c.Request.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"like_count": count})
}

// FavoritePost 处理收藏（POST /api/v1/posts/:id/favorite，需登录）。
func (h *ReactionHandler) FavoritePost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	count, added, err := h.reactions.FavoritePost(c.Request.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"favorite_count": count, "added": added})
}

// UnfavoritePost 处理取消收藏（DELETE /api/v1/posts/:id/favorite，需登录）。
func (h *ReactionHandler) UnfavoritePost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	count, err := h.reactions.UnfavoritePost(c.Request.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"favorite_count": count})
}

// PostState 处理帖子互动状态（GET /api/v1/posts/:id/state）。
func (h *ReactionHandler) PostState(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	state, err := h.reactions.GetPostState(c.Request.Context(), postID, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, state)
}

// guestReq 匿名身份请求体。
type guestReq struct {
	Nickname string `json:"nickname"` // 匿名昵称（可选，默认匿名访客+随机后缀）
}

// GuestIdentity 处理匿名身份签发（POST /api/v1/guest-identity）。
func (h *ReactionHandler) GuestIdentity(c *gin.Context) {
	var req guestReq
	// 请求体缺省时忽略（昵称可选）
	_ = c.ShouldBindJSON(&req)

	token, name, err := h.guests.Issue(req.Nickname)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"guest_token": token, "guest_name": name})
}
