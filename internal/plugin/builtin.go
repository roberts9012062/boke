// internal/plugin/builtin.go
// 内置插件钩子注册表（M3.2 演示框架端到端）：
//   pluginID → 钩子注册项；插件安装/启用时注册、禁用/卸载时注销。
// 说明：MVP 内置实现验证钩子链路；M3.2b 起插件全部进程外化——
//       comment-anti-spam 的内置演示实现已由 marketplace-repo/comment-anti-spam
//       进程外实现取代（多级检测 + 可配置 + AI 判定），此处不再保留内置条目
//       （保留会使 isProcessPlugin 判定失效，进程版永远不被拉起）。
package plugin

// builtinHooks 内置插件钩子注册表（pluginID → 钩子列表；当前为空——全部进程外化）。
var builtinHooks = map[string][]HookRegistration{}

// BuiltinHookRegistrations 返回指定插件的内置钩子注册项（无则 nil——该插件按进程外处理）。
func BuiltinHookRegistrations(pluginID string) []HookRegistration {
	return builtinHooks[pluginID]
}
