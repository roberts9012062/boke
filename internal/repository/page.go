// internal/repository/page.go
// 自定义页面数据访问层（pgx 原生 SQL + 结构化扫描）。
// 仅供 service 层调用（分层单向依赖：handler → service → repository）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// PageRepo 自定义页面数据访问（连接器类）。
type PageRepo struct {
	pool *pgxpool.Pool
}

// NewPageRepo 创建自定义页面仓库。
func NewPageRepo(pool *pgxpool.Pool) *PageRepo {
	return &PageRepo{pool: pool}
}

// pageColumns 页面查询列清单（列表/详情共用，扫描顺序严格对应）。
const pageColumns = `id, slug, title, content, content_format, description, status, created_at, updated_at`

// scanPage 将查询行扫描为 CustomPage 实体。
func scanPage(row pgx.Row) (model.CustomPage, error) {
	var p model.CustomPage
	err := row.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Content, &p.ContentFormat,
		&p.Description, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return model.CustomPage{}, wrapNotFound(err)
	}
	return p, nil
}

// List 全量页面列表（后台管理，含草稿；按更新时间倒序）。
func (r *PageRepo) List(ctx context.Context) ([]model.CustomPage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+pageColumns+` FROM custom_pages
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.CustomPage, 0)
	for rows.Next() {
		var p model.CustomPage
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.Content, &p.ContentFormat,
			&p.Description, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// GetByID 按 ID 查询页面（后台编辑回显）。
func (r *PageRepo) GetByID(ctx context.Context, id int64) (model.CustomPage, error) {
	return scanPage(r.pool.QueryRow(ctx,
		`SELECT `+pageColumns+` FROM custom_pages WHERE id = $1`, id))
}

// GetBySlug 按 slug 查询页面。
// 参数：slug 路由标识；publishedOnly 为 true 时仅返回已发布页面（前台用，草稿视同不存在）。
func (r *PageRepo) GetBySlug(ctx context.Context, slug string, publishedOnly bool) (model.CustomPage, error) {
	query := `SELECT ` + pageColumns + ` FROM custom_pages WHERE slug = $1`
	if publishedOnly {
		query += ` AND status = 'published'`
	}
	return scanPage(r.pool.QueryRow(ctx, query, slug))
}

// SlugExists 判断 slug 是否已被其他页面占用（excludeID 排除自身，更新场景防误判）。
func (r *PageRepo) SlugExists(ctx context.Context, slug string, excludeID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM custom_pages WHERE slug = $1 AND id <> $2)`,
		slug, excludeID).Scan(&exists)
	return exists, err
}

// Create 创建页面（返回新页面 ID）。
func (r *PageRepo) Create(ctx context.Context, p model.CustomPage) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO custom_pages (slug, title, content, content_format, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		p.Slug, p.Title, p.Content, p.ContentFormat, p.Description, p.Status).Scan(&id)
	return id, err
}

// Update 更新页面（全量覆盖，updated_at 触发器式刷新）。
func (r *PageRepo) Update(ctx context.Context, id int64, p model.CustomPage) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE custom_pages
		SET slug = $1, title = $2, content = $3, content_format = $4,
		    description = $5, status = $6, updated_at = now()
		WHERE id = $7`,
		p.Slug, p.Title, p.Content, p.ContentFormat, p.Description, p.Status, id)
	return err
}

// Delete 删除页面（物理删除：页面无评论等关联数据，无需软删）。
func (r *PageRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM custom_pages WHERE id = $1`, id)
	return err
}
