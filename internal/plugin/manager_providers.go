// internal/plugin/manager_providers.go
// 进程管理器的依赖回调类型与能力门控辅助函数（从 manager.go 拆出，行数硬性指标）。
// 说明：provider 回调由 service 层实现注入，避免 internal/plugin → repository 反向依赖；
//      装配方（server.go）以延迟绑定闭包规避 service↔manager 创建顺序环。
package plugin

import (
	"context"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// LicenseProvider 许可证查询回调（M3.5：service 层实现，避免 plugin→repository 依赖）。
// 返回：许可证信息（nil=无记录，demo 模式）；错误=查询失败（按 free 兜底）。
type LicenseProvider func(ctx context.Context, pluginID string) (*proto.LicenseInfo, error)

// ConfigProvider 插件配置查询回调（M3.7 设置功能：service 层实现，避免 plugin→repository 依赖）。
// 返回：配置键值对（仅 schema 声明的 key；nil/空=无配置）。
type ConfigProvider func(ctx context.Context, pluginID string) (map[string]string, error)

// CapabilityProvider 插件登记能力查询回调（P2 加固：service 层实现，读 plugin_instances.capabilities）。
// 返回：安装时登记的能力列表（空=无登记；运行时门控与二进制自报取交集——收紧策略）。
type CapabilityProvider func(ctx context.Context, pluginID string) ([]string, error)

// stringListContains 判断字符串列表是否包含目标（能力门控用）。
func stringListContains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// intersectStrings 两列表交集（保持 a 中顺序；纯函数）。
func intersectStrings(a []string, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, item := range b {
		set[item] = true
	}
	out := make([]string, 0, len(a))
	for _, item := range a {
		if set[item] {
			out = append(out, item)
		}
	}
	return out
}
