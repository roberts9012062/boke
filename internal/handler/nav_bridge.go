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
	"strings"

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

// navSaveLink 同步写入的单条导航载荷（与插件 Link 模型对齐的插件侧子集）。
type navSaveLink struct {
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Sort        int      `json:"sort"`
}

// OpenSave 开放网关写入端点（POST /api/v1/open/nav/links，X-Api-Key 鉴权 + 目录授权 navlinks.save）。
// 浏览器插件「同步到站点」通道：body {links:[…]} 批量写入——
// 先拉插件现有链接取 URL 集合（已存在跳过，不覆盖站点侧编辑），再逐条转调插件 POST /links
// 创建（System 身份，SDK 桥接设计内路径）；单条失败计数不中断。返回 {created, skipped, failed}。
func (h *NavBridgeHandler) OpenSave(c *gin.Context) {
	if h.pluginSvc == nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件服务未配置"))
		return
	}
	var req struct {
		Links []navSaveLink `json:"links" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体需包含非空 links 数组"))
		return
	}
	// 基础清洗：URL 必须为 http/https，name 缺省取 URL；上限 500 条/次
	existingURL := make(map[string]bool)
	status, data, err := h.callPlugin(c.Request.Context())
	if err == nil && status == 200 {
		var current struct {
			Links []struct {
				URL string `json:"url"`
			} `json:"links"`
		}
		if json.Unmarshal(data, &current) == nil {
			for _, item := range current.Links {
				existingURL[item.URL] = true
			}
		}
		// 拉不到现有数据不阻塞写入（仅失去跳过能力）
	}

	created, skipped, failed := 0, 0, 0
	for i := range req.Links {
		if len(req.Links) > 500 && i >= 500 {
			failed += len(req.Links) - 500
			break
		}
		link := req.Links[i]
		link.URL = trimNavURL(link.URL)
		if link.URL == "" {
			failed++
			continue
		}
		if link.Name == "" {
			link.Name = link.URL
		}
		if existingURL[link.URL] {
			skipped++
			continue
		}
		body, marshalErr := json.Marshal(link)
		if marshalErr != nil {
			failed++
			continue
		}
		s, _, callErr := h.pluginSvc.CallAPI(c.Request.Context(), navLinksPluginID, "POST", "/links", body, bridgeSystemCaller)
		if callErr != nil || s >= 300 {
			failed++
			continue
		}
		existingURL[link.URL] = true
		created++
	}
	resp.OK(c, gin.H{"created": created, "skipped": skipped, "failed": failed})
}

// trimNavURL 清洗同步 URL（去空白；非 http/https 返回空串拒绝）。
func trimNavURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return ""
	}
	return trimmed
}
