// internal/service/page.go
// 自定义页面业务：请求校验（slug 格式/标题/状态枚举/长度上限）+ 增删改查编排。
// 前台仅暴露已发布页面（草稿视同不存在）；slug 全局唯一由本层校验。
package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// toBizErr 仓库错误转业务错误（记录不存在 → 404 语义；其余原样透传给 handler 记日志）。
func toBizErr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return errs.ErrNotFound
	}
	return err
}

// 页面字段长度上限（与迁移 018 的表结构对齐）。
const (
	maxPageTitleLen    = 200       // 标题 ≤200 字符
	maxPageDescLen     = 500       // SEO 描述 ≤500 字符
	maxPageSlugLen     = 100       // slug ≤100 字符
	maxPageContentByte = 200 * 1024 // 正文 ≤200KB（富文本 HTML 含标签）
)

// pageSlugPattern slug 格式：小写字母/数字开头结尾，中间可含连字符（如 about-me、links2）。
var pageSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// PageService 自定义页面业务（连接器类）。
type PageService struct {
	pages *repository.PageRepo // 页面数据访问
}

// NewPageService 创建自定义页面服务。
func NewPageService(pages *repository.PageRepo) *PageService {
	return &PageService{pages: pages}
}

// normalizePageInput 归一化并校验页面输入（创建/更新共用；纯函数，仅返回归一化结果与错误）。
// 参数：slug/title/content/contentFormat/description/status 原始输入。
// 返回：归一化后的各字段与校验错误（不满足规则时返回 errs 校验错误）。
func normalizePageInput(slug string, title string, content string, contentFormat string, description string, status string) (string, string, string, string, string, string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)

	// slug 格式校验（小写字母/数字/连字符，作为 URL 路径段安全）
	if slug == "" || len(slug) > maxPageSlugLen || !pageSlugPattern.MatchString(slug) {
		return "", "", "", "", "", "", errs.New(errs.CodeValidation,
			"路由标识需为小写字母/数字/连字符组合（如 about-me），长度 1-100")
	}
	if title == "" || len(title) > maxPageTitleLen {
		return "", "", "", "", "", "", errs.New(errs.CodeValidation, "页面标题不能为空，且不超过 200 字符")
	}
	if len(content) > maxPageContentByte {
		return "", "", "", "", "", "", errs.New(errs.CodeValidation, "页面内容过大（上限 200KB）")
	}
	if len(description) > maxPageDescLen {
		return "", "", "", "", "", "", errs.New(errs.CodeValidation, "SEO 描述不超过 500 字符")
	}

	// 正文格式：默认 html（编辑器产物），接受 html/markdown/page 三种
	//（page = AI 构建器生成的完整 HTML 文档，前台沙箱 iframe 整页渲染）
	contentFormat = strings.ToLower(strings.TrimSpace(contentFormat))
	if contentFormat != model.PageFormatHTML &&
		contentFormat != model.PageFormatMarkdown &&
		contentFormat != model.PageFormatPage {
		contentFormat = model.PageFormatHTML
	}

	// 状态：默认草稿，仅接受 draft/published 两种
	if status != model.PageStatusDraft && status != model.PageStatusPublished {
		status = model.PageStatusDraft
	}
	return slug, title, content, contentFormat, description, status, nil
}

// List 后台页面列表（含草稿，转列表 DTO 轻量化输出）。
func (s *PageService) List(ctx context.Context) ([]model.AdminPageItem, error) {
	pages, err := s.pages.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]model.AdminPageItem, 0, len(pages))
	for _, p := range pages {
		items = append(items, model.AdminPageItem{
			ID:          p.ID,
			Slug:        p.Slug,
			Title:       p.Title,
			Status:      p.Status,
			Description: p.Description,
			UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
			CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		})
	}
	return items, nil
}

// GetByID 后台编辑回显（返回完整实体，含正文）。
func (s *PageService) GetByID(ctx context.Context, id int64) (model.CustomPage, error) {
	p, err := s.pages.GetByID(ctx, id)
	if err != nil {
		return model.CustomPage{}, toBizErr(err)
	}
	return p, nil
}

// GetBySlug 前台按 slug 取已发布页面（草稿/不存在统一返回 404，不泄露存在性）。
func (s *PageService) GetBySlug(ctx context.Context, slug string) (model.PageDetail, error) {
	p, err := s.pages.GetBySlug(ctx, strings.ToLower(strings.TrimSpace(slug)), true)
	if err != nil {
		return model.PageDetail{}, toBizErr(err)
	}
	return model.PageDetail{
		Slug:          p.Slug,
		Title:         p.Title,
		Content:       p.Content,
		ContentFormat: p.ContentFormat,
		Description:   p.Description,
		UpdatedAt:     p.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// Create 创建页面（slug 唯一性校验 + 归一化校验）。
// 返回：新页面 ID。
func (s *PageService) Create(ctx context.Context, req model.CreatePageReq) (int64, error) {
	slug, title, content, format, description, status, err := normalizePageInput(
		req.Slug, req.Title, req.Content, req.ContentFormat, req.Description, req.Status)
	if err != nil {
		return 0, err
	}
	// slug 唯一性（创建场景 excludeID=0 即不排除任何记录）
	exists, err := s.pages.SlugExists(ctx, slug, 0)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errs.New(errs.CodeConflict, "路由标识已被占用："+slug)
	}
	return s.pages.Create(ctx, model.CustomPage{
		Slug:          slug,
		Title:         title,
		Content:       content,
		ContentFormat: format,
		Description:   description,
		Status:        status,
	})
}

// Update 更新页面（slug 变更时校验未被其他页面占用）。
func (s *PageService) Update(ctx context.Context, id int64, req model.UpdatePageReq) error {
	// 先确认页面存在（不存在直接 404，避免对不存在 ID 执行更新）
	if _, err := s.pages.GetByID(ctx, id); err != nil {
		return toBizErr(err)
	}
	slug, title, content, format, description, status, err := normalizePageInput(
		req.Slug, req.Title, req.Content, req.ContentFormat, req.Description, req.Status)
	if err != nil {
		return err
	}
	exists, err := s.pages.SlugExists(ctx, slug, id)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeConflict, "路由标识已被占用："+slug)
	}
	return s.pages.Update(ctx, id, model.CustomPage{
		Slug:          slug,
		Title:         title,
		Content:       content,
		ContentFormat: format,
		Description:   description,
		Status:        status,
	})
}

// Delete 删除页面。
func (s *PageService) Delete(ctx context.Context, id int64) error {
	// 先确认存在（统一 404 语义）
	if _, err := s.pages.GetByID(ctx, id); err != nil {
		return toBizErr(err)
	}
	return s.pages.Delete(ctx, id)
}
