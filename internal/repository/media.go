// internal/repository/media.go
// 媒体资源数据访问（media_assets 表）。
package repository

import (
	"context"

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
func (r *MediaRepo) Create(ctx context.Context, m MediaAsset) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO media_assets (owner_id, type, storage_key, url, mime_type, size_bytes, width, height, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
