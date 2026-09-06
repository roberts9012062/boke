// 中继站对接数据访问：单行配置 + 大世界内容缓存（纯 SQL，无业务）。
package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// RelayRepo 中继站配置与缓存仓储。
type RelayRepo struct {
	pool *pgxpool.Pool
}

// NewRelayRepo 构造中继站仓储。
func NewRelayRepo(pool *pgxpool.Pool) *RelayRepo {
	return &RelayRepo{pool: pool}
}

// Config 读取单行配置（恒存在，迁移 seed 保证）。
func (r *RelayRepo) Config(ctx context.Context) (model.RelayConfig, error) {
	var c model.RelayConfig
	err := r.pool.QueryRow(ctx, `
		SELECT enabled, url, site_key, mode, default_category, claim_token,
		       local_retention_days, relay_meta_json, last_seq, updated_at
		FROM relay_config WHERE id = 1`).Scan(
		&c.Enabled, &c.URL, &c.SiteKey, &c.Mode, &c.DefaultCategory, &c.ClaimToken,
		&c.LocalRetentionDays, &c.RelayMetaJSON, &c.LastSeq, &c.UpdatedAt)
	return c, err
}

// SaveConfigParams 配置保存参数（全量显式）。
type SaveConfigParams struct {
	Enabled            bool
	URL                string
	SiteKey            string
	Mode               string
	DefaultCategory    string
	LocalRetentionDays int
}

// SaveConfig 全量保存配置并刷新 updated_at（触发订阅任务重启由 service 层负责）。
func (r *RelayRepo) SaveConfig(ctx context.Context, p SaveConfigParams) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE relay_config SET
			enabled = $1, url = $2, site_key = $3, mode = $4,
			default_category = $5, local_retention_days = $6, updated_at = now()
		WHERE id = 1`,
		p.Enabled, p.URL, p.SiteKey, p.Mode, p.DefaultCategory, p.LocalRetentionDays)
	return err
}

// SaveClaim 保存申请凭据（审核制 v1.4）：等待中继站审批，期间 key 为空、订阅关闭。
func (r *RelayRepo) SaveClaim(ctx context.Context, url string, mode string, claimToken string, category string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE relay_config SET
			enabled = false, url = $1, mode = $2, site_key = '', claim_token = $3,
			default_category = $4, updated_at = now()
		WHERE id = 1`, url, mode, claimToken, category)
	return err
}

// SaveClaimedKey 凭据兑现：审批通过领取 key 后保存并清空凭据（key 隐藏保管）。
func (r *RelayRepo) SaveClaimedKey(ctx context.Context, siteKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE relay_config SET site_key = $1, claim_token = '', updated_at = now()
		WHERE id = 1`, siteKey)
	return err
}

// SaveMeta 缓存握手元信息。
func (r *RelayRepo) SaveMeta(ctx context.Context, metaJSON string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_config SET relay_meta_json = $1 WHERE id = 1`, metaJSON)
	return err
}

// SaveCursor 推进订阅游标（持久化断点，重启续拉）。
func (r *RelayRepo) SaveCursor(ctx context.Context, lastSeq int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE relay_config SET last_seq = $1 WHERE id = 1`, lastSeq)
	return err
}

// ResetCursor 游标清零（开关重新开启 / 全量重置）。
func (r *RelayRepo) ResetCursor(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `UPDATE relay_config SET last_seq = 0 WHERE id = 1`)
	return err
}

// UpsertCache 幂等写入缓存条目（content_id 唯一，重复投递忽略）。
func (r *RelayRepo) UpsertCache(ctx context.Context, item model.RelayCacheItem, retentionDays int) error {
	payloadJSON, err := json.Marshal(item.Payload)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO relay_content_cache (content_id, payload_json, published_at, fetched_at, expires_at)
		VALUES ($1, $2, $3, now(), now() + ($4 * interval '1 day'))
		ON CONFLICT (content_id) DO UPDATE SET
			payload_json = EXCLUDED.payload_json,
			published_at = EXCLUDED.published_at,
			fetched_at = now(),
			expires_at = now() + ($4 * interval '1 day')`,
		item.ContentID, payloadJSON, item.PublishedAt, retentionDays)
	return err
}

// DeleteCache 删除指定缓存（content.delete 事件）。
func (r *RelayRepo) DeleteCache(ctx context.Context, contentID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM relay_content_cache WHERE content_id = $1`, contentID)
	return err
}

// ListCache 大世界列表（分页，按发布时间倒序；可选分类过滤）。
func (r *RelayRepo) ListCache(ctx context.Context, category string, before time.Time, limit int) ([]model.RelayCacheItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT content_id, payload_json, published_at
		FROM relay_content_cache
		WHERE published_at < $1
		  AND ($2 = '' OR payload_json->>'category' = $2)
		ORDER BY published_at DESC LIMIT $3`, before, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]model.RelayCacheItem, 0, limit)
	for rows.Next() {
		var item model.RelayCacheItem
		var payloadRaw []byte
		if err := rows.Scan(&item.ContentID, &payloadRaw, &item.PublishedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payloadRaw, &item.Payload); err != nil {
			continue // 脏数据跳过，不阻断列表
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// SweepExpired 清理过期缓存（定时 + 惰性双保险），返回删除数。
func (r *RelayRepo) SweepExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM relay_content_cache WHERE expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CacheCount 缓存条数（后台展示）。
func (r *RelayRepo) CacheCount(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM relay_content_cache`).Scan(&n)
	return n, err
}
