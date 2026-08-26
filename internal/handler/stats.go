// internal/handler/stats.go
// 站点统计公开桥接控制器（stats-pro 插件）：匿名访客的浏览上报通道。
//
// 背景：插件代理 API 挂 RequireAuth（匿名 401），而流量统计要求访客无需登录
// 即可上报——故宿主开公开端点、以 System 身份调用插件（与 /api/v1/tts、
// /api/v1/music/* 桥接同模式）。管理员查看统计走插件代理 API（登录态）。
//
// 端点：POST /api/v1/stats/hit（body {post_id?, visitor_id?} → {counted}）
//
// 防滥用：每 IP 滑动窗口限流 + 请求体大小限制；post_id/visitor_id 合法性
// 由插件侧校验（宿主不重复业务规则）。
package handler

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
)

// statsPluginID 统计插件 ID（桥接目标）。
const statsPluginID = "stats-pro"

// 公开端点限流与请求体上限。
const (
	statsWindow  = time.Minute
	statsHitLimit = 120   // 上报：120 次/分钟/IP（正常浏览远低于此，防脚本刷量）
	statsBucketCap = 20000 // 限流桶上限（超过清空，防内存无限增长）
	statsBodyMax  = 1 << 12 // 请求体 4KB 上限（仅两个短字段）
)

// statsBucket 单 IP 限流桶（滑动窗口计数）。
type statsBucket struct {
	count    int
	windowAt time.Time
}

// StatsHandler 统计公开桥接控制器（连接器类）。
type StatsHandler struct {
	pluginSvc *service.PluginService // 插件服务（CallAPI 直达插件进程）

	mu        sync.Mutex
	buckets   map[string]*statsBucket // 限流桶（key=client IP）
	lastFlush time.Time               // 桶清空时间戳
}

// NewStatsHandler 创建统计桥接控制器。
func NewStatsHandler(pluginSvc *service.PluginService) *StatsHandler {
	return &StatsHandler{
		pluginSvc:  pluginSvc,
		buckets:    make(map[string]*statsBucket),
		lastFlush:  time.Now(),
	}
}

// allow 每 IP 滑动窗口限流判定并计数（超限返回 false）。
func (h *StatsHandler) allow(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	// 周期性清空桶（防 map 无限增长）
	if len(h.buckets) > statsBucketCap || now.Sub(h.lastFlush) >= 10*time.Minute {
		h.buckets = make(map[string]*statsBucket)
		h.lastFlush = now
	}
	b, ok := h.buckets[key]
	if !ok || now.Sub(b.windowAt) >= statsWindow {
		b = &statsBucket{windowAt: now}
		h.buckets[key] = b
	}
	if b.count >= statsHitLimit {
		return false
	}
	b.count++
	return true
}

// Hit 访客浏览上报（POST /api/v1/stats/hit；转发插件 /hit，System 身份）。
func (h *StatsHandler) Hit(c *gin.Context) {
	if !h.allow(c.ClientIP()) {
		c.Data(http.StatusTooManyRequests, "application/json; charset=utf-8",
			[]byte(`{"error":"上报过于频繁，请稍后再试"}`))
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, statsBodyMax))
	if err != nil {
		c.Data(http.StatusBadRequest, "application/json; charset=utf-8",
			[]byte(`{"error":"请求体读取失败"}`))
		return
	}
	status, data, err := h.pluginSvc.CallAPI(c.Request.Context(), statsPluginID, http.MethodPost, "/hit", body, bridgeSystemCaller)
	if err != nil {
		// 插件未启用：静默成功（前台脚本无感知，统计缺席不报错打扰访客）
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(`{"counted":false}`))
		return
	}
	c.Data(status, "application/json; charset=utf-8", data)
}
