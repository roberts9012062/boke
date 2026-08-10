// internal/handler/user.go
// 用户控制器：公开资料查询（他人主页），仅做参数绑定与响应组装。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yueyan/boke/internal/service"
	"github.com/yueyan/boke/pkg/errs"
	"github.com/yueyan/boke/pkg/resp"
)

// UserHandler 用户控制器（连接器类）。
type UserHandler struct {
	auth *service.AuthService
}

// NewUserHandler 创建用户控制器。
func NewUserHandler(authSvc *service.AuthService) *UserHandler {
	return &UserHandler{auth: authSvc}
}

// GetUser 处理用户公开资料（GET /api/v1/users/:id，邮箱脱敏）。
func (h *UserHandler) GetUser(c *gin.Context) {
	// 解析路径参数：用户 ID
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	// 他人视角：邮箱脱敏
	profile, err := h.auth.GetProfile(c.Request.Context(), id, false)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, profile)
}
