// internal/service/admin.go
// 后台管理业务逻辑（需求 4.x）：仪表盘聚合、内容/评论/用户管理、站点设置。
package service

import (
	"context"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/casbin"
	"github.com/roberts9012062/boke/internal/media"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// AdminService 后台服务（连接器类）。
type AdminService struct {
	admin    *repository.AdminRepo     // 后台数据访问
	posts    *repository.PostRepo      // 帖子（上下架/删除）
	comments *repository.CommentRepo   // 评论（删除）
	settings *repository.SettingRepo   // 站点设置
	enforcer *casbin.Enforcer          // 角色查询
	bans     *repository.BanRepo       // 封禁记录（M2：封禁落库）
	users    *repository.UserRepo      // 用户（M2：角色调整）
	postSvc  *PostService              // 帖子服务（M2：后台编辑）
	medias   *repository.MediaRepo     // 媒体（M2.9：媒体库）
	tags     *repository.TagRepo       // 标签（M2.9：标签分类）
	store    *media.Store              // 媒体存储（M2.9：删除磁盘文件）
	hooks    plugin.Dispatcher         // 插件钩子调度器（M3.2 扩展框架）
	audit    *repository.AuditRepo     // 审计日志（M5：角色变更留痕）
	reports  *repository.ReportRepo    // 举报工单（仪表盘待处理块，走查纠偏补）
}

// NewAdminService 创建后台服务。
func NewAdminService(admin *repository.AdminRepo, posts *repository.PostRepo, comments *repository.CommentRepo, settings *repository.SettingRepo, enforcer *casbin.Enforcer, bans *repository.BanRepo, users *repository.UserRepo, postSvc *PostService, medias *repository.MediaRepo, tags *repository.TagRepo, store *media.Store, hooks plugin.Dispatcher, audit *repository.AuditRepo, reports *repository.ReportRepo) *AdminService {
	return &AdminService{admin: admin, posts: posts, comments: comments, settings: settings, enforcer: enforcer, bans: bans, users: users, postSvc: postSvc, medias: medias, tags: tags, store: store, hooks: hooks, audit: audit, reports: reports}
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
	Pending     ReportPending             `json:"pending"`      // 待处理块（设计稿：评论待审/内容举报/敏感词命中；走查纠偏补）
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

	// 待处理块（设计稿：评论待审/内容举报/敏感词命中；走查纠偏补）
	hiddenComments, err := s.admin.CountHiddenComments(ctx)
	if err != nil {
		return nil, err
	}
	pendingReports, err := s.reports.CountPending(ctx)
	if err != nil {
		return nil, err
	}
	sensitiveHits, err := s.admin.TotalSensitiveHits(ctx)
	if err != nil {
		return nil, err
	}

	data := &DashboardData{
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
		Pending:       ReportPending{Comments: hiddenComments, Reports: pendingReports, Sensitive: sensitiveHits},
	}
	// ---------- 插件钩子：admin.page（同步扩展点，M3.2；结果忽略，插件可扩展仪表盘指标） ----------
	s.hooks.Dispatch(ctx, plugin.HookAdminPage, plugin.Event{Payload: data})
	return data, nil
}

// ---------- 内容管理 ----------

// ListPosts 内容管理列表（M5：author 角色仅返回自己帖子，数据隔离）。
func (s *AdminService) ListPosts(ctx context.Context, contentType string, status string, keyword string, actorID int64, role string, page int, pageSize int) ([]model.Post, int64, error) {
	authorID := int64(0)
	if role == casbin.RoleAuthor {
		authorID = actorID
	}
	return s.admin.ListAdminPosts(ctx, contentType, status, keyword, authorID, page, pageSize)
}

// SetPostStatus 上下架（published ↔ taken_down；author 仅能操作自己帖子）。
func (s *AdminService) SetPostStatus(ctx context.Context, postID int64, status string, actorID int64, role string) error {
	if status != model.PostStatusPublished && status != model.PostStatusTakenDown {
		return errs.New(errs.CodeBadRequest, "状态仅支持 published / taken_down")
	}
	if err := s.checkPostOwner(ctx, postID, actorID, role); err != nil {
		return err
	}
	return s.posts.SetStatus(ctx, postID, status, nil)
}

// DeletePost 删除内容（软删；author 仅能操作自己帖子）。
func (s *AdminService) DeletePost(ctx context.Context, postID int64, actorID int64, role string) error {
	if err := s.checkPostOwner(ctx, postID, actorID, role); err != nil {
		return err
	}
	return s.posts.SetStatus(ctx, postID, model.PostStatusDeleted, nil)
}

// GetPostDetail 后台编辑详情（author 仅能查看自己帖子）。
func (s *AdminService) GetPostDetail(ctx context.Context, postID int64, actorID int64, role string) (*model.AdminPostDetail, error) {
	if err := s.checkPostOwner(ctx, postID, actorID, role); err != nil {
		return nil, err
	}
	return s.postSvc.GetAdminDetail(ctx, postID)
}

// UpdatePost 后台编辑保存（author 仅能编辑自己帖子）。
func (s *AdminService) UpdatePost(ctx context.Context, postID int64, req model.AdminUpdatePostReq, actorID int64, role string) error {
	if err := s.checkPostOwner(ctx, postID, actorID, role); err != nil {
		return err
	}
	return s.postSvc.UpdateByAdmin(ctx, postID, req)
}

// checkPostOwner 帖子归属校验（M5 author 数据隔离：非本人帖子拒绝操作）。
// 说明：非 author 角色（superadmin/editor）放行；author 校验帖子作者。
func (s *AdminService) checkPostOwner(ctx context.Context, postID int64, actorID int64, role string) error {
	if role != casbin.RoleAuthor {
		return nil
	}
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return errs.ErrNotFound
	}
	if post.AuthorID != actorID {
		return errs.New(errs.CodeForbidden, "只能操作自己的帖子")
	}
	return nil
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

// SetCommentStatus 评论隐藏/恢复（M2：visible ↔ hidden；前台详情列表仅展示 visible）。
func (s *AdminService) SetCommentStatus(ctx context.Context, commentID int64, status string) error {
	if status != model.CommentStatusVisible && status != model.CommentStatusHidden {
		return errs.New(errs.CodeBadRequest, "状态仅支持 visible / hidden")
	}
	return s.comments.SetStatus(ctx, commentID, status)
}

// CommentStats 评论统计（设计稿后台评论统计条：全部/今日新增/已屏蔽）。
func (s *AdminService) CommentStats(ctx context.Context) (*repository.CommentStats, error) {
	return s.admin.CommentStats(ctx)
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
	// 封禁前保护最后一名超级管理员（避免全站无可用管理员）
	if status == "banned" {
		target, err := s.users.FindByID(ctx, userID)
		if err != nil {
			return errs.ErrNotFound
		}
		if err := s.guardLastSuperadmin(ctx, s.enforcer.GetRole(target.Username)); err != nil {
			return err
		}
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

// UserStats 用户统计条（设计稿《后台用户》：全部/本周新增/活跃/已禁言，类型定义见 repository）。
func (s *AdminService) UserStats(ctx context.Context) (*repository.UserStats, error) {
	return s.admin.UserStats(ctx)
}

// SetUserRole 角色调整（M5：五级角色，users.role 落库 + casbin 内存策略即时生效）。
// 参数：actorID 操作者；userID 目标用户；role 目标角色；ip/ua 审计信息。
// 规则：白名单五角色；不能调整自己的角色（防自降级锁死）；调整后需该用户重新登录生效（JWT claims）。
func (s *AdminService) SetUserRole(ctx context.Context, actorID int64, userID int64, role string, ip string, ua string) error {
	if !casbin.IsBuiltinRole(role) {
		return errs.New(errs.CodeBadRequest, "角色仅支持 superadmin / editor / author / visitor / restricted")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return errs.ErrNotFound
	}
	// 自我保护：不可调整自己的角色
	if actorID == userID {
		return errs.New(errs.CodeStateConflict, "不能调整自己的角色")
	}
	// 变更前角色（审计快照；casbin 内存为当前生效值）
	beforeRole := s.enforcer.GetRole(user.Username)

	// 最后一名超级管理员保护：superadmin 降级前须存在其他超级管理员（防系统锁死）
	if beforeRole == casbin.RoleSuperAdmin && role != casbin.RoleSuperAdmin {
		if err := s.guardLastSuperadmin(ctx, beforeRole); err != nil {
			return err
		}
	}

	// 落库 + 内存策略同步（顺序：先内存后落库，失败时内存已生效不影响可用性）
	if err := s.enforcer.SetRole(user.Username, role); err != nil {
		return err
	}
	if err := s.users.SetRoleByID(ctx, userID, role); err != nil {
		return err
	}

	// 审计（角色变更入 audit_logs，架构 9.2；失败静默不影响主流程）
	_ = s.audit.Insert(ctx, repository.AuditEntry{
		ActorID: actorID, Action: "set_role", ResourceType: "user", ResourceID: userID,
		BeforeData: `"` + beforeRole + `"`, AfterData: `"` + role + `"`,
		IP: ip, UserAgent: ua,
	})
	return nil
}

// guardLastSuperadmin 最后一名超级管理员保护（降级/封禁前校验）。
// 参数：targetRole 目标用户当前角色；ctx 查询上下文。
// 返回：目标角色不是 superadmin 或仍存在其他 superadmin 时返回 nil，否则返回业务错误。
func (s *AdminService) guardLastSuperadmin(ctx context.Context, targetRole string) error {
	if targetRole != casbin.RoleSuperAdmin {
		return nil
	}
	count, err := s.users.CountByRole(ctx, casbin.RoleSuperAdmin)
	if err != nil {
		return err
	}
	if count <= 1 {
		return errs.New(errs.CodeStateConflict, "系统至少保留一名超级管理员")
	}
	return nil
}

// ---------- 媒体库（M2.9，设计稿《后台媒体》） ----------

// MediaStats 媒体统计条（全部文件/图片/音频/视频）。
func (s *AdminService) MediaStats(ctx context.Context) (*repository.MediaStats, error) {
	return s.medias.Stats(ctx)
}

// ListMedia 后台媒体列表（类型/关键词/分页 + 引用数）。
func (s *AdminService) ListMedia(ctx context.Context, mediaType string, keyword string, page int, pageSize int) ([]repository.MediaAdminRow, int64, error) {
	return s.medias.ListAdmin(ctx, mediaType, keyword, page, pageSize)
}

// DeleteMedia 删除媒体（解除帖子引用 + 删记录 + 删磁盘文件；文件删除失败静默）。
func (s *AdminService) DeleteMedia(ctx context.Context, mediaID int64) error {
	storageKey, err := s.medias.Delete(ctx, mediaID)
	if err != nil {
		return err
	}
	_ = s.store.Remove(storageKey)
	return nil
}

// ---------- 标签分类（M2.9，设计稿《后台标签》） ----------

// TagStats 标签统计条（全部/热门/本周新建/未使用）。
func (s *AdminService) TagStats(ctx context.Context) (*repository.TagStats, error) {
	return s.tags.Stats(ctx)
}

// ListTags 后台标签列表（关键词/分页）。
func (s *AdminService) ListTags(ctx context.Context, keyword string, page int, pageSize int) ([]repository.TagAdminRow, int64, error) {
	return s.tags.ListAdmin(ctx, keyword, page, pageSize)
}

// RenameTag 重命名标签（name + slug + category 同步；重名返回冲突）。
func (s *AdminService) RenameTag(ctx context.Context, tagID int64, name string, slug string, category string) error {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)
	category = strings.TrimSpace(category)
	if name == "" || len([]rune(name)) > 20 || slug == "" || len([]rune(slug)) > 50 {
		return errs.New(errs.CodeBadRequest, "标签名需为 1-20 字符，别名需为 1-50 字符")
	}
	if len([]rune(category)) > 50 {
		return errs.New(errs.CodeBadRequest, "分类不能超过 50 字符")
	}
	// 重名检查（排除自身）
	existing, err := s.tags.FindByName(ctx, name)
	if err == nil && existing != tagID {
		return errs.New(errs.CodeConflict, "标签「"+name+"」已存在")
	}
	return s.tags.Rename(ctx, tagID, name, slug, category)
}

// MergeTag 合并标签（src → dst，src 删除，帖子关联转移）。
func (s *AdminService) MergeTag(ctx context.Context, srcID int64, dstID int64) error {
	if srcID == dstID {
		return errs.New(errs.CodeBadRequest, "不能合并到自身")
	}
	return s.tags.Merge(ctx, srcID, dstID)
}

// DeleteTag 删除标签（解除帖子关联 + 删除标签）。
func (s *AdminService) DeleteTag(ctx context.Context, tagID int64) error {
	return s.tags.Delete(ctx, tagID)
}

// ---------- 站点设置 ----------// Settings 站点设置读取。
func (s *AdminService) Settings(ctx context.Context) (map[string]string, error) {
	return s.settings.All(ctx)
}

// SaveSettings 站点设置保存（站点名/描述/注册开关/评论开关/默认主题/维护开关）。
func (s *AdminService) SaveSettings(ctx context.Context, updates map[string]string) error {
	// 白名单校验（防注入任意键；plugin_ 前缀为插件配置键，M3.2 schema 驱动设置页）
	allowed := map[string]bool{
		"site_name": true, "site_description": true,
		"allow_register": true, "comment_open": true,
		"theme": true, "maintenance_mode": true, // 维护开关（M2）
		"plugin_source": true, // 插件源仓库（M3.1，owner/repo）
	}
	for key := range updates {
		if !allowed[key] && !strings.HasPrefix(key, "plugin_") {
			return errs.New(errs.CodeBadRequest, "不支持的设置项："+key)
		}
	}
	return s.settings.SetMany(ctx, updates)
}
