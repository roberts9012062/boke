// internal/handler/plugin_market.go
// 插件商城控制器（M5 文件夹结构）：商城清单 + 插件介绍 README（详情弹窗渲染展示）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/resp"
)

// Market 插件商城（GET /api/v1/admin/plugins/market?source=）。
// 参数：source 自定义插件源仓库（可选，空 = settings 默认）。
func (h *PluginHandler) Market(c *gin.Context) {
	manifest, items, actualSource, err := h.plugins.Market(c.Request.Context(), c.Query("source"))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"source": actualSource, "name": manifest.Name, "description": manifest.Description, "items": items})
}

// Readme 插件介绍（GET /api/v1/admin/plugins/market/:id/readme?source=）。
// 参数：:id 插件 ID；source 可选插件源。返回：插件文件夹 README.md 原文（前端渲染 Markdown）。
func (h *PluginHandler) Readme(c *gin.Context) {
	content, err := h.plugins.Readme(c.Request.Context(), c.Query("source"), c.Param("id"))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"readme": content})
}
