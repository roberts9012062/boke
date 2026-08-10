// pkg/resp/resp.go
// 统一响应结构（依据《架构设计文档》11.2 统一响应格式）。
//
// 约定：所有接口返回 JSON：
//   { "code": 0, "message": "ok", "data": {...}, "request_id": "req_xxx" }
// 成功时 code=0；失败时 code 为 errs 错误码，message 为面向用户的提示。
package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/errs"
)

// Body 统一响应体。
type Body struct {
	Code      int    `json:"code"`      // 错误码（0 = 成功）
	Message   string `json:"message"`   // 提示文案
	Data      any    `json:"data"`      // 业务数据（成功时）
	RequestID string `json:"request_id"` // 请求 ID（贯穿日志）
}

// requestIDFrom 从上下文读取请求 ID（由中间件注入，无则留空）。
func requestIDFrom(c *gin.Context) string {
	if rid, ok := c.Get("request_id"); ok {
		if s, ok := rid.(string); ok {
			return s
		}
	}
	return ""
}

// OK 返回成功响应（data 可为 nil）。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:      errs.CodeOK,
		Message:   "ok",
		Data:      data,
		RequestID: requestIDFrom(c),
	})
}

// Fail 返回失败响应（携带错误码与提示文案）。
func Fail(c *gin.Context, httpStatus int, err *errs.Err) {
	c.JSON(httpStatus, Body{
		Code:      err.Code,
		Message:   err.Message,
		Data:      nil,
		RequestID: requestIDFrom(c),
	})
}

// FailFrom 将任意 error 转为失败响应（未知错误归为系统内部错误）。
func FailFrom(c *gin.Context, err error) {
	bizErr := errs.From(err)
	// 默认 HTTP 状态码：鉴权 401 / 无权限 403 / 参数 400 / 未找到 404，其余 500
	status := http.StatusInternalServerError
	switch bizErr.Code {
	case errs.CodeUnauthorized, errs.CodeTokenExpired:
		status = http.StatusUnauthorized
	case errs.CodeForbidden:
		status = http.StatusForbidden
	case errs.CodeBadRequest, errs.CodeValidation, errs.CodeConflict, errs.CodeStateConflict:
		status = http.StatusBadRequest
	case errs.CodeNotFound:
		status = http.StatusNotFound
	case errs.CodeRateLimit:
		status = http.StatusTooManyRequests
	}
	Fail(c, status, bizErr)
}
