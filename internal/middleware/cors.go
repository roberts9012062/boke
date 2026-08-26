// internal/middleware/cors.go
// CORS 中间件：开发环境允许前端（localhost:3000）跨域访问 API。
// 按配置的来源白名单放行（架构文档 3.1 中间件链：CORS 环节）。
// 特例：/api/v1/open/ 开放网关面向浏览器插件等外部应用（X-Api-Key 本身即凭证），
//       回显任意请求来源并放行 X-Api-Key 头，保证跨域预检通过。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 开放网关路径前缀（命中即回显任意 Origin；产品语义：凭 Key 远程授权调用）。
const openAPIPathPrefix = "/api/v1/open/"

// CORS 返回跨域中间件。
// origin 为允许的跨域来源（如 http://localhost:3000），为空时允许全部来源。
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch {
		case strings.HasPrefix(c.Request.URL.Path, openAPIPathPrefix) && c.GetHeader("Origin") != "":
			// 开放网关：向任意外部应用（含 chrome-extension:// 扩展源）回显来源
			c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
			c.Header("Vary", "Origin")
		case origin != "":
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		default:
			c.Header("Access-Control-Allow-Origin", "*")
		}
		// 允许的请求头与方法（JWT 鉴权头 + API Key 头 + 媒体上传）
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Api-Key")
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
