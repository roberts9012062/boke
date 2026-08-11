// internal/service/report.go
// 数据报表业务（M4-报表，设计稿《数据报表》#235/#242）：
// 统计卡（浏览/获赞/评论/今日新帖 + 环比）+ 四维趋势（浏览/新帖/获赞/评论）+ 待处理块 + CSV 导出。
// 说明：统计卡口径与仪表盘一致（近 7 日环比）；趋势图支持 7/30 日切换（页头「近 30 日趋势 · 导出 CSV」）。
package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"time"

	"github.com/roberts9012062/boke/internal/repository"
)

// 报表页趋势天数（设计稿：近 7 日 / 近 30 日切换）。
const (
	reportDaysMin = 7  // 趋势最小天数
	reportDaysMax = 30 // 趋势最大天数（默认）
)

// ReportOverview 报表页聚合数据。
type ReportOverview struct {
	Views7d     int64                            `json:"views_7d"`     // 近 7 日浏览
	ViewsTrend  float64                          `json:"views_trend"`  // 环比（%）
	Likes7d     int64                            `json:"likes_7d"`     // 近 7 日获赞
	LikesTrend  float64                          `json:"likes_trend"`  // 环比（%）
	Comments7d  int64                            `json:"comments_7d"`  // 近 7 日评论
	CommentsTrend float64                        `json:"comments_trend"` // 环比（%）
	PostsToday  int64                            `json:"posts_today"`  // 今日新帖（已发布）
	PendingAudit int64                           `json:"pending_audit"` // 待审需处理（评论待审数，设计稿统计卡徽标）
	Trend       []repository.ReportTrendPoint    `json:"trend"`        // 四维趋势（7/30 日）
	TypeCounts  map[string]int64                 `json:"type_counts"`  // 内容分布（环形图）
	Activities  []repository.ActivityRow         `json:"activities"`   // 最近动态
	Pending     ReportPending                    `json:"pending"`      // 待处理块（评论/举报/敏感词）
}

// ReportPending 待处理块（设计稿：评论待审 N（处理）/ 内容举报 N（查看）/ 敏感词命中 N（复核））。
type ReportPending struct {
	Comments   int64 `json:"comments"`    // 评论待审（hidden）
	Reports    int64 `json:"reports"`     // 内容举报（pending 工单）
	Sensitive  int64 `json:"sensitive"`   // 敏感词命中（累计）
}

// ReportService 数据报表服务（连接器类）。
type ReportService struct {
	admin   *repository.AdminRepo  // 后台聚合（统计/趋势/待处理）
	reports *repository.ReportRepo // 举报工单（待处理）
}

// NewReportService 创建报表服务。
func NewReportService(admin *repository.AdminRepo, reports *repository.ReportRepo) *ReportService {
	return &ReportService{admin: admin, reports: reports}
}

// Overview 报表页聚合。
// 参数：days 趋势天数（7 或 30，其余回退 30）。
func (s *ReportService) Overview(ctx context.Context, days int) (*ReportOverview, error) {
	// 趋势天数规范化（7/30 之外按默认 30）
	if days != reportDaysMin {
		days = reportDaysMax
	}

	// 统计卡（口径与仪表盘一致：近 7 日 + 上 7 日环比）
	now := time.Now()
	weekAgo := now.Add(-7 * 24 * time.Hour)
	prevWeek := now.Add(-14 * 24 * time.Hour)

	views7d, _, err := s.admin.StatsSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	viewsPrev, _, err := s.admin.StatsSince(ctx, prevWeek)
	if err != nil {
		return nil, err
	}
	likes7d, err := s.admin.LikesSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	likesPrev, err := s.admin.LikesSince(ctx, prevWeek)
	if err != nil {
		return nil, err
	}
	comments7d, err := s.admin.CountCommentsSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	commentsPrev, err := s.admin.CountCommentsSince(ctx, prevWeek)
	if err != nil {
		return nil, err
	}

	// 今日新帖（已发布）
	postsToday, err := s.admin.CountTodayPosts(ctx)
	if err != nil {
		return nil, err
	}

	// 四维趋势 + 内容分布 + 最近动态
	trendSeries, err := s.admin.ReportTrendSeries(ctx, days)
	if err != nil {
		return nil, err
	}
	typeCounts, err := s.admin.CountPostsByType(ctx)
	if err != nil {
		return nil, err
	}
	activities, err := s.admin.RecentActivity(ctx)
	if err != nil {
		return nil, err
	}

	// 待处理块（评论待审/内容举报/敏感词命中）
	pending, err := s.pendingStats(ctx)
	if err != nil {
		return nil, err
	}

	return &ReportOverview{
		Views7d:       views7d,
		ViewsTrend:    trend(views7d, viewsPrev),
		Likes7d:       likes7d,
		LikesTrend:    trend(likes7d, likesPrev),
		Comments7d:    comments7d,
		CommentsTrend: trend(comments7d, commentsPrev),
		PostsToday:    postsToday,
		PendingAudit:  pending.Comments,
		Trend:         trendSeries,
		TypeCounts:    typeCounts,
		Activities:    activities,
		Pending:       pending,
	}, nil
}

// pendingStats 待处理统计（评论待审 hidden / 举报 pending / 敏感词命中合计）。
func (s *ReportService) pendingStats(ctx context.Context) (ReportPending, error) {
	comments, err := s.admin.CountHiddenComments(ctx)
	if err != nil {
		return ReportPending{}, err
	}
	reports, err := s.reports.CountPending(ctx)
	if err != nil {
		return ReportPending{}, err
	}
	sensitive, err := s.admin.TotalSensitiveHits(ctx)
	if err != nil {
		return ReportPending{}, err
	}
	return ReportPending{Comments: comments, Reports: reports, Sensitive: sensitive}, nil
}

// ExportTrendCSV 导出趋势 CSV（设计稿页头「导出 CSV」）。
// 说明：CSV 头部「日期,浏览,新帖,获赞,评论」，按当前视图天数导出。
func (s *ReportService) ExportTrendCSV(ctx context.Context, days int) ([]byte, error) {
	if days != reportDaysMin {
		days = reportDaysMax
	}
	trendSeries, err := s.admin.ReportTrendSeries(ctx, days)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	// 头部（UTF-8 BOM 便于 Excel 中文兼容）
	buf.WriteString("\xEF\xBB\xBF")
	if err := writer.Write([]string{"日期", "浏览", "新帖", "获赞", "评论"}); err != nil {
		return nil, err
	}
	for _, p := range trendSeries {
		if err := writer.Write([]string{
			p.Date,
			formatInt64(p.Views),
			formatInt64(p.Posts),
			formatInt64(p.Likes),
			formatInt64(p.Comments),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// formatInt64 数字转字符串（CSV 单元格）。
func formatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}
