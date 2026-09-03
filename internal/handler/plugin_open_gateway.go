// internal/handler/plugin_open_gateway.go
// 插件开放网关泛化转发控制器：/api/v1/open/plugins/:id/*path（开放组，ApiKeyAuth 已鉴权）。
//
// 一条路由服务全部插件的声明式开放端点——插件清单 open_endpoints 声明的接口经此
// 以 System 身份直达插件进程（与 nav_bridge / video 等硬编码桥接同模式，但零宿主代码）：
//   - 路由命中：聚合器 FindRoute(method + 实际路径)（白名单精确匹配，未声明 404）
//   - 转发：body 透传 → 插件端推导路径；插件 200 数据包网关 {code,message,data}，
//     插件业务错误（200+{error}）转 400，插件 401/403 语义透传（Key 授权面外的门禁）
package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// PluginOpenGatewayHandler 插件开放网关控制器（连接器类）。
type PluginOpenGatewayHandler struct {
	pluginSvc *service.PluginService        // 插件服务（CallAPI 直达插件进程）
	catalog   *service.PluginOpenCatalog     // 插件开放目录聚合器（路由白名单）
}

// NewPluginOpenGatewayHandler 创建插件开放网关控制器。
func NewPluginOpenGatewayHandler(pluginSvc *service.PluginService, catalog *service.PluginOpenCatalog) *PluginOpenGatewayHandler {
	return &PluginOpenGatewayHandler{pluginSvc: pluginSvc, catalog: catalog}
}

// Gateway 泛化转发（/api/v1/open/plugins/:id/*path；GET/POST）。
// 中间件已按「Method + 实际路径」校验 Key 授权，此处只需查声明并转发。
func (h *PluginOpenGatewayHandler) Gateway(c *gin.Context) {
	if h.pluginSvc == nil || h.catalog == nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件开放网关未配置"))
		return
	}
	openPath := c.Request.URL.Path
	target, ok := h.catalog.FindRoute(c.Request.Method, openPath)
	if !ok {
		// 方法不匹配或路径未声明（如声明了 GET 却以 POST 访问）
		resp.Fail(c, 404, errs.ErrNotFound)
		return
	}
	var body []byte
	if c.Request.Method == "POST" {
		raw, err := c.GetRawData()
		if err != nil {
			resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体读取失败"))
			return
		}
		body = raw
	}
	// 受信 body 合并：插件声明的身份语义（如 {"admin":true}）注入并**覆盖**外部同名键
	// ——凭 Key 的调用即站长授权，同时防外部伪造身份字段
	body = mergeTrustedBody(body, target.TrustedBody)
	// 以声明的插件端方法转发（对外 GET 可映射插件 POST——外部语义与插件实现解耦）
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), target.PluginID, target.PluginMethod, target.PluginPath, body, bridgeSystemCaller)
	if err != nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件未启用或不可达"))
		return
	}
	if status != 200 {
		// 插件端门禁语义（401/403 等）透传给外部应用
		resp.Fail(c, status, errs.New(errs.CodeForbidden, pluginGatewayErrorMessage(data)))
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		resp.Fail(c, 502, errs.New(errs.CodeInternal, "插件响应解析失败"))
		return
	}
	if msg, hasErr := payload["error"].(string); hasErr && msg != "" {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, msg))
		return
	}
	resp.OK(c, payload)
}

// pluginGatewayErrorMessage 从插件错误响应体提取 error 字段（缺省给通用文案；纯函数）。
func pluginGatewayErrorMessage(data []byte) string {
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil && body.Error != "" {
		return body.Error
	}
	return "插件拒绝该请求"
}

// mergeTrustedBody 合并受信 body（纯函数）：外部 body 解析为对象后并入 trusted
// （trusted 同名键覆盖——防外部伪造身份字段）；全空返回 nil（无 body 转发）。
// 外部 body 非法 JSON 时仅以 trusted 为准（宽松：不给伪造机会也不因格式拒绝）。
func mergeTrustedBody(external []byte, trusted map[string]any) []byte {
	if len(trusted) == 0 {
		return external
	}
	merged := make(map[string]any, len(trusted))
	if len(external) > 0 {
		var parsed map[string]any
		if json.Unmarshal(external, &parsed) == nil {
			for k, v := range parsed {
				merged[k] = v
			}
		}
	}
	for k, v := range trusted {
		merged[k] = v // 受信键覆盖外部同名键
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return external
	}
	return raw
}
