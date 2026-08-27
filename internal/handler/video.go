// internal/handler/video.go
// B站视频公开桥接控制器（bilibili-video 插件）：匿名访客的播放/扫码通道。
//
// 背景：插件代理 API 挂 RequireAuth（匿名 401），而帖子内嵌视频要求游客可播放、
// 游客可扫码登录自己的 B 站账号——故宿主开公开端点、以 System 身份调用插件
// （与音乐桥接 /api/v1/music/:provider/url 同模式；插件端按 need_login 语义
// 自行区分访客 token 与站长会话）。响应直通插件 JSON。
package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	sdk "github.com/roberts9012062/boke/pkg/plugin-sdk"
)

// bilibiliPluginID B站视频插件 ID（桥接目标）。
const bilibiliPluginID = "bilibili-video"

// bridgeSystemCaller 公开桥接统一系统调用者身份（插件侧放行匿名语义端点）。
var bridgeSystemCaller = sdk.CallerIdentity{System: true}

// VideoHandler B站视频公开桥接控制器（连接器类）。
type VideoHandler struct {
	pluginSvc *service.PluginService // 插件服务（CallAPI 直达插件进程）
}

// NewVideoHandler 创建 B站视频桥接控制器。
func NewVideoHandler(pluginSvc *service.PluginService) *VideoHandler {
	return &VideoHandler{pluginSvc: pluginSvc}
}

// bridge 通用桥接：透传请求 body 到插件端点，响应直通（纯转发，无业务）。
func (h *VideoHandler) bridge(c *gin.Context, pluginPath string) {
	if h.pluginSvc == nil {
		c.JSON(503, gin.H{"error": "插件服务未配置"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(400, gin.H{"error": "请求体读取失败"})
		return
	}
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), bilibiliPluginID, "POST", pluginPath, body, bridgeSystemCaller)
	if err != nil {
		c.JSON(503, gin.H{"error": "B站视频插件未启用或不可达"})
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}

// Resolve 解析 B 站地址 → 视频信息 + 清晰度档位（POST /api/v1/video/bilibili/resolve）。
func (h *VideoHandler) Resolve(c *gin.Context) {
	h.bridge(c, "/resolve")
}

// URL 解析播放地址（POST /api/v1/video/bilibili/url；body 含 bvid/cid/qn/guest_token）。
func (h *VideoHandler) URL(c *gin.Context) {
	h.bridge(c, "/video/url")
}

// QrInit 游客扫码初始化（POST /api/v1/video/bilibili/qr-init）。
func (h *VideoHandler) QrInit(c *gin.Context) {
	h.bridge(c, "/qr-init")
}

// GuestQrCheck 游客扫码轮询（POST /api/v1/video/bilibili/guest-qr-check；成功签发 guest_token）。
func (h *VideoHandler) GuestQrCheck(c *gin.Context) {
	h.bridge(c, "/guest-qr-check")
}

// GuestStatus 游客 token 有效性查询（POST /api/v1/video/bilibili/guest-status）。
func (h *VideoHandler) GuestStatus(c *gin.Context) {
	h.bridge(c, "/guest-status")
}

// Image B 站图床代理（GET /api/v1/video/bilibili/image?src=<图床地址>，公开）。
// 背景：B 站图床（*.hdslb.com：封面/头像）同样有 Referer 防盗链，webview 直连
// 可能注入页面 Referer 即 403——前端封面/头像统一经本同源代理加载。
// 说明：B 站接口返回的图床地址可能是 http（如 view 的 pic），统一升级 https 请求。
func (h *VideoHandler) Image(c *gin.Context) {
	src := c.Query("src")
	parsed, err := url.Parse(src)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || !strings.HasSuffix(parsed.Host, ".hdslb.com") {
		c.JSON(403, gin.H{"error": "非法图片地址"})
		return
	}
	parsed.Scheme = "https"
	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		c.JSON(400, gin.H{"error": "图片地址无效"})
		return
	}
	req.Header.Set("User-Agent", streamUA) // 图床同样要求浏览器 UA + 空 Referer
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(502, gin.H{"error": "图片拉取失败"})
		return
	}
	defer resp.Body.Close()
	if v := resp.Header.Get("Content-Type"); v != "" {
		c.Header("Content-Type", v)
	}
	c.Header("Cache-Control", "public, max-age=3600") // 图床内容可短期缓存
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// streamUA 流代理统一浏览器 UA（B 站 CDN 要求：浏览器 UA + 空 Referer 才放行）。
const streamUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// Stream 视频流代理（GET /api/v1/video/bilibili/stream?src=<CDN 地址>，公开）。
// 背景：B 站 CDN 对 Referer 严格校验（页面直连时部分 webview 注入 Referer 即 403），
// 前端统一经本同源代理加载——服务端以「浏览器 UA + 无 Referer」转发并透传 Range，
// 浏览器按同源流播放（拖动进度条走 Range 请求）。
// allowedStreamHost B 站视频 CDN 域群白名单（纯函数；仅 https）。
// B 站 playurl 多节点轮询，流地址域名不止 bilivideo.com：
//   *.bilivideo.com / *.bilivideo.cn（含 mcdn.bilivideo.cn 等子域）
//   upos-*.akamaized.net（海外 CDN；限定 upos 前缀防泛 akamaized 滥用）
func allowedStreamHost(host string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))
	if lower == "" {
		return false
	}
	if strings.HasSuffix(lower, ".bilivideo.com") || lower == "bilivideo.com" {
		return true
	}
	if strings.HasSuffix(lower, ".bilivideo.cn") || lower == "bilivideo.cn" {
		return true
	}
	if strings.HasPrefix(lower, "upos-") && strings.HasSuffix(lower, ".akamaized.net") {
		return true
	}
	return false
}

func (h *VideoHandler) Stream(c *gin.Context) {
	src := c.Query("src")
	parsed, err := url.Parse(src)
	if err != nil || parsed.Scheme != "https" || !allowedStreamHost(parsed.Host) {
		c.JSON(403, gin.H{"error": "非法视频源地址"})
		return
	}
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		c.JSON(400, gin.H{"error": "视频源地址无效"})
		return
	}
	req.Header.Set("User-Agent", streamUA)
	if rng := c.GetHeader("Range"); rng != "" {
		req.Header.Set("Range", rng) // 透传断点/分段请求（206 响应直通）
	}
	// 流式响应不能设整体超时（1080P 流可达几十 MB，整体超时会掐断长视频）——
	// 仅限制连接与响应头等待，body 由客户端读取节奏决定
	streamClient := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		c.JSON(502, gin.H{"error": "视频源拉取失败"})
		return
	}
	defer resp.Body.Close()
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(key); v != "" {
			c.Header(key, v)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}
