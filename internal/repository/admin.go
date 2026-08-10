// internal/repository/admin.go
// 后台管理数据访问：仪表盘聚合、内容/评论/用户管理查询。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// AdminRepo 后台数据访问（连接器类）。
type AdminRepo struct {
	pool *pgxpool.Pool
}

// NewAdminRepo 创建后台仓库。
func NewAdminRepo(pool *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{pool: pool}
}

// DashboardStats 仪表盘聚合数据。
type DashboardStats struct {
	// 近 7 日四项指标 + 环比（较上周）
	Views7d    int64 // 近 7 日浏览
	ViewsPrev  int64 // 上 7 日浏览（环比基准）
	Likes7d    int64 // 近 7 日获赞
	LikesPrev  int64
	Comments7d int64 // 近 7 日评论
	CommentsPrev int64
	Posts7d    int64 // 近 7 日新帖
	PostsPrev  int64
	// 内容分布（各类型帖子数）
	TypeCounts map[string]int64 // content_type → 数量
}

// StatsSince 统计指定时间段内的帖子指标。
// 参数：since 起始时间；返回浏览/新帖数。
// 口径说明（M1.7）：浏览无独立埋点表，取区间内发布帖子的累计 view_count（近似值，
//   真实日浏览需埋点表，规划 P1）；新帖仅统计已发布（草稿不计入「新帖」）。
func (r *AdminRepo) StatsSince(ctx context.Context, since time.Time) (views int64, posts int64, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(sum(view_count), 0),
			count(*)
		FROM posts
		WHERE status = 'published' AND created_at >= $1`, since).Scan(&views, &posts)
	return views, posts, err
}

// LikesSince 统计指定时间段内的真实获赞数（post_reactions 按时间聚合，M1.7 修正口径）。
func (r *AdminRepo) LikesSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM post_reactions WHERE type = 'like' AND created_at >= $1`, since).Scan(&count)
	return count, err
}

// CountCommentsSince 统计指定时间段内的评论数。
func (r *AdminRepo) CountCommentsSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM comments WHERE created_at >= $1`, since).Scan(&count)
	return count, err
}

// CountPostsByType 各类型帖子数（内容分布）。
func (r *AdminRepo) CountPostsByType(ctx context.Context) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT content_type, count(*) FROM posts
		WHERE status != 'deleted'
		GROUP BY content_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var contentType string
		var count int64
		if err := rows.Scan(&contentType, &count); err != nil {
			return nil, err
		}
		counts[contentType] = count
	}
	return counts, rows.Err()
}

// TrendPoint 趋势点（近 N 日每日互动，日期格式 MM-DD）。
type TrendPoint struct {
	Date     string `json:"date"`     // 日期（MM-DD）
	Posts    int64  `json:"posts"`    // 当日新帖（已发布）
	Likes    int64  `json:"likes"`    // 当日获赞（post_reactions 按日聚合）
	Comments int64  `json:"comments"` // 当日评论
}

// TrendSeries 近 N 日互动趋势（按日聚合，无数据日期补 0）。
// 说明：浏览无时间维度数据源，趋势图展示新帖/获赞/评论三项（需求 4.2 互动趋势）。
func (r *AdminRepo) TrendSeries(ctx context.Context, days int) ([]TrendPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH days AS (
			-- 生成近 N 天日期序列（含今天）
			SELECT generate_series(current_date - ($1 - 1), current_date, interval '1 day')::date AS day
		),
		post_d AS (
			SELECT date_trunc('day', created_at)::date AS day, count(*) AS n
			FROM posts WHERE status = 'published' AND created_at >= current_date - ($1 - 1)
			GROUP BY 1
		),
		like_d AS (
			SELECT date_trunc('day', created_at)::date AS day, count(*) AS n
			FROM post_reactions WHERE type = 'like' AND created_at >= current_date - ($1 - 1)
			GROUP BY 1
		),
		comment_d AS (
			SELECT date_trunc('day', created_at)::date AS day, count(*) AS n
			FROM comments WHERE created_at >= current_date - ($1 - 1)
			GROUP BY 1
		)
		SELECT to_char(d.day, 'MM-DD'),
			COALESCE(p.n, 0), COALESCE(l.n, 0), COALESCE(c.n, 0)
		FROM days d
		LEFT JOIN post_d p ON p.day = d.day
		LEFT JOIN like_d l ON l.day = d.day
		LEFT JOIN comment_d c ON c.day = d.day
		ORDER BY d.day`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]TrendPoint, 0, days)
	for rows.Next() {
		var p TrendPoint
		if err := rows.Scan(&p.Date, &p.Posts, &p.Likes, &p.Comments); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// ListAdminPosts 内容管理列表（类型/状态筛选 + 关键词搜索 + 分页）。
func (r *AdminRepo) ListAdminPosts(ctx context.Context, contentType string, status string, keyword string, page int, pageSize int) ([]model.Post, int64, error) {
	// 动态 WHERE（参数化）
	where := "WHERE p.status != 'deleted'"
	args := make([]any, 0, 4)
	if contentType != "" {
		args = append(args, contentType)
		where += fmt.Sprintf(" AND p.content_type = $%d", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND p.status = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(" AND (p.title ILIKE $%d OR p.content ILIKE $%d)", len(args), len(args))
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts p `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, p.media_ids FROM posts p `+where+`
		ORDER BY p.updated_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)-1)+` OFFSET $`+fmt.Sprintf("%d", len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	posts := make([]model.Post, 0)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, 0, err
		}
		posts = append(posts, post)
	}
	return posts, total, rows.Err()
}

// ListAdminComments 评论管理列表（状态筛选 + 关键词 + 分页）。
func (r *AdminRepo) ListAdminComments(ctx context.Context, status string, keyword string, page int, pageSize int) ([]CommentRow, int64, error) {
	where := "WHERE c.status != 'deleted'"
	args := make([]any, 0, 3)
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND c.status = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(" AND c.content ILIKE $%d", len(args))
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM comments c `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.post_id, c.author_id, c.parent_id, c.content, c.floor, c.status,
		       c.like_count, c.guest_name, c.guest_token_hash, c.created_at, c.updated_at,
		       COALESCE(u.nickname, ''), COALESCE(u.username, '')
		FROM comments c
		LEFT JOIN users u ON u.id = c.author_id
		`+where+`
		ORDER BY c.created_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)-1)+` OFFSET $`+fmt.Sprintf("%d", len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	comments := make([]CommentRow, 0)
	for rows.Next() {
		var c CommentRow
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.AuthorID, &c.ParentID, &c.Content, &c.Floor, &c.Status,
			&c.LikeCount, &c.GuestName, &c.GuestTokenHash, &c.CreatedAt, &c.UpdatedAt,
			&c.AuthorNickname, &c.AuthorUsername,
		); err != nil {
			return nil, 0, err
		}
		comments = append(comments, c)
	}
	return comments, total, rows.Err()
}

// ListAdminUsers 用户管理列表（关键词搜索 + 分页）。
func (r *AdminRepo) ListAdminUsers(ctx context.Context, keyword string, page int, pageSize int) ([]UserAdminRow, int64, error) {
	where := "WHERE 1=1"
	args := make([]any, 0, 1)
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(" AND (username ILIKE $%d OR email ILIKE $%d OR nickname ILIKE $%d)", len(args), len(args), len(args))
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM users `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.email, u.username, u.nickname, u.bio, u.status, u.last_login_at, u.created_at,
		       (SELECT count(*) FROM posts p WHERE p.author_id = u.id AND p.status != 'deleted')
		FROM users u `+where+`
		ORDER BY u.created_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)-1)+` OFFSET $`+fmt.Sprintf("%d", len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]UserAdminRow, 0)
	for rows.Next() {
		var u UserAdminRow
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Username, &u.Nickname, &u.Bio, &u.Status,
			&u.LastLoginAt, &u.CreatedAt, &u.PostCount,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// CountUsers 全部用户数（封禁管理统计条，设计稿「全部用户」）。
func (r *AdminRepo) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count)
	return count, err
}

// CountBannedUsers 已封禁用户数（设计稿「已禁言」）。
func (r *AdminRepo) CountBannedUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE status = 'banned'`).Scan(&count)
	return count, err
}

// SetUserStatus 封禁/解封用户（status=active/banned）。
func (r *AdminRepo) SetUserStatus(ctx context.Context, userID int64, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET status = $2, updated_at = now() WHERE id = $1`, userID, status)
	return err
}

// RecentActivity 最近动态（新帖/新评论/新用户，各取 5 条混合）。
func (r *AdminRepo) RecentActivity(ctx context.Context) ([]ActivityRow, error) {
	rows, err := r.pool.Query(ctx, `
		(SELECT 'post' AS kind, p.id, COALESCE(u.nickname, '') AS actor, p.title AS content, p.created_at
		 FROM posts p JOIN users u ON u.id = p.author_id
		 WHERE p.status != 'deleted' ORDER BY p.created_at DESC LIMIT 5)
		UNION ALL
		(SELECT 'comment' AS kind, c.id, COALESCE(u.nickname, c.guest_name) AS actor, c.content, c.created_at
		 FROM comments c LEFT JOIN users u ON u.id = c.author_id
		 WHERE c.status != 'deleted' ORDER BY c.created_at DESC LIMIT 5)
		UNION ALL
		(SELECT 'user' AS kind, u.id, u.nickname, '@' || u.username, u.created_at
		 FROM users u ORDER BY u.created_at DESC LIMIT 5)
		ORDER BY created_at DESC LIMIT 8`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ActivityRow, 0)
	for rows.Next() {
		var a ActivityRow
		if err := rows.Scan(&a.Kind, &a.ID, &a.Actor, &a.Content, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

// ---------- 行类型 ----------

// CommentRow 评论行（后台列表，含作者信息）。
type CommentRow struct {
	ID             int64      `json:"id"`              // 评论 ID
	PostID         int64      `json:"post_id"`         // 所属帖子
	AuthorID       *int64     `json:"author_id"`       // 作者（NULL = 匿名）
	ParentID       *int64     `json:"parent_id"`       // 父评论
	Content        string     `json:"content"`         // 内容
	Floor          int        `json:"floor"`           // 楼层
	Status         string     `json:"status"`          // 状态
	LikeCount      int64      `json:"like_count"`      // 点赞
	GuestName      string     `json:"guest_name"`      // 匿名昵称
	GuestTokenHash string     `json:"guest_token_hash"` // 匿名哈希
	CreatedAt      time.Time  `json:"created_at"`      // 时间
	UpdatedAt      time.Time  `json:"updated_at"`
	AuthorNickname string     `json:"author_nickname"` // 作者昵称（匿名空）
	AuthorUsername string     `json:"author_username"` // 作者账号（匿名空）
}

// UserAdminRow 用户行（后台列表）。
type UserAdminRow struct {
	ID          int64      `json:"id"`           // 用户 ID
	Email       string     `json:"email"`        // 邮箱
	Username    string     `json:"username"`     // 账号
	Nickname    string     `json:"nickname"`     // 昵称
	Bio         string     `json:"bio"`          // 简介
	Status      string     `json:"status"`       // 状态
	LastLoginAt *time.Time `json:"last_login_at"` // 最后登录
	CreatedAt   time.Time  `json:"created_at"`   // 注册时间
	PostCount   int64      `json:"post_count"`   // 帖子数
	Role        string     `json:"role"`         // 角色（service 层 casbin 补充）
}

// ActivityRow 最近动态行。
type ActivityRow struct {
	Kind      string    `json:"kind"`      // post / comment / user
	ID        int64     `json:"id"`        // 资源 ID
	Actor     string    `json:"actor"`     // 操作者
	Content   string    `json:"content"`   // 内容摘要
	CreatedAt time.Time `json:"created_at"` // 时间
}
