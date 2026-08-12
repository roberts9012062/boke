// internal/handler/plugin_config.go
// 插件设置控制器（M3.7 设置功能端到端）：详情 / 配置读写。
// 独立文件避免 plugin.go 超 400 行约束（契约扩展后插件控制器职责拆分）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// PluginConfigHandler 插件设置控制器（连接器类）。
type PluginConfigHandler struct {
	plugins *service.PluginService // 插件服务
}

// NewPluginConfigHandler 创建插件设置控制器。
func NewPluginConfigHandler(plugins *service.PluginService) *PluginConfigHandler {
	return &PluginConfigHandler{plugins: plugins}
}

// Detail 插件详情（GET /api/v1/admin/plugins/:id，M3.7 设置页数据源）。
// 返回：{plugin: {id, plugin_id, name, version, state, settings_schema, config}}。
func (h *PluginConfigHandler) Detail(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	detail, err := h.plugins.Detail(c.Request.Context(), instanceID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"plugin": detail})
}

// GetConfig 读取插件配置（GET /api/v1/admin/plugins/:id/config，M3.7 设置页回显）。
func (h *PluginConfigHandler) GetConfig(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	values, err := h.plugins.GetConfig(c.Request.Context(), instanceID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"config": values})
}

// SaveConfig 保存插件配置（PUT /api/v1/admin/plugins/:id/config，body: {values:{...}}）。
// service 层按 schema 过滤未声明键并推送运行中进程（即时生效）。
func (h *PluginConfigHandler) SaveConfig(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Values map[string]string `json:"values"` // 配置键值对（仅 schema 声明的 key 生效）
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Values == nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	saved, err := h.plugins.SetConfig(c.Request.Context(), instanceID, req.Values)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"config": saved})
}
