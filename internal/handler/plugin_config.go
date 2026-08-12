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

// resolveInstanceID 解析 :id 参数为实例 ID：兼容「数字实例 ID」与「插件 ID 字符串」——
// nav 动态入口/直达链接用插件 ID（如 /admin/plugins/seo-optimizer/settings），
// 需解析为实例 ID 再走设置接口（否则前端 Number() 得 NaN → 400）。
func (h *PluginConfigHandler) resolveInstanceID(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
		return id, true
	}
	// 插件 ID → 实例 ID（未安装返回 false → 404）
	instanceID, err := h.plugins.InstanceIDByPluginID(c.Request.Context(), raw)
	if err != nil {
		return 0, false
	}
	return instanceID, true
}

// Detail 插件详情（GET /api/v1/admin/plugins/:id，M3.7 设置页数据源）。
// 返回：{plugin: {id, plugin_id, name, version, state, settings_schema, config}}。
func (h *PluginConfigHandler) Detail(c *gin.Context) {
	instanceID, ok := h.resolveInstanceID(c)
	if !ok {
		resp.FailFrom(c, errs.ErrNotFound)
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
	instanceID, ok := h.resolveInstanceID(c)
	if !ok {
		resp.FailFrom(c, errs.ErrNotFound)
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
	instanceID, ok := h.resolveInstanceID(c)
	if !ok {
		resp.FailFrom(c, errs.ErrNotFound)
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
