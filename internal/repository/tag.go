// internal/repository/tag.go
// 标签数据访问层（tags / post_tags 表）。
// 标签语义：帖子带 # 标签即进入话题聚合（需求 3.6）。
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// TagRepo 标签数据访问（连接器类）。
type TagRepo struct {
	pool *pgxpool.Pool
}

// NewTagRepo 创建标签仓库。
func NewTagRepo(pool *pgxpool.Pool) *TagRepo {
	return &TagRepo{pool: pool}
}

// FindByName 按名称查询标签（不存在返回 ErrNotFound）。
func (r *TagRepo) FindByName(ctx context.Context, name string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		"SELECT id FROM tags WHERE name = $1", name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// FindWithStats 按名称查询标签完整信息（含统计，话题详情用）。
func (r *TagRepo) FindWithStats(ctx context.Context, name string) (TagRow, error) {
	var tag TagRow
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, slug, description, post_count FROM tags WHERE name = $1`, name).
		Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description, &tag.PostCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return TagRow{}, ErrNotFound
	}
	return tag, err
}

// Create 创建标签（返回新标签 ID）。
func (r *TagRepo) Create(ctx context.Context, name string, slug string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tags (name, slug) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, name, slug).Scan(&id)
	return id, err
}

// LinkPost 关联帖子与标签（幂等）。
func (r *TagRepo) LinkPost(ctx context.Context, postID int64, tagID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2)
		ON CONFLICT (post_id, tag_id) DO NOTHING`, postID, tagID)
	return err
}

// UnlinkPost 解除帖子与标签的关联（更新帖子时重建标签用）。
func (r *TagRepo) UnlinkPost(ctx context.Context, postID int64) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM post_tags WHERE post_id = $1", postID)
	return err
}

// IncrPostCount 标签引用计数 +1（关联时调用）。
func (r *TagRepo) IncrPostCount(ctx context.Context, tagID int64) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE tags SET post_count = post_count + 1 WHERE id = $1", tagID)
	return err
}

// DecrPostCount 标签引用计数 -1（解除关联时调用，最低 0）。
func (r *TagRepo) DecrPostCount(ctx context.Context, tagID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tags SET post_count = GREATEST(post_count - 1, 0) WHERE id = $1`, tagID)
	return err
}

// ListByPost 查询帖子关联的标签（按名称升序）。
func (r *TagRepo) ListByPost(ctx context.Context, postID int64) ([]TagRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug FROM post_tags pt
		JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id = $1
		ORDER BY t.name`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]TagRow, 0)
	for rows.Next() {
		var tag TagRow
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// TagRow 标签行（查询结果，含统计字段）。
type TagRow struct {
	ID          int64  // 标签 ID
	Name        string // 标签名
	Slug        string // URL 别名
	Description string // 描述
	PostCount   int64  // 帖子数（冗余计数）
}

// Search 标签检索（名称模糊匹配，搜索页话题 Tab）。
func (r *TagRepo) Search(ctx context.Context, keyword string, limit int) ([]TagRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, description, post_count FROM tags
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY post_count DESC
		LIMIT $2`, keyword, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]TagRow, 0)
	for rows.Next() {
		var tag TagRow
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description, &tag.PostCount); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// ---------- 话题关注（topic_follows，需求 3.6） ----------

// ListHot 热门话题列表（按帖数降序，limit 条）。
func (r *TagRepo) ListHot(ctx context.Context, limit int) ([]TagRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, description, post_count FROM tags
		WHERE post_count > 0
		ORDER BY post_count DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]TagRow, 0)
	for rows.Next() {
		var tag TagRow
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description, &tag.PostCount); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// PostsByTopic 话题帖子流：按标签名查已发布帖子（分页）。
// 参数：sort 排序方式（latest=最新发布在前 / hot=按点赞数）。
func (r *TagRepo) PostsByTopic(ctx context.Context, name string, sort string, page int, pageSize int) ([]model.Post, int64, error) {
	where := `WHERE p.status = 'published' AND p.id IN (
		SELECT pt.post_id FROM post_tags pt
		JOIN tags t ON t.id = pt.tag_id
		WHERE t.name = $1
	)`

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts p `+where, name).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 排序：hot 按点赞数（设计稿「热门」Tab），默认最新发布
	orderBy := "p.published_at DESC NULLS LAST"
	if sort == "hot" {
		orderBy = "p.like_count DESC, p.published_at DESC"
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, p.media_ids FROM posts p `+where+`
		ORDER BY `+orderBy+`
		LIMIT $2 OFFSET $3`, name, pageSize, (page-1)*pageSize)
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

// CountFollowers 话题关注数统计。
func (r *TagRepo) CountFollowers(ctx context.Context, tagID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM topic_follows WHERE tag_id = $1`, tagID).Scan(&count)
	return count, err
}

// BrowseCount 话题浏览数（该话题帖子浏览量求和）。
func (r *TagRepo) BrowseCount(ctx context.Context, tagID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(p.view_count), 0) FROM post_tags pt
		JOIN posts p ON p.id = pt.post_id
		WHERE pt.tag_id = $1 AND p.status = 'published'`, tagID).Scan(&count)
	return count, err
}

// FollowTopic 关注话题（幂等）。
func (r *TagRepo) FollowTopic(ctx context.Context, userID int64, tagID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO topic_follows (user_id, tag_id) VALUES ($1, $2)
		ON CONFLICT (user_id, tag_id) DO NOTHING`, userID, tagID)
	return err
}

// UnfollowTopic 取消关注话题（幂等）。
func (r *TagRepo) UnfollowTopic(ctx context.Context, userID int64, tagID int64) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM topic_follows WHERE user_id = $1 AND tag_id = $2`, userID, tagID)
	return err
}

// IsFollowingTopic 查询是否已关注话题。
func (r *TagRepo) IsFollowingTopic(ctx context.Context, userID int64, tagID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM topic_follows WHERE user_id = $1 AND tag_id = $2)`,
		userID, tagID).Scan(&exists)
	return exists, err
}

// ListFollowingTopics 查询用户关注的话题列表。
func (r *TagRepo) ListFollowingTopics(ctx context.Context, userID int64) ([]TagRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.slug FROM topic_follows tf
		JOIN tags t ON t.id = tf.tag_id
		WHERE tf.user_id = $1
		ORDER BY t.post_count DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]TagRow, 0)
	for rows.Next() {
		var tag TagRow
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}
