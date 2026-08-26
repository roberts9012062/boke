// internal/repository/plugin.go
// 插件实例数据访问（plugin_instances 表，M3.1 插件商城/我的插件）。
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PluginInstance 插件实例实体（plugin_instances 表）。
type PluginInstance struct {
	ID        int64     // 实例 ID
	PluginID  string    // 插件 ID（市场清单中的 id）
	Name      string    // 插件名称
	Version   string    // 当前版本
	RepoURL   string    // 来源仓库
	State     string    // 状态：installed/running/disabled/uninstalled
	Pubkey    string    // 许可证公钥（M3.5：付费插件包内 pubkey.pem 登记）
	Capabilities []string // 能力登记（P2 加固：安装时落库；运行时门控与二进制自报取交集）
	LastError string    // 最近错误
	CreatedAt time.Time // 安装时间
}

// PluginRepo 插件实例数据访问（连接器类）。
type PluginRepo struct {
	pool *pgxpool.Pool
}

// NewPluginRepo 创建插件仓库。
func NewPluginRepo(pool *pgxpool.Pool) *PluginRepo {
	return &PluginRepo{pool: pool}
}

// Create 安装插件（写入实例，默认 running）。
func (r *PluginRepo) Create(ctx context.Context, inst PluginInstance) (int64, error) {
	var id int64
	capsRaw, err := json.Marshal(inst.Capabilities)
	if err != nil {
		return 0, err
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO plugin_instances (plugin_id, name, version, repo_url, state, capabilities)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING id`,
		inst.PluginID, inst.Name, inst.Version, inst.RepoURL, inst.State, string(capsRaw)).Scan(&id)
	return id, err
}

// Exists 判断插件是否已安装（未卸载）。
func (r *PluginRepo) Exists(ctx context.Context, pluginID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM plugin_instances
			WHERE plugin_id = $1 AND state != 'uninstalled'
		)`, pluginID).Scan(&exists)
	return exists, err
}

// FindByID 按实例 ID 查询（生命周期联动：启用/禁用/卸载前取插件 ID）。
func (r *PluginRepo) FindByID(ctx context.Context, id int64) (PluginInstance, error) {
	var inst PluginInstance
	var pubkey *string    // 免费插件无公钥：pubkey_pem 为 NULL（pgx 扫 **string 得 nil）
	var capsRaw []byte    // capabilities JSONB（默认 '[]'）
	err := r.pool.QueryRow(ctx, `
		SELECT id, plugin_id, name, version, repo_url, state, pubkey_pem, capabilities, last_error, created_at
		FROM plugin_instances WHERE id = $1`, id).Scan(
		&inst.ID, &inst.PluginID, &inst.Name, &inst.Version,
		&inst.RepoURL, &inst.State, &pubkey, &capsRaw, &inst.LastError, &inst.CreatedAt)
	if err != nil {
		return PluginInstance{}, wrapNotFound(err)
	}
	if pubkey != nil {
		inst.Pubkey = *pubkey
	}
	inst.Capabilities = decodeCapabilities(capsRaw)
	return inst, nil
}

// FindByPluginID 查询插件全部记录（含已卸载；plugin_id 唯一约束）。
// 返回：记录；不存在返回 ErrNotFound。
func (r *PluginRepo) FindByPluginID(ctx context.Context, pluginID string) (PluginInstance, error) {
	var inst PluginInstance
	var pubkey *string // 免费插件无公钥：pubkey_pem 为 NULL
	var capsRaw []byte // capabilities JSONB（默认 '[]'）
	err := r.pool.QueryRow(ctx, `
		SELECT id, plugin_id, name, version, repo_url, state, pubkey_pem, capabilities, last_error, created_at
		FROM plugin_instances WHERE plugin_id = $1`, pluginID).Scan(
		&inst.ID, &inst.PluginID, &inst.Name, &inst.Version,
		&inst.RepoURL, &inst.State, &pubkey, &capsRaw, &inst.LastError, &inst.CreatedAt)
	if err != nil {
		return PluginInstance{}, wrapNotFound(err)
	}
	if pubkey != nil {
		inst.Pubkey = *pubkey
	}
	inst.Capabilities = decodeCapabilities(capsRaw)
	return inst, nil
}

// SetPubkey 登记/更新插件许可证公钥（M3.5：安装解包后登记 pubkey.pem）。
func (r *PluginRepo) SetPubkey(ctx context.Context, pluginID string, pubkeyPEM string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET pubkey_pem = $2, updated_at = now() WHERE plugin_id = $1`,
		pluginID, pubkeyPEM)
	return err
}

// Reinstall 重新安装（复用已卸载记录：状态恢复 installed + 版本/来源/能力登记更新）。
// 说明（M3.1）：plugin_id 唯一约束，卸载为软删（uninstalled），重装需 UPDATE 复用而非 INSERT；
//              M3.3 起安装默认 installed，激活成功（内置注册/进程外拉起）后转 running；
//              capabilities 为安装来源声明的能力（P2 加固：运行时门控取交集依据）。
func (r *PluginRepo) Reinstall(ctx context.Context, instanceID int64, name string, version string, repoURL string, capabilities []string) error {
	capsRaw, err := json.Marshal(capabilities)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE plugin_instances SET state = 'installed', name = $2, version = $3, repo_url = $4, capabilities = $5::jsonb, updated_at = now()
		WHERE id = $1`, instanceID, name, version, repoURL, string(capsRaw))
	return err
}

// ListInstalled 已安装插件列表（排除卸载，按安装时间正序）。
// 正序：先安装的在前（后台侧栏插件动态入口顺序跟随安装先后，SEO 等核心插件在上）。
func (r *PluginRepo) ListInstalled(ctx context.Context) ([]PluginInstance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, plugin_id, name, version, repo_url, state, capabilities, last_error, created_at
		FROM plugin_instances
		WHERE state != 'uninstalled'
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PluginInstance, 0)
	for rows.Next() {
		var inst PluginInstance
		var capsRaw []byte // capabilities JSONB（默认 '[]'）
		if err := rows.Scan(&inst.ID, &inst.PluginID, &inst.Name, &inst.Version,
			&inst.RepoURL, &inst.State, &capsRaw, &inst.LastError, &inst.CreatedAt); err != nil {
			return nil, err
		}
		inst.Capabilities = decodeCapabilities(capsRaw)
		items = append(items, inst)
	}
	return items, rows.Err()
}

// SetState 启用/禁用插件（running / disabled）。
// 说明：主动状态变更视为恢复操作，同时清除 last_error（避免「已熔断」等历史错误残留误导展示）。
func (r *PluginRepo) SetState(ctx context.Context, instanceID int64, state string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET state = $2, last_error = '', updated_at = now() WHERE id = $1`,
		instanceID, state)
	return err
}

// SetStateByPluginID 按插件 ID 更新状态与最近错误（M3.3 进程管理器崩溃熔断回调）。
func (r *PluginRepo) SetStateByPluginID(ctx context.Context, pluginID string, state string, lastError string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET state = $2, last_error = $3, updated_at = now() WHERE plugin_id = $1`,
		pluginID, state, lastError)
	return err
}

// UpdateVersion 更新插件版本与来源（M3.4 升级/重装预留：替换二进制后更新记录）。
func (r *PluginRepo) UpdateVersion(ctx context.Context, instanceID int64, version string, repoURL string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET version = $2, repo_url = $3, updated_at = now() WHERE id = $1`,
		instanceID, version, repoURL)
	return err
}

// GetConfig 读取插件配置（config JSONB → map；无配置返回空 map）。
func (r *PluginRepo) GetConfig(ctx context.Context, instanceID int64) (map[string]string, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT config FROM plugin_instances WHERE id = $1`, instanceID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	values := map[string]string{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err // 数据损坏按错误返回（由调用方兜底）
		}
	}
	return values, nil
}

// SetConfig 保存插件配置（整体覆盖 config JSONB；values 已由 service 层按 schema 过滤）。
func (r *PluginRepo) SetConfig(ctx context.Context, instanceID int64, values map[string]string) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE plugin_instances SET config = $2::jsonb, updated_at = now() WHERE id = $1`,
		instanceID, string(raw))
	return err
}

// Delete 卸载插件（软删标记 uninstalled，保留审计痕迹）。
func (r *PluginRepo) Delete(ctx context.Context, instanceID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET state = 'uninstalled', updated_at = now() WHERE id = $1`, instanceID)
	return err
}

// decodeCapabilities 解析 capabilities JSONB（空/损坏返回空列表——收紧策略：无登记=无扩展能力）。
func decodeCapabilities(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	caps := make([]string, 0)
	if err := json.Unmarshal(raw, &caps); err != nil {
		return []string{}
	}
	return caps
}
