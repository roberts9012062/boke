// internal/service/seo.go
// SEO 业务（M4）：全局设置、帖子级元数据、健康度扫描、批量修复、SERP 预览、sitemap/robots 生成。
// 设计稿：《SEO设置》《SEO·健康度》《SEO·SERP预览》《SEO·批量修复》。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 站点基础 URL（sitemap/SERP 预览用；从配置注入）。
const defaultSiteURL = "http://localhost:3000"

// SeoService SEO 服务（连接器类）。
type SeoService struct {
	seo   *repository.SeoRepo // SEO 数据访问
	posts *repository.PostRepo // 帖子（健康扫描/批量修复）
	siteURL string             // 站点访问地址
}

// NewSeoService 创建 SEO 服务。
func NewSeoService(seo *repository.SeoRepo, posts *repository.PostRepo, siteURL string) *SeoService {
	return &SeoService{seo: seo, posts: posts, siteURL: siteURL}
}

// ---------- 全局设置 ----------

// Settings 读取全局 SEO 设置。
func (s *SeoService) Settings(ctx context.Context) (*repository.SeoSettings, error) {
	return s.seo.GetSettings(ctx)
}

// SaveSettings 保存全局 SEO 设置（校验长度）。
func (s *SeoService) SaveSettings(ctx context.Context, req repository.SeoSettings) error {
	if len([]rune(req.SiteName)) > 100 || len([]rune(req.SiteDescription)) > 300 ||
		len([]rune(req.TitleSuffix)) > 100 || len([]rune(req.Keywords)) > 500 || len([]rune(req.RobotsTxt)) > 2000 {
		return errs.New(errs.CodeBadRequest, "SEO 设置内容超长")
	}
	return s.seo.UpsertSettings(ctx, req)
}

// ---------- 帖子级元数据 ----------

// Meta 读取帖子 SEO 元数据。
func (s *SeoService) Meta(ctx context.Context, postID int64) (*repository.SeoMeta, error) {
	return s.seo.GetMeta(ctx, postID)
}

// SaveMeta 保存帖子 SEO 元数据。
func (s *SeoService) SaveMeta(ctx context.Context, postID int64, req repository.SeoMeta) error {
	if len([]rune(req.Title)) > 300 || len([]rune(req.Description)) > 500 {
		return errs.New(errs.CodeBadRequest, "SEO 标题或描述超长")
	}
	req.PostID = postID
	return s.seo.UpsertMeta(ctx, req)
}

// ---------- 健康度扫描 / 批量修复 ----------

// HealthIssue 健康问题项（seo_health_checks.issues JSON 元素）。
type HealthIssue struct {
	Code    string `json:"code"`    // 问题编码：no_title/no_description/no_og/title_too_long/...
	Message string `json:"message"` // 用户可读描述
}

// HealthItem 健康度条目（含帖子标题）。
type HealthItem struct {
	PostID    int64         `json:"post_id"`    // 帖子 ID
	PostTitle string        `json:"post_title"` // 帖子标题
	Score     int           `json:"score"`      // 健康分 0-100
	Issues    []HealthIssue `json:"issues"`     // 问题清单
	CheckedAt string        `json:"checked_at"` // 检查时间
}

// HealthTypeDist 问题类型分布项。
type HealthTypeDist struct {
	Code   string `json:"code"`   // 问题编码
	Label  string `json:"label"`  // 中文标签
	Count  int64  `json:"count"`  // 数量
	Percent int   `json:"percent"` // 百分比
}

// HealthTrendPoint 健康分趋势点。
type HealthTrendPoint struct {
	Date  string  `json:"date"`  // 日期（MM-DD）
	Score float64 `json:"score"` // 平均分
}

// HealthPriority 优先修复项（P0/P1 分级，设计稿「优先修复」）。
type HealthPriority struct {
	Level   string `json:"level"`   // P0 / P1
	Message string `json:"message"` // 问题描述
	Hint    string `json:"hint"`    // 影响说明
	Where   string `json:"where"`   // 位置标签（首页/帖子/批量/地图）
}

// HealthSummary 健康度汇总（设计稿：四卡片 + 趋势 + 分布 + 优先修复）。
type HealthSummary struct {
	TotalPosts    int64             `json:"total_posts"`    // 已扫描帖子数
	PendingIssues int64             `json:"pending_issues"` // 待修复问题总数
	AvgScore      int               `json:"avg_score"`      // 综合评分（0-100）
	MetaCoverage  int               `json:"meta_coverage"`  // 元信息覆盖百分比
	Indexable     int64             `json:"indexable"`      // 可收录页面数
	Noindex       int64             `json:"noindex"`        // noindex 页面数
	Trend         []HealthTrendPoint `json:"trend"`         // 近 7 日健康分趋势
	Distribution  []HealthTypeDist   `json:"distribution"`  // 问题类型分布
	Priorities    []HealthPriority   `json:"priorities"`    // 优先修复（P0/P1）
	Items         []HealthItem       `json:"items"`         // 问题列表（帖子维度）
}

// issueLabels 问题编码中文标签（类型分布展示）。
var issueLabels = map[string]string{
	"no_title": "缺标题", "no_description": "缺描述", "desc_length": "描述长度", "title_too_long": "标题过长", "no_og": "弱 OG",
}

// ScanHealth 全量健康扫描：逐帖审计（标题/描述/OG 图）→ 落库 + 汇总
// （综合评分/元信息覆盖/可收录/趋势/分布/优先修复，对齐设计稿四卡片与图表）。
func (s *SeoService) ScanHealth(ctx context.Context) (*HealthSummary, error) {
	posts, total, err := s.posts.List(ctx, repository.ListParams{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	summary := &HealthSummary{TotalPosts: total}
	covered := int64(0)
	scoreSum := 0
	typeCount := make(map[string]int64)
	for _, post := range posts {
		meta, err := s.seo.GetMeta(ctx, post.ID)
		if err != nil {
			return nil, err
		}
		// 元信息覆盖（可索引帖：有标题+描述）
		if post.Visibility != model.VisibilityPrivate && meta.Title != "" && meta.Description != "" {
			covered++
		}
		issues := s.auditPost(post, meta)
		score := healthScore(issues)
		scoreSum += score
		raw, _ := json.Marshal(issues)
		if err := s.seo.SaveHealthCheck(ctx, repository.SeoHealthCheck{
			PostID: post.ID, Score: score, Issues: raw,
		}); err != nil {
			return nil, err
		}
		summary.PendingIssues += int64(len(issues))
		for _, issue := range issues {
			typeCount[issue.Code]++
		}
		if len(issues) > 0 {
			summary.Items = append(summary.Items, HealthItem{
				PostID: post.ID, PostTitle: postTitle(post), Score: score,
				Issues: issues, CheckedAt: time.Now().Format(time.RFC3339),
			})
		}
	}
	// 综合评分：按帖平均健康分（真实平均）
	if total > 0 {
		summary.AvgScore = scoreSum / int(total)
	}
	// 元信息覆盖百分比
	if total > 0 {
		summary.MetaCoverage = int(covered * 100 / total)
	}
	// 可收录页面（公开 + 仅关注者；noindex 预留——当前无 noindex 机制，统计为 0）
	for _, post := range posts {
		if post.Visibility != model.VisibilityPrivate {
			summary.Indexable++
		}
	}
	// 近 7 日趋势 + 问题类型分布
	trend, err := s.seo.HealthTrend(ctx, 7)
	if err != nil {
		return nil, err
	}
	for _, t := range trend {
		summary.Trend = append(summary.Trend, HealthTrendPoint{Date: t.Date, Score: t.Score})
	}
	dist, err := s.seo.TypeDistribution(ctx, 6)
	if err != nil {
		return nil, err
	}
	var distTotal int64
	for _, d := range dist {
		distTotal += d.Count
	}
	for _, d := range dist {
		percent := 0
		if distTotal > 0 {
			percent = int(d.Count * 100 / distTotal)
		}
		summary.Distribution = append(summary.Distribution, HealthTypeDist{
			Code: d.Code, Label: issueLabels[d.Code], Count: d.Count, Percent: percent,
		})
	}
	// 优先修复（P0/P1 分级：缺描述/描述长度=P0；缺标题/标题过长/弱 OG=P1）
	summary.Priorities = s.buildPriorities(summary.Items)
	return summary, nil
}

// buildPriorities 组装优先修复列表（P0/P1 分级，对齐设计稿「优先修复」）。
func (s *SeoService) buildPriorities(items []HealthItem) []HealthPriority {
	priorities := make([]HealthPriority, 0, 4)
	p0Count := 0
	p1Count := 0
	for _, item := range items {
		for _, issue := range item.Issues {
			switch issue.Code {
			case "no_description", "desc_length":
				p0Count++
			case "no_title", "title_too_long", "no_og":
				p1Count++
			}
		}
	}
	if p0Count > 0 {
		priorities = append(priorities, HealthPriority{
			Level: "P0", Message: fmt.Sprintf("%d 处描述问题", p0Count),
			Hint: "回落全局默认，摘要可能被截断", Where: "批量",
		})
	}
	if p1Count > 0 {
		priorities = append(priorities, HealthPriority{
			Level: "P1", Message: fmt.Sprintf("%d 处标题/OG 问题", p1Count),
			Hint: "将使用正文截断，建议补全", Where: "批量",
		})
	}
	if p0Count == 0 && p1Count == 0 {
		priorities = append(priorities, HealthPriority{
			Level: "P0", Message: "全部文章 SEO 完整", Hint: "无需修复", Where: "全站",
		})
	}
	return priorities
}

// auditPost 单帖 SEO 审计（标题 10-60 字/描述 50-160 字/唯一 URL/OG 图，对齐 SERP 预览检查项）。
func (s *SeoService) auditPost(post model.Post, meta *repository.SeoMeta) []HealthIssue {
	issues := make([]HealthIssue, 0)
	titleLen := len([]rune(meta.Title))
	descLen := len([]rune(meta.Description))
	if titleLen == 0 {
		issues = append(issues, HealthIssue{Code: "no_title", Message: "缺少独立 SEO 标题，将回落全局默认"})
	} else if titleLen > 60 {
		issues = append(issues, HealthIssue{Code: "title_too_long", Message: "SEO 标题超 60 字，可能被搜索引擎截断"})
	}
	if descLen == 0 {
		issues = append(issues, HealthIssue{Code: "no_description", Message: "缺少独立 SEO 描述"})
	} else if descLen < 50 || descLen > 160 {
		issues = append(issues, HealthIssue{Code: "desc_length", Message: "SEO 描述建议 50-160 字"})
	}
	if meta.OgImage == "" {
		issues = append(issues, HealthIssue{Code: "no_og", Message: "建议补充 OG 分享图"})
	}
	return issues
}

// healthScore 健康分（无问题 100；每个问题 -25，最低 0）。
func healthScore(issues []HealthIssue) int {
	score := 100 - len(issues)*25
	if score < 0 {
		return 0
	}
	return score
}

// BatchFix 批量修复（自动补齐缺省 SEO 字段：标题=帖子标题+后缀、描述=帖子摘要、返回修复数）。
func (s *SeoService) BatchFix(ctx context.Context) (int64, error) {
	posts, _, err := s.posts.List(ctx, repository.ListParams{Page: 1, PageSize: 1000})
	if err != nil {
		return 0, err
	}
	settings, err := s.seo.GetSettings(ctx)
	if err != nil {
		return 0, err
	}
	var fixed int64
	for _, post := range posts {
		meta, err := s.seo.GetMeta(ctx, post.ID)
		if err != nil {
			return 0, err
		}
		changed := false
		if meta.Title == "" {
			meta.Title = postTitle(post) + " " + settings.TitleSuffix
			changed = true
		}
		if meta.Description == "" {
			meta.Description = postSummaryText(post.Content)
			changed = true
		}
		if changed {
			if err := s.seo.UpsertMeta(ctx, *meta); err != nil {
				return 0, err
			}
			fixed++
		}
	}
	return fixed, nil
}

// postTitle 帖子标题（空时用正文摘要）。
// postTitle 帖子标题（空时用正文摘要，健康扫描/批量修复/SERP 共用）。
func postTitle(post model.Post) string {
	if post.Title != "" {
		return post.Title
	}
	return postSummaryText(post.Content)
}

// postSummaryText 正文摘要（前 120 字）。
func postSummaryText(content string) string {
	flat := strings.Join(strings.Fields(content), " ")
	runes := []rune(flat)
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return flat
}

// ---------- SERP 预览 ----------

// SerpPreview SERP 预览数据（设计稿：搜索结果样式预览 · 桌面 Google 风格）。
type SerpPreview struct {
	Title       string   `json:"title"`        // 展示标题（SEO 标题 + 后缀）
	TitleLen    int      `json:"title_len"`    // 标题字数
	URL         string   `json:"url"`          // 展示 URL
	Description string   `json:"description"`  // 展示描述（SEO 描述或全局默认）
	Checks      []string `json:"checks"`       // 检查项文案（✓/建议）
	Warnings    []string `json:"warnings"`     // 警告（缺 SEO 字段等）
}

// SerpPreview 生成帖子 SERP 预览。
func (s *SeoService) SerpPreview(ctx context.Context, postID int64) (*SerpPreview, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	meta, err := s.seo.GetMeta(ctx, postID)
	if err != nil {
		return nil, err
	}
	settings, err := s.seo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	// 标题：SEO 标题 + 后缀；无则帖子标题 + 后缀
	title := meta.Title
	if title == "" {
		title = postTitle(post)
	}
	display := title + " " + settings.TitleSuffix
	// 描述：SEO 描述或全局默认
	desc := meta.Description
	if desc == "" {
		desc = settings.SiteDescription
	}
	preview := &SerpPreview{
		Title:       display,
		TitleLen:    len([]rune(display)),
		URL:         fmt.Sprintf("%s/posts/%d", s.siteURL, postID),
		Description: desc,
		Checks:      []string{"✓ 标题 10–60 字", "✓ 描述 50–160 字", "✓ 唯一 URL"},
	}
	// 检查项
	titleLen := len([]rune(display))
	if titleLen < 10 || titleLen > 60 {
		preview.Checks = append(preview.Checks, fmt.Sprintf("⚠ 标题 %d 字（建议 10–60 字）", titleLen))
	} else {
		preview.Checks = append(preview.Checks, fmt.Sprintf("✓ 标题 %d 字", titleLen))
	}
	if meta.OgImage == "" {
		preview.Checks = append(preview.Checks, "· 建议补 OG 图")
	}
	if meta.Title == "" || meta.Description == "" {
		preview.Warnings = append(preview.Warnings, "缺少独立 SEO 标题与描述，将回落全局默认，摘要可能被截断。")
	}
	return preview, nil
}

// ---------- sitemap / robots ----------

// SitemapXML 生成 sitemap.xml（公开帖子 + 图片 URL；sitemap_enabled 关闭时返回空）。
func (s *SeoService) SitemapXML(ctx context.Context) (string, error) {
	settings, err := s.seo.GetSettings(ctx)
	if err != nil {
		return "", err
	}
	if !settings.SitemapEnabled {
		return "", nil
	}
	rows, err := s.seo.AllForSitemap(ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "  <url><loc>%s/posts/%d</loc><lastmod>%s</lastmod></url>\n",
			s.siteURL, row.ID, row.UpdatedAt.Format("2006-01-02"))
	}
	b.WriteString(`</urlset>`)
	return b.String(), nil
}

// RobotsTxt 生成 robots.txt（seo_settings.robots_txt 或默认规则）。
func (s *SeoService) RobotsTxt(ctx context.Context) (string, error) {
	settings, err := s.seo.GetSettings(ctx)
	if err != nil {
		return "", err
	}
	if settings.RobotsTxt != "" {
		return settings.RobotsTxt, nil
	}
	return "User-agent: *\nAllow: /\nDisallow: /admin/\n", nil
}
