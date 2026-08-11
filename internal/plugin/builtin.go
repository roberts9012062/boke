// internal/plugin/builtin.go
// 内置插件钩子注册表（M3.2 演示框架端到端）：
//   pluginID → 钩子注册项；插件安装/启用时注册、禁用/卸载时注销。
// 说明：MVP 内置实现验证钩子链路（comment-anti-spam = 评论反垃圾）；
//       M3.2b go-plugin 阶段替换为进程外插件（接口不变，业务零侵入）。
package plugin

import (
	"context"
	"strings"
)

// builtinHooks 内置插件钩子注册表（pluginID → 钩子列表）。
var builtinHooks = map[string][]HookRegistration{
	// 评论反垃圾（设计稿：智能识别垃圾评论与外链，保护讨论区干净有序）
	"comment-anti-spam": {
		{Hook: HookCommentBeforeSave, Handler: antiSpamHandler},
	},
}

// BuiltinHookRegistrations 返回指定插件的内置钩子注册项（无则空）。
func BuiltinHookRegistrations(pluginID string) []HookRegistration {
	return builtinHooks[pluginID]
}

// antiSpamHandler 评论反垃圾处理器（示例）：拦截含外链特征的评论。
// 说明：MVP 简单特征（http/https/www 链接）；M4 AI 审核可替换为模型判定。
func antiSpamHandler(ctx context.Context, ev Event) (Result, error) {
	content, ok := ev.Payload.(string)
	if !ok {
		return Result{OK: true}, nil
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "www.") {
		return Result{OK: false, Reason: "评论包含外链，已由「评论反垃圾」插件拦截"}, nil
	}
	return Result{OK: true}, nil
}
