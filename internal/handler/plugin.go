// internal/handler/plugin.go
// 插件控制器（M3.1）：插件商城（GitHub 清单）+ 插件管理（安装/启用禁用/卸载）。
// M3.3 新增：Call 插件自定义 API 代理（/api/plugins/{id}/** 转发子进程）。
package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// PluginHandler 插件控制器（连接器类）。
type PluginHandler struct {
	plugins *service.PluginService // 插件服务
}

// NewPluginHandler 创建插件控制器。
func NewPluginHandler(plugins *service.PluginService) *PluginHandler {
	return &PluginHandler{plugins: plugins}
}

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

// ListInstalled 已安装插件（GET /api/v1/admin/plugins）。
func (h *PluginHandler) ListInstalled(c *gin.Context) {
	items, err := h.plugins.ListInstalled(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// Install 安装插件（POST /api/v1/admin/plugins/install，body: {plugin_id}）。
func (h *PluginHandler) Install(c *gin.Context) {
	var req struct {
		PluginID string `json:"plugin_id"` // 清单插件 ID
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PluginID == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.plugins.Install(c.Request.Context(), req.PluginID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"installed": true})
}

// SetState 启用/禁用（PUT /api/v1/admin/plugins/:id/state，body: {state}）。
func (h *PluginHandler) SetState(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		State string `json:"state"` // running / disabled
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.plugins.SetState(c.Request.Context(), instanceID, req.State); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"state": req.State})
}

// Uninstall 卸载插件（DELETE /api/v1/admin/plugins/:id）。
func (h *PluginHandler) Uninstall(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.plugins.Uninstall(c.Request.Context(), instanceID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"uninstalled": true})
}

// Call 插件自定义 API 代理（/api/v1/plugins/:id/*path，M3.3）。
// 说明：转发到插件子进程 PluginAPI.Call；响应为插件自定义格式（非统一包装）。
func (h *PluginHandler) Call(c *gin.Context) {
	pluginID := c.Param("id")
	path := c.Param("path")
	if path == "" {
		path = "/"
	}
	// 请求体（GET 无体；其余方法读取）
	var body []byte
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		body, _ = io.ReadAll(c.Request.Body)
	}
	status, data, err := h.plugins.CallAPI(c.Request.Context(), pluginID, c.Request.Method, path, body)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}
