// internal/handler/social.go
// 社交控制器：话题、搜索、通知、用户关系（关注/粉丝/编辑资料/收藏/赞过）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yueyan/boke/internal/middleware"
	"github.com/yueyan/boke/internal/service"
	"github.com/yueyan/boke/pkg/errs"
	"github.com/yueyan/boke/pkg/resp"
)

// SocialHandler 社交控制器（连接器类，聚合 M1.5 各业务）。
type SocialHandler struct {
	topics *service.TopicService    // 话题
	search *service.SearchService   // 搜索
	notify *service.NotificationService // 通知
	follow *service.FollowService   // 用户关系
	logger *zap.Logger              // 错误日志（5xx 留痕）
}

// NewSocialHandler 创建社交控制器。
func NewSocialHandler(
	topics *service.TopicService,
	search *service.SearchService,
	notify *service.NotificationService,
	follow *service.FollowService,
	logger *zap.Logger,
) *SocialHandler {
	return &SocialHandler{topics: topics, search: search, notify: notify, follow: follow, logger: logger}
}

// ---------- 话题 ----------

// ListTopics 话题列表（GET /api/v1/topics）。
func (h *SocialHandler) ListTopics(c *gin.Context) {
	topics, err := h.topics.List(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, topics)
}

// GetTopic 话题详情（GET /api/v1/topics/:name）。
func (h *SocialHandler) GetTopic(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	topic, err := h.topics.Detail(c.Request.Context(), name, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, topic)
}

// ListTopicPosts 话题帖子流（GET /api/v1/topics/:name/posts）。
func (h *SocialHandler) ListTopicPosts(c *gin.Context) {
	name := c.Param("name")
	sort := c.DefaultQuery("sort", "latest")
	page, pageSize := parsePage(c)
	items, total, err := h.topics.Posts(c.Request.Context(), name, sort, page, pageSize, middleware.GetUserID(c))
	if err != nil {
		h.logger.Error("话题帖子流失败", zap.String("path", c.Request.URL.Path), zap.String("name", name), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// FollowTopic 关注话题（POST /api/v1/topics/:name/follow，需登录）。
func (h *SocialHandler) FollowTopic(c *gin.Context) {
	if err := h.topics.Follow(c.Request.Context(), c.Param("name"), middleware.GetUserID(c)); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"following": true})
}

// UnfollowTopic 取消关注话题（DELETE /api/v1/topics/:name/follow，需登录）。
func (h *SocialHandler) UnfollowTopic(c *gin.Context) {
	if err := h.topics.Unfollow(c.Request.Context(), c.Param("name"), middleware.GetUserID(c)); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"following": false})
}

// ---------- 搜索 ----------

// Search 搜索（GET /api/v1/search?q=&page=&page_size=）。
func (h *SocialHandler) Search(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请输入搜索关键词"))
		return
	}
	page, pageSize := parsePage(c)
	result, err := h.search.Search(c.Request.Context(), keyword, page, pageSize, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, result)
}

// ---------- 通知 ----------

// ListNotifications 通知列表（GET /api/v1/notifications?type=&page=，需登录）。
func (h *SocialHandler) ListNotifications(c *gin.Context) {
	notifyType := c.Query("type")
	page, pageSize := parsePage(c)
	items, total, err := h.notify.List(c.Request.Context(), middleware.GetUserID(c), notifyType, page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// CountUnreadNotifications 未读数（GET /api/v1/notifications/unread-count，需登录，角标轮询）。
func (h *SocialHandler) CountUnreadNotifications(c *gin.Context) {
	count, err := h.notify.CountUnread(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"unread": count})
}

// MarkNotificationRead 单条已读（PUT /api/v1/notifications/:id/read，需登录）。
func (h *SocialHandler) MarkNotificationRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.notify.MarkRead(c.Request.Context(), middleware.GetUserID(c), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"read": true})
}

// MarkAllNotificationsRead 全部已读（PUT /api/v1/notifications/read-all，需登录）。
func (h *SocialHandler) MarkAllNotificationsRead(c *gin.Context) {
	if err := h.notify.MarkAllRead(c.Request.Context(), middleware.GetUserID(c)); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"read": true})
}

// ---------- 用户关系 ----------

// FollowUser 关注用户（PUT /api/v1/users/:id/follow，需登录）。
func (h *SocialHandler) FollowUser(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	added, err := h.follow.Follow(c.Request.Context(), middleware.GetUserID(c), targetID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"following": true, "added": added})
}

// UnfollowUser 取消关注（DELETE /api/v1/users/:id/follow，需登录）。
func (h *SocialHandler) UnfollowUser(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if _, err := h.follow.Unfollow(c.Request.Context(), middleware.GetUserID(c), targetID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"following": false})
}

// ListFollowers 粉丝列表（GET /api/v1/users/:id/followers）。
func (h *SocialHandler) ListFollowers(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	page, pageSize := parsePage(c)
	items, total, err := h.follow.Followers(c.Request.Context(), userID, middleware.GetUserID(c), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// ListFollowing 关注列表（GET /api/v1/users/:id/following）。
func (h *SocialHandler) ListFollowing(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	page, pageSize := parsePage(c)
	items, total, err := h.follow.Following(c.Request.Context(), userID, middleware.GetUserID(c), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// UpdateProfile 编辑资料（PUT /api/v1/me/profile，需登录）。
func (h *SocialHandler) UpdateProfile(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname"` // 昵称
		Bio      string `json:"bio"`      // 简介
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.follow.UpdateProfile(c.Request.Context(), middleware.GetUserID(c), req.Nickname, req.Bio); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"updated": true})
}

// UpdateAvatar 更新头像（PUT /api/v1/me/avatar，需登录）。
// 说明：前端先上传媒体（POST /media）拿到地址，再调用本接口写入用户头像。
func (h *SocialHandler) UpdateAvatar(c *gin.Context) {
	var req struct {
		AvatarURL string `json:"avatar_url"` // 头像地址（/media/...）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.follow.UpdateAvatar(c.Request.Context(), middleware.GetUserID(c), req.AvatarURL); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"avatar_url": req.AvatarURL})
}

// Favorites 我的收藏（GET /api/v1/me/favorites，需登录）。
func (h *SocialHandler) Favorites(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.follow.Favorites(c.Request.Context(), middleware.GetUserID(c), page, pageSize)
	if err != nil {
		// 内部错误落日志（5xx 留痕，便于排查）
		h.logger.Error("查询我的收藏失败", zap.Int64("uid", middleware.GetUserID(c)), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// LikedPosts 用户赞过的帖子（GET /api/v1/users/:id/liked）。
func (h *SocialHandler) LikedPosts(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	page, pageSize := parsePage(c)
	items, total, err := h.follow.LikedPosts(c.Request.Context(), userID, middleware.GetUserID(c), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// UserPosts 用户主页帖子流（GET /api/v1/users/:id/posts?type=，type 过滤媒体 Tab）。
func (h *SocialHandler) UserPosts(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	contentType := c.Query("type")
	page, pageSize := parsePage(c)
	posts, total, err := h.topics.ListByAuthor(c.Request.Context(), userID, contentType, page, pageSize, middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": posts})
}

// parsePage 解析分页参数（page 从 1 起，page_size 默认 20 上限 100）。
func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
