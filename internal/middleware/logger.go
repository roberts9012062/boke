// internal/middleware/logger.go
// 请求日志中间件：记录每个请求的方法、路径、状态码、耗时与请求 ID，
// 输出到 zap 日志（logs/ 目录，由 server 装配时注入 logger）。
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger 返回请求日志中间件。
// logger 为已配置的输出器（文件 + 控制台）。
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// 记录请求开始
		logger.Info("请求开始",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("request_id", GetRequestID(c)),
			zap.String("ip", c.ClientIP()),
		)
		c.Next()
		// 记录请求结束（状态码与耗时）
		logger.Info("请求结束",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("request_id", GetRequestID(c)),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("cost", time.Since(start)),
		)
	}
}
