// internal/repository/plugin_order.go
// 插件购买订单数据访问（plugin_orders 表，M3.9 支付渠道）。
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PluginOrder 插件订单实体。
type PluginOrder struct {
	ID         int64     // 订单 ID
	PluginID   string    // 插件 ID
	InstanceID int64     // 插件实例 ID
	Price      int       // 金额（¥）
	State      string    // pending / paid / failed
	LicenseJWT string    // 服务端签发的许可证（支付成功后）
	CreatedAt  time.Time // 创建时间
}

// 订单状态常量。
const (
	OrderPending = "pending" // 待支付
	OrderPaid    = "paid"    // 已支付（已签发许可证）
	OrderFailed  = "failed"  // 失败
)

// PluginOrderRepo 插件订单数据访问（连接器类）。
type PluginOrderRepo struct {
	pool *pgxpool.Pool
}

// NewPluginOrderRepo 创建订单仓库。
func NewPluginOrderRepo(pool *pgxpool.Pool) *PluginOrderRepo {
	return &PluginOrderRepo{pool: pool}
}

// Create 创建订单（pending；返回订单 ID）。
func (r *PluginOrderRepo) Create(ctx context.Context, pluginID string, instanceID int64, price int) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO plugin_orders (plugin_id, instance_id, price) VALUES ($1, $2, $3) RETURNING id`,
		pluginID, instanceID, price).Scan(&id)
	return id, err
}

// MarkPaid 支付成功：写入许可证并置为 paid（原子更新）。
func (r *PluginOrderRepo) MarkPaid(ctx context.Context, orderID int64, licenseJWT string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_orders SET state = 'paid', license_jwt = $2, updated_at = now() WHERE id = $1`,
		orderID, licenseJWT)
	return err
}

// MarkFailed 支付失败置为 failed。
func (r *PluginOrderRepo) MarkFailed(ctx context.Context, orderID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE plugin_orders SET state = 'failed', updated_at = now() WHERE id = $1`, orderID)
	return err
}

// FindByID 查询订单（不存在返回 wrapNotFound）。
func (r *PluginOrderRepo) FindByID(ctx context.Context, orderID int64) (PluginOrder, error) {
	var o PluginOrder
	err := r.pool.QueryRow(ctx,
		`SELECT id, plugin_id, instance_id, price, state, license_jwt, created_at
		 FROM plugin_orders WHERE id = $1`, orderID).Scan(
		&o.ID, &o.PluginID, &o.InstanceID, &o.Price, &o.State, &o.LicenseJWT, &o.CreatedAt)
	if err != nil {
		return PluginOrder{}, wrapNotFound(err)
	}
	return o, nil
}
