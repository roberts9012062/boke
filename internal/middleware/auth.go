// internal/middleware/auth.go
// JWT 鉴权中间件：解析 Authorization: Bearer <access_token>，
// 校验通过后将用户 ID 与角色注入上下文（供 handler/service 读取）。
package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yueyan/boke/internal/auth"
	"github.com/yueyan/boke/pkg/errs"
	"github.com/yueyan/boke/pkg/resp"
)

// 上下文键（gin.Context.Set/Get 使用）。
const (
	ctxUserIDKey = "auth_user_id" // 用户 ID
	ctxRoleKey   = "auth_role"    // 角色
)

// RequireAuth 返回 JWT 鉴权中间件（要求已登录）。
// manager 为 JWT 管理器（解析 access token）。
func RequireAuth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Authorization 头提取 Bearer token
		tokenString, err := extractBearerToken(c)
		if err != nil {
			resp.Fail(c, 401, errs.ErrUnauthorized)
			c.Abort()
			return
		}

		// 解析并校验 access token
		claims, err := manager.ParseAccess(tokenString)
		if err != nil {
			// token 过期：返回特定错误码（前端触发静默刷新）
			if errors.Is(err, auth.ErrTokenExpired) {
				resp.Fail(c, 401, errs.ErrTokenExpired)
				c.Abort()
				return
			}
			resp.Fail(c, 401, errs.ErrUnauthorized)
			c.Abort()
			return
		}

		// 注入用户 ID 与角色
		c.Set(ctxUserIDKey, claims.UserID)
		c.Set(ctxRoleKey, claims.Role)
		c.Next()
	}
}

// RequireAdmin 返回管理员角色中间件（要求已登录且角色为 admin）。
// 必须挂在 RequireAuth 之后。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ctxRoleKey)
		if role != "admin" {
			resp.Fail(c, 403, errs.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// OptionalAuth 返回可选鉴权中间件：携带有效 token 时注入用户身份，
// 未携带或 token 无效时不拦截（登录/匿名皆可访问的接口，如评论发表、帖子详情）。
// 说明：公开接口需识别登录用户（如作者查看自己的私密帖）时必须挂本中间件。
func OptionalAuth(manager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提取 Bearer token；无 token 直接放行（匿名访问）
		tokenString, err := extractBearerToken(c)
		if err != nil {
			c.Next()
			return
		}
		// 解析成功则注入身份；失败（过期/无效）也放行（按匿名处理）
		if claims, err := manager.ParseAccess(tokenString); err == nil {
			c.Set(ctxUserIDKey, claims.UserID)
			c.Set(ctxRoleKey, claims.Role)
		}
		c.Next()
	}
}

// GetUserID 从上下文读取当前用户 ID（鉴权后使用）。
func GetUserID(c *gin.Context) int64 {
	if v, ok := c.Get(ctxUserIDKey); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// GetRole 从上下文读取当前用户角色（鉴权后使用）。
func GetRole(c *gin.Context) string {
	if v, ok := c.Get(ctxRoleKey); ok {
		if role, ok := v.(string); ok {
			return role
		}
	}
	return "user"
}

// extractBearerToken 提取 Authorization 头中的 Bearer token。
func extractBearerToken(c *gin.Context) (string, error) {
	header := c.GetHeader("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("缺少 Bearer token")
	}
	return parts[1], nil
}
