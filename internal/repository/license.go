// internal/repository/license.go
// 插件许可证数据访问（plugin_licenses 表，M3.5：激活/状态查询）。
package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PluginLicense 插件许可证实体（plugin_licenses 表）。
type PluginLicense struct {
	ID         int64      // 记录 ID
	PluginID   string     // 插件 ID
	Licensee   string     // 被许可方（站点 ID / 用户）
	Edition    string     // 版本：free / pro
	Features   []string   // 授权功能列表（JSONB）
	LicenseJWT string     // 许可证原文（Ed25519 签名 JWT）
	ExpiresAt  *time.Time // 到期时间（空=永久）
	CreatedAt  time.Time  // 创建时间
	UpdatedAt  time.Time  // 更新时间
}

// LicenseRepo 插件许可证数据访问（连接器类）。
type LicenseRepo struct {
	pool *pgxpool.Pool
}

// NewLicenseRepo 创建许可证仓库。
func NewLicenseRepo(pool *pgxpool.Pool) *LicenseRepo {
	return &LicenseRepo{pool: pool}
}

// Save 写入/覆盖许可证（按 plugin_id：存在更新、不存在插入——单站点单许可证）。
func (r *LicenseRepo) Save(ctx context.Context, lic PluginLicense) error {
	features, err := json.Marshal(lic.Features)
	if err != nil {
		return err
	}
	var expires any
	if lic.ExpiresAt != nil {
		expires = *lic.ExpiresAt
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO plugin_licenses (plugin_id, licensee, edition, features, license_jwt, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (plugin_id) DO UPDATE SET
			licensee = EXCLUDED.licensee, edition = EXCLUDED.edition,
			features = EXCLUDED.features, license_jwt = EXCLUDED.license_jwt,
			expires_at = EXCLUDED.expires_at, updated_at = now()`,
		lic.PluginID, lic.Licensee, lic.Edition, features, lic.LicenseJWT, expires)
	return err
}

// FindByPluginID 查询许可证（不存在返回 ErrNotFound）。
func (r *LicenseRepo) FindByPluginID(ctx context.Context, pluginID string) (*PluginLicense, error) {
	var lic PluginLicense
	var features []byte
	var expires *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id, plugin_id, licensee, edition, features, license_jwt, expires_at, created_at, updated_at
		FROM plugin_licenses WHERE plugin_id = $1`, pluginID).Scan(
		&lic.ID, &lic.PluginID, &lic.Licensee, &lic.Edition,
		&features, &lic.LicenseJWT, &expires, &lic.CreatedAt, &lic.UpdatedAt)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	lic.Features = nil
	_ = json.Unmarshal(features, &lic.Features)
	lic.ExpiresAt = expires
	return &lic, nil
}

// DeleteByPluginID 删除许可证（断开激活）。
func (r *LicenseRepo) DeleteByPluginID(ctx context.Context, pluginID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM plugin_licenses WHERE plugin_id = $1`, pluginID)
	return err
}
