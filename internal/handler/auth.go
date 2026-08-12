// internal/handler/auth.go
// 认证控制器：HTTP 参数绑定与响应组装（无业务判断，全部委托 service 层）。
package handler

import (
	"time"

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

// 插件沙箱短期令牌有效期（1 小时；docs/plugin-dev-guide.md 8.2）。
const sandboxTokenTTL = time.Hour

// NewAuthHandler 创建认证控制器。
func NewAuthHandler(authSvc *service.AuthService, jwtMgr *auth.Manager) *AuthHandler {
	return &AuthHandler{auth: authSvc, jwt: jwtMgr}
}

// SandboxToken 签发插件 iframe 沙箱短期令牌（POST /api/v1/plugin-sandbox-token）。
// 说明：与登录令牌同格式（TokenType=access），插件 iframe 凭此直接调用插件代理 API；
//       有效期 1 小时，仅限当前登录用户上下文。
func (h *AuthHandler) SandboxToken(c *gin.Context) {
	token, err := h.jwt.GenerateShortToken(middleware.GetUserID(c), middleware.GetRole(c), sandboxTokenTTL)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"token": token, "expires_in": int(sandboxTokenTTL.Seconds())})
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

// ChangePassword 处理修改密码（PUT /api/v1/me/password，需登录；账号安全页）。
// body：{current_password, new_password}——校验当前密码后更新，密码版本自增使其他设备退出。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password"` // 当前密码
		NewPassword     string `json:"new_password"`     // 新密码
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.auth.ChangePassword(c.Request.Context(), middleware.GetUserID(c), req.CurrentPassword, req.NewPassword); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"changed": true})
}

// Deactivate 注销账号（POST /api/v1/me/deactivate，需登录；需求 3.9）。
// 删除用户与全部数据（不可恢复），成功后前端登出清会话。
func (h *AuthHandler) Deactivate(c *gin.Context) {
	if err := h.auth.DeactivateAccount(c.Request.Context(), middleware.GetUserID(c), c.ClientIP(), c.Request.UserAgent()); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deactivated": true})
}
