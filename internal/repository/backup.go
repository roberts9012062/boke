// internal/repository/backup.go
// 备份数据访问（M4-报表）：backup_records 记录管理 + 全站数据导出查询。
// 说明：应用级导出（JSON/CSV/ZIP），不依赖 pg_dump（差异记录）；db_dump 字段留空。
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 备份状态（schema：running / success / failed）。
const (
	BackupRunning = "running" // 执行中
	BackupSuccess = "success" // 成功
	BackupFailed  = "failed"  // 失败
)

// 备份类型（schema 注释：manual=手动 / scheduled=定时）。
const (
	BackupManual    = "manual"    // 手动
	BackupScheduled = "scheduled" // 定时（后置）
)

// BackupRecord 备份记录实体（backup_records 表）。
type BackupRecord struct {
	ID        int64     // 记录 ID
	Type      string    // manual / scheduled
	Status    string    // running / success / failed
	FilePath  string    // 打包文件路径（data/backup/ 相对或绝对）
	FileSize  int64     // 文件大小（字节）
	DBDump    string    // 数据库 dump 路径（应用级导出留空）
	MediaSnapshot string // 媒体快照路径（媒体库备份时记录）
	CreatedAt time.Time // 备份时间
}

// BackupRepo 备份记录数据访问（连接器类）。
type BackupRepo struct {
	pool *pgxpool.Pool
}

// NewBackupRepo 创建备份仓库。
func NewBackupRepo(pool *pgxpool.Pool) *BackupRepo {
	return &BackupRepo{pool: pool}
}

// Create 写入备份记录（返回 ID）。
func (r *BackupRepo) Create(ctx context.Context, record BackupRecord) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO backup_records (type, status, file_path, file_size, db_dump, media_snapshot)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		record.Type, record.Status, record.FilePath, record.FileSize, record.DBDump, record.MediaSnapshot).Scan(&id)
	return id, err
}

// List 备份记录列表（新备份在前，最多 100 条）。
func (r *BackupRepo) List(ctx context.Context) ([]BackupRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, type, status, file_path, file_size, db_dump, media_snapshot, created_at
		FROM backup_records ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]BackupRecord, 0)
	for rows.Next() {
		var b BackupRecord
		if err := rows.Scan(&b.ID, &b.Type, &b.Status, &b.FilePath, &b.FileSize, &b.DBDump, &b.MediaSnapshot, &b.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// FindByID 按 ID 查备份记录（无记录返回 false）。
func (r *BackupRepo) FindByID(ctx context.Context, id int64) (*BackupRecord, bool, error) {
	var b BackupRecord
	err := r.pool.QueryRow(ctx, `
		SELECT id, type, status, file_path, file_size, db_dump, media_snapshot, created_at
		FROM backup_records WHERE id = $1`, id).Scan(
		&b.ID, &b.Type, &b.Status, &b.FilePath, &b.FileSize, &b.DBDump, &b.MediaSnapshot, &b.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &b, true, nil
}

// Delete 删除备份记录（不删文件，文件由调用方按 FilePath 清理）。
func (r *BackupRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM backup_records WHERE id = $1`, id)
	return err
}

// DeleteOlderThan 删除指定类型早于截止时间的备份记录（过期清理）。
// 返回：被删除记录的 file_path 列表（调用方据此清理磁盘文件）。
func (r *BackupRepo) DeleteOlderThan(ctx context.Context, backupType string, cutoff time.Time) ([]string, error) {
	// 先取待删记录的文件路径（删记录前查询）
	rows, err := r.pool.Query(ctx, `
		SELECT file_path FROM backup_records
		WHERE type = $1 AND created_at < $2 AND file_path <> ''`, backupType, cutoff)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return nil, err
		}
		paths = append(paths, p)
	}
	rows.Close()

	_, err = r.pool.Exec(ctx, `
		DELETE FROM backup_records WHERE type = $1 AND created_at < $2`, backupType, cutoff)
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// ---------- 全站数据导出（应用级） ----------

// 导出范围常量（设计稿《备份导出》范围：内容 + 用户 + 媒体）。
const (
	ScopeContent = "content" // 内容：posts / comments / tags / post_tags
	ScopeUsers   = "users"   // 用户：users
	ScopeMedia   = "media"   // 媒体：media_assets（元数据；文件本体在媒体库备份）
)

// scopeTables 范围 → 导出表清单（应用级导出数据源；顺序稳定便于 CSV 多文件一致）。
var scopeTables = map[string][]string{
	ScopeContent: {"posts", "comments", "tags", "post_tags"},
	ScopeUsers:   {"users"},
	ScopeMedia:   {"media_assets"},
}

// ExportAllData 按范围全量导出（通用行导出：SELECT * → 键值行列表）。
// 参数：scope 范围列表（content/users/media 组合）；返回：表名 → 行数组。
// 说明：通用导出不绑定实体结构（备份场景数据完整性优先，KISS）。
func (r *BackupRepo) ExportAllData(ctx context.Context, scopes []string) (map[string][]map[string]any, error) {
	result := make(map[string][]map[string]any)
	seen := make(map[string]bool) // 表去重（范围可能有交集）
	for _, scope := range scopes {
		for _, table := range scopeTables[scope] {
			if seen[table] {
				continue
			}
			seen[table] = true
			rows, err := r.pool.Query(ctx, `SELECT * FROM `+table)
			if err != nil {
				return nil, err
			}
			// 动态取列名（pgx rows.Columns + 值扫描）
			columns := rows.FieldDescriptions()
			names := make([]string, 0, len(columns))
			for _, col := range columns {
				names = append(names, string(col.Name))
			}
			items := make([]map[string]any, 0)
			for rows.Next() {
				values, err := rows.Values()
				if err != nil {
					rows.Close()
					return nil, err
				}
				row := make(map[string]any, len(names))
				for i, name := range names {
					row[name] = values[i]
				}
				items = append(items, row)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
			result[table] = items
		}
	}
	return result, nil
}
