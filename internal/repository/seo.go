// internal/repository/seo.go
// SEO 数据访问（M4）：seo_settings（单行全局设置）/ seo_meta（帖子级元数据）/ seo_health_checks（健康度）。
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeoSettings SEO 全局设置（seo_settings 单行，id=1）。
type SeoSettings struct {
	SiteName        string `json:"site_name"`         // 站点名称
	SiteDescription string `json:"site_description"`  // 站点描述（默认描述）
	TitleSuffix     string `json:"title_suffix"`      // 站点标题后缀（拼在文章标题后）
	Keywords        string `json:"keywords"`          // 默认关键词（逗号分隔）
	OgTitle         string `json:"og_title"`          // 默认 OG 标题
	RobotsTxt       string `json:"robots_txt"`        // robots.txt 内容
	SitemapEnabled  bool   `json:"sitemap_enabled"`   // 是否生成 sitemap
}

// SeoMeta 帖子级 SEO 元数据（seo_meta，post_id 唯一）。
type SeoMeta struct {
	PostID       int64  `json:"post_id"`       // 帖子 ID
	Title        string `json:"title"`         // SEO 标题
	Description  string `json:"description"`   // SEO 描述
	Keywords     string `json:"keywords"`      // 关键词
	CanonicalURL string `json:"canonical_url"` // 规范链接（URL 别名）
	OgImage      string `json:"og_image"`      // 分享图
	Summary      string `json:"summary"`       // AI 摘要（M4 AI 阶段）
	URLAlias     string `json:"url_alias"`     // URL 别名（/p/{alias} 短链，M4.1 插件通道）
	Robots       string `json:"robots"`        // 收录策略（index,follow 等；空=跟随全局）
}

// SeoHealthCheck 帖子健康度（seo_health_checks）。
type SeoHealthCheck struct {
	PostID    int64     // 帖子 ID
	Score     int       // 健康分（0-100）
	Issues    []byte    // 问题清单 JSON（[{code,message}]）
	CheckedAt time.Time // 检查时间
}

// SeoRepo SEO 数据访问（连接器类）。
type SeoRepo struct {
	pool *pgxpool.Pool
}

// NewSeoRepo 创建 SEO 仓库。
func NewSeoRepo(pool *pgxpool.Pool) *SeoRepo {
	return &SeoRepo{pool: pool}
}

// GetSettings 读取全局 SEO 设置（单行；无记录时返回默认值）。
func (r *SeoRepo) GetSettings(ctx context.Context) (*SeoSettings, error) {
	settings := &SeoSettings{
		SiteName:        "月言",
		SiteDescription: "月光下慢慢写，记录日常与灵感。",
		TitleSuffix:     "· 月言",
		SitemapEnabled:  true,
	}
	err := r.pool.QueryRow(ctx, `
		SELECT site_name, site_description, title_suffix, keywords, og_title, robots_txt, sitemap_enabled
		FROM seo_settings WHERE id = 1`).Scan(
		&settings.SiteName, &settings.SiteDescription, &settings.TitleSuffix,
		&settings.Keywords, &settings.OgTitle, &settings.RobotsTxt, &settings.SitemapEnabled)
	if err != nil {
		// 无记录：返回默认（不报错，首次保存时创建）
		if isNoRows(err) {
			return settings, nil
		}
		return nil, err
	}
	return settings, nil
}

// UpsertSettings 保存全局 SEO 设置（单行 upsert）。
func (r *SeoRepo) UpsertSettings(ctx context.Context, s SeoSettings) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO seo_settings (id, site_name, site_description, title_suffix, keywords, og_title, robots_txt, sitemap_enabled)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			site_name = EXCLUDED.site_name,
			site_description = EXCLUDED.site_description,
			title_suffix = EXCLUDED.title_suffix,
			keywords = EXCLUDED.keywords,
			og_title = EXCLUDED.og_title,
			robots_txt = EXCLUDED.robots_txt,
			sitemap_enabled = EXCLUDED.sitemap_enabled,
			updated_at = now()`,
		s.SiteName, s.SiteDescription, s.TitleSuffix, s.Keywords, s.OgTitle, s.RobotsTxt, s.SitemapEnabled)
	return err
}

// GetMeta 读取帖子 SEO 元数据（无记录返回空结构）。
func (r *SeoRepo) GetMeta(ctx context.Context, postID int64) (*SeoMeta, error) {
	meta := &SeoMeta{PostID: postID}
	err := r.pool.QueryRow(ctx, `
		SELECT post_id, title, description, keywords, canonical_url, og_image, summary, url_alias, robots
		FROM seo_meta WHERE post_id = $1`, postID).Scan(
		&meta.PostID, &meta.Title, &meta.Description, &meta.Keywords,
		&meta.CanonicalURL, &meta.OgImage, &meta.Summary, &meta.URLAlias, &meta.Robots)
	if err != nil {
		if isNoRows(err) {
			return meta, nil
		}
		return nil, err
	}
	return meta, nil
}

// UpsertMeta 保存帖子 SEO 元数据（upsert；空值覆盖）。
func (r *SeoRepo) UpsertMeta(ctx context.Context, m SeoMeta) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO seo_meta (post_id, title, description, keywords, canonical_url, og_image, summary, url_alias, robots)
		VALUES ($1, $2, $3, $4, $5, $6, '', $7, $8)
		ON CONFLICT (post_id) DO UPDATE SET
			title = EXCLUDED.title, description = EXCLUDED.description,
			keywords = EXCLUDED.keywords, canonical_url = EXCLUDED.canonical_url,
			og_image = EXCLUDED.og_image, url_alias = EXCLUDED.url_alias,
			robots = EXCLUDED.robots, updated_at = now()`,
		m.PostID, m.Title, m.Description, m.Keywords, m.CanonicalURL, m.OgImage, m.URLAlias, m.Robots)
	if err != nil {
		// URL 别名全局唯一（uq_seo_meta_url_alias）：占用冲突给出友好提示
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errors.New("URL 别名已占用，请更换")
		}
		return err
	}
	return nil
}

// FindByAlias 按 URL 别名查帖子 ID（/p/{alias} 短链解析；不存在返回 wrapNotFound）。
func (r *SeoRepo) FindByAlias(ctx context.Context, alias string) (int64, error) {
	var postID int64
	err := r.pool.QueryRow(ctx,
		`SELECT post_id FROM seo_meta WHERE url_alias = $1 AND url_alias <> ''`, alias).Scan(&postID)
	if err != nil {
		return 0, wrapNotFound(err)
	}
	return postID, nil
}

// UpdateSummary 写入 AI 生成的帖子摘要（seo_meta.summary，M4-AI 场景）。
// 说明：仅更新 summary 列，不影响其他 SEO 字段；无记录时创建。
func (r *SeoRepo) UpdateSummary(ctx context.Context, postID int64, summary string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO seo_meta (post_id, summary)
		VALUES ($1, $2)
		ON CONFLICT (post_id) DO UPDATE SET
			summary = EXCLUDED.summary, updated_at = now()`,
		postID, summary)
	return err
}

// AllForSitemap 全部公开帖子（sitemap 生成：id/标题/更新时间/媒体 URL）。
// 返回：帖子行（含首图 URL 便于图片 sitemap）。
func (r *SeoRepo) AllForSitemap(ctx context.Context) ([]PostSitemapRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.title, p.updated_at, p.media_ids, p.cover_url
		FROM posts p
		WHERE p.status = 'published' AND p.visibility = 'public'
		ORDER BY p.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PostSitemapRow, 0)
	for rows.Next() {
		var row PostSitemapRow
		var mediaIDs []byte
		if err := rows.Scan(&row.ID, &row.Title, &row.UpdatedAt, &mediaIDs, &row.CoverURL); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

// PostSitemapRow sitemap 帖子行。
type PostSitemapRow struct {
	ID        int64     // 帖子 ID
	Title     string    // 标题
	UpdatedAt time.Time // 更新时间（lastmod）
	CoverURL  string    // 封面图（图片 sitemap）
}

// PagesForSitemap 全部已发布自定义页面（sitemap 生成：slug/更新时间）。
func (r *SeoRepo) PagesForSitemap(ctx context.Context) ([]PageSitemapRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT slug, updated_at FROM custom_pages
		WHERE status = 'published'
		ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PageSitemapRow, 0)
	for rows.Next() {
		var row PageSitemapRow
		if err := rows.Scan(&row.Slug, &row.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

// PageSitemapRow sitemap 自定义页面行。
type PageSitemapRow struct {
	Slug      string    // 路由标识（前台 /pages/{slug}）
	UpdatedAt time.Time // 更新时间（lastmod）
}

// SaveHealthCheck 写入帖子健康度（覆盖同帖）。
func (r *SeoRepo) SaveHealthCheck(ctx context.Context, h SeoHealthCheck) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO seo_health_checks (post_id, score, issues)
		VALUES ($1, $2, $3)
		ON CONFLICT (post_id) DO UPDATE SET score = EXCLUDED.score, issues = EXCLUDED.issues, checked_at = now()`,
		h.PostID, h.Score, h.Issues)
	return err
}

// TrendPoint 健康分趋势点（近 N 日，按日平均分）。
type SeoHealthTrendPoint struct {
	Date  string  // 日期（MM-DD）
	Score float64 // 当日平均健康分
}

// HealthTrend 近 N 日健康分趋势（按 checked_at 按日聚合；无数据日期补 0）。
func (r *SeoRepo) HealthTrend(ctx context.Context, days int) ([]SeoHealthTrendPoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT to_char(d.day, 'MM-DD'), COALESCE(avg(h.score), 0)
		FROM generate_series(current_date - ($1 - 1), current_date, interval '1 day') AS d(day)
		LEFT JOIN seo_health_checks h ON date_trunc('day', h.checked_at)::date = d.day
		GROUP BY d.day ORDER BY d.day`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]SeoHealthTrendPoint, 0, days)
	for rows.Next() {
		var p SeoHealthTrendPoint
		if err := rows.Scan(&p.Date, &p.Score); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// TypeDistribution 问题类型分布（按 issue code 计数，前 N 类）。
func (r *SeoRepo) TypeDistribution(ctx context.Context, limit int) ([]HealthTypeCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT issue->>'code' AS code, count(*) AS cnt
		FROM seo_health_checks h, jsonb_array_elements(h.issues) AS issue
		GROUP BY 1 ORDER BY cnt DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]HealthTypeCount, 0)
	for rows.Next() {
		var c HealthTypeCount
		if err := rows.Scan(&c.Code, &c.Count); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// HealthTypeCount 问题类型计数。
type HealthTypeCount struct {
	Code  string // 问题编码
	Count int64  // 数量
}

// CountMetaCoverage 元信息覆盖（有 SEO 标题+描述的帖子数 / 可索引公开帖数）。
// 返回：covered 覆盖数；total 可索引总数（公开 + 仅关注者，不含私密）。
func (r *SeoRepo) CountMetaCoverage(ctx context.Context) (covered int64, total int64, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE m.post_id IS NOT NULL AND m.title != '' AND m.description != ''),
			count(*)
		FROM posts p
		LEFT JOIN seo_meta m ON m.post_id = p.id
		WHERE p.status = 'published' AND p.visibility != 'private'`).Scan(&covered, &total)
	return covered, total, err
}

// ListHealthChecks 健康度记录列表（按检查时间倒序）。
func (r *SeoRepo) ListHealthChecks(ctx context.Context, limit int) ([]SeoHealthCheck, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT post_id, score, issues, checked_at FROM seo_health_checks
		ORDER BY checked_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SeoHealthCheck, 0)
	for rows.Next() {
		var h SeoHealthCheck
		if err := rows.Scan(&h.PostID, &h.Score, &h.Issues, &h.CheckedAt); err != nil {
			return nil, err
		}
		items = append(items, h)
	}
	return items, rows.Err()
}

// isNoRows 判断是否无记录错误（pgx.ErrNoRows）。
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
