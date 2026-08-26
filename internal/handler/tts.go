// internal/handler/tts.go
// TTS 朗读公开桥接控制器（tts-reader 插件）：匿名访客的合成/音频通道。
//
// 背景：插件代理 API 挂 RequireAuth（匿名 401），而朗读要求游客无需登录即可听——
// 故宿主开公开端点、以 System 身份调用插件（与音乐桥接 /api/v1/music/*、
// B站视频桥接 /api/v1/video/bilibili/* 同模式）。响应直通插件数据。
//
// 端点（均为 POST + JSON body：SDK APIMux 精确匹配且代理丢弃 query，音频 id 经 body 传递）：
//   POST /api/v1/tts        合成（{text, voice?, rate?}）→ {id} 或 {error}
//   POST /api/v1/tts/audio  取音频（{id}）→ 原始音频字节（audio/mpeg）
//
// 防滥用：公开合成端点做每 IP 滑动窗口限流 + 请求体大小限制；文本长度上限由插件
// 侧配置强制（max_chars），宿主不重复业务校验。
package handler

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
)

// ttsPluginID 朗读插件 ID（桥接目标）。
const ttsPluginID = "tts-reader"

// 公开端点限流（每 IP 滑动窗口；合成是重资源操作，限制更严）。
const (
	ttsWindow       = time.Minute
	ttsSynthLimit   = 10     // 合成：10 次/分钟/IP
	ttsAudioLimit   = 60     // 取音频：60 次/分钟/IP（读缓存，较轻）
	ttsBucketCap    = 20000  // 限流桶上限（超过清空，防内存无限增长）
	ttsBodyMaxSize  = 1 << 20 // 合成请求体 1MB 上限
	ttsAudioBodyMax = 1 << 16 // 音频请求体 64KB 上限（仅 id）
)

// ttsBucket 单 IP 限流桶（滑动窗口计数）。
type ttsBucket struct {
	synthCount int
	audioCount int
	windowAt   time.Time
}

// TTSHandler 朗读公开桥接控制器（连接器类）。
type TTSHandler struct {
	pluginSvc *service.PluginService // 插件服务（CallAPI 直达插件进程）

	mu        sync.Mutex
	buckets   map[string]*ttsBucket // 限流桶（key=client IP）
	lastFlush time.Time             // 桶清空时间戳
}

// NewTTSHandler 创建朗读桥接控制器。
func NewTTSHandler(pluginSvc *service.PluginService) *TTSHandler {
	return &TTSHandler{
		pluginSvc: pluginSvc,
		buckets:   make(map[string]*ttsBucket),
		lastFlush: time.Now(),
	}
}

// allow 每 IP 滑动窗口限流判定并计数（超限返回 false）。
func (h *TTSHandler) allow(key string, kind string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	// 周期性清空桶（防 map 无限增长）
	if len(h.buckets) > ttsBucketCap || now.Sub(h.lastFlush) >= 10*time.Minute {
		h.buckets = make(map[string]*ttsBucket)
		h.lastFlush = now
	}
	b, ok := h.buckets[key]
	if !ok || now.Sub(b.windowAt) >= ttsWindow {
		b = &ttsBucket{windowAt: now}
		h.buckets[key] = b
	}
	switch kind {
	case "synth":
		if b.synthCount >= ttsSynthLimit {
			return false
		}
		b.synthCount++
	case "audio":
		if b.audioCount >= ttsAudioLimit {
			return false
		}
		b.audioCount++
	}
	return true
}

// jsonErr 输出 JSON 错误响应（纯转发辅助）。
func (h *TTSHandler) jsonOut(c *gin.Context, status int, data []byte) {
	c.Data(status, "application/json; charset=utf-8", data)
}

// Synthesize 合成朗读音频（POST /api/v1/tts；body {text, voice?, rate?} → {id}）。
func (h *TTSHandler) Synthesize(c *gin.Context) {
	if !h.allow(c.ClientIP(), "synth") {
		h.jsonOut(c, http.StatusTooManyRequests, []byte(`{"error":"请求过于频繁，请稍后再试"}`))
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, ttsBodyMaxSize))
	if err != nil {
		h.jsonOut(c, http.StatusBadRequest, []byte(`{"error":"请求体读取失败"}`))
		return
	}
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), ttsPluginID, http.MethodPost, "/tts", body, bridgeSystemCaller)
	if err != nil {
		h.jsonOut(c, http.StatusServiceUnavailable, []byte(`{"error":"朗读插件未启用或不可达"}`))
		return
	}
	h.jsonOut(c, status, data)
}

// Audio 取朗读音频（POST /api/v1/tts/audio；body {id} → 音频字节 audio/mpeg）。
func (h *TTSHandler) Audio(c *gin.Context) {
	if !h.allow(c.ClientIP(), "audio") {
		h.jsonOut(c, http.StatusTooManyRequests, []byte(`{"error":"请求过于频繁，请稍后再试"}`))
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, ttsAudioBodyMax))
	if err != nil {
		h.jsonOut(c, http.StatusBadRequest, []byte(`{"error":"请求体读取失败"}`))
		return
	}
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), ttsPluginID, http.MethodPost, "/tts/audio", body, bridgeSystemCaller)
	if err != nil {
		h.jsonOut(c, http.StatusServiceUnavailable, []byte(`{"error":"朗读插件未启用或不可达"}`))
		return
	}
	if status >= 400 {
		h.jsonOut(c, status, data)
		return
	}
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(status, "audio/mpeg", data)
}
