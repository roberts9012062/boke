// internal/repository/notification.go
// 通知数据访问（notifications 表）。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 通知类型（需求 3.8：赞/评论/回复/关注/系统；M2 私信）。
const (
	NotifyLike    = "like"    // 赞了你的帖子
	NotifyComment = "comment" // 评论了你
	NotifyReply   = "reply"   // 回复了你的评论
	NotifyFollow  = "follow"  // 开始关注你
	NotifySystem  = "system"  // 系统通知
	NotifyMessage = "message" // 私信（M2：收到私信）
)

// Notification 通知实体（notifications 表结构，含 004 迁移字段）。
type Notification struct {
	ID        int64      // 通知 ID
	UserID    int64      // 接收者
	ActorID   int64      // 触发者（0 = 系统）
	Type      string     // 类型
	Title     string     // 标题
	Content   string     // 内容
	Link      string     // 跳转链接
	PostID    int64      // 相关帖子 ID
	ReadAt    *time.Time // 已读时间（NULL = 未读）
	CreatedAt time.Time  // 创建时间
}

// NotificationRepo 通知数据访问（连接器类）。
type NotificationRepo struct {
	pool *pgxpool.Pool
}

// NewNotificationRepo 创建通知仓库。
func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

// Create 创建通知（dedup=true 时同触发者同类型同帖子的未读通知不重复，
// 用于点赞/关注等高频操作；评论类允许重复传 false）。
func (r *NotificationRepo) Create(ctx context.Context, n Notification, dedup bool) error {
	if dedup {
		// INSERT ... SELECT ... WHERE NOT EXISTS：存在未读同源通知则跳过
		_, err := r.pool.Exec(ctx, `
			INSERT INTO notifications (user_id, actor_id, type, title, content, link, post_id)
			SELECT $1, $2, $3, $4, $5, $6, $7
			WHERE NOT EXISTS (
				SELECT 1 FROM notifications
				WHERE user_id = $1 AND actor_id = $2 AND type = $3 AND post_id = $7 AND read_at IS NULL
			)`,
			n.UserID, n.ActorID, n.Type, n.Title, n.Content, n.Link, n.PostID)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, actor_id, type, title, content, link, post_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.UserID, n.ActorID, n.Type, n.Title, n.Content, n.Link, n.PostID)
	return err
}

// NotificationListParams 通知列表参数。
type NotificationListParams struct {
	UserID   int64  // 接收者
	Type     string // 类型过滤（空 = 全部）
	Page     int    // 页码
	PageSize int    // 每页条数
}

// List 分页查询通知（最新在前）。
func (r *NotificationRepo) List(ctx context.Context, p NotificationListParams) ([]Notification, int64, error) {	where := "WHERE user_id = $1"
	args := []any{p.UserID}
	if p.Type != "" {
		args = append(args, p.Type)
		where += " AND type = $2"
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, actor_id, type, title, content, link, post_id, read_at, created_at
		FROM notifications `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)-1)+` OFFSET $`+fmt.Sprintf("%d", len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		var readAt *time.Time
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.ActorID, &n.Type, &n.Title, &n.Content,
			&n.Link, &n.PostID, &readAt, &n.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		n.ReadAt = readAt
		items = append(items, n)
	}
	return items, total, rows.Err()
}

// CountUnread 未读通知数（角标轮询）。
func (r *NotificationRepo) CountUnread(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&count)
	return count, err
}

// MarkRead 单条已读。
func (r *NotificationRepo) MarkRead(ctx context.Context, userID int64, notificationID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND user_id = $2`, notificationID, userID)
	return err
}

// MarkAllRead 全部已读。
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now()
		WHERE user_id = $1 AND read_at IS NULL`, userID)
	return err
}
