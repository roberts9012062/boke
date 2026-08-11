// internal/repository/plugin.go
// 插件实例数据访问（plugin_instances 表，M3.1 插件商城/我的插件）。
package repository

import (
	"context"
	"time"

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
	err := r.pool.QueryRow(ctx, `
		INSERT INTO plugin_instances (plugin_id, name, version, repo_url, state)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		inst.PluginID, inst.Name, inst.Version, inst.RepoURL, inst.State).Scan(&id)
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
	err := r.pool.QueryRow(ctx, `
		SELECT id, plugin_id, name, version, repo_url, state, last_error, created_at
		FROM plugin_instances WHERE id = $1`, id).Scan(
		&inst.ID, &inst.PluginID, &inst.Name, &inst.Version,
		&inst.RepoURL, &inst.State, &inst.LastError, &inst.CreatedAt)
	if err != nil {
		return PluginInstance{}, wrapNotFound(err)
	}
	return inst, nil
}

// FindByPluginID 查询插件全部记录（含已卸载；plugin_id 唯一约束）。
// 返回：记录；不存在返回 ErrNotFound。
func (r *PluginRepo) FindByPluginID(ctx context.Context, pluginID string) (PluginInstance, error) {
	var inst PluginInstance
	err := r.pool.QueryRow(ctx, `
		SELECT id, plugin_id, name, version, repo_url, state, last_error, created_at
		FROM plugin_instances WHERE plugin_id = $1`, pluginID).Scan(
		&inst.ID, &inst.PluginID, &inst.Name, &inst.Version,
		&inst.RepoURL, &inst.State, &inst.LastError, &inst.CreatedAt)
	if err != nil {
		return PluginInstance{}, wrapNotFound(err)
	}
	return inst, nil
}

// Reinstall 重新安装（复用已卸载记录：状态恢复 running + 版本/来源更新）。
// 说明（M3.1）：plugin_id 唯一约束，卸载为软删（uninstalled），重装需 UPDATE 复用而非 INSERT。
func (r *PluginRepo) Reinstall(ctx context.Context, instanceID int64, version string, repoURL string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE plugin_instances SET state = 'running', version = $2, repo_url = $3, updated_at = now()
		WHERE id = $1`, instanceID, version, repoURL)
	return err
}

// ListInstalled 已安装插件列表（排除卸载，按安装时间倒序）。
func (r *PluginRepo) ListInstalled(ctx context.Context) ([]PluginInstance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, plugin_id, name, version, repo_url, state, last_error, created_at
		FROM plugin_instances
		WHERE state != 'uninstalled'
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PluginInstance, 0)
	for rows.Next() {
		var inst PluginInstance
		if err := rows.Scan(&inst.ID, &inst.PluginID, &inst.Name, &inst.Version,
			&inst.RepoURL, &inst.State, &inst.LastError, &inst.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, inst)
	}
	return items, rows.Err()
}

// SetState 启用/禁用插件（running / disabled）。
func (r *PluginRepo) SetState(ctx context.Context, instanceID int64, state string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET state = $2, updated_at = now() WHERE id = $1`, instanceID, state)
	return err
}

// Delete 卸载插件（软删标记 uninstalled，保留审计痕迹）。
func (r *PluginRepo) Delete(ctx context.Context, instanceID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_instances SET state = 'uninstalled', updated_at = now() WHERE id = $1`, instanceID)
	return err
}
