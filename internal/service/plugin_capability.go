// internal/service/plugin_capability.go
// 插件能力授权模型（M3.8）：
//   capabilities 枚举——基础能力（hooks/api/frontend/settings）默认授予；
//   扩展能力（data.read）需声明且运行时门控（仅授权插件获得数据服务 brokerID）。
//   安装校验：声明未知能力 → 拒绝安装（防越权行为声明）。
package service

// 插件能力枚举（清单 capabilities 字段取值；新增能力需同步 knownCapabilitySet）。
const (
	CapabilityHooks    = "hooks"     // 钩子（基础：订阅主进程钩子点）
	CapabilityAPI      = "api"       // 自定义 API（基础：暴露 /api/plugins/{id}/**）
	CapabilityFrontend = "frontend"  // 前端扩展（基础：槽位渲染与侧栏入口）
	CapabilitySettings = "settings"  // 设置项（基础：schema 驱动设置页）
	CapabilityDataRead = "data.read" // 只读数据服务（扩展：经 broker 查询脱敏数据——运行时门控）
	// 规划能力（后置）：admin.page（后台页面）、ai（AI 能力）
)

// knownCapabilitySet 已知能力集合（安装校验依据；未知声明拒绝安装）。
var knownCapabilitySet = map[string]bool{
	CapabilityHooks: true, CapabilityAPI: true,
	CapabilityFrontend: true, CapabilitySettings: true,
	CapabilityDataRead: true,
}

// unknownCapabilities 返回声明中不在已知能力集合内的项（空=全部合法）。
func unknownCapabilities(declared []string) []string {
	unknown := make([]string, 0, len(declared))
	for _, cap := range declared {
		if !knownCapabilitySet[cap] {
			unknown = append(unknown, cap)
		}
	}
	return unknown
}

// knownCapabilitiesList 已知能力列表（错误提示用，稳定顺序）。
func knownCapabilitiesList() []string {
	return []string{
		CapabilityHooks, CapabilityAPI,
		CapabilityFrontend, CapabilitySettings, CapabilityDataRead,
	}
}
