// internal/service/admin.go
// 后台管理业务逻辑（需求 4.x）：仪表盘聚合、内容/评论/用户管理、站点设置。
package service

import (
	"context"
	"time"

	"github.com/yueyan/boke/internal/casbin"
	"github.com/yueyan/boke/internal/model"
	"github.com/yueyan/boke/internal/repository"
	"github.com/yueyan/boke/pkg/errs"
)

// AdminService 后台服务（连接器类）。
type AdminService struct {
	admin    *repository.AdminRepo     // 后台数据访问
	posts    *repository.PostRepo      // 帖子（上下架/删除）
	comments *repository.CommentRepo   // 评论（删除）
	settings *repository.SettingRepo   // 站点设置
	enforcer *casbin.Enforcer          // 角色查询
	bans     *repository.BanRepo       // 封禁记录（M2：封禁落库）
}

// NewAdminService 创建后台服务。
func NewAdminService(admin *repository.AdminRepo, posts *repository.PostRepo, comments *repository.CommentRepo, settings *repository.SettingRepo, enforcer *casbin.Enforcer, bans *repository.BanRepo) *AdminService {
	return &AdminService{admin: admin, posts: posts, comments: comments, settings: settings, enforcer: enforcer, bans: bans}
}

// DashboardData 仪表盘数据。
type DashboardData struct {
	Views7d     int64                     `json:"views_7d"`     // 近 7 日浏览（区间发布帖累计浏览，口径见仓库注释）
	ViewsTrend  float64                   `json:"views_trend"`  // 环比（%）
	Likes7d     int64                     `json:"likes_7d"`     // 近 7 日获赞（真实：post_reactions 聚合）
	LikesTrend  float64                   `json:"likes_trend"`
	Comments7d  int64                     `json:"comments_7d"`  // 近 7 日评论
	CommentsTrend float64                 `json:"comments_trend"`
	Posts7d     int64                     `json:"posts_7d"`     // 近 7 日新帖（已发布）
	PostsTrend  float64                   `json:"posts_trend"`
	TypeCounts  map[string]int64          `json:"type_counts"`  // 内容分布
	TrendSeries []repository.TrendPoint   `json:"trend_series"` // 近 7 日互动趋势（M1.7 新增）
	Activities  []repository.ActivityRow  `json:"activities"`   // 最近动态
}

// trend 环比计算（上期 0 时返回 0）。
func trend(current int64, prev int64) float64 {
	if prev == 0 {
		return 0
	}
	return float64(current-prev) / float64(prev) * 100
}

// Dashboard 仪表盘聚合（近 7 日 + 环比 + 内容分布 + 趋势 + 最近动态）。
func (s *AdminService) Dashboard(ctx context.Context) (*DashboardData, error) {
	now := time.Now()
	weekAgo := now.Add(-7 * 24 * time.Hour)
	prevWeek := now.Add(-14 * 24 * time.Hour)

	// 近 7 日与上 7 日指标（浏览/新帖同一查询；获赞按 post_reactions 真实聚合）
	views7d, posts7d, err := s.admin.StatsSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	viewsPrev, postsPrev, err := s.admin.StatsSince(ctx, prevWeek)
	if err != nil {
		return nil, err
	}
	likes7d, err := s.admin.LikesSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	likesPrev, err := s.admin.LikesSince(ctx, prevWeek)
	if err != nil {
		return nil, err
	}
	// 评论数（近 7 日与上 7 日）
	comments7d, err := s.admin.CountCommentsSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	commentsPrev, err := s.admin.CountCommentsSince(ctx, prevWeek)
	if err != nil {
		return nil, err
	}

	// 内容分布 + 互动趋势 + 最近动态
	typeCounts, err := s.admin.CountPostsByType(ctx)
	if err != nil {
		return nil, err
	}
	trendSeries, err := s.admin.TrendSeries(ctx, 7)
	if err != nil {
		return nil, err
	}
	activities, err := s.admin.RecentActivity(ctx)
	if err != nil {
		return nil, err
	}

	return &DashboardData{
		Views7d:       views7d,
		ViewsTrend:    trend(views7d, viewsPrev),
		Likes7d:       likes7d,
		LikesTrend:    trend(likes7d, likesPrev),
		Comments7d:    comments7d,
		CommentsTrend: trend(comments7d, commentsPrev),
		Posts7d:       posts7d,
		PostsTrend:    trend(posts7d, postsPrev),
		TypeCounts:    typeCounts,
		TrendSeries:   trendSeries,
		Activities:    activities,
	}, nil
}

// ---------- 内容管理 ----------

// ListPosts 内容管理列表。
func (s *AdminService) ListPosts(ctx context.Context, contentType string, status string, keyword string, page int, pageSize int) ([]model.Post, int64, error) {
	return s.admin.ListAdminPosts(ctx, contentType, status, keyword, page, pageSize)
}

// SetPostStatus 上下架（published ↔ taken_down）。
func (s *AdminService) SetPostStatus(ctx context.Context, postID int64, status string) error {
	if status != model.PostStatusPublished && status != model.PostStatusTakenDown {
		return errs.New(errs.CodeBadRequest, "状态仅支持 published / taken_down")
	}
	return s.posts.SetStatus(ctx, postID, status, nil)
}

// DeletePost 删除内容（软删）。
func (s *AdminService) DeletePost(ctx context.Context, postID int64) error {
	return s.posts.SetStatus(ctx, postID, model.PostStatusDeleted, nil)
}

// ---------- 评论管理 ----------

// ListComments 评论管理列表。
func (s *AdminService) ListComments(ctx context.Context, status string, keyword string, page int, pageSize int) ([]repository.CommentRow, int64, error) {
	return s.admin.ListAdminComments(ctx, status, keyword, page, pageSize)
}

// DeleteComment 删除评论（软删）。
func (s *AdminService) DeleteComment(ctx context.Context, commentID int64) error {
	return s.comments.SoftDelete(ctx, commentID)
}

// ---------- 用户管理 ----------

// ListUsers 用户管理列表（补充 casbin 角色）。
func (s *AdminService) ListUsers(ctx context.Context, keyword string, page int, pageSize int) ([]repository.UserAdminRow, int64, error) {
	users, total, err := s.admin.ListAdminUsers(ctx, keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	for i := range users {
		users[i].Role = s.enforcer.GetRole(users[i].Username)
	}
	return users, total, nil
}

// SetUserStatus 封禁/解封（active/banned；M2 封禁写入 ban_records）。
// 参数：userID 目标用户；status 状态；operatorID 操作者；reason 封禁原因；until 解封时间（nil=永久）。
func (s *AdminService) SetUserStatus(ctx context.Context, userID int64, status string, operatorID int64, reason string, until *time.Time) error {
	if status != "active" && status != "banned" {
		return errs.New(errs.CodeBadRequest, "状态仅支持 active / banned")
	}
	if err := s.admin.SetUserStatus(ctx, userID, status); err != nil {
		return err
	}
	// 封禁落库（ban_records 留痕；解封不写记录）
	if status == "banned" {
		if reason == "" {
			reason = "管理员封禁"
		}
		if err := s.bans.Create(ctx, repository.BanRecord{
			UserID:    userID,
			Reason:    reason,
			Until:     until,
			CreatedBy: operatorID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// UserStats 用户统计（封禁管理统计条：全部用户/已禁言）。
type UserStats struct {
	Total  int64 `json:"total"`  // 全部用户
	Banned int64 `json:"banned"` // 已禁言
}

// UserStats 用户统计（封禁管理页）。
func (s *AdminService) UserStats(ctx context.Context) (*UserStats, error) {
	total, err := s.admin.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	banned, err := s.admin.CountBannedUsers(ctx)
	if err != nil {
		return nil, err
	}
	return &UserStats{Total: total, Banned: banned}, nil
}

// ---------- 站点设置 ----------

// Settings 站点设置读取。
func (s *AdminService) Settings(ctx context.Context) (map[string]string, error) {
	return s.settings.All(ctx)
}

// SaveSettings 站点设置保存（站点名/描述/注册开关/评论开关/默认主题）。
func (s *AdminService) SaveSettings(ctx context.Context, updates map[string]string) error {
	// 白名单校验（防注入任意键）
	allowed := map[string]bool{
		"site_name": true, "site_description": true,
		"allow_register": true, "comment_open": true,
		"theme": true,
	}
	for key := range updates {
		if !allowed[key] {
			return errs.New(errs.CodeBadRequest, "不支持的设置项："+key)
		}
	}
	return s.settings.SetMany(ctx, updates)
}
