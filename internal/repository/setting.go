// internal/repository/setting.go
// 站点设置数据访问（settings 表 key-value，值存 JSONB）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SettingRepo 站点设置数据访问（连接器类）。
type SettingRepo struct {
	pool *pgxpool.Pool
}

// NewSettingRepo 创建设置仓库。
func NewSettingRepo(pool *pgxpool.Pool) *SettingRepo {
	return &SettingRepo{pool: pool}
}

// All 读取全部设置（JSONB 值 → 原始字符串）。
func (r *SettingRepo) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		// JSONB 值去除引号（seed 数据为 "月言" 形式）
		raw := string(value)
		if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
			raw = raw[1 : len(raw)-1]
		}
		result[key] = raw
	}
	return result, rows.Err()
}

// SetMany 批量保存设置（值以 JSON 字符串写入）。
func (r *SettingRepo) SetMany(ctx context.Context, updates map[string]string) error {
	for key, value := range updates {
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO settings (key, value, description) VALUES ($1, $2::jsonb, '')
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
			key, `"`+value+`"`); err != nil {
			return err
		}
	}
	return nil
}
