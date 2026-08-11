// internal/repository/admin_report.go
// 数据报表聚合（M4-报表）：四维趋势 / 待处理统计。
// 说明：方法挂在 AdminRepo 上（与 admin.go 同类型拆分文件，避免单文件超 400 行）。
package repository

import (
	"context"
)

// ReportTrendPoint 报表页趋势点（四维：浏览/新帖/获赞/评论，按日）。
type ReportTrendPoint struct {
	Date     string `json:"date"`     // 日期（MM-DD）
	Views    int64  `json:"views"`    // 浏览（post_views.viewed_at）
	Posts    int64  `json:"posts"`    // 新帖（已发布）
	Likes    int64  `json:"likes"`    // 获赞（post_reactions）
	Comments int64  `json:"comments"` // 评论
}

// ReportTrendSeries 报表页四维按日聚合（近 N 日，generate_series 补零）。
// 说明：报表页与仪表盘 TrendSeries 的差异是增加「浏览」维度（post_views 按日计数）。
func (r *AdminRepo) ReportTrendSeries(ctx context.Context, days int) ([]ReportTrendPoint, error) {
	rows, err := r.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series(current_date - ($1::int - 1), current_date, '1 day'::interval)::date AS day
		)
		SELECT to_char(d.day, 'MM-DD') AS date,
		       COALESCE(v.cnt, 0)   AS views,
		       COALESCE(p.cnt, 0)   AS posts,
		       COALESCE(l.cnt, 0)   AS likes,
		       COALESCE(c.cnt, 0)   AS comments
		FROM days d
		LEFT JOIN (SELECT viewed_at::date AS day, count(*) AS cnt FROM post_views WHERE viewed_at >= current_date - ($1::int - 1) GROUP BY 1) v ON v.day = d.day
		LEFT JOIN (SELECT created_at::date AS day, count(*) AS cnt FROM posts WHERE status = 'published' AND created_at >= current_date - ($1::int - 1) GROUP BY 1) p ON p.day = d.day
		LEFT JOIN (SELECT created_at::date AS day, count(*) AS cnt FROM post_reactions WHERE type = 'like' AND created_at >= current_date - ($1::int - 1) GROUP BY 1) l ON l.day = d.day
		LEFT JOIN (SELECT created_at::date AS day, count(*) AS cnt FROM comments WHERE status <> 'deleted' AND created_at >= current_date - ($1::int - 1) GROUP BY 1) c ON c.day = d.day
		ORDER BY d.day`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ReportTrendPoint, 0)
	for rows.Next() {
		var p ReportTrendPoint
		if err := rows.Scan(&p.Date, &p.Views, &p.Posts, &p.Likes, &p.Comments); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// CountHiddenComments 评论待审数（comments status='hidden'，报表页「评论待审」）。
func (r *AdminRepo) CountHiddenComments(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM comments WHERE status = 'hidden'`).Scan(&count)
	return count, err
}

// CountTodayPosts 今日新帖数（已发布，报表页统计卡「今日新帖」）。
func (r *AdminRepo) CountTodayPosts(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts WHERE status = 'published' AND created_at >= current_date`).Scan(&count)
	return count, err
}

// TotalSensitiveHits 敏感词命中合计（sensitive_words.hit_count 累计，报表页「敏感词命中」）。
func (r *AdminRepo) TotalSensitiveHits(ctx context.Context) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(sum(hit_count), 0) FROM sensitive_words`).Scan(&total)
	return total, err
}
