// internal/middleware/cors.go
// CORS 中间件：开发环境允许前端（localhost:3000）跨域访问 API。
// 按配置的来源白名单放行（架构文档 3.1 中间件链：CORS 环节）。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 返回跨域中间件。
// origin 为允许的跨域来源（如 http://localhost:3000），为空时允许全部来源。
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 按配置决定来源白名单
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		// 允许的请求头与方法（JWT 鉴权头 + 媒体上传）
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "86400")

		// 预检请求直接返回 204
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
