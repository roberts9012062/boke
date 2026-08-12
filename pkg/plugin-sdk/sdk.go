// pkg/plugin-sdk/sdk.go
// 插件开发 SDK（M3.3）：第三方插件作者唯一依赖的公共包。
// 对齐 docs/architecture.md 6.3 SDK 接口设计；与主进程 internal/plugin 通过
// proto/plugin.proto 契约通信（进程外 go-plugin + gRPC）。
package sdk

import "context"

// Info 插件信息（Info() 返回，主进程校验与安装清单一致）。
type Info struct {
	ID          string // 插件 ID（唯一，与清单 id 一致）
	Name        string // 插件名称
	Version     string // 版本号
	Author      string // 作者
	Description string // 一句话描述
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
