// internal/model/openapi.go
// 接口开放模块数据模型：API Key 实体 + 开放接口目录（与前端 api-openapi.ts 手工同步）。
// 说明：目录（catalog）是单一数据源——后台「接口开放」页面展示、/open 网关鉴权、
//       AI 开发手册生成均基于同一份目录数据。
package model

import "time"

// ---------- 实体 ----------

// OpenAPIKey 开放接口调用凭证（open_api_keys 表结构）。
type OpenAPIKey struct {
	ID         int64      `json:"id"`          // 凭证 ID
	Name       string     `json:"name"`        // 备注名（可选）
	Key        string     `json:"key"`         // API Key（oa_ 前缀明文，后台可见便于复制）
	Endpoints  []string   `json:"endpoints"`   // 已授权接口标识数组（与目录 CatalogEntry.Endpoint 对应）
	UserID     int64      `json:"user_id"`     // 绑定用户 ID（0=未绑定；/open/me 资料来源）
	ExpiresAt  *time.Time `json:"expires_at"`  // 过期时间（NULL=永久有效）
	LastUsedAt *time.Time `json:"last_used_at"` // 最近调用时间（NULL=从未调用）
	CreatedAt  time.Time  `json:"created_at"`  // 创建时间
}

// ---------- 目录（catalog） ----------

// CatalogParam 开放接口的参数说明（后台展示与 AI 手册生成用）。
type CatalogParam struct {
	Name        string `json:"name"`        // 参数名（query / path / body 字段）
	Type        string `json:"type"`        // 类型：string / integer / array / object
	Location    string `json:"location"`    // 位置：query / path / body
	Required    bool   `json:"required"`    // 是否必填
	Description string `json:"description"` // 参数说明
}

// CatalogEntry 开放接口目录项（一个可被授权给 Key 的接口）。
type CatalogEntry struct {
	Endpoint    string         `json:"endpoint"`             // 接口标识（唯一，Key 绑定用）
	Method      string         `json:"method"`               // HTTP 方法（GET / POST）
	Path        string         `json:"path"`                 // 开放网关路径（/api/v1/open/...，含路由参数）
	Name        string         `json:"name"`                 // 接口名称（中文）
	Description string         `json:"description"`          // 功能描述
	Params      []CatalogParam `json:"params"`               // 参数说明列表
	Source      string         `json:"source"`               // 来源：host=宿主内置 | plugin=插件声明（open_endpoints）
	PluginName  string         `json:"plugin_name,omitempty"` // 来源插件名（source=plugin 时展示）
}

// 目录条目来源取值。
const (
	CatalogSourceHost   = "host"   // 宿主内置（model.OpenAPICatalog 静态目录）
	CatalogSourcePlugin = "plugin" // 插件声明（data/plugins/{id}/manifest.json 的 open_endpoints 聚合）
)

// OpenAPICatalog 返回开放接口目录（纯函数；与 router.go 开放组的注册一一对应）。
// 变更约束：增删接口须同步 ① router.go /open 组路由 ② 此目录 ③ 前端无需改动（读目录渲染）。
func OpenAPICatalog() []CatalogEntry {
	return []CatalogEntry{
		{
			Endpoint:    "me.profile",
			Method:      "GET",
			Path:        "/api/v1/open/me",
			Name:        "我的资料",
			Description: "凭 Key 返回其绑定用户的公开资料（昵称、头像、简介、帖子/获赞计数）；浏览器插件等外部应用用它展示当前站点登录身份",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "posts.create",
			Method:      "POST",
			Path:        "/api/v1/open/posts",
			Name:        "发布文章",
			Description: "凭 Key 以绑定用户身份发布文章（浏览器插件「AI 生成文章」通道；正文支持 markdown/html，可携带标签、媒体与 SEO 输入）",
			Params: []CatalogParam{
				{Name: "title", Type: "string", Location: "body", Required: true, Description: "文章标题"},
				{Name: "content", Type: "string", Location: "body", Required: true, Description: "正文（≤20000 字）"},
				{Name: "content_format", Type: "string", Location: "body", Required: false, Description: "markdown / html（空=markdown）"},
				{Name: "tags", Type: "array", Location: "body", Required: false, Description: "标签（≤5 个）"},
				{Name: "media_ids", Type: "array", Location: "body", Required: false, Description: "关联媒体 ID（AI 配图转存后的 media_id）"},
				{Name: "seo", Type: "object", Location: "body", Required: false, Description: "{seo_title, seo_description} SEO 输入"},
				{Name: "status", Type: "string", Location: "body", Required: false, Description: "draft=草稿 / published=发布（空=published）"},
			},
		},
		{
			Endpoint:    "posts.list",
			Method:      "GET",
			Path:        "/api/v1/open/posts",
			Name:        "帖子列表",
			Description: "时间线帖子分页（说说与文章混合，按发布时间倒序；匿名视角仅返回公开帖）",
			Params: []CatalogParam{
				{Name: "type", Type: "string", Location: "query", Required: false, Description: "媒体形态过滤：text/image/audio/video，空=全部"},
				{Name: "kind", Type: "string", Location: "query", Required: false, Description: "帖子形态过滤：moment=说说 / article=文章，空=全部形态"},
				{Name: "page", Type: "integer", Location: "query", Required: false, Description: "页码，从 1 起，默认 1"},
				{Name: "page_size", Type: "integer", Location: "query", Required: false, Description: "每页条数，默认 20，上限 100"},
			},
		},
		{
			Endpoint:    "posts.detail",
			Method:      "GET",
			Path:        "/api/v1/open/posts/:id",
			Name:        "帖子详情",
			Description: "帖子完整内容（标题、正文、标签、图集、互动计数；说说摘要与文章全文）",
			Params: []CatalogParam{
				{Name: "id", Type: "integer", Location: "path", Required: true, Description: "帖子 ID"},
			},
		},
		{
			Endpoint:    "posts.comments",
			Method:      "GET",
			Path:        "/api/v1/open/posts/:id/comments",
			Name:        "帖子评论",
			Description: "指定帖子的评论列表（树形结构，含楼层与回复）",
			Params: []CatalogParam{
				{Name: "id", Type: "integer", Location: "path", Required: true, Description: "帖子 ID"},
			},
		},
		{
			Endpoint:    "topics.list",
			Method:      "GET",
			Path:        "/api/v1/open/topics",
			Name:        "话题列表",
			Description: "全部话题（含帖子计数与关注数）",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "topics.posts",
			Method:      "GET",
			Path:        "/api/v1/open/topics/:name/posts",
			Name:        "话题帖子",
			Description: "指定话题下的帖子流（最新或热门排序，分页）",
			Params: []CatalogParam{
				{Name: "name", Type: "string", Location: "path", Required: true, Description: "话题名（不带 #）"},
				{Name: "sort", Type: "string", Location: "query", Required: false, Description: "排序：latest=最新（默认）/ hot=热门"},
				{Name: "page", Type: "integer", Location: "query", Required: false, Description: "页码，从 1 起，默认 1"},
				{Name: "page_size", Type: "integer", Location: "query", Required: false, Description: "每页条数，默认 20"},
			},
		},
		{
			Endpoint:    "search",
			Method:      "GET",
			Path:        "/api/v1/open/search",
			Name:        "搜索",
			Description: "全文搜索（标题/正文/标签关键词匹配，分页）",
			Params: []CatalogParam{
				{Name: "q", Type: "string", Location: "query", Required: true, Description: "搜索关键词"},
				{Name: "page", Type: "integer", Location: "query", Required: false, Description: "页码，从 1 起，默认 1"},
				{Name: "page_size", Type: "integer", Location: "query", Required: false, Description: "每页条数，默认 20"},
			},
		},
		{
			Endpoint:    "users.profile",
			Method:      "GET",
			Path:        "/api/v1/open/users/:id",
			Name:        "用户主页",
			Description: "用户公开资料（昵称、头像、简介、帖子/获赞计数）",
			Params: []CatalogParam{
				{Name: "id", Type: "integer", Location: "path", Required: true, Description: "用户 ID"},
			},
		},
		{
			Endpoint:    "users.posts",
			Method:      "GET",
			Path:        "/api/v1/open/users/:id/posts",
			Name:        "用户帖子",
			Description: "指定用户发布的公开帖子流（分页，可按媒体形态过滤）",
			Params: []CatalogParam{
				{Name: "id", Type: "integer", Location: "path", Required: true, Description: "用户 ID"},
				{Name: "type", Type: "string", Location: "query", Required: false, Description: "媒体形态过滤：text/image/audio/video，空=全部"},
				{Name: "page", Type: "integer", Location: "query", Required: false, Description: "页码，从 1 起，默认 1"},
				{Name: "page_size", Type: "integer", Location: "query", Required: false, Description: "每页条数，默认 20"},
			},
		},
		{
			Endpoint:    "pages.detail",
			Method:      "GET",
			Path:        "/api/v1/open/pages/:slug",
			Name:        "自定义页面",
			Description: "自定义页面内容（仅已发布页面，含标题与正文 HTML）",
			Params: []CatalogParam{
				{Name: "slug", Type: "string", Location: "path", Required: true, Description: "页面别名（slug）"},
			},
		},
		{
			Endpoint:    "site.meta",
			Method:      "GET",
			Path:        "/api/v1/open/meta",
			Name:        "站点信息",
			Description: "站点元信息（站名、描述、备案号、社交链接等）",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "ai.models",
			Method:      "GET",
			Path:        "/api/v1/open/ai/models",
			Name:        "AI 模型列表",
			Description: "站点已配置的可用 AI 模型清单（启用的供应商及其模型；API Key 不回显），对话前先拉取选择模型",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "ai.chat",
			Method:      "POST",
			Path:        "/api/v1/open/ai/chat",
			Name:        "AI 对话",
			Description: "调用站点配置的 AI 模型对话（OpenAI 兼容消息格式；模型取自 ai.models 列表；用量计入站点 AI 统计）",
			Params: []CatalogParam{
				{Name: "model", Type: "string", Location: "body", Required: true, Description: "模型名（取自 AI 模型列表接口）"},
				{Name: "messages", Type: "array", Location: "body", Required: true, Description: "对话消息数组：[{role: system|user|assistant, content: 文本}]，按顺序携带上下文"},
				{Name: "max_tokens", Type: "integer", Location: "body", Required: false, Description: "最大输出 token（默认 300，上限 16000）"},
				{Name: "web_search", Type: "string", Location: "body", Required: false, Description: "true=联网回答（站点需配置 SearXNG；响应附 search_results 引用来源）"},
			},
		},
		{
			Endpoint:    "media.transfer",
			Method:      "POST",
			Path:        "/api/v1/open/media/transfer",
			Name:        "图片转存",
			Description: "外链图片落到站点媒体库（源站防盗链图片发布后裂图的解决通道）：返回本站持久地址与媒体 ID，插件发布前替换引用；仅放行公网 http/https 图片地址",
			Params: []CatalogParam{
				{Name: "url", Type: "string", Location: "body", Required: true, Description: "外链图片地址（http/https 公网）"},
			},
		},
		{
			Endpoint:    "media.upload",
			Method:      "POST",
			Path:        "/api/v1/open/media",
			Name:        "媒体上传",
			Description: "本地文件上传到站点媒体库（浏览器插件「写说说」通道：本地图/粘贴图以 Key 绑定用户身份上传，返回本站持久地址与媒体 ID；类型与大小校验沿用主站媒体规则）",
			Params: []CatalogParam{
				{Name: "file", Type: "string", Location: "body", Required: true, Description: "上传文件（multipart 字段名 file）"},
			},
		},
		{
			Endpoint:    "navlinks.save",
			Method:      "POST",
			Path:        "/api/v1/open/nav/links",
			Name:        "导航同步写入",
			Description: "批量把导航条目写入精品导航插件（浏览器插件「同步到站点」通道）：URL 已存在于站点时跳过（不覆盖站点侧编辑），新条目创建；返回 created/skipped/failed 计数。需站点安装并启用「精品导航」插件",
			Params: []CatalogParam{
				{Name: "links", Type: "array", Location: "body", Required: true, Description: "导航条目数组，每项 {name, url, category, tags, description, icon, sort}；url 须为 http/https，单次 ≤500 条"},
			},
		},
		{
			Endpoint:    "ai.chat.stream",
			Method:      "POST",
			Path:        "/api/v1/open/ai/chat/stream",
			Name:        "AI 流式对话",
			Description: "与 AI 对话同参，响应为 SSE 事件流（data: {\"text\":\"增量\"} 逐块下发，data: [DONE] 结束；web_search=true 时首个事件为引用来源）——逐字渲染场景（浏览器插件对话气泡）",
			Params: []CatalogParam{
				{Name: "model", Type: "string", Location: "body", Required: true, Description: "模型名（取自 AI 模型列表接口）"},
				{Name: "messages", Type: "array", Location: "body", Required: true, Description: "对话消息数组（含历史轮次）"},
				{Name: "max_tokens", Type: "integer", Location: "body", Required: false, Description: "最大输出 token（默认 300，上限 16000）"},
				{Name: "web_search", Type: "string", Location: "body", Required: false, Description: "true=联网回答（首个事件下发 search_results 来源）"},
			},
		},
		{
			Endpoint:    "ai.search",
			Method:      "POST",
			Path:        "/api/v1/open/ai/search",
			Name:        "联网搜索",
			Description: "聚合搜索引擎检索（SearXNG：Google/Bing/DuckDuckGo 等聚合；浏览器插件等外部应用凭 Key 直查，返回标题/摘要/来源地址）",
			Params: []CatalogParam{
				{Name: "query", Type: "string", Location: "body", Required: true, Description: "搜索关键词"},
				{Name: "limit", Type: "integer", Location: "body", Required: false, Description: "返回条数（默认 5，上限 10）"},
			},
		},
		{
			Endpoint:    "ai.assist",
			Method:      "POST",
			Path:        "/api/v1/open/ai/assist",
			Name:        "AI 辅助（多模态）",
			Description: "发帖 AI 辅助（MiniMax 多模态）：内容扩写/润色（返回文本）、按内容配图/配乐（生成物已转存本站媒体库，返回地址与媒体 ID）、图片识别（传图片地址返回描述）",
			Params: []CatalogParam{
				{Name: "action", Type: "string", Location: "body", Required: true, Description: "动作：expand=扩写 / polish=润色 / image=配图 / music=配乐 / recognize=识图"},
				{Name: "content", Type: "string", Location: "body", Required: false, Description: "帖子正文（expand/polish/image/music 的输入）"},
				{Name: "image_url", Type: "string", Location: "body", Required: false, Description: "待识别图片的公网地址（recognize 必填）"},
			},
		},
		{
			Endpoint:    "navlinks.list",
			Method:      "GET",
			Path:        "/api/v1/open/nav/links",
			Name:        "导航列表",
			Description: "精品导航插件收藏的全部站点（名称/地址/分类/标签/简介/内嵌图标 dataURL，含聚合分类与标签清单）；浏览器插件等外部应用凭 Key 同步站点导航数据。需安装并启用「精品导航」插件",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "navlinks.private.list",
			Method:      "GET",
			Path:        "/api/v1/open/nav/private/links",
			Name:        "私有导航列表",
			Description: "精品导航中可见性为「私有」的收藏站点（响应结构与「导航列表」一致，仅含私有条目）；浏览器插件凭 Key 同步站长私有导航。需安装并启用「精品导航」插件",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "navlinks.private.config",
			Method:      "GET",
			Path:        "/api/v1/open/nav/private/config",
			Name:        "私有导航设置读取",
			Description: "读取私有导航访问设置：访问方式（self=仅自己可见 / password=密码访问）、是否已设访问密码、私有页标题/副标题、私有条目数；不含任何密码材料。需安装并启用「精品导航」插件",
			Params:      []CatalogParam{},
		},
		{
			Endpoint:    "navlinks.private.save",
			Method:      "POST",
			Path:        "/api/v1/open/nav/private/config",
			Name:        "私有导航设置写入",
			Description: "修改私有导航访问设置（浏览器插件对接通道）：访问方式切换、设置访问密码（6-64 位，留空=保持现有）、私有页标题/副标题；修改密码后前台旧解锁自动失效。需安装并启用「精品导航」插件",
			Params: []CatalogParam{
				{Name: "mode", Type: "string", Location: "body", Required: true, Description: "访问方式：self=仅自己可见 / password=密码访问"},
				{Name: "password", Type: "string", Location: "body", Required: false, Description: "访问密码（6-64 位；留空=保持现有密码；切 password 模式必须已设密码或本次提供）"},
				{Name: "title", Type: "string", Location: "body", Required: false, Description: "私有页标题（≤30 字，空=默认「私有导航」）"},
				{Name: "subtitle", Type: "string", Location: "body", Required: false, Description: "私有页副标题（≤60 字）"},
			},
		},
	}
}

// CatalogIndex 返回「路由模板 → 接口标识」索引（网关中间件按 FullPath+Method 反查用；纯函数）。
// key 格式："GET /api/v1/open/posts/:id"。
func CatalogIndex() map[string]string {
	index := make(map[string]string)
	for _, entry := range OpenAPICatalog() {
		index[entry.Method+" "+entry.Path] = entry.Endpoint
	}
	return index
}

// CatalogEndpoints 返回全部合法接口标识集合（创建 Key 时校验用；纯函数）。
func CatalogEndpoints() map[string]bool {
	set := make(map[string]bool)
	for _, entry := range OpenAPICatalog() {
		set[entry.Endpoint] = true
	}
	return set
}

// ---------- DTO ----------

// CreateOpenAPIKeyReq 生成 Key 请求（后台「接口开放」页面）。
type CreateOpenAPIKeyReq struct {
	Name       string   `json:"name"`        // 备注名（可选，≤100 字符）
	Endpoints  []string `json:"endpoints"`   // 勾选的接口标识（≥1，须在目录内）
	ExpireDays *int     `json:"expire_days"` // 过期天数（正整数；nil/0=永久有效）
}
