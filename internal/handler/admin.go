// internal/handler/admin.go
// 后台控制器（需求 4.x）：仪表盘、内容管理、评论管理、用户管理、站点设置。
// 路由统一挂 RequireAuth + RequireAdmin（仅 admin 角色，router 层已配置）。
package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// AdminHandler 后台控制器（连接器类）。
type AdminHandler struct {
	admin *service.AdminService // 后台业务
}

// NewAdminHandler 创建后台控制器。
func NewAdminHandler(admin *service.AdminService) *AdminHandler {
	return &AdminHandler{admin: admin}
}

// Dashboard 仪表盘（GET /api/v1/admin/dashboard）。
func (h *AdminHandler) Dashboard(c *gin.Context) {
	data, err := h.admin.Dashboard(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, data)
}

// ListPosts 内容管理列表（GET /api/v1/admin/posts）。
func (h *AdminHandler) ListPosts(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.admin.ListPosts(c.Request.Context(),
		c.Query("type"), c.Query("status"), c.Query("q"), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// SetPostStatus 上下架（PUT /api/v1/admin/posts/:id/status）。
func (h *AdminHandler) SetPostStatus(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Status string `json:"status"` // published / taken_down
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.SetPostStatus(c.Request.Context(), postID, req.Status); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"status": req.Status})
}

// DeletePost 删除内容（DELETE /api/v1/admin/posts/:id）。
func (h *AdminHandler) DeletePost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.DeletePost(c.Request.Context(), postID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// GetPost 后台编辑详情（GET /api/v1/admin/posts/:id，设计稿《后台编辑》四画板）。
func (h *AdminHandler) GetPost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	detail, err := h.admin.GetPostDetail(c.Request.Context(), postID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, detail)
}

// UpdatePost 后台编辑保存（PUT /api/v1/admin/posts/:id）。
// body：{title, content, tags, media_ids, visibility, status}——status draft=保存草稿 / published=更新发布。
func (h *AdminHandler) UpdatePost(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req model.AdminUpdatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.UpdatePost(c.Request.Context(), postID, req); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"id": postID})
}

// ListComments 评论管理列表（GET /api/v1/admin/comments）。
func (h *AdminHandler) ListComments(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.admin.ListComments(c.Request.Context(),
		c.Query("status"), c.Query("q"), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// DeleteComment 删除评论（DELETE /api/v1/admin/comments/:id）。
func (h *AdminHandler) DeleteComment(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || commentID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.DeleteComment(c.Request.Context(), commentID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// CommentStats 评论统计（GET /api/v1/admin/comments/stats，设计稿统计条）。
func (h *AdminHandler) CommentStats(c *gin.Context) {
	stats, err := h.admin.CommentStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// SetCommentStatus 评论隐藏/恢复（PUT /api/v1/admin/comments/:id/status）。
// body：{status: visible|hidden}——隐藏后前台列表不再展示（M2 评论治理）。
func (h *AdminHandler) SetCommentStatus(c *gin.Context) {
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || commentID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Status string `json:"status"` // visible / hidden
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.SetCommentStatus(c.Request.Context(), commentID, req.Status); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"status": req.Status})
}

// ListUsers 用户管理列表（GET /api/v1/admin/users）。
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.admin.ListUsers(c.Request.Context(), c.Query("q"), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// SetUserStatus 封禁/解封（PUT /api/v1/admin/users/:id/status）。
// body：{status, reason?, until?}——M2 封禁支持原因与期限（写 ban_records）。
func (h *AdminHandler) SetUserStatus(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Status string `json:"status"` // active / banned
		Reason string `json:"reason"` // 封禁原因（banned 时可选）
		Until  string `json:"until"`  // 解封时间（ISO8601，空 = 永久）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	// 解析解封时间（空 = 永久封禁）
	var until *time.Time
	if req.Until != "" {
		parsed, err := time.Parse(time.RFC3339, req.Until)
		if err != nil {
			resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "解封时间格式不正确"))
			return
		}
		until = &parsed
	}
	if err := h.admin.SetUserStatus(c.Request.Context(), userID, req.Status, middleware.GetUserID(c), req.Reason, until); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"status": req.Status})
}

// SetUserRole 角色调整（PUT /api/v1/admin/users/:id/role）。
// body：{role: admin|user}——落库 + casbin 即时生效；该用户需重新登录生效（JWT 角色登录时签发）。
func (h *AdminHandler) SetUserRole(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Role string `json:"role"` // admin / user
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.SetUserRole(c.Request.Context(), userID, req.Role); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"role": req.Role})
}

// UserStats 用户统计（GET /api/v1/admin/users/stats，封禁管理页统计条）。
func (h *AdminHandler) UserStats(c *gin.Context) {
	stats, err := h.admin.UserStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// ---------- 媒体库（M2.9） ----------

// MediaStats 媒体统计（GET /api/v1/admin/media/stats，设计稿统计条）。
func (h *AdminHandler) MediaStats(c *gin.Context) {
	stats, err := h.admin.MediaStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// ListMedia 媒体列表（GET /api/v1/admin/media?type=&q=&page=）。
func (h *AdminHandler) ListMedia(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.admin.ListMedia(c.Request.Context(), c.Query("type"), c.Query("q"), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// DeleteMedia 删除媒体（DELETE /api/v1/admin/media/:id，解除引用 + 删文件）。
func (h *AdminHandler) DeleteMedia(c *gin.Context) {
	mediaID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || mediaID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.DeleteMedia(c.Request.Context(), mediaID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// ---------- 标签分类（M2.9） ----------

// TagStats 标签统计（GET /api/v1/admin/tags/stats，设计稿统计条）。
func (h *AdminHandler) TagStats(c *gin.Context) {
	stats, err := h.admin.TagStats(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, stats)
}

// ListTags 标签列表（GET /api/v1/admin/tags?q=&page=）。
func (h *AdminHandler) ListTags(c *gin.Context) {
	page, pageSize := parsePage(c)
	items, total, err := h.admin.ListTags(c.Request.Context(), c.Query("q"), page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// RenameTag 重命名标签（PUT /api/v1/admin/tags/:id，body: {name, slug}）。
func (h *AdminHandler) RenameTag(c *gin.Context) {
	tagID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || tagID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Name     string `json:"name"`     // 新名称
		Slug     string `json:"slug"`     // 新别名
		Category string `json:"category"` // 分类（设计稿：情绪/栏目/体裁/临时）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.RenameTag(c.Request.Context(), tagID, req.Name, req.Slug, req.Category); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"renamed": true})
}

// MergeTag 合并标签（POST /api/v1/admin/tags/:id/merge，body: {target_id}）。
func (h *AdminHandler) MergeTag(c *gin.Context) {
	tagID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || tagID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		TargetID int64 `json:"target_id"` // 目标标签
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.MergeTag(c.Request.Context(), tagID, req.TargetID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"merged": true})
}

// DeleteTag 删除标签（DELETE /api/v1/admin/tags/:id，解除关联）。
func (h *AdminHandler) DeleteTag(c *gin.Context) {
	tagID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || tagID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.DeleteTag(c.Request.Context(), tagID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}

// GetSettings 站点设置（GET /api/v1/admin/settings）。
func (h *AdminHandler) GetSettings(c *gin.Context) {
	settings, err := h.admin.Settings(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, settings)
}

// SaveSettings 保存站点设置（PUT /api/v1/admin/settings）。
func (h *AdminHandler) SaveSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.admin.SaveSettings(c.Request.Context(), req); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"saved": true})
}
