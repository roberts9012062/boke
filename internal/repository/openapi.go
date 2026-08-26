// internal/repository/openapi.go
// 接口开放 API Key 数据访问层（pgx 原生 SQL + 结构化扫描）。
// 仅供 service 层与网关鉴权中间件调用。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// OpenAPIKeyRepo 开放接口凭证数据访问（连接器类）。
type OpenAPIKeyRepo struct {
	pool *pgxpool.Pool
}

// NewOpenAPIKeyRepo 创建凭证仓库。
func NewOpenAPIKeyRepo(pool *pgxpool.Pool) *OpenAPIKeyRepo {
	return &OpenAPIKeyRepo{pool: pool}
}

// openAPIKeyColumns 凭证查询列清单（顺序与 scanOpenAPIKey 严格一致）。
const openAPIKeyColumns = `id, name, key, endpoints, expires_at, last_used_at, created_at`

// scanOpenAPIKey 将查询行扫描为 OpenAPIKey 实体（endpoints 为 TEXT[] 直接扫描）。
func scanOpenAPIKey(row interface{ Scan(dest ...any) error }) (model.OpenAPIKey, error) {
	var k model.OpenAPIKey
	err := row.Scan(&k.ID, &k.Name, &k.Key, &k.Endpoints, &k.ExpiresAt, &k.LastUsedAt, &k.CreatedAt)
	if err != nil {
		return model.OpenAPIKey{}, wrapNotFound(err)
	}
	return k, nil
}

// Create 创建凭证（返回新 ID）。
func (r *OpenAPIKeyRepo) Create(ctx context.Context, k model.OpenAPIKey) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO open_api_keys (name, key, endpoints, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		k.Name, k.Key, k.Endpoints, k.ExpiresAt,
	).Scan(&id)
	return id, err
}

// Delete 删除凭证（返回是否找到并删除）。
func (r *OpenAPIKeyRepo) Delete(ctx context.Context, id int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM open_api_keys WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// List 全部凭证（按创建时间倒序，后台列表用）。
func (r *OpenAPIKeyRepo) List(ctx context.Context) ([]model.OpenAPIKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+openAPIKeyColumns+` FROM open_api_keys
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]model.OpenAPIKey, 0)
	for rows.Next() {
		k, err := scanOpenAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// FindByKey 按 Key 明文查询凭证（网关鉴权用）。
func (r *OpenAPIKeyRepo) FindByKey(ctx context.Context, key string) (model.OpenAPIKey, error) {
	return scanOpenAPIKey(r.pool.QueryRow(ctx,
		`SELECT `+openAPIKeyColumns+` FROM open_api_keys WHERE key = $1`, key))
}

// TouchLastUsed 更新最近调用时间（网关放行后异步记录，失败可忽略）。
func (r *OpenAPIKeyRepo) TouchLastUsed(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE open_api_keys SET last_used_at = now() WHERE id = $1`, id)
	return err
}
