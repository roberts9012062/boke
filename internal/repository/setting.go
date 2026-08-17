// internal/repository/setting.go
// 站点设置数据访问（settings 表 key-value，值存 JSONB）。
package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
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
		result[key] = unquoteJSONB(value)
	}
	return result, rows.Err()
}

// Get 读取单个设置项。
// 返回：值（JSONB 去引号）与是否存在；读取失败时 ok=false（调用方按默认值处理）。
func (r *SettingRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var value []byte
	err := r.pool.QueryRow(ctx,
		`SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return unquoteJSONB(value), true, nil
}

// unquoteJSONB 解出 JSONB 值的真实字符串。
// 说明：pgx 读 jsonb 得到的是 JSON 序列化形式——多行值（如 PEM 公钥）内部的
// 换行是字面 "\n" 两字符，仅去首尾引号会让下游（pem.Decode 等）解析失败；
// 故用 json.Unmarshal 完整解码，非合法 JSON 字符串时原样返回（兼容历史纯文本行）。
func unquoteJSONB(raw []byte) string {
	var decoded string
	if err := json.Unmarshal(raw, &decoded); err == nil {
		return decoded
	}
	return string(raw)
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

// SetJSON 保存 JSON 对象设置（M5：权限矩阵 role_permissions 用；
// 与 SetMany 的区别：rawJSON 为合法 JSON 对象文本，直接绑定 jsonb 不包引号）。
func (r *SettingRepo) SetJSON(ctx context.Context, key string, rawJSON string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO settings (key, value, description) VALUES ($1, $2::jsonb, '')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, rawJSON)
	return err
}
