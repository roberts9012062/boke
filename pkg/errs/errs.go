// pkg/errs/errs.go
// 统一错误码体系（依据《架构设计文档》11.3 错误码段位划分）。
//
// 段位约定：
//   0xxx 成功；1xxx 鉴权；2xxx 参数校验；3xxx 资源冲突；
//   4xxx 插件；5xxx AI；6xxx 系统内部。
// 使用方式：业务层返回 *Err（含错误码与面向用户的提示文案），
//           handler 层经 resp.Fail 统一序列化。
package errs

import (
	"errors"
	"fmt"
)

// Err 业务错误：携带错误码与用户可读提示。
type Err struct {
	Code    int    // 错误码（见各段位常量）
	Message string // 面向用户的提示文案（前端 Toast 直接展示）
}

// Error 实现 error 接口（同时保留结构化信息）。
func (e *Err) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// New 构造业务错误。
func New(code int, message string) *Err {
	return &Err{Code: code, Message: message}
}

// From 将任意 error 包装为业务错误（未知错误统一归为系统内部错误 6001）。
func From(err error) *Err {
	var bizErr *Err
	if errors.As(err, &bizErr) {
		return bizErr
	}
	return &Err{Code: CodeInternal, Message: "系统繁忙，请稍后再试"}
}

// ---------- 0xxx：成功 ----------

const (
	CodeOK int = 0 // 成功
)

// ---------- 1xxx：鉴权 ----------

const (
	CodeUnauthorized int = 1001 // 未登录（缺少或无效凭证）
	CodeTokenExpired int = 1002 // token 已过期
	CodeForbidden    int = 1003 // 无权限（角色不足）
)

// ---------- 2xxx：参数校验 ----------

const (
	CodeBadRequest    int = 2001 // 参数错误
	CodeNotFound      int = 2002 // 资源不存在
	CodeValidation    int = 2003 // 校验失败（业务规则不满足）
)

// ---------- 3xxx：资源冲突 ----------

const (
	CodeConflict      int = 3001 // 重名/重复提交
	CodeStateConflict int = 3002 // 当前状态不允许该操作
)

// ---------- 6xxx：系统 ----------

const (
	CodeInternal   int = 6001 // 系统内部错误
	CodeUpstream   int = 6002 // 上游依赖不可用
	CodeRateLimit  int = 6003 // 请求过于频繁（限流）
)

// 常用业务错误实例（避免重复构造）。
var (
	ErrUnauthorized = New(CodeUnauthorized, "请先登录")
	ErrTokenExpired = New(CodeTokenExpired, "登录已过期，请重新登录")
	ErrForbidden    = New(CodeForbidden, "没有操作权限")
	ErrNotFound     = New(CodeNotFound, "资源不存在")
	ErrBadRequest   = New(CodeBadRequest, "参数错误")
	ErrRateLimit    = New(CodeRateLimit, "操作过于频繁，请稍后再试")
)
