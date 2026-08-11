// internal/middleware/maintenance.go
// 全站维护中间件（M2）：维护开关开启时拦截前台 API（503），放行以下路径：
//   - /api/v1/admin/*：管理员可进入后台关闭开关
//   - /api/v1/auth/*：登录/刷新接口保持可用（否则维护中无法登录后台）
//   - /api/v1/me：登录态资料（后台页面加载与管理员登录流程依赖，维护期间后台可访问的前提）
//   - /api/v1/meta：站点元信息（前端维护页展示与拦截判定需要）
// 说明：/healthz 与 /media 静态资源在 engine 层注册，不经过本中间件。
package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// Maintenance 返回全站维护中间件。
// 参数：isOn 维护开关判定函数（读取 settings，实时生效）。
func Maintenance(isOn func(ctx context.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 放行：后台 / 认证 / 登录态资料 / 站点元信息
		if strings.HasPrefix(path, "/api/v1/admin") ||
			strings.HasPrefix(path, "/api/v1/auth") ||
			path == "/api/v1/me" ||
			path == "/api/v1/meta" {
			c.Next()
			return
		}
		// 维护开启：统一返回 503 维护错误（前端捕获后跳转维护页）
		if isOn(c.Request.Context()) {
			resp.Fail(c, 503, errs.ErrMaintenance)
			c.Abort()
			return
		}
		c.Next()
	}
}
