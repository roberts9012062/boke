// internal/plugin/hook.go
// 插件钩子契约（M3.2 扩展框架核心）：钩子名常量、事件/结果类型、处理器签名。
// 对齐 docs/architecture.md 第 6 章 + docs/plugin-dev-guide.md 第 5 章钩子表。
// D2 解耦改造：钩子规格收敛为单张 hookSpecs 表（唯一事实源）——
// 此前 syncHooks/knownHooks/allHookNames 三张平行表，新增钩子需手工同步 4 处（僵化坏味道）。
// B1（Cordis 对标）：同步/异步二值升级为显式分发模式（serial/waterfall/emit）——
// 分发模式是钩子的公开约定（对齐 dsh 事件目录 @mode），改写型钩子为链式改写管道。
package plugin

import (
	"context"
	"sort"
)

// 钩子名常量（业务服务与插件共同约定；serial=可拦截，waterfall=链式改写，emit=事后通知）。
const (
	HookPostBeforePublish = "post.before_publish" // 发帖/编辑保存前（serial，可拦截）
	HookPostAfterPublish  = "post.after_publish"  // 发布成功后（emit）
	HookCommentBeforeSave = "comment.before_save" // 评论/回复保存前（serial，可拦截）
	HookCommentAfterSave  = "comment.after_save"  // 评论保存后（emit）
	HookSearchQuery       = "search.query"        // 搜索查询（waterfall，链式改写关键词）
	HookNotificationSend  = "notification.send"   // 通知发送（emit）
	HookAdminPage         = "admin.page"          // 后台页面（serial，扩展点占位）
	HookContentRender     = "content.render"      // 帖子内容渲染（waterfall，链式改写正文）
	HookAPIMiddleware     = "api.middleware"      // API 请求拦截（serial，可阻断写请求）
	HookAIBeforeGenerate  = "ai.before_generate"  // AI 生成前（waterfall，链式改写输入）
	HookAIAfterGenerate   = "ai.after_generate"   // AI 生成后（emit）
)

// DispatchMode 钩子分发模式（Cordis 事件四模式的 Go 化裁剪：parallel 无场景不引入）。
//   - serial：顺序执行，任一拒绝（OK=false）即短路返回（拦截语义：before_* / api.middleware）
//   - waterfall：链式改写管道——下游处理器收到上游改写后的载荷，拒绝同样短路
//     （改写语义：content.render / search.query / ai.before_generate；多插件基于彼此结果组合）
//   - emit：异步观察，事后通知，不阻塞调用方（after_* / notification.send）
type DispatchMode string

// 分发模式常量（hookSpecs 表唯一使用处；业务代码经 HookMode() 查询）。
const (
	ModeSerial    DispatchMode = "serial"    // 顺序拦截（同步）
	ModeWaterfall DispatchMode = "waterfall" // 链式改写（同步）
	ModeEmit      DispatchMode = "emit"      // 异步通知
)

// hookSpec 钩子规格（单表条目：分发模式是钩子的唯一固有属性，同步性由模式蕴含）。
type hookSpec struct {
	mode DispatchMode // 分发模式（serial/waterfall 为同步；emit 为异步）
}

// hookSpecs 钩子规格表（唯一事实源：新增钩子在此加一行并选定分发模式，
// 同步性、已知性、注销遍历名单均由本表派生——不再需要手工维护多张平行表）。
var hookSpecs = map[string]hookSpec{
	HookPostBeforePublish: {mode: ModeSerial},
	HookPostAfterPublish:  {mode: ModeEmit},
	HookCommentBeforeSave: {mode: ModeSerial},
	HookCommentAfterSave:  {mode: ModeEmit},
	HookSearchQuery:       {mode: ModeWaterfall},
	HookNotificationSend:  {mode: ModeEmit},
	HookAdminPage:         {mode: ModeSerial},
	HookContentRender:     {mode: ModeWaterfall},
	HookAPIMiddleware:     {mode: ModeSerial},
	HookAIBeforeGenerate:  {mode: ModeWaterfall},
	HookAIAfterGenerate:   {mode: ModeEmit},
}

// knownHooks 已知钩子全集（hookSpecs 派生；进程外插件适配器注册时校验，契约外钩子跳过）。
var knownHooks = deriveKnownHooks(hookSpecs)

// allHookNames 全部钩子名（hookSpecs 派生；进程外插件适配器注销时遍历）。
var allHookNames = sortedHookNames(hookSpecs)

// deriveKnownHooks 规格表 → 已知性集合（纯函数）。
func deriveKnownHooks(specs map[string]hookSpec) map[string]bool {
	known := make(map[string]bool, len(specs))
	for name := range specs {
		known[name] = true
	}
	return known
}

// sortedHookNames 规格表 → 排序名列表（纯函数；排序保证遍历稳定，便于测试与日志）。
func sortedHookNames(specs map[string]hookSpec) []string {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsHookRegistered 判断钩子名是否为主进程已知钩子（契约外扩展不注册）。
func IsHookRegistered(hook string) bool {
	return knownHooks[hook]
}

// IsSyncHook 判断钩子是否为同步（serial/waterfall 同步；emit 异步）。
// 说明：B1 起由分发模式派生（同步性不再是独立属性），签名保持兼容。
func IsSyncHook(hook string) bool {
	mode, ok := hookModeOf(hook)
	return ok && mode != ModeEmit
}

// HookMode 查询钩子分发模式（未知钩子返回 ModeSerial + false；调度与文档目录用）。
func HookMode(hook string) (DispatchMode, bool) {
	return hookModeOf(hook)
}

// hookModeOf 规格表查询（内部统一入口，避免多处直接读表）。
func hookModeOf(hook string) (DispatchMode, bool) {
	spec, ok := hookSpecs[hook]
	return spec.mode, ok
}

// Event 钩子事件（TraceID 贯穿请求，ActorID 操作者）。
type Event struct {
	TraceID string // 请求追踪 ID（贯穿日志）
	ActorID int64  // 操作者用户 ID（0=匿名/系统）
	Payload any    // 钩子载荷（各钩子自定义结构，核心传值副本，插件不可修改）
}

// Result 钩子结果。
//   - OK=false：同步拦截钩子拒绝（Reason 为用户可读原因，阻断核心流程）
//   - Modify：可改写钩子（search.query）返回改写后的载荷
type Result struct {
	OK     bool   // 是否放行（false = 拒绝）
	Reason string // 拒绝原因（用户可读）
	Modify any    // 改写结果（可选）
}

// Handler 钩子处理器签名（插件实现；panic/超时由调度器隔离，不影响核心）。
type Handler func(ctx context.Context, ev Event) (Result, error)

// HookRegistration 插件钩子注册项（内置插件注册表与 go-plugin 加载共用）。
type HookRegistration struct {
	Hook     string  // 钩子名
	Handler  Handler // 处理器
	Priority int     // 执行优先级（D3：小值先执行，同值按注册顺序；进程外适配器暂为 0）
}
