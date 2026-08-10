// internal/repository/relation.go
// 用户关系数据访问（user_relations：关注/收藏/黑名单）+ 关注流查询。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// 关系类型（user_relations.type）。
const (
	RelationFollow    = "follow"    // 关注
	RelationFavorite  = "favorite"  // 收藏（用户维度预留）
	RelationBlacklist = "blacklist" // 黑名单
)

// RelationRepo 用户关系数据访问（连接器类）。
type RelationRepo struct {
	pool *pgxpool.Pool
}

// NewRelationRepo 创建关系仓库。
func NewRelationRepo(pool *pgxpool.Pool) *RelationRepo {
	return &RelationRepo{pool: pool}
}

// Follow 关注用户（幂等；返回是否新增）。
func (r *RelationRepo) Follow(ctx context.Context, userID int64, targetID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO user_relations (user_id, target_id, type) VALUES ($1, $2, 'follow')
		ON CONFLICT (user_id, target_id, type) DO NOTHING`, userID, targetID)
	return tag.RowsAffected() > 0, err
}

// Unfollow 取消关注（幂等；返回是否取消）。
func (r *RelationRepo) Unfollow(ctx context.Context, userID int64, targetID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM user_relations WHERE user_id = $1 AND target_id = $2 AND type = 'follow'`,
		userID, targetID)
	return tag.RowsAffected() > 0, err
}

// IsFollowing 查询是否已关注。
func (r *RelationRepo) IsFollowing(ctx context.Context, userID int64, targetID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_relations
		WHERE user_id = $1 AND target_id = $2 AND type = 'follow')`,
		userID, targetID).Scan(&exists)
	return exists, err
}

// ListRelation 关系列表通用查询（关注/粉丝）。
// 参数：followers 为 true 查粉丝（谁关注了 userID），false 查关注（userID 关注了谁）。
// 返回：目标用户 ID 列表与总数。
func (r *RelationRepo) ListRelation(ctx context.Context, userID int64, followers bool, page int, pageSize int) ([]int64, int64, error) {
	// 粉丝：target_id = userID；关注：user_id = userID
	cond := "user_id = $1"
	selectCol := "target_id"
	if followers {
		cond = "target_id = $1"
		selectCol = "user_id"
	}
	where := "WHERE type = 'follow' AND " + cond

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_relations `+where, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+selectCol+` FROM user_relations `+where+`
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	return ids, total, rows.Err()
}

// FollowingIDs 我关注的全部用户 ID（关注流 feed 用）。
func (r *RelationRepo) FollowingIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT target_id FROM user_relations
		WHERE user_id = $1 AND type = 'follow'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FollowingFeed 关注流：关注用户的最新已发布帖子（分页）。
func (r *RelationRepo) FollowingFeed(ctx context.Context, userID int64, page int, pageSize int) ([]model.Post, int64, error) {
	where := `WHERE p.status = 'published' AND p.author_id IN (
		SELECT target_id FROM user_relations WHERE user_id = $1 AND type = 'follow'
	)`

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts p `+where, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, p.media_ids FROM posts p `+where+`
		ORDER BY p.published_at DESC NULLS LAST
		LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
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
