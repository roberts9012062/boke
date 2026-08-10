// internal/middleware/request_id.go
// 请求 ID 中间件：为每个请求生成唯一 ID（透传 X-Request-ID），
// 贯穿日志与错误响应（架构文档 11.1：请求统一带 X-Request-ID）。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
)

// requestIDKey 上下文存取键（gin.Context.Set/Get 使用）。
const requestIDKey = "request_id"

// generateRequestID 生成请求 ID：时间戳 + 6 字节随机数（hex 编码）。
func generateRequestID() string {
	randBytes := make([]byte, 6)
	// 随机数生成失败时用时间戳兜底（概率极低，不影响可用性）
	_, _ = rand.Read(randBytes)
	return "req_" + time.Now().Format("20060102150405") + "_" + hex.EncodeToString(randBytes)
}

// RequestID 返回请求 ID 中间件。
// 客户端已携带 X-Request-ID 时透传，否则生成新的。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先透传客户端请求 ID，便于跨服务追踪
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = generateRequestID()
		}
		// 注入上下文，响应头与错误响应回显
		c.Set(requestIDKey, rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

// GetRequestID 从上下文读取请求 ID（供日志与统一响应使用）。
func GetRequestID(c *gin.Context) string {
	if rid, ok := c.Get(requestIDKey); ok {
		if s, ok := rid.(string); ok {
			return s
		}
	}
	return ""
}
