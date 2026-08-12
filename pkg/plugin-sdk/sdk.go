// pkg/plugin-sdk/sdk.go
// 插件开发 SDK（M3.3）：第三方插件作者唯一依赖的公共包。
// 对齐 docs/architecture.md 6.3 SDK 接口设计；与主进程 internal/plugin 通过
// proto/plugin.proto 契约通信（进程外 go-plugin + gRPC）。
// M3.5 新增：许可查询（License/FeatureEnabled）——主进程激活时下发，插件只读。
package sdk

import (
	"context"
	"sync"
)

// Info 插件信息（Info() 返回，主进程校验与安装清单一致）。
type Info struct {
	ID          string         // 插件 ID（唯一，与清单 id 一致）
	Name        string         // 插件名称
	Version     string         // 版本号
	Author      string         // 作者
	Description string         // 一句话描述
	Settings    []SettingField // 设置项声明（设置页 schema 驱动；可空=无配置项）
}

// SettingField 插件设置项声明（主进程经 Info RPC 收集，设置页通用渲染器展示）。
type SettingField struct {
	Key     string   // 设置键（存 plugin_instances.config：config["{key}"]）
	Label   string   // 展示标签
	Type    string   // 控件类型：text / switch / select
	Default string   // 默认值
	Options []string // select 选项列表
}

// Event 钩子事件（与主进程 internal/plugin.Event 对齐；Payload 为 JSON bytes，
// 主进程序列化传输，插件侧反序列化使用——各钩子载荷结构由插件自行定义）。
type Event struct {
	TraceID string // 请求追踪 ID（贯穿日志）
	ActorID int64  // 操作者用户 ID（0=匿名/系统）
	Payload []byte // 载荷 JSON（主进程传值副本，插件不可修改主进程数据）
}

// Result 钩子结果（与主进程契约一致）。
//   - OK=false：同步拦截钩子拒绝（Reason 为用户可读原因，阻断核心流程）
//   - Modify：可改写钩子（search.query）返回改写后的载荷 JSON
type Result struct {
	OK     bool   // 是否放行（false = 拒绝）
	Reason string // 拒绝原因（用户可读）
	Modify []byte // 改写结果 JSON（可空）
}

// Hook 钩子声明（插件在 Hooks() 中返回，订阅主进程钩子点）。
type Hook struct {
	Name     string                              // 钩子名（对齐主进程钩子表，见 internal/plugin/hook.go）
	Sync     bool                                // true=同步（可拦截/改写），false=异步（事后通知）
	Priority int                                 // 执行优先级（小先执行，主进程注册表按注册顺序执行）
	Handler  func(ctx context.Context, ev Event) (Result, error) // 钩子处理器
}

// Plugin 插件接口（插件作者实现；Serve 完成握手、注册、优雅退出）。
type Plugin interface {
	// Info 返回插件信息（名称/版本/作者/描述）。
	Info() Info
	// OnActivate 启用回调（初始化资源；失败则插件不进入 running）。
	OnActivate(ctx context.Context) error
	// OnDeactivate 停用回调（保存状态/释放资源）。
	OnDeactivate(ctx context.Context) error
	// Hooks 声明订阅的钩子。
	Hooks() []Hook
}

// APIHandler 插件自定义 API 处理器（RegisterAPI 注册；path 为完整路径）。
// 返回：status HTTP 状态码、resp 响应体 JSON、err 内部错误。
type APIHandler func(ctx context.Context, method string, path string, body []byte) (status int, resp []byte, err error)

// APIProvider 可选接口：实现 RegisterAPI 的插件可暴露自定义 API，
// 主进程统一挂载 /api/plugins/{id}/** 代理转发（见 docs/architecture.md 6.4）。
type APIProvider interface {
	// RegisterAPI 注册自定义 API 路由（method+path 精确匹配）。
	RegisterAPI(api *APIMux)
}

// APIMux 插件自定义 API 路由表（method+path 精确匹配；路径参数后置）。
type APIMux struct {
	routes map[string]APIHandler // 键：method + " " + path
}

// NewAPIMux 创建空路由表。
func NewAPIMux() *APIMux {
	return &APIMux{routes: make(map[string]APIHandler)}
}

// Handle 注册处理器（如 Handle("GET", "/ping", fn)）。
func (m *APIMux) Handle(method string, path string, handler APIHandler) {
	m.routes[method+" "+path] = handler
}

// Find 查找处理器（不存在返回 nil）。
func (m *APIMux) Find(method string, path string) APIHandler {
	return m.routes[method+" "+path]
}

// ---------- 许可证（M3.5：主进程激活时下发，插件只读） ----------

// LicenseInfo 插件许可（主进程唯一数据源；FeatureEnabled 判断付费功能）。
type LicenseInfo struct {
	Edition   string   // 版本：free（demo）/ pro（已激活）
	Features  []string // 授权功能列表
	ExpiresAt int64    // 到期时间戳（Unix 秒；0=永久）
	Degraded  bool     // 已降级（超宽限期未续费，功能锁定）
}

// FeatureEnabled 判断付费功能是否可用（降级/未授权一律 false）。
// 说明：付费插件在功能入口处调用（demo 降级逻辑收敛于此，勿放前端）。
func (l *LicenseInfo) FeatureEnabled(name string) bool {
	if l == nil || l.Degraded {
		return false
	}
	for _, f := range l.Features {
		if f == name {
			return true
		}
	}
	return false
}

// 插件许可内存状态（server.Serve 激活回调更新；并发安全）。
var (
	licenseMu    sync.RWMutex
	licenseState = LicenseInfo{} // 默认 free（未激活/demo）
)

// SetLicense 更新插件许可（server 激活回调使用；插件作者不调用）。
func SetLicense(l LicenseInfo) {
	licenseMu.Lock()
	licenseState = l
	licenseMu.Unlock()
}

// License 返回当前许可只读快照（付费插件查询功能开关）。
func License(ctx context.Context) LicenseInfo {
	licenseMu.RLock()
	defer licenseMu.RUnlock()
	return licenseState
}

// ---------- 插件配置（主进程下发，插件只读） ----------

// 插件配置内存状态（server.Serve SetConfig 回调更新；并发安全）。
var (
	configMu    sync.RWMutex
	configState = map[string]string{} // 配置键值对（仅 schema 声明的 key）
)

// SetConfig 更新插件配置（server 配置下发回调使用；插件作者不调用）。
func SetConfig(values map[string]string) {
	configMu.Lock()
	configState = make(map[string]string, len(values))
	for k, v := range values {
		configState[k] = v
	}
	configMu.Unlock()
}

// Config 返回当前配置只读快照（插件 handler 内查询配置项）。
func Config(ctx context.Context) map[string]string {
	configMu.RLock()
	defer configMu.RUnlock()
	out := make(map[string]string, len(configState))
	for k, v := range configState {
		out[k] = v
	}
	return out
}
