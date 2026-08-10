// internal/repository/reaction.go
// 互动数据访问：帖子点赞/收藏（post_reactions）、评论点赞（comment_likes）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 互动类型（post_reactions.type）。
const (
	ReactionLike     = "like"     // 点赞
	ReactionFavorite = "favorite" // 收藏
)

// ReactionRepo 互动数据访问（连接器类）。
type ReactionRepo struct {
	pool *pgxpool.Pool
}

// NewReactionRepo 创建互动仓库。
func NewReactionRepo(pool *pgxpool.Pool) *ReactionRepo {
	return &ReactionRepo{pool: pool}
}

// Add 添加互动（点赞/收藏，幂等：已存在返回 false）。
// 说明：posts.like_count 冗余计数在同一事务内由 service 层调用 IncrPostLike。
func (r *ReactionRepo) Add(ctx context.Context, userID int64, postID int64, reactionType string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO post_reactions (user_id, post_id, type) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, post_id, type) DO NOTHING`,
		userID, postID, reactionType)
	return tag.RowsAffected() > 0, err
}

// Remove 移除互动（取消点赞/取消收藏，幂等）。
func (r *ReactionRepo) Remove(ctx context.Context, userID int64, postID int64, reactionType string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM post_reactions WHERE user_id = $1 AND post_id = $2 AND type = $3`,
		userID, postID, reactionType)
	return tag.RowsAffected() > 0, err
}

// Has 查询用户是否已互动（点赞/收藏状态展示）。
func (r *ReactionRepo) Has(ctx context.Context, userID int64, postID int64, reactionType string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM post_reactions WHERE user_id = $1 AND post_id = $2 AND type = $3)`,
		userID, postID, reactionType).Scan(&exists)
	return exists, err
}

// CountFavorite 收藏数统计（详情/列表展示）。
func (r *ReactionRepo) CountFavorite(ctx context.Context, postID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM post_reactions WHERE post_id = $1 AND type = 'favorite'`, postID).Scan(&count)
	return count, err
}

// IncrPostLike 帖子点赞计数 +1（与 post_reactions 写入同事务）。
func (r *ReactionRepo) IncrPostLike(ctx context.Context, postID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET like_count = like_count + 1 WHERE id = $1`, postID)
	return err
}

// DecrPostLike 帖子点赞计数 -1（取消赞，最低 0）。
func (r *ReactionRepo) DecrPostLike(ctx context.Context, postID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET like_count = GREATEST(like_count - 1, 0) WHERE id = $1`, postID)
	return err
}

// AddCommentLike 评论点赞（幂等）。
func (r *ReactionRepo) AddCommentLike(ctx context.Context, userID int64, commentID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO comment_likes (user_id, comment_id) VALUES ($1, $2)
		ON CONFLICT (user_id, comment_id) DO NOTHING`,
		userID, commentID)
	return tag.RowsAffected() > 0, err
}

// RemoveCommentLike 取消评论点赞（幂等）。
func (r *ReactionRepo) RemoveCommentLike(ctx context.Context, userID int64, commentID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2`,
		userID, commentID)
	return tag.RowsAffected() > 0, err
}

// HasCommentLike 查询用户是否已赞评论。
func (r *ReactionRepo) HasCommentLike(ctx context.Context, userID int64, commentID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM comment_likes WHERE user_id = $1 AND comment_id = $2)`,
		userID, commentID).Scan(&exists)
	return exists, err
}
