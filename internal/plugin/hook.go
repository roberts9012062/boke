// internal/plugin/hook.go
// 插件钩子契约（M3.2 扩展框架核心）：钩子名常量、事件/结果类型、处理器签名。
// 对齐 docs/architecture.md 第 6 章 + docs/plugin-dev-guide.md 第 5 章钩子表。
package plugin

import "context"

// 钩子名常量（业务服务与插件共同约定；同步=可拦截/可改写，异步=事后通知）。
const (
	HookPostBeforePublish = "post.before_publish" // 发帖/编辑保存前（同步，可拦截）
	HookPostAfterPublish  = "post.after_publish"  // 发布成功后（异步）
	HookCommentBeforeSave = "comment.before_save" // 评论/回复保存前（同步，可拦截）
	HookCommentAfterSave  = "comment.after_save"  // 评论保存后（异步）
	HookSearchQuery       = "search.query"        // 搜索查询（同步，可改写关键词）
	HookNotificationSend  = "notification.send"   // 通知发送（异步）
	HookAdminPage         = "admin.page"          // 后台页面（同步，扩展点占位）
	HookContentRender     = "content.render"      // 帖子内容渲染（M3.9 同步，可改写正文）
	HookAPIMiddleware     = "api.middleware"      // API 请求拦截（M3.9 同步，可阻断写请求）
	HookAIBeforeGenerate  = "ai.before_generate"  // AI 生成前（M3.9 同步，可改写输入）
	HookAIAfterGenerate   = "ai.after_generate"   // AI 生成后（M3.9 异步）
)

// 同步钩子集合（超时 + panic 恢复；拒绝可阻断核心流程）。
var syncHooks = map[string]bool{
	HookPostBeforePublish: true,
	HookCommentBeforeSave: true,
	HookSearchQuery:       true,
	HookAdminPage:         true,
	HookContentRender:     true,
	HookAPIMiddleware:     true,
	HookAIBeforeGenerate:  true,
}

// knownHooks 已知钩子全集（含同步/异步；进程外插件适配器注册时校验，契约外钩子跳过）。
var knownHooks = map[string]bool{
	HookPostBeforePublish: true,
	HookPostAfterPublish:  true,
	HookCommentBeforeSave: true,
	HookCommentAfterSave:  true,
	HookSearchQuery:       true,
	HookNotificationSend:  true,
	HookAdminPage:         true,
	HookContentRender:     true,
	HookAPIMiddleware:     true,
	HookAIBeforeGenerate:  true,
	HookAIAfterGenerate:   true,
}

// allHookNames 全部钩子名（进程外插件适配器注销时遍历）。
var allHookNames = []string{
	HookPostBeforePublish, HookPostAfterPublish,
	HookCommentBeforeSave, HookCommentAfterSave,
	HookSearchQuery, HookNotificationSend, HookAdminPage,
	HookContentRender, HookAPIMiddleware,
	HookAIBeforeGenerate, HookAIAfterGenerate,
}

// IsHookRegistered 判断钩子名是否为主进程已知钩子（契约外扩展不注册）。
func IsHookRegistered(hook string) bool {
	return knownHooks[hook]
}

// IsSyncHook 判断钩子是否为同步（可拦截/改写）。
func IsSyncHook(hook string) bool {
	return syncHooks[hook]
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
	Hook    string  // 钩子名
	Handler Handler // 处理器
}
