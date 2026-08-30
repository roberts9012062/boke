// internal/handler/nav_bridge.go
// 精品导航公开桥接控制器（nav-links 插件）：前台访客数据通道 + 开放网关端点。
//
// 背景：插件代理 API 挂 RequireAuth（匿名 401），而前台导航页要求访客可浏览、
// 浏览器插件要求凭 Key 拉取——故宿主开两条通道、均以 System 身份调用插件
// POST /links/public（与 B站视频桥接 /api/v1/video/bilibili 同模式）：
//   - GET /api/v1/nav/links        公开（前台导航页数据源；直通插件 JSON）
//   - GET /api/v1/open/nav/links   开放网关（navlinks.list，X-Api-Key 鉴权；
//                                  响应按网关惯例包 {code,message,data}）
package handler

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// navLinksPluginID 精品导航插件 ID（桥接目标）。
const navLinksPluginID = "nav-links"

// NavBridgeHandler 精品导航桥接控制器（连接器类）。
type NavBridgeHandler struct {
	pluginSvc *service.PluginService // 插件服务（CallAPI 直达插件进程）
}

// NewNavBridgeHandler 创建精品导航桥接控制器。
func NewNavBridgeHandler(pluginSvc *service.PluginService) *NavBridgeHandler {
	return &NavBridgeHandler{pluginSvc: pluginSvc}
}

// callPlugin 调插件公开数据端点（统一 POST /links/public，系统身份）。
func (h *NavBridgeHandler) callPlugin(ctx context.Context) (int, []byte, error) {
	return h.pluginSvc.CallAPI(ctx, navLinksPluginID, "POST", "/links/public", []byte("{}"), bridgeSystemCaller)
}

// PublicLinks 前台导航页数据（GET /api/v1/nav/links，公开；直通插件 JSON）。
// 插件未启用时返回空结构而非错误——前台展示占位比报错体验更好。
func (h *NavBridgeHandler) PublicLinks(c *gin.Context) {
	if h.pluginSvc == nil {
		c.JSON(503, gin.H{"error": "插件服务未配置"})
		return
	}
	status, data, err := h.callPlugin(c.Request.Context())
	if err != nil {
		c.JSON(503, gin.H{"error": "精品导航插件未启用或不可达"})
		return
	}
	// 短缓存：数据含内嵌图标（体积可观），30 秒浏览器缓存显著减轻插件压力
	c.Header("Cache-Control", "public, max-age=30")
	c.Data(status, "application/json; charset=utf-8", data)
}

// OpenList 开放网关端点（GET /api/v1/open/nav/links，X-Api-Key 鉴权 + 目录授权 navlinks.list）。
// 响应按网关惯例包装 {code,message,data}（浏览器插件按 code=0 判定，对齐既有开放接口）。
func (h *NavBridgeHandler) OpenList(c *gin.Context) {
	if h.pluginSvc == nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件服务未配置"))
		return
	}
	status, data, err := h.callPlugin(c.Request.Context())
	if err != nil || status != 200 {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "精品导航插件未启用或不可达"))
		return
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		resp.Fail(c, 502, errs.New(errs.CodeInternal, "导航数据解析失败"))
		return
	}
	resp.OK(c, payload)
}
