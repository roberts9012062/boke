// internal/handler/nav_bridge.go
// 精品导航公开桥接控制器（nav-links 插件）：前台访客数据通道 + 开放网关端点。
//
// 背景：插件代理 API 挂 RequireAuth（匿名 401），而前台导航页要求访客可浏览、
// 浏览器插件要求凭 Key 拉取——故宿主开两条通道、均以 System 身份调用插件
// POST /links/public（与 B站视频桥接 /api/v1/video/bilibili 同模式）：
//   - GET /api/v1/nav/links        公开（前台导航页数据源；直通插件 JSON，仅开放条目）
//   - GET /api/v1/open/nav/links   开放网关（navlinks.list，X-Api-Key 鉴权；
//                                  响应按网关惯例包 {code,message,data}）
//
// 私有导航门禁通道（v1.3.14 起，同以 System 身份调插件 /private/**）：
//   - GET  /api/v1/nav/private/meta    门禁元数据（模式/是否已设密码/私有页文案/条数）
//   - POST /api/v1/nav/private/unlock  密码解锁（{password} → {token}，7 天有效）
//   - GET  /api/v1/nav/private/links   私有数据（管理员登录态或解锁 token 二选一放行）
package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
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
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Category   string   `json:"category"`
	Tags       []string `json:"tags"`
	Description string  `json:"description"`
	Icon       string   `json:"icon"`
	Visibility string   `json:"visibility,omitempty"` // 空=开放（默认）；private=私有（同步通道预留透传）
	Sort       int      `json:"sort"`
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

// ---------- 私有导航门禁通道（前台私有页数据源；均以 System 身份调插件 /private/**） ----------

// callPluginJSON 调插件端点并把 JSON 响应按网关惯例包装 {code,message,data}。
// 插件不可达/非 200 返回 false（响应已写入），调用方直接 return。
// 插件业务错误惯例为 200 + {"error":"..."}——转网关 400，浏览器插件按 code 判定不误判成功。
func (h *NavBridgeHandler) callPluginJSON(c *gin.Context, method string, pluginPath string, body []byte) bool {
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), navLinksPluginID, method, pluginPath, body, bridgeSystemCaller)
	if err != nil || status != 200 {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "精品导航插件未启用或不可达"))
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		resp.Fail(c, 502, errs.New(errs.CodeInternal, "导航数据解析失败"))
		return false
	}
	if msg, ok := payload["error"].(string); ok && msg != "" {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, msg))
		return false
	}
	resp.OK(c, payload)
	return true
}

// PrivateMeta 门禁元数据（GET /api/v1/nav/private/meta，公开；直通插件 JSON）。
// 只含访问模式/是否已设密码/私有页文案/私有条数，无任何密钥材料——可安全给访客。
func (h *NavBridgeHandler) PrivateMeta(c *gin.Context) {
	if h.pluginSvc == nil {
		c.JSON(503, gin.H{"error": "插件服务未配置"})
		return
	}
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), navLinksPluginID, "POST", "/private/meta", []byte("{}"), bridgeSystemCaller)
	if err != nil {
		c.JSON(503, gin.H{"error": "精品导航插件未启用或不可达"})
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}

// PrivateUnlock 密码解锁（POST /api/v1/nav/private/unlock，公开；body {password} 透传）。
// 插件校验访问密码后签发 7 天 token；错误（401 密码不对 / 403 非 password 模式）按状态码透传。
func (h *NavBridgeHandler) PrivateUnlock(c *gin.Context) {
	if h.pluginSvc == nil {
		c.JSON(503, gin.H{"error": "插件服务未配置"})
		return
	}
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(400, gin.H{"error": "请求体读取失败"})
		return
	}
	status, data, callErr := h.pluginSvc.CallAPI(c.Request.Context(), navLinksPluginID, "POST", "/private/unlock", body, bridgeSystemCaller)
	if callErr != nil {
		c.JSON(503, gin.H{"error": "精品导航插件未启用或不可达"})
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}

// PrivateLinks 私有数据（GET /api/v1/nav/private/links，挂 OptionalAuth）。
// 凭证判定：管理员登录态（Bearer → OptionalAuth 注入角色）→ body {admin:true}；
// 否则访客解锁 token（X-Nav-Token 头）→ body {token}。插件按鉴权矩阵放行或 401/403。
// 私有内容响应绝不缓存（no-store），避免网关/浏览器留存受保护数据。
func (h *NavBridgeHandler) PrivateLinks(c *gin.Context) {
	if h.pluginSvc == nil {
		c.JSON(503, gin.H{"error": "插件服务未配置"})
		return
	}
	role := middleware.GetRole(c)
	isAdmin := role == "admin" || role == "superadmin"
	reqBody, marshalErr := json.Marshal(map[string]any{
		"admin": isAdmin,
		"token": c.GetHeader("X-Nav-Token"),
	})
	if marshalErr != nil {
		c.JSON(500, gin.H{"error": "请求组装失败"})
		return
	}
	status, data, callErr := h.pluginSvc.CallAPI(c.Request.Context(), navLinksPluginID, "POST", "/private/links", reqBody, bridgeSystemCaller)
	if callErr != nil {
		c.JSON(503, gin.H{"error": "精品导航插件未启用或不可达"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(status, "application/json; charset=utf-8", data)
}

// ---------- 私有导航开放网关（浏览器插件凭 X-Api-Key 对接；均以 System 身份调插件） ----------
//
// 信任链：开放 Key 由站长在后台生成并逐条目勾选授权（ApiKeyAuth 中间件校验），
// 命中即视为站长授权行为——桥接插件时等同管理员通道（与 navlinks.save 同模型）。

// OpenPrivateList 私有导航数据（GET /api/v1/open/nav/private/links，navlinks.private.list）。
// 响应与 navlinks.list 同构（links/categories/tags/settings），仅含私有条目。
func (h *NavBridgeHandler) OpenPrivateList(c *gin.Context) {
	if h.pluginSvc == nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件服务未配置"))
		return
	}
	h.callPluginJSON(c, "POST", "/private/links", []byte(`{"admin":true}`))
}

// OpenPrivateConfigGet 私有访问设置读取（GET /api/v1/open/nav/private/config，navlinks.private.config）。
// 返回 {mode, has_password, title, subtitle, count}——不含任何密码哈希等密钥材料。
func (h *NavBridgeHandler) OpenPrivateConfigGet(c *gin.Context) {
	if h.pluginSvc == nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件服务未配置"))
		return
	}
	h.callPluginJSON(c, "GET", "/private/config", nil)
}

// OpenPrivateConfigSave 私有访问设置写入（POST /api/v1/open/nav/private/config，navlinks.private.save）。
// body {mode, password?, title?, subtitle?} 透传插件；password 非空才更新（6-64 位），
// 修改密码会轮换解锁密钥（前台旧解锁 token 全部失效）。
func (h *NavBridgeHandler) OpenPrivateConfigSave(c *gin.Context) {
	if h.pluginSvc == nil {
		resp.Fail(c, 503, errs.New(errs.CodeInternal, "插件服务未配置"))
		return
	}
	body, err := c.GetRawData()
	if err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体读取失败"))
		return
	}
	h.callPluginJSON(c, "POST", "/private/config", body)
}
