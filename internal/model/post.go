// internal/model/post.go
// 帖子模块数据模型：Post 实体 + 发帖/列表/详情 DTO（与前端 src/types 手工同步）。
package model

import (
	"encoding/json"
	"time"
)

// 帖子内容类型（设计稿四形态：文字/图片/音频/视频）。
const (
	PostTypeText  = "text"  // 文字帖
	PostTypeImage = "image" // 图片帖
	PostTypeAudio = "audio" // 音频帖
	PostTypeVideo = "video" // 视频帖（M2 启用）
)

// 帖子状态（附录 B 状态字典）。
const (
	PostStatusDraft     = "draft"     // 草稿
	PostStatusPublished = "published" // 已发布
	PostStatusTakenDown = "taken_down" // 已下架
	PostStatusDeleted   = "deleted"   // 已删除（软删）
)

// 可见性（需求 3.4 + 设计稿《可见性》弹层：公开/仅关注者/仅自己；密码帖设计稿未覆盖，后置）。
const (
	VisibilityPublic    = "public"    // 公开：所有人可见，可被推荐
	VisibilityFollowers = "followers" // 仅关注者：互相关注的人可见
	VisibilityPrivate   = "private"   // 私密：仅自己可见（草稿箱式）
)

// ---------- 实体 ----------

// Post 帖子实体（posts 表结构）。
// 说明：json tag 供后台管理列表直接序列化（前台用 PostSummary/PostDetail DTO）。
type Post struct {
	ID            int64      `json:"id"`             // 帖子 ID
	AuthorID      int64      `json:"author_id"`      // 作者 ID
	Title         string     `json:"title"`          // 标题（可空）
	Summary       string     `json:"summary"`        // 摘要（自动生成）
	Content       string     `json:"content"`        // 正文（≤2000 字）
	ContentFormat string     `json:"content_format"` // 正文格式：markdown / html（空=markdown）
	ContentType   string     `json:"content_type"`   // 内容类型
	Status        string     `json:"status"`         // 状态
	Visibility    string     `json:"visibility"`     // 可见性
	CoverURL      string     `json:"cover_url"`      // 封面图
	GalleryStyle  string     `json:"gallery_style"`  // 图片展示风格（grid/carousel/flip/stack/masonry/polaroid；空=网格）
	MediaIDs      []int64    `json:"media_ids"`      // 关联媒体 ID
	ViewCount     int64      `json:"view_count"`     // 浏览量
	LikeCount     int64      `json:"like_count"`     // 点赞数
	CommentCount  int64      `json:"comment_count"`  // 评论数
	PublishedAt   *time.Time `json:"published_at"`   // 发布时间
	CreatedAt     time.Time  `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time  `json:"updated_at"`     // 更新时间
}

// ---------- DTO ----------

// PostSeoInput 发帖/编辑时的 SEO 输入（M4.1 插件通道：前台发帖 SEO 面板由插件渲染，
// 值随请求提交，主进程写入 seo_meta；无插件时字段为空不落库）。
type PostSeoInput struct {
	SEOTitle       string `json:"seo_title"`       // SEO 标题（默认用正文摘要）
	SEODescription string `json:"seo_description"` // SEO 描述
	URLAlias       string `json:"url_alias"`       // URL 别名（/p/{alias} 短链）
	Robots         string `json:"robots"`          // 收录策略（index,follow 等；空=跟随全局）
}

// CreatePostReq 发帖/存草稿请求（需求 3.4）。
type CreatePostReq struct {
	ContentType   string        `json:"content_type"`   // 内容类型：text/image/audio（video M2 置灰）
	Title         string        `json:"title"`          // 标题（可选）
	Content       string        `json:"content"`        // 正文（≤2000 字）
	ContentFormat string        `json:"content_format"` // 正文格式：markdown / html（空=markdown）
	Tags          []string      `json:"tags"`           // 标签（≤5 个，每个 ≤20 字符）
	MediaIDs      []int64       `json:"media_ids"`      // 关联媒体 ID（图片多张有序/音频一张）
	GalleryStyle  string        `json:"gallery_style"`  // 图片展示风格（空=默认网格）
	Visibility    string        `json:"visibility"`     // 可见性：public/private
	Status        string        `json:"status"`         // draft=存草稿 / published=发布
	Seo           *PostSeoInput `json:"seo,omitempty"`  // SEO 输入（插件面板提交；nil=不写 seo_meta）
}

// UpdatePostReq 更新帖子请求（草稿继续编辑/已发布编辑）。
type UpdatePostReq struct {
	Title         *string       `json:"title"`          // 标题（nil 表示不修改）
	Content       *string       `json:"content"`        // 正文
	ContentFormat *string       `json:"content_format"` // 正文格式（nil=不修改）
	Tags          []string      `json:"tags"`           // 标签（空数组=清空，nil=不修改）
	MediaIDs      []int64       `json:"media_ids"`      // 媒体（空数组=清空，nil=不修改）
	GalleryStyle  *string       `json:"gallery_style"`  // 图片展示风格（nil=不修改）
	Visibility    *string       `json:"visibility"`     // 可见性
	Seo           *PostSeoInput `json:"seo,omitempty"`  // SEO 输入（插件面板提交；nil=不更新 seo_meta）
}

// TagDTO 标签信息（列表/详情展示）。
type TagDTO struct {
	Name string `json:"name"` // 标签名（含 # 前缀）
	Slug string `json:"slug"` // URL 别名
}

// AuthorDTO 作者信息（列表/详情展示）。
type AuthorDTO struct {
	ID        int64  `json:"id"`         // 用户 ID
	Username  string `json:"username"`   // 账号名
	Nickname  string `json:"nickname"`   // 昵称
	AvatarURL string `json:"avatar_url"` // 头像地址
}

// MediaDTO 媒体信息（列表/详情展示）。
type MediaDTO struct {
	ID       int64  `json:"id"`        // 媒体 ID
	Type     string `json:"type"`      // 类型：image/audio
	URL      string `json:"url"`       // 访问地址
	MimeType string `json:"mime_type"` // MIME 类型
	SizeBytes int64 `json:"size_bytes"` // 文件大小
	Width    int    `json:"width"`     // 宽（图片）
	Height   int    `json:"height"`    // 高（图片）
}

// MusicEmbedDTO 音乐嵌入信息（列表卡片直接渲染迷你播放器；M7 音乐嵌入增强）。
// 两种形态：
//   - 第三方 iframe（QQ 音乐/旧网易云）：Platform/Kind/URL 有值
//   - 网易云歌曲引用（自研播放器）：Platform=netease + SongID 有值（URL 为空，播放地址实时经插件获取）
type MusicEmbedDTO struct {
	Platform string `json:"platform"`          // 平台：qq / netease
	Kind     string `json:"kind"`              // 类型：song / playlist / album
	URL      string `json:"url,omitempty"`     // 播放器 iframe src（第三方 iframe 形态）
	SongID   string `json:"song_id,omitempty"` // 网易云歌曲 ID（自研播放器形态）
	Title    string `json:"title,omitempty"`   // 歌名
	Artist   string `json:"artist,omitempty"`  // 歌手
	CoverURL string `json:"cover_url,omitempty"` // 封面
}

// PostSummary 帖子摘要（时间线列表项）。
type PostSummary struct {
	ID           int64     `json:"id"`            // 帖子 ID
	Title        string    `json:"title"`         // 标题
	Summary      string    `json:"summary"`       // 摘要（正文截断）
	ContentType  string    `json:"content_type"`  // 类型
	Visibility   string    `json:"visibility"`    // 可见性
	Author       AuthorDTO `json:"author"`        // 作者
	Tags         []TagDTO  `json:"tags"`          // 标签
	Media        []MediaDTO `json:"media"`        // 媒体预览（图片封面/音频）
	GalleryStyle string     `json:"gallery_style"` // 图片展示风格（图片帖）
	Music        *MusicEmbedDTO `json:"music,omitempty"` // 音乐嵌入（正文内嵌 QQ/网易云，列表渲染迷你播放器）
	Bilibili     json.RawMessage `json:"bilibili,omitempty"` // B站视频块参数（data-props JSON，列表经插件内容块渲染播放器）
	LikeCount    int64     `json:"like_count"`    // 点赞数
	CommentCount int64     `json:"comment_count"` // 评论数
	ViewCount    int64     `json:"view_count"`    // 浏览量
	FavoriteCount int64    `json:"favorite_count"` // 收藏数（M1.7：列表聚合补齐）
	FavoritedAt  string    `json:"favorited_at,omitempty"` // 收藏时间（仅「我的收藏」列表返回）
	PublishedAt  string    `json:"published_at"`  // 发布时间（ISO8601）
}

// PostDetail 帖子详情（详情页）。
type PostDetail struct {
	PostSummary
	Content       string         `json:"content"`        // 完整正文
	ContentFormat string         `json:"content_format"` // 正文格式：markdown / html
	IsAuthor      bool           `json:"is_author"`      // 是否作者本人（编辑/删除权限）
	CanView       bool           `json:"can_view"`       // 当前用户是否有权查看（私密帖校验）
	Seo           *PostSeoOutput `json:"seo,omitempty"`  // SEO 输出（M4.1 插件通道：标题/描述/收录策略）
}

// PostSeoOutput 详情页 SEO 输出（robots 收录策略 / 自定义标题描述；无记录为 nil）。
type PostSeoOutput struct {
	Title       string `json:"title"`       // SEO 标题（空=用默认）
	Description string `json:"description"` // SEO 描述
	URLAlias    string `json:"url_alias"`   // URL 别名（编辑回填）
	Robots      string `json:"robots"`      // 收录策略（index,follow 等；空=跟随全局）
}

// AdminPostDetail 后台编辑帖子详情（设计稿《后台编辑·文字/图片/音频/视频》）。
// 包含发布信息（类型/状态/可见性/创建/更新）、互动数据（赞/评/览）与操作所需字段。
type AdminPostDetail struct {
	ID            int64      `json:"id"`             // 帖子 ID
	Title         string     `json:"title"`          // 标题
	Content       string     `json:"content"`        // 正文（文字帖/图说/说明）
	ContentFormat string     `json:"content_format"` // 正文格式：markdown / html
	ContentType   string     `json:"content_type"`   // 内容类型：text/image/audio/video
	Status        string     `json:"status"`         // 状态：draft/published/taken_down
	Visibility    string     `json:"visibility"`     // 可见性：public/followers/private
	CoverURL      string     `json:"cover_url"`      // 封面图（视频帖独立封面）
	Tags          []string   `json:"tags"`           // 标签名（不带 #，表单直接编辑）
	Media         []MediaDTO `json:"media"`          // 媒体列表（按 media_ids 顺序）
	GalleryStyle  string     `json:"gallery_style"`  // 图片展示风格
	ViewCount     int64      `json:"view_count"`     // 浏览量（互动数据·览）
	LikeCount     int64      `json:"like_count"`     // 点赞数（互动数据·赞）
	CommentCount  int64      `json:"comment_count"`  // 评论数（互动数据·评）
	Author        AuthorDTO  `json:"author"`         // 作者（发布信息）
	CreatedAt     string     `json:"created_at"`     // 创建时间（发布信息·创建）
	UpdatedAt     string     `json:"updated_at"`     // 更新时间（发布信息·更新）
	PublishedAt   string     `json:"published_at"`   // 发布时间（空串 = 未发布）
}

// AdminUpdatePostReq 后台更新帖子请求（设计稿：保存草稿 / 更新发布）。
// 说明：内容类型不允许修改（发布信息面板只读）；状态字段决定按钮语义。
type AdminUpdatePostReq struct {
	Title         string   `json:"title"`          // 标题（≤100 字符）
	Content       string   `json:"content"`        // 正文（≤2000 字）
	ContentFormat string   `json:"content_format"` // 正文格式：markdown / html（空=markdown）
	Tags          []string `json:"tags"`           // 标签（≤5 个，每个 ≤20 字符）
	MediaIDs      []int64  `json:"media_ids"`      // 关联媒体 ID（图片多张有序/音频或视频一张）
	Visibility    string   `json:"visibility"`     // 可见性：public/followers/private
	CoverURL      *string  `json:"cover_url"`      // 封面图（视频帖「更换封面」；nil=按类型推断保留）
	Status        string   `json:"status"`         // draft=保存草稿 / published=更新发布
}

// UploadResult 媒体上传结果。
type UploadResult struct {
	ID       int64  `json:"id"`        // 媒体 ID
	Type     string `json:"type"`      // 类型：image/audio
	URL      string `json:"url"`       // 访问地址
	MimeType string `json:"mime_type"` // MIME 类型
	SizeBytes int64 `json:"size_bytes"` // 文件大小
}
