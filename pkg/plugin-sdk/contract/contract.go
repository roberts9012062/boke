// pkg/plugin-sdk/contract/contract.go
// 插件通信契约（gob 序列化的纯 Go 结构体）：主进程（internal/plugin）与插件子进程
// （pkg/plugin-sdk/server）共用，经 hashicorp/go-plugin 的 NetRPC 协议（net/rpc + gob）
// 传输。取代旧版 protobuf 契约——gRPC/protobuf 全家桶静态链入使每个插件二进制
// 膨胀约 10MB+，net/rpc 为标准库自带，插件体积回归 Go runtime 基线（约 4~6MB）。
//
// 对齐 docs/architecture.md 6.2 三服务设计（生命周期/钩子/自定义 API 合并为单
// Core net/rpc 服务）；数据服务经 MuxBroker 二级通道（服务名固定 Plugin）。
// 传输演进：v3 = gRPC（已废弃）；v4 = 进程桥 net/rpc + gob（当前，见 process 包）。
package contract

// Empty 空请求（无参数方法）。
type Empty struct{}

// PluginInfo 插件信息（Info 返回，主进程校验插件声明与清单一致）。
type PluginInfo struct {
	ID           string         // 插件 ID（唯一，与清单 id 一致）
	Name         string         // 插件名称
	Version      string         // 版本号
	Author       string         // 作者
	Description  string         // 一句话描述
	Hooks        []string       // 声明的钩子（对齐主进程钩子表，如 "post.after_publish"）
	Apis         []string       // 自定义 API 路径（预留，如 "/ping"）
	Settings     []SettingField // 设置项声明（设置页 schema 驱动）
	Capabilities []string       // 能力声明（授权模型：hooks/api/frontend/settings 基础 + data.read 扩展）
}

// SettingField 插件设置项声明（设置页 schema 驱动通用渲染器）。
type SettingField struct {
	Key     string   // 设置键（存 plugin_instances.config：config["{key}"]）
	Label   string   // 展示标签
	Type    string   // 控件类型：text / switch / select
	Default string   // 默认值
	Options []string // select 选项列表
}

// ConfigInfo 插件配置（主进程 → 插件：启动激活时下发 + 保存配置时推送）。
type ConfigInfo struct {
	Values map[string]string // 配置键值对（仅 schema 声明的 key）
}

// Status 激活/停用/配置下发结果。
type Status struct {
	OK    bool   // 是否成功
	Error string // 错误信息（OK=false 时）
}

// LicenseInfo 许可证信息（主进程激活时下发，插件只读；主进程是唯一数据源）。
type LicenseInfo struct {
	Edition   string   // 版本：free（demo）/ pro（已激活）
	Features  []string // 授权功能列表（FeatureEnabled 判断依据）
	ExpiresAt int64    // 到期时间戳（Unix 秒；0=永久）
	Degraded  bool     // 已降级（超宽限期未续费，功能锁定）
}

// ActivateRequest 激活请求（许可证 + 数据服务回连凭据一次性下发）。
type ActivateRequest struct {
	License   LicenseInfo // 许可证信息
	DataAddr  string      // 宿主数据服务监听地址（空=未授权/未提供；仅声明 data.read 能力时下发）
	DataToken string      // 数据服务连接凭据（与地址成对下发；回连首行鉴权）
}

// HookRequest 钩子执行请求（主进程 → 插件；payload 为 JSON bytes）。
type HookRequest struct {
	Hook    string // 钩子名（对齐主进程钩子常量）
	TraceID string // 请求追踪 ID（贯穿日志）
	ActorID int64  // 操作者用户 ID（0=匿名/系统）
	Payload []byte // 载荷 JSON（各钩子自定义结构，主进程传值副本）
}

// HookResponse 钩子执行响应。
//   - OK=false：同步拦截钩子拒绝（Reason 为用户可读原因，阻断核心流程）
//   - Modify：可改写钩子（search.query）返回改写后的载荷 JSON
type HookResponse struct {
	OK     bool   // 是否放行（false=拒绝）
	Reason string // 拒绝原因（用户可读）
	Modify []byte // 改写结果 JSON（可空）
	Error  string // 插件内部错误（主进程记录 last_error，不阻断核心）
}

// StreamEvent 流式事件（主进程 → 插件：异步钩子推送，与 HookRequest 同构；
// 经 broker 裸通道 gob 流式编码，持续发送无需逐次握手）。
type StreamEvent struct {
	Hook    string // 钩子名（异步钩子：post.after_publish/comment.after_save/...）
	TraceID string // 请求追踪 ID（贯穿日志）
	ActorID int64  // 操作者用户 ID（0=匿名/系统）
	Payload []byte // 载荷 JSON（各钩子自定义结构）
}

// APICall 插件自定义 API 调用（主进程代理转发；调用者身份内联传输——
// net/rpc 无 metadata 机制，身份随请求字段直传）。
type APICall struct {
	Method      string // HTTP 方法（GET/POST/PUT/DELETE...）
	Path        string // 路径（如 "/ping"）
	Body        []byte // 请求体 JSON（可空）
	CallerID    int64  // 调用者用户 ID（0=匿名/未知）
	CallerRole  string // 调用者角色（superadmin/admin/author/visitor；空=未知）
	CallerSystem bool  // 系统调用（宿主内部桥接，非外部用户）
}

// APICallResult 插件 API 响应。
type APICallResult struct {
	Status int32  // HTTP 状态码（200 正常）
	Body   []byte // 响应体 JSON
	Error  string // 插件内部错误（非 HTTP 错误）
}

// ---------- 数据服务（插件经 MuxBroker 调用主进程只读数据，能力授权） ----------

// UserRequest 用户查询请求。
type UserRequest struct {
	UserID int64 // 用户 ID
}

// UserInfo 用户信息（脱敏：不含邮箱/手机/密码等敏感字段）。
type UserInfo struct {
	ID        int64  // 用户 ID
	Nickname  string // 昵称
	AvatarURL string // 头像 URL
	Role      string // 角色（superadmin/admin/author/visitor）
	Bio       string // 个人简介
}

// PostRequest 帖子查询请求。
type PostRequest struct {
	PostID int64 // 帖子 ID
}

// PostInfo 帖子信息（脱敏：不含正文全文等敏感内容）。
type PostInfo struct {
	ID         int64  // 帖子 ID
	Title      string // 标题
	Status     string // 状态（published/draft/private/...）
	AuthorID   int64  // 作者 ID
	AuthorName string // 作者昵称
}

// SettingsSnapshot 站点设置快照（主进程仅下发白名单公开键，不含密钥类敏感键）。
type SettingsSnapshot struct {
	Values map[string]string // 白名单键值对
}

// AIModel 可用 AI 供应商模型组（脱敏：不含 API Key）。
type AIModel struct {
	Name   string   // 供应商名称
	Models []string // 可用模型列表
}

// AIModelList 可用 AI 模型清单（插件 AI 辅助：模型下拉数据源）。
type AIModelList struct {
	Models []AIModel // 供应商模型组（空=未配置 AI）
}

// GenerateRequest AI 文本生成请求（插件 AI 辅助：SEO 标题/描述生成）。
type GenerateRequest struct {
	Model   string // 模型名（精确匹配供应商 models）
	Prompt  string // 生成指令（如「为以下内容生成 SEO 标题」）
	Content string // 输入内容（正文/描述）
}

// GenerateResult AI 生成结果。
type GenerateResult struct {
	Text string // 生成文本
}

// OpenAPIKeyInfo 开放接口 API Key 信息（含明文 Key——用于插件与浏览器插件联动场景：
// 插件把 Key 远传给配套浏览器插件，后者凭 X-Api-Key 调用 /api/v1/open/*）。
// 时间为 RFC3339 字符串；空串 = 永久有效（ExpiresAt）/ 从未使用（LastUsedAt）。
type OpenAPIKeyInfo struct {
	ID         int64    // 凭证 ID
	Name       string   // 备注名
	Key        string   // API Key 明文（oa_ 前缀）
	Endpoints  []string // 已授权接口标识（如 posts.list；对应开放接口目录）
	ExpiresAt  string   // 过期时间（RFC3339；空=永久有效）
	LastUsedAt string   // 最近调用时间（RFC3339；空=从未调用）
	CreatedAt  string   // 创建时间（RFC3339）
}

// OpenAPIKeyList API Key 清单（数据服务 GetOpenAPIKeys 响应）。
type OpenAPIKeyList struct {
	Keys []OpenAPIKeyInfo // 全部凭证（按创建时间倒序；空=尚未生成任何 Key）
}
