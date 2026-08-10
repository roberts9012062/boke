// internal/repository/audit.go
// 审计日志数据访问（audit_logs 表写入）。
// 约定：登录/注册等关键操作写审计（架构文档 9.2）；actor_id=0 表示系统。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditEntry 审计日志条目（对应 audit_logs 表字段）。
type AuditEntry struct {
	ActorID      int64  // 操作者 ID（0 = 系统）
	Action       string // 动作：register / login / logout / delete_post ...
	ResourceType string // 资源类型：user / post / comment ...
	ResourceID   int64  // 资源 ID
	BeforeData   string // 变更前快照（JSON 字符串，可为空）
	AfterData    string // 变更后快照（JSON 字符串，可为空）
	IP           string // 操作者 IP
	UserAgent    string // 浏览器 UA
}

// AuditRepo 审计日志数据访问（连接器类）。
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo 创建审计仓库。
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// Insert 写入一条审计日志。
// 参数为 AuditEntry 结构（before/after 传空字符串时写入 NULL）。
func (r *AuditRepo) Insert(ctx context.Context, e AuditEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, before_data, after_data, ip, user_agent)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8)`,
		e.ActorID, e.Action, e.ResourceType, e.ResourceID,
		e.BeforeData, e.AfterData, e.IP, e.UserAgent,
	)
	return err
}
