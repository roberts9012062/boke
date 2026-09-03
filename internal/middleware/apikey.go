// internal/middleware/apikey.go
// 开放接口 API Key 鉴权中间件（/api/v1/open/* 网关）：
// 校验 X-Api-Key 请求头 → 查凭证 → 校验未过期 → 按路由模板反查目录得到接口标识
// → 校验该 Key 已授权此接口 → 放行并异步记录最近调用时间。
// 说明：中间件挂在开放组级；接口标识用 gin FullPath()（路由模板，如 /api/v1/open/posts/:id）
//       + Method 反查目录索引，无需在每个路由上单独标注。
package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// X-Api-Key 请求头常量（外部应用调用开放接口的凭证头）。
const apiKeyHeader = "X-Api-Key"

// gin context 键：当前凭证绑定的用户 ID（0=未绑定，/open/me 等身份类接口使用）。
const apiKeyUserIDKey = "apikey_user_id"

// catalogRouteIndex 目录「Method + 路由模板 → 接口标识」索引（进程内单次构建）。
var catalogRouteIndex = model.CatalogIndex()

// ApiKeyAuth 返回开放接口鉴权中间件。
// keys 为凭证仓库；放行的请求以匿名视角复用现有公开 handler（私密帖不可见）。
// pluginRoutes 为插件声明的开放端点索引（「Method + 实际路径 → 接口标识」，
// 泛化网关 /open/plugins/:id/*path 用——其 FullPath 是通配模板，须按实际路径匹配）；
// 传 nil 表示无插件声明（聚合器未装配的降级场景）。
func ApiKeyAuth(keys *repository.OpenAPIKeyRepo, pluginRoutes func() map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ---------- 提取凭证 ----------
		key := c.GetHeader(apiKeyHeader)
		if key == "" {
			resp.Fail(c, 401, errs.New(errs.CodeUnauthorized, "缺少 X-Api-Key 请求头"))
			c.Abort()
			return
		}

		// ---------- 查询凭证（不存在 = 无效 Key） ----------
		record, err := keys.FindByKey(c.Request.Context(), key)
		if err != nil {
			resp.Fail(c, 401, errs.New(errs.CodeUnauthorized, "API Key 无效"))
			c.Abort()
			return
		}

		// ---------- 过期校验（expires_at 为空 = 永久有效） ----------
		if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
			resp.Fail(c, 401, errs.New(errs.CodeUnauthorized, "API Key 已过期"))
			c.Abort()
			return
		}

		// ---------- 接口授权校验（路由模板反查目录 → Key 绑定集合包含判断） ----------
		// 两级索引：静态目录按「Method + 路由模板」（FullPath，含 :id 参数段）；
		// 插件声明按「Method + 实际请求路径」（泛化通配路由的 FullPath 无法静态索引）
		endpoint, ok := catalogRouteIndex[c.Request.Method+" "+c.FullPath()]
		if !ok && pluginRoutes != nil {
			endpoint, ok = pluginRoutes()[c.Request.Method+" "+c.Request.URL.Path]
		}
		if !ok {
			// 目录外的路由不应注册在开放组（防御性兜底）
			resp.Fail(c, 404, errs.ErrNotFound)
			c.Abort()
			return
		}
		authorized := false
		for _, ep := range record.Endpoints {
			if ep == endpoint {
				authorized = true
				break
			}
		}
		if !authorized {
			resp.Fail(c, 403, errs.New(errs.CodeForbidden, "该 API Key 未授权此接口"))
			c.Abort()
			return
		}

		// ---------- 记录最近调用（异步、失败忽略，不阻塞业务请求） ----------
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = keys.TouchLastUsed(ctx, record.ID)
		}()

		// ---------- 注入凭证身份（供 /open/me 等凭 Key 取用户的接口读取） ----------
		c.Set(apiKeyUserIDKey, record.UserID)

		c.Next()
	}
}

// GetAPIKeyUserID 从上下文读取当前凭证绑定的用户 ID（ApiKeyAuth 放行后使用；0=未绑定）。
func GetAPIKeyUserID(c *gin.Context) int64 {
	if v, ok := c.Get(apiKeyUserIDKey); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}
