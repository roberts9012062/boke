// internal/model/page.go
// 自定义页面数据模型：CustomPage 实体 + 管理/前台 DTO（与前端 src/lib/api-pages.ts 手工同步）。
// 说明：自定义页面是后台创建的独立内容页（关于页/友链页等），
// 前台经 /pages/{slug} 访问；slug 全局唯一，由管理员自定义。
package model

import "time"

// 自定义页面状态。
const (
	PageStatusDraft     = "draft"     // 草稿（前台不可见）
	PageStatusPublished = "published" // 已发布（前台可见）
)

// 自定义页面正文格式（html/markdown 与帖子约定一致；page 为 AI 构建器产出的独立整页文档）。
const (
	PageFormatHTML     = "html"     // 富文本 HTML（Tiptap 编辑器产物，正文片段）
	PageFormatMarkdown = "markdown" // Markdown（预留）
	PageFormatPage     = "page"     // AI 生成的完整 HTML 文档（前台沙箱 iframe 整页渲染）
)

// ---------- 实体 ----------

// CustomPage 自定义页面实体（custom_pages 表结构）。
type CustomPage struct {
	ID            int64     `json:"id"`             // 页面 ID
	Slug          string    `json:"slug"`           // 路由标识（前台访问 /pages/{slug}）
	Title         string    `json:"title"`          // 页面标题
	Content       string    `json:"content"`        // 正文（富文本 HTML）
	ContentFormat string    `json:"content_format"` // 正文格式：html / markdown
	Description   string    `json:"description"`    // SEO 描述
	Status        string    `json:"status"`         // 状态：draft / published
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time `json:"updated_at"`     // 更新时间
}

// ---------- DTO ----------

// CreatePageReq 创建页面请求（后台）。
type CreatePageReq struct {
	Slug          string `json:"slug"`           // 路由标识（小写字母/数字/连字符，≤100 字符）
	Title         string `json:"title"`          // 标题（非空，≤200 字符）
	Content       string `json:"content"`        // 正文（富文本 HTML，≤200KB）
	ContentFormat string `json:"content_format"` // 正文格式（空=html）
	Description   string `json:"description"`    // SEO 描述（≤500 字符）
	Status        string `json:"status"`         // 状态（空=draft）
}

// UpdatePageReq 更新页面请求（后台；除 slug 外全量覆盖更新）。
type UpdatePageReq struct {
	Slug          string `json:"slug"`           // 路由标识（允许修改，仍需唯一）
	Title         string `json:"title"`          // 标题
	Content       string `json:"content"`        // 正文
	ContentFormat string `json:"content_format"` // 正文格式（空=html）
	Description   string `json:"description"`    // SEO 描述
	Status        string `json:"status"`         // 状态
}

// AdminPageItem 后台页面列表项（不含正文，列表轻量化）。
type AdminPageItem struct {
	ID          int64  `json:"id"`           // 页面 ID
	Slug        string `json:"slug"`         // 路由标识
	Title       string `json:"title"`        // 标题
	Status      string `json:"status"`       // 状态：draft / published
	Description string `json:"description"`  // SEO 描述
	UpdatedAt   string `json:"updated_at"`   // 更新时间（ISO8601）
	CreatedAt   string `json:"created_at"`   // 创建时间（ISO8601）
}

// PageDetail 前台页面详情（仅已发布页面返回）。
type PageDetail struct {
	Slug          string `json:"slug"`           // 路由标识
	Title         string `json:"title"`          // 标题
	Content       string `json:"content"`        // 正文
	ContentFormat string `json:"content_format"` // 正文格式：html / markdown
	Description   string `json:"description"`    // SEO 描述
	UpdatedAt     string `json:"updated_at"`     // 更新时间（ISO8601）
}
