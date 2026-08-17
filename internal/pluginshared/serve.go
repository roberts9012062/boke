// internal/pluginshared/serve.go
// 插件前端共享 SDK 分发（E2 去重）：/plugin-sdk/shared.js（同源 ESM，embed 静态内容）。
// 插件页面/槽位模块以绝对路径 import 使用（escapeHtml/试播控制器/页面骨架），
// 消除 qq-music 与 netease-music 两套前端各 3 份的复制粘贴。
package pluginshared

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// sharedFS 内嵌共享 SDK 资源（本包目录 shared.js）。
//
//go:embed shared.js
var sharedFS embed.FS

// SharedJS 共享 SDK HTTP 处理器（GET /plugin-sdk/shared.js）。
// 缓存策略：no-store——SDK 随宿主发版更新，插件资产不自带副本，始终取最新。
func SharedJS(c *gin.Context) {
	content, err := sharedFS.ReadFile("shared.js")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "text/javascript; charset=utf-8")
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	_, _ = c.Writer.Write(content)
}
