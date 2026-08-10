// internal/middleware/recovery.go
// 恢复中间件：捕获 handler 层未处理 panic，记录错误日志并返回统一 500 响应，
// 避免单个请求异常导致进程崩溃（架构文档 3.1 中间件链：恢复 → 请求ID → …）。
package middleware

import (
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// Recovery 返回恢复中间件。
// logger 用于记录 panic 详情（含请求路径与堆栈）。
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 记录 panic 详情：请求路径、请求 ID、堆栈
				logger.Error("请求处理发生 panic",
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", GetRequestID(c)),
					zap.Any("panic", r),
					zap.Stack("stack"),
				)
				// 返回统一内部错误响应
				resp.Fail(c, 500, errs.New(errs.CodeInternal, "系统繁忙，请稍后再试"))
				// 中止后续 handler，避免重复响应
				c.Abort()
			}
		}()
		c.Next()
	}
}
