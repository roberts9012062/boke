// internal/handler/auth.go
// 认证控制器：HTTP 参数绑定与响应组装（无业务判断，全部委托 service 层）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/auth"
	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// AuthHandler 认证控制器（连接器类）。
type AuthHandler struct {
	auth *service.AuthService
	jwt  *auth.Manager
}

// NewAuthHandler 创建认证控制器。
func NewAuthHandler(authSvc *service.AuthService, jwtMgr *auth.Manager) *AuthHandler {
	return &AuthHandler{auth: authSvc, jwt: jwtMgr}
}

// ForgotPassword 请求密码重置（POST /api/v1/auth/forgot-password，body: {email}）。
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"` // 注册邮箱
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.auth.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"sent": true})
}

// ResetPassword 重置密码（POST /api/v1/auth/reset-password，body: {token, new_password}）。
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token"`        // 重置令牌
		NewPassword string `json:"new_password"` // 新密码
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.auth.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"reset": true})
}

// refreshReq 刷新令牌请求体。
type refreshReq struct {
	RefreshToken string `json:"refresh_token"` // refresh 令牌
}

// Register 处理注册请求（POST /api/v1/auth/register）。
func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	// 绑定失败细节由 service 校验（handler 不做业务判断）
	tokenPair, err := h.auth.Register(c.Request.Context(), req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, tokenPair)
}

// Login 处理登录请求（POST /api/v1/auth/login）。
func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	tokenPair, err := h.auth.Login(c.Request.Context(), req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, tokenPair)
}

// Logout 处理登出请求（POST /api/v1/auth/logout，需登录）。
func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshReq
	// 请求体缺省时忽略（登出不强制携带 refresh token）
	_ = c.ShouldBindJSON(&req)

	// 解析 refresh token 取 tokenID（用于撤销黑名单）
	tokenID := ""
	if req.RefreshToken != "" {
		if claims, err := h.jwt.ParseRefresh(req.RefreshToken); err == nil {
			tokenID = claims.TokenID
		}
	}
	if err := h.auth.Logout(c.Request.Context(), tokenID, middleware.GetUserID(c), c.ClientIP(), c.Request.UserAgent()); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"logged_out": true})
}

// Refresh 处理令牌刷新（POST /api/v1/auth/refresh）。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	tokenPair, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, tokenPair)
}

// Me 处理当前用户资料（GET /api/v1/me，需登录）。
func (h *AuthHandler) Me(c *gin.Context) {
	profile, err := h.auth.GetProfile(c.Request.Context(), middleware.GetUserID(c), true)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, profile)
}
