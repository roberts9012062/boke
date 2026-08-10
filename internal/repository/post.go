// internal/repository/post.go
// 帖子数据访问层（pgx 原生 SQL + 结构化扫描）。
// 仅供 service 层调用（分层单向依赖：handler → service → repository）。
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// PostRepo 帖子数据访问（连接器类）。
type PostRepo struct {
	pool *pgxpool.Pool
}

// NewPostRepo 创建帖子仓库。
func NewPostRepo(pool *pgxpool.Pool) *PostRepo {
	return &PostRepo{pool: pool}
}

// postColumns 帖子查询列清单（不含 media_ids，单独扫描）。
const postColumns = `id, author_id, title, summary, content, content_type, status, visibility, cover_url, view_count, like_count, comment_count, published_at, created_at, updated_at`

// scanPost 将查询行扫描为 Post 实体（media_ids 需单独处理）。
func scanPost(row pgx.Row) (model.Post, error) {
	var p model.Post
	var mediaIDs []byte
	err := row.Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Summary, &p.Content,
		&p.ContentType, &p.Status, &p.Visibility, &p.CoverURL,
		&p.ViewCount, &p.LikeCount, &p.CommentCount,
		&p.PublishedAt, &p.CreatedAt, &p.UpdatedAt, &mediaIDs,
	)
	if err != nil {
		return model.Post{}, wrapNotFound(err)
	}
	// 解析媒体 ID 数组（JSONB → []int64）
	if len(mediaIDs) > 0 {
		if err := json.Unmarshal(mediaIDs, &p.MediaIDs); err != nil {
			return model.Post{}, fmt.Errorf("解析媒体 ID 失败：%w", err)
		}
	}
	return p, nil
}

// scanFavoritePost 扫描收藏帖子行（与 scanPost 字段同步，末尾多扫收藏时间列）。
func scanFavoritePost(row pgx.Row) (FavoritePostRow, error) {
	var p model.Post
	var mediaIDs []byte
	var favoritedAt time.Time
	err := row.Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Summary, &p.Content,
		&p.ContentType, &p.Status, &p.Visibility, &p.CoverURL,
		&p.ViewCount, &p.LikeCount, &p.CommentCount,
		&p.PublishedAt, &p.CreatedAt, &p.UpdatedAt, &mediaIDs,
		&favoritedAt,
	)
	if err != nil {
		return FavoritePostRow{}, wrapNotFound(err)
	}
	// 解析媒体 ID 数组（JSONB → []int64）
	if len(mediaIDs) > 0 {
		if err := json.Unmarshal(mediaIDs, &p.MediaIDs); err != nil {
			return FavoritePostRow{}, fmt.Errorf("解析媒体 ID 失败：%w", err)
		}
	}
	return FavoritePostRow{Post: p, FavoritedAt: favoritedAt}, nil
}

// Create 创建帖子（返回新帖子 ID）。
func (r *PostRepo) Create(ctx context.Context, p model.Post) (int64, error) {
	// 序列化媒体 ID 数组（JSONB）。
	// 注意：简单查询协议下 []byte 会被按 bytea 编码（\x...），JSONB 无法解析，
	// 因此统一转 string 按文本传递（历史故障：SQLSTATE 22P02）。
	mediaJSON, err := marshalMediaIDs(p.MediaIDs)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.pool.QueryRow(ctx, `
		INSERT INTO posts (author_id, title, summary, content, content_type, status, visibility, cover_url, media_ids, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		p.AuthorID, p.Title, p.Summary, p.Content, p.ContentType,
		p.Status, p.Visibility, p.CoverURL, mediaJSON, p.PublishedAt,
	).Scan(&id)
	return id, err
}

// FindByID 按 ID 查询帖子。
func (r *PostRepo) FindByID(ctx context.Context, id int64) (model.Post, error) {
	return scanPost(r.pool.QueryRow(ctx,
		`SELECT `+postColumns+`, media_ids FROM posts WHERE id = $1`, id))
}

// Update 更新帖子字段（仅更新非零值；发布/删除等状态变更走专用方法）。
// 返回：是否找到并更新（不存在返回 false）。
func (r *PostRepo) Update(ctx context.Context, p model.Post) (bool, error) {
	mediaJSON, err := marshalMediaIDs(p.MediaIDs)
	if err != nil {
		return false, err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE posts SET
			title = $2, summary = $3, content = $4, content_type = $5,
			visibility = $6, cover_url = $7, media_ids = $8, updated_at = now()
		WHERE id = $1`,
		p.ID, p.Title, p.Summary, p.Content, p.ContentType,
		p.Visibility, p.CoverURL, mediaJSON,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// marshalMediaIDs 序列化媒体 ID 数组为 JSON 文本（nil → "[]"，避免 JSON null）。
func marshalMediaIDs(ids []int64) (string, error) {
	if ids == nil {
		return "[]", nil
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return "", fmt.Errorf("序列化媒体 ID 失败：%w", err)
	}
	return string(data), nil
}

// SetStatus 变更帖子状态（发布/下架/删除）。
func (r *PostRepo) SetStatus(ctx context.Context, id int64, status string, publishedAt any) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE posts SET status = $2, published_at = COALESCE($3, published_at), updated_at = now()
		WHERE id = $1`,
		id, status, publishedAt)
	return err
}

// IncrView 浏览量 +1（详情页访问）。
func (r *PostRepo) IncrView(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET view_count = view_count + 1 WHERE id = $1`, id)
	return err
}

// IncrCommentCount 评论计数 +1（发表评论时，事务内同步）。
func (r *PostRepo) IncrCommentCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET comment_count = comment_count + 1 WHERE id = $1`, id)
	return err
}

// DecrCommentCount 评论计数 -1（删除评论时，最低 0）。
func (r *PostRepo) DecrCommentCount(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET comment_count = GREATEST(comment_count - 1, 0) WHERE id = $1`, id)
	return err
}

// ListParams 列表查询参数。
type ListParams struct {
	ContentType string // 类型过滤（空 = 全部）
	AuthorID    int64  // 作者过滤（0 = 不限）
	Page        int    // 页码（从 1 起）
	PageSize    int    // 每页条数
}

// List 分页查询已发布帖子（时间线/主页帖子流）。
// 返回：帖子列表与总数。
func (r *PostRepo) List(ctx context.Context, p ListParams) ([]model.Post, int64, error) {
	// 动态 WHERE（参数化，防注入）
	where := "WHERE status = 'published'"
	args := make([]any, 0, 4)
	if p.ContentType != "" {
		args = append(args, p.ContentType)
		where += fmt.Sprintf(" AND content_type = $%d", len(args))
	}
	if p.AuthorID > 0 {
		args = append(args, p.AuthorID)
		where += fmt.Sprintf(" AND author_id = $%d", len(args))
	}

	// 总数
	var total int64
	countSQL := `SELECT count(*) FROM posts ` + where
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 分页数据（最新发布在前）
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, media_ids FROM posts `+where+`
		ORDER BY published_at DESC NULLS LAST, id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)),
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

// Search 全文检索（需求 3.7）：标题/正文/标签关键词匹配，已发布帖子分页。
// 说明：MVP 用 ILIKE 模糊匹配（pg_trgm 扩展已启用，数据量小性能足够）；
//       M2 可升级 zhparser FTS 方案（架构文档 8.2）。
func (r *PostRepo) Search(ctx context.Context, keyword string, page int, pageSize int) ([]model.Post, int64, error) {
	// 关键词匹配帖子（标题/正文）或标签
	where := `WHERE p.status = 'published' AND (
		p.title ILIKE '%' || $1 || '%'
		OR p.content ILIKE '%' || $1 || '%'
		OR p.id IN (
			SELECT pt.post_id FROM post_tags pt
			JOIN tags t ON t.id = pt.tag_id
			WHERE t.name ILIKE '%' || $1 || '%'
		)
	)`

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts p `+where, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, p.media_ids FROM posts p `+where+`
		ORDER BY p.published_at DESC NULLS LAST
		LIMIT $2 OFFSET $3`, keyword, pageSize, (page-1)*pageSize)
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

// CountWhere 按动态 WHERE 统计（收藏/赞过等自定义查询共用）。
// 注意：where 必须以 `WHERE p.status = ...` 形式书写（p 为 posts 别名），参数从 $1 起。
func (r *PostRepo) CountWhere(ctx context.Context, where string, arg any, out *int64) error {
	return r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts p `+where, arg).Scan(out)
}

// ListWhere 按动态 WHERE 分页查询已发布帖子（收藏/赞过等自定义查询共用）。
func (r *PostRepo) ListWhere(ctx context.Context, where string, arg any, page int, pageSize int) ([]model.Post, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, p.media_ids FROM posts p `+where+`
		ORDER BY p.published_at DESC NULLS LAST
		LIMIT $2 OFFSET $3`, arg, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]model.Post, 0)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

// ListDrafts 查询用户草稿（按更新时间倒序）。
func (r *PostRepo) ListDrafts(ctx context.Context, authorID int64) ([]model.Post, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+postColumns+`, media_ids FROM posts
		WHERE author_id = $1 AND status = 'draft'
		ORDER BY updated_at DESC`, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]model.Post, 0)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}

// FavoritePostRow 收藏的帖子行（含收藏时间，用于「我的收藏」列表）。
type FavoritePostRow struct {
	Post        model.Post // 帖子实体
	FavoritedAt time.Time  // 收藏时间（post_reactions.created_at）
}

// ListFavorites 收藏的已发布帖子列表（按收藏时间倒序，分页）。
// 数据来源：post_reactions（type='favorite'）关联 posts。
func (r *PostRepo) ListFavorites(ctx context.Context, userID int64, page int, pageSize int) ([]FavoritePostRow, int64, error) {
	// 收藏关联查询（p 为 posts 别名；postColumns 需加 p. 前缀避免 JOIN 后列歧义）
	postCols := "p." + strings.ReplaceAll(postColumns, ", ", ", p.")
	const base = `FROM posts p
		JOIN post_reactions r ON r.post_id = p.id
		WHERE r.user_id = $1 AND r.type = 'favorite' AND p.status = 'published'`

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) `+base, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+postCols+`, p.media_ids, r.created_at `+base+`
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]FavoritePostRow, 0)
	for rows.Next() {
		row, err := scanFavoritePost(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, row)
	}
	return items, total, rows.Err()
}

// CountFavoritesByPosts 批量查询帖子收藏数（post_reactions type='favorite' 按帖聚合）。
// 返回：post_id → 收藏数（无收藏记录的帖子不出现在结果中）。
func (r *PostRepo) CountFavoritesByPosts(ctx context.Context, postIDs []int64) (map[int64]int64, error) {
	// 空列表直接返回空映射（避免 ANY($1) 空数组语义问题）
	if len(postIDs) == 0 {
		return map[int64]int64{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT post_id, count(*) FROM post_reactions
		WHERE type = 'favorite' AND post_id = ANY($1)
		GROUP BY post_id`, postIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int64)
	for rows.Next() {
		var postID int64
		var count int64
		if err := rows.Scan(&postID, &count); err != nil {
			return nil, err
		}
		counts[postID] = count
	}
	return counts, rows.Err()
}
