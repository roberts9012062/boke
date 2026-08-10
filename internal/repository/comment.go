// internal/repository/comment.go
// 评论数据访问层（comments 表，楼中楼嵌套查询）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// CommentRepo 评论数据访问（连接器类）。
type CommentRepo struct {
	pool *pgxpool.Pool
}

// NewCommentRepo 创建评论仓库。
func NewCommentRepo(pool *pgxpool.Pool) *CommentRepo {
	return &CommentRepo{pool: pool}
}

// commentColumns 评论查询列清单。
const commentColumns = `id, post_id, author_id, parent_id, content, floor, status, like_count, guest_name, guest_token_hash, created_at, updated_at`

// scanComment 将查询行扫描为 Comment 实体。
func scanComment(row pgx.Row) (model.Comment, error) {
	var c model.Comment
	err := row.Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.ParentID, &c.Content,
		&c.Floor, &c.Status, &c.LikeCount, &c.GuestName,
		&c.GuestTokenHash, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// Create 创建评论（返回新评论 ID 与楼层号）。
func (r *CommentRepo) Create(ctx context.Context, c model.Comment) (int64, int, error) {
	var id int64
	var floor int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO comments (post_id, author_id, parent_id, content, floor, status, like_count, guest_name, guest_token_hash)
		VALUES ($1, $2, $3, $4, COALESCE((SELECT max(floor) FROM comments WHERE post_id = $1), 0) + 1, 'visible', 0, $5, $6)
		RETURNING id, floor`,
		c.PostID, c.AuthorID, c.ParentID, c.Content, c.GuestName, c.GuestTokenHash,
	).Scan(&id, &floor)
	return id, floor, err
}

// FindByID 按 ID 查询评论。
func (r *CommentRepo) FindByID(ctx context.Context, id int64) (model.Comment, error) {
	return scanComment(r.pool.QueryRow(ctx,
		`SELECT `+commentColumns+` FROM comments WHERE id = $1`, id))
}

// ListByPost 查询帖子全部可见评论（顶层 + 子回复，按楼层正序）。
func (r *CommentRepo) ListByPost(ctx context.Context, postID int64) ([]model.Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+commentColumns+` FROM comments
		WHERE post_id = $1 AND status = 'visible'
		ORDER BY floor`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]model.Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// CountByPost 统计帖子可见评论数（冗余计数同步用：顶层 + 回复全部计入）。
func (r *CommentRepo) CountByPost(ctx context.Context, postID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM comments WHERE post_id = $1 AND status = 'visible'`, postID).Scan(&count)
	return count, err
}

// SyncPostCommentCount 将可见评论数写回 posts.comment_count（评论增删后同步）。
func (r *CommentRepo) SyncPostCommentCount(ctx context.Context, postID int64, count int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET comment_count = $2 WHERE id = $1`, postID, count)
	return err
}

// SoftDelete 软删除评论（status=deleted）。
func (r *CommentRepo) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE comments SET status = 'deleted', updated_at = now() WHERE id = $1`, id)
	return err
}

// IncrLike 评论点赞数 +1（事务内与 comment_likes 同步）。
func (r *CommentRepo) IncrLike(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE comments SET like_count = like_count + 1 WHERE id = $1`, id)
	return err
}

// DecrLike 评论点赞数 -1（取消赞，最低 0）。
func (r *CommentRepo) DecrLike(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE comments SET like_count = GREATEST(like_count - 1, 0) WHERE id = $1`, id)
	return err
}

// HasGuestCommentRecently 判断匿名 token 是否在限频窗口内已评论（1 条/分钟）。
// 参数：guestTokenHash 匿名 token 哈希；windowSeconds 限频窗口（秒）。
func (r *CommentRepo) HasGuestCommentRecently(ctx context.Context, guestTokenHash string, windowSeconds int) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM comments
			WHERE guest_token_hash = $1 AND created_at > now() - make_interval(secs => $2)
		)`, guestTokenHash, windowSeconds).Scan(&exists)
	return exists, err
}
