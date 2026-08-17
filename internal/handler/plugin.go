// internal/handler/plugin.go
// 插件控制器（M3.1）：插件商城（GitHub 清单）+ 插件管理（安装/启用禁用/卸载）。
// M3.3 新增：Call 插件自定义 API 代理（/api/plugins/{id}/** 转发子进程）。
// M3.4 新增：Upload 本地 .bpk 上传安装。
// M3.5 新增：许可证激活/状态 + GitHub OAuth 连接。
package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/plugin-sdk"
	"github.com/roberts9012062/boke/pkg/resp"
)

// .bpk 上传大小上限（50MB，与服务层校验一致）。
const maxBpkUpload = 50 << 20

// PluginHandler 插件控制器（连接器类）。
type PluginHandler struct {
	plugins *service.PluginService // 插件服务
	oauth   *service.OAuthService  // GitHub OAuth（M3.5，可空）
}

// NewPluginHandler 创建插件控制器。
func NewPluginHandler(plugins *service.PluginService, oauth *service.OAuthService) *PluginHandler {
	return &PluginHandler{plugins: plugins, oauth: oauth}
}

// HookAPI 插件 api.middleware 钩子入口（M3.9：router 中间件调用；返回 false=已阻断并写入 403）。
// 仅对写请求生效（GET/HEAD 在中间件层已放行）。
func (h *PluginHandler) HookAPI(c *gin.Context) bool {
	ok, reason := h.plugins.CheckAPIMiddleware(c.Request.Context(), c.Request.Method, c.Request.URL.Path, middleware.GetUserID(c))
	if ok {
		return true
	}
	resp.Fail(c, 403, errs.New(errs.CodeForbidden, reason))
	return false
}

// Market / Readme 商城控制器见 plugin_market.go（M5 文件夹结构）。

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
// 安全（P1 加固）：透传调用者 userID/role（gRPC metadata）——插件侧可做 per-endpoint 鉴权
// （如登录导入/配置写入要求管理员；此前任意登录用户可触达全部插件端点）。
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
	caller := sdk.CallerIdentity{UserID: middleware.GetUserID(c), Role: middleware.GetRole(c)}
	status, data, err := h.plugins.CallAPI(c.Request.Context(), pluginID, c.Request.Method, path, body, caller)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}

// Upload 本地上传 .bpk 安装（POST /api/v1/admin/plugins/upload，multipart file=<.bpk>，M3.4）。
// 说明：开发/验证便利通道（正式分发走 GitHub Release）；内部完成校验/解包/注册/激活。
func (h *PluginHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	defer file.Close()
	// 大小上限（服务层同样校验；此处提前拒绝大文件上传）
	if header.Size > maxBpkUpload {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "插件包超过大小上限（50MB）"))
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "读取插件包失败"))
		return
	}
	// ?upgrade=1：本地升级验证通道（跳过已安装冲突，替换版本）
	upgrade := c.Query("upgrade") == "1"
	if err := h.plugins.InstallFromBPK(c.Request.Context(), content, "本地上传", upgrade); err != nil {
		fmt.Fprintf(os.Stderr, "[plugin-upload] 安装失败（%s）：%v\n", header.Filename, err)
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"installed": true, "name": header.Filename, "upgraded": upgrade})
}

// Extensions 前台插件扩展清单（GET /api/v1/plugin-extensions，公开，M3.6）。
// 说明：页面槽位加载插件扩展用（公开接口避免前台未登录 401）；返回 running 且含前端资产的插件。
func (h *PluginHandler) Extensions(c *gin.Context) {
	items, err := h.plugins.FrontendExtensions(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// Updates 检查可更新插件（GET /api/v1/plugin-updates，M3.6 后置：一键升级角标）。
func (h *PluginHandler) Updates(c *gin.Context) {
	items, err := h.plugins.CheckUpdates(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// Upgrade 一键升级插件（POST /api/v1/admin/plugins/:id/upgrade，M3.6 后置）。
func (h *PluginHandler) Upgrade(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.plugins.UpdatePlugin(c.Request.Context(), instanceID); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"upgraded": true})
}

// Asset 插件前端资源静态服务（GET /plugin-assets/:id/*filepath，M3.6）。
// 说明：公开访问（页面渲染时无需登录）；资源已在安装时 checksums 全量校验（落盘即可信）；
//       pluginID 白名单 + 路径前缀校验防穿越（binstore.Dir 同源）。
// 安全（P0 加固）：仅放行 frontend/ 子目录——plugin.bin/pubkey.pem/manifest.json 等
//       包内文件（二进制可被匿名下载 = 逆向与凭据泄露面）一律 404；
//       非 running 状态插件（停用/卸载软删残留）资产不再公开可读。
func (h *PluginHandler) Asset(c *gin.Context) {
	pluginID := c.Param("id")
	// 路径白名单清理（frontend/ 子树内；目录访问回退默认首页）
	rel, ok := sanitizeAssetPath(c.Param("filepath"))
	if !ok {
		resp.Fail(c, 404, errs.ErrNotFound)
		return
	}
	baseDir := h.plugins.PublicAssetDir(c.Request.Context(), pluginID) // 空=ID 不合法或插件未运行
	if baseDir == "" {
		resp.Fail(c, 404, errs.ErrNotFound)
		return
	}
	// 双保险：Join 后前缀校验（拒绝逃逸插件目录）
	target := filepath.Join(baseDir, filepath.FromSlash(rel))
	if !strings.HasPrefix(target, baseDir+string(os.PathSeparator)) {
		resp.Fail(c, 404, errs.ErrNotFound)
		return
	}
	if _, err := os.Stat(target); err != nil {
		resp.Fail(c, 404, errs.ErrNotFound)
		return
	}
	// 分级缓存（E5）：manifest.json 是扩展发现入口——no-store 保证升级后即时生效；
	// 其余资产（JS/CSS/图标）允许协商缓存（no-cache：每次校验 Last-Modified，命中回 304，
	// 省带宽不减新鲜度——此前全量 no-store 每次页面渲染都全量回源）
	if strings.HasSuffix(rel, "manifest.json") {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
	} else {
		c.Header("Cache-Control", "no-cache, must-revalidate")
	}
	c.File(target)
}

// sanitizeAssetPath 清理并校验插件资产请求路径（纯函数）。
// 规则：URL 语义 Clean（消除 ../ 穿越）→ 必须落在 frontend/ 子树内；
//      根/目录访问回退 frontend/index.html（iframe 沙箱入口）。
// 返回：安全相对路径；false=路径不合法或白名单外（plugin.bin/pubkey.pem 等包内文件）。
func sanitizeAssetPath(relPath string) (string, bool) {
	clean := path.Clean(relPath)
	if clean == "." || clean == "/" {
		return "frontend/index.html", true
	}
	clean = strings.TrimPrefix(clean, "/")
	if clean == "frontend" {
		return "frontend/index.html", true
	}
	if !strings.HasPrefix(clean, "frontend/") {
		return "", false
	}
	return clean, true
}

// ActivateLicense 激活许可证（POST /api/v1/admin/plugins/:id/license，body: {license_jwt}，M3.5）。
// 说明：:id 为实例 ID（与 SetState 一致）；验签成功后若插件在运行则重启进程（让 SDK 许可缓存生效）。
func (h *PluginHandler) ActivateLicense(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		LicenseJWT string `json:"license_jwt"` // 作者签发的 license.jwt 原文
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.LicenseJWT == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	pluginID, err := h.plugins.PluginIDByInstance(c.Request.Context(), instanceID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	if err := h.plugins.ActivateLicense(c.Request.Context(), pluginID, req.LicenseJWT); err != nil {
		resp.FailFrom(c, err)
		return
	}
	// 重启插件进程（running 时）让许可生效
	if h.plugins.IsRunning(pluginID) {
		_ = h.plugins.Restart(c.Request.Context(), pluginID)
	}
	status, err := h.plugins.LicenseStatus(c.Request.Context(), pluginID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"activated": true, "license": status})
}

// LicenseStatus 查询许可证状态（GET /api/v1/admin/plugins/:id/license，M3.5；:id 为实例 ID）。
func (h *PluginHandler) LicenseStatus(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	pluginID, err := h.plugins.PluginIDByInstance(c.Request.Context(), instanceID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	status, err := h.plugins.LicenseStatus(c.Request.Context(), pluginID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"license": status})
}

// OAuthAuthorize 发起 GitHub 连接（GET /api/v1/admin/plugins/oauth/authorize，M3.5）。
// 返回：跳转 URL（前端 window.location 跳转；凭证未配置返回 enabled=false）。
func (h *PluginHandler) OAuthAuthorize(c *gin.Context) {
	if h.oauth == nil || !h.oauth.Enabled() {
		resp.OK(c, gin.H{"enabled": false})
		return
	}
	url, err := h.oauth.AuthorizeURL()
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"enabled": true, "url": url})
}

// OAuthCallback GitHub 授权回调（GET /api/v1/admin/plugins/oauth/callback?code=&state=，M3.5）。
// 说明：state 为 P2 加固新增（CSRF 校验，缺失/过期拒绝）；回调后 token 加密存储 + ghclient 更新。
func (h *PluginHandler) OAuthCallback(c *gin.Context) {
	if h.oauth == nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	status, err := h.oauth.Callback(c.Request.Context(), c.Query("code"), c.Query("state"))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"status": status})
}

// OAuthStatus 查询连接状态（GET /api/v1/admin/plugins/oauth/status，M3.5）。
func (h *PluginHandler) OAuthStatus(c *gin.Context) {
	if h.oauth == nil {
		resp.OK(c, gin.H{"status": service.OAuthStatusDTO{Enabled: false}})
		return
	}
	status, err := h.oauth.Status(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"status": status})
}

// OAuthDisconnect 断开 GitHub 连接（POST /api/v1/admin/plugins/oauth/disconnect，M3.5）。
func (h *PluginHandler) OAuthDisconnect(c *gin.Context) {
	if h.oauth == nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	// 断开后回退 .env 静态 token（由 server 装配注入到 OAuthService 的 fallback）
	if err := h.oauth.Disconnect(c.Request.Context(), h.oauth.FallbackToken()); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"disconnected": true})
}
