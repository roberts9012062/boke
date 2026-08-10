// internal/repository/moderation.go
// 内容治理数据访问（M2）：举报工单（reports）、敏感词（sensitive_words）、封禁记录（ban_records）。
// 说明：三个连接器同属「内容治理」域，合并于一个文件（每层文件数 ≤8 约束）。
package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 举报状态（附录 B 状态字典扩展）。
const (
	ReportPending    = "pending"    // 待处理
	ReportProcessing = "processing" // 处理中
	ReportResolved   = "resolved"   // 已解决
	ReportRejected   = "rejected"   // 已驳回
)

// 敏感词级别（schema：forbidden=直接拦截 / review=进入审核）。
const (
	WordForbidden = "forbidden" // 直接拦截（发帖/评论拒绝）
	WordReview    = "review"    // 进入审核（MVP 放行，后续接入审核队列）
)

// Report 举报工单实体（reports 表）。
type Report struct {
	ID         int64     // 工单 ID
	ReporterID int64     // 举报人
	TargetType string    // 对象类型：post / comment / user
	TargetID   int64     // 对象 ID
	Reason     string    // 举报原因（预置选项）
	Detail     string    // 补充说明
	Status     string    // 状态：pending / processing / resolved / rejected
	CreatedAt  time.Time // 提交时间
}

// SensitiveWord 敏感词实体（sensitive_words 表）。
type SensitiveWord struct {
	ID        int64     // 词 ID
	Word      string    // 词内容
	Level     string    // 级别：forbidden / review
	CreatedAt time.Time // 添加时间
}

// BanRecord 封禁记录实体（ban_records 表）。
type BanRecord struct {
	ID        int64      // 记录 ID
	UserID    int64      // 被封禁用户
	Reason    string     // 封禁原因
	Until     *time.Time // 解封时间（NULL = 永久）
	CreatedBy int64      // 操作者
	CreatedAt time.Time  // 封禁时间
}

// ReportRepo 举报工单数据访问（连接器类）。
type ReportRepo struct {
	pool *pgxpool.Pool
}

// NewReportRepo 创建举报工单仓库。
func NewReportRepo(pool *pgxpool.Pool) *ReportRepo {
	return &ReportRepo{pool: pool}
}

// Create 提交举报（返回工单 ID）。
func (r *ReportRepo) Create(ctx context.Context, report Report) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO reports (reporter_id, target_type, target_id, reason, detail, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		report.ReporterID, report.TargetType, report.TargetID, report.Reason, report.Detail, ReportPending).Scan(&id)
	return id, err
}

// List 工单列表（状态过滤 + 分页，新提交在前）。
func (r *ReportRepo) List(ctx context.Context, status string, page int, pageSize int) ([]Report, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if status != "" {
		args = append(args, status)
		where += " WHERE status = $1"
	}

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM reports `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT id, reporter_id, target_type, target_id, reason, detail, status, created_at
		FROM reports `+where+`
		ORDER BY created_at DESC
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Report, 0)
	for rows.Next() {
		var rep Report
		if err := rows.Scan(&rep.ID, &rep.ReporterID, &rep.TargetType, &rep.TargetID, &rep.Reason, &rep.Detail, &rep.Status, &rep.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, rep)
	}
	return items, total, rows.Err()
}

// SetStatus 更新工单状态（处理中/已解决/已驳回）。
func (r *ReportRepo) SetStatus(ctx context.Context, reportID int64, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE reports SET status = $2 WHERE id = $1`, reportID, status)
	return err
}

// CountPending 待处理工单数（后台角标）。
func (r *ReportRepo) CountPending(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM reports WHERE status = 'pending'`).Scan(&count)
	return count, err
}

// CountResolvedToday 今日已处理数（resolved/rejected 按 created_at 当日，设计稿「今日已审」）。
func (r *ReportRepo) CountResolvedToday(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM reports
		WHERE status IN ('resolved', 'rejected') AND created_at >= current_date`).Scan(&count)
	return count, err
}

// SensitiveRepo 敏感词数据访问（连接器类）。
type SensitiveRepo struct {
	pool *pgxpool.Pool
}

// NewSensitiveRepo 创建敏感词仓库。
func NewSensitiveRepo(pool *pgxpool.Pool) *SensitiveRepo {
	return &SensitiveRepo{pool: pool}
}

// Create 添加敏感词（重复返回 false）。
func (r *SensitiveRepo) Create(ctx context.Context, word string, level string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO sensitive_words (word, level) VALUES ($1, $2)
		ON CONFLICT (word) DO NOTHING`, word, level)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Delete 删除敏感词（按词内容）。
func (r *SensitiveRepo) Delete(ctx context.Context, word string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM sensitive_words WHERE word = $1`, word)
	return err
}

// List 词库列表（关键词搜索 + 分页）。
func (r *SensitiveRepo) List(ctx context.Context, keyword string, page int, pageSize int) ([]SensitiveWord, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = "WHERE word ILIKE $1"
	}

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM sensitive_words `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.pool.Query(ctx, `
		SELECT id, word, level, created_at FROM sensitive_words `+where+`
		ORDER BY created_at DESC
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)),
		args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]SensitiveWord, 0)
	for rows.Next() {
		var w SensitiveWord
		if err := rows.Scan(&w.ID, &w.Word, &w.Level, &w.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, w)
	}
	return items, total, rows.Err()
}

// AllForbidden 全部 forbidden 词（内存匹配用，启动时加载）。
func (r *SensitiveRepo) AllForbidden(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT word FROM sensitive_words WHERE level = 'forbidden'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	words := make([]string, 0)
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return nil, err
		}
		words = append(words, word)
	}
	return words, rows.Err()
}

// CountByLevel 按级别统计词数（forbidden 拦截 / review 审核，设计稿统计条）。
func (r *SensitiveRepo) CountByLevel(ctx context.Context) (forbidden int64, review int64, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(sum(CASE WHEN level = 'forbidden' THEN 1 ELSE 0 END), 0),
			COALESCE(sum(CASE WHEN level = 'review' THEN 1 ELSE 0 END), 0)
		FROM sensitive_words`).Scan(&forbidden, &review)
	return forbidden, review, err
}

// BanRepo 封禁记录数据访问（连接器类）。
type BanRepo struct {
	pool *pgxpool.Pool
}

// NewBanRepo 创建封禁记录仓库。
func NewBanRepo(pool *pgxpool.Pool) *BanRepo {
	return &BanRepo{pool: pool}
}

// Create 写入封禁记录。
func (r *BanRepo) Create(ctx context.Context, record BanRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ban_records (user_id, reason, until, created_by)
		VALUES ($1, $2, $3, $4)`,
		record.UserID, record.Reason, record.Until, record.CreatedBy)
	return err
}

// List 封禁记录列表（关联用户昵称，分页）。
func (r *BanRepo) List(ctx context.Context, page int, pageSize int) ([]BanRecord, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM ban_records`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, reason, until, created_by, created_at
		FROM ban_records
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]BanRecord, 0)
	for rows.Next() {
		var b BanRecord
		if err := rows.Scan(&b.ID, &b.UserID, &b.Reason, &b.Until, &b.CreatedBy, &b.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, b)
	}
	return items, total, rows.Err()
}
