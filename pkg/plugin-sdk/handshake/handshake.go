// pkg/plugin-sdk/handshake/handshake.go
// go-plugin 握手配置（单一事实源）：宿主（internal/plugin/manager.go）与插件侧
// （pkg/plugin-sdk/server/serve.go）共同引用——此前宿主反向导入插件侧 server 包
// 取常量，依赖方向错误且注释自认「修改需两侧同步」（D1 解耦改造）。
// 变更约束：MagicCookieValue 或 ProtocolVersion 不兼容变更时，两侧需同步发版。
package handshake

import (
	"github.com/hashicorp/go-plugin"
)

// Handshake 握手配置（ProtocolVersion 协商 + MagicCookie 防误启动）。
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  3,                       // 协议版本（升级不兼容时协商）
	MagicCookieKey:   "YUEYAN_PLUGIN_COOKIE",  // 防误启动（校验子进程环境变量）
	MagicCookieValue: "yueyan-blog-plugin-v1", // 主进程启动子进程时注入
}
