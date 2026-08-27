// internal/repository/media.go
// 媒体资源数据访问（media_assets 表）。
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MediaAsset 媒体资源实体（media_assets 表结构）。
type MediaAsset struct {
	ID        int64  // 媒体 ID
	OwnerID   int64  // 上传者
	Type      string // 类型：image / audio / video / file
	StorageKey string // 存储键（本地路径）
	URL       string // 访问地址
	MimeType  string // MIME 类型
	SizeBytes int64  // 文件大小
	Width     int    // 宽（图片）
	Height    int    // 高（图片）
	Status    string // 状态：ready / processing / failed
}

// MediaRepo 媒体资源数据访问（连接器类）。
type MediaRepo struct {
	pool *pgxpool.Pool
}

// NewMediaRepo 创建媒体仓库。
func NewMediaRepo(pool *pgxpool.Pool) *MediaRepo {
	return &MediaRepo{pool: pool}
}

// Create 创建媒体记录（返回新媒体 ID）。
// owner_id 为 0（系统生成，AI 辅助产物）时归属站长——媒体读取链路按非空
// int64 扫描 owner_id，NULL 行会击穿时间线等聚合查询，故系统产物统一归
// 站长（superadmin 最小 ID）而非留空。
func (r *MediaRepo) Create(ctx context.Context, m MediaAsset) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO media_assets (owner_id, type, storage_key, url, mime_type, size_bytes, width, height, status)
		VALUES (COALESCE(NULLIF($1, 0), (SELECT min(id) FROM users WHERE role = 'superadmin')), $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		m.OwnerID, m.Type, m.StorageKey, m.URL, m.MimeType,
		m.SizeBytes, m.Width, m.Height, m.Status,
	).Scan(&id)
	return id, err
}

// FindByIDs 按 ID 列表批量查询媒体（保持传入顺序）。
func (r *MediaRepo) FindByIDs(ctx context.Context, ids []int64) ([]MediaAsset, error) {
	if len(ids) == 0 {
		return []MediaAsset{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, type, storage_key, url, mime_type, size_bytes, width, height, status
		FROM media_assets WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 按 id 索引，便于保持传入顺序
	byID := make(map[int64]MediaAsset, len(ids))
	for rows.Next() {
		var m MediaAsset
		if err := rows.Scan(
			&m.ID, &m.OwnerID, &m.Type, &m.StorageKey, &m.URL,
			&m.MimeType, &m.SizeBytes, &m.Width, &m.Height, &m.Status,
		); err != nil {
			return nil, err
		}
		byID[m.ID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 按传入顺序组装
	result := make([]MediaAsset, 0, len(ids))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			result = append(result, m)
		}
	}
	return result, nil
}

// ---------- 后台媒体库（M2.9，设计稿《后台媒体》） ----------

// MediaAdminRow 后台媒体行（含引用数，设计稿表格：文件/类型/大小/引用/上传/操作）。
type MediaAdminRow struct {
	ID        int64     `json:"id"`         // 媒体 ID
	Type      string    `json:"type"`       // 类型：image/audio/video
	URL       string    `json:"url"`        // 访问地址
	MimeType  string    `json:"mime_type"`  // MIME 类型
	SizeBytes int64     `json:"size_bytes"` // 文件大小
	Width     int       `json:"width"`      // 宽（图片/视频）
	Height    int       `json:"height"`     // 高（图片/视频）
	FileName  string    `json:"file_name"`  // 文件名（url 最后一段；原名未存，差异记录）
	RefCount  int64     `json:"ref_count"`  // 被帖子引用数
	CreatedAt time.Time `json:"created_at"` // 上传时间
}

// MediaStats 媒体统计条（设计稿：全部文件/图片/音频/视频）。
type MediaStats struct {
	Total int64 `json:"total"` // 全部文件
	Image int64 `json:"image"` // 图片
	Audio int64 `json:"audio"` // 音频
	Video int64 `json:"video"` // 视频
}

// Stats 媒体统计条（一次查询四项）。
func (r *MediaRepo) Stats(ctx context.Context) (*MediaStats, error) {
	var stats MediaStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE type = 'image'),
			count(*) FILTER (WHERE type = 'audio'),
			count(*) FILTER (WHERE type = 'video')
		FROM media_assets WHERE status = 'ready'`).Scan(&stats.Total, &stats.Image, &stats.Audio, &stats.Video)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// ListAdmin 后台媒体列表（类型筛选 + 文件名关键词 + 分页 + 引用数）。
func (r *MediaRepo) ListAdmin(ctx context.Context, mediaType string, keyword string, page int, pageSize int) ([]MediaAdminRow, int64, error) {
	where := "WHERE m.status = 'ready'"
	args := make([]any, 0, 3)
	if mediaType != "" {
		args = append(args, mediaType)
		where += fmt.Sprintf(" AND m.type = $%d", len(args))
	}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(" AND m.url ILIKE $%d", len(args))
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM media_assets m `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.type, m.url, m.mime_type, m.size_bytes, m.width, m.height, m.created_at,
		       (SELECT count(*) FROM posts p WHERE p.media_ids @> jsonb_build_array(m.id)),
		       split_part(m.url, '/', array_length(string_to_array(m.url, '/'), 1))
		FROM media_assets m `+where+`
		ORDER BY m.created_at DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)-1)+` OFFSET $`+fmt.Sprintf("%d", len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]MediaAdminRow, 0)
	for rows.Next() {
		var row MediaAdminRow
		if err := rows.Scan(&row.ID, &row.Type, &row.URL, &row.MimeType, &row.SizeBytes,
			&row.Width, &row.Height, &row.CreatedAt, &row.RefCount, &row.FileName); err != nil {
			return nil, 0, err
		}
		items = append(items, row)
	}
	return items, total, rows.Err()
}

// Delete 删除媒体（事务）：解除帖子引用（media_ids 移除 + cover_url 清空）→ 删除记录。
// 返回：storageKey（供调用方删除磁盘文件）与错误。
func (r *MediaRepo) Delete(ctx context.Context, id int64) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 查询存储键（删除磁盘文件用）
	var storageKey string
	if err := tx.QueryRow(ctx,
		`SELECT storage_key FROM media_assets WHERE id = $1`, id).Scan(&storageKey); err != nil {
		return "", err
	}

	// 解除帖子引用：media_ids 移除该 id（数字 JSONB 数组按值移除并保序）；
	// cover_url 指向该媒体时清空。
	// 注意：jsonb `- text` 仅匹配字符串元素，无法命中数字数组（历史故障：删除后残留死引用）；
	//       改为 jsonb_array_elements 按值过滤重聚（$2 为 bigint，与 jsonb 数字元素类型一致）。
	if _, err := tx.Exec(ctx, `
		UPDATE posts SET
			media_ids = COALESCE((
				SELECT jsonb_agg(elem ORDER BY ord)
				FROM jsonb_array_elements(posts.media_ids) WITH ORDINALITY AS arr(elem, ord)
				WHERE elem <> to_jsonb($2::bigint)
			), '[]'::jsonb),
			cover_url = CASE WHEN cover_url = $1 THEN '' ELSE cover_url END
		WHERE media_ids @> jsonb_build_array($2::bigint)`,
		"/media/"+storageKey, id); err != nil {
		return "", err
	}

	// 删除媒体记录
	if _, err := tx.Exec(ctx, `DELETE FROM media_assets WHERE id = $1`, id); err != nil {
		return "", err
	}
	return storageKey, tx.Commit(ctx)
}
