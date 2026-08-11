// internal/handler/role.go
// 角色权限控制器（M5，设计稿《后台角色》#96/#101）：
// 角色矩阵查询 + 权限域编辑（矩阵页「权限」入口）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// RoleHandler 角色权限控制器（连接器类）。
type RoleHandler struct {
	roles *service.RoleService // 角色权限服务
}

// NewRoleHandler 创建角色权限控制器。
func NewRoleHandler(roles *service.RoleService) *RoleHandler {
	return &RoleHandler{roles: roles}
}

// Matrix 角色矩阵（GET /api/v1/admin/roles：角色/人数/权限域/状态）。
func (h *RoleHandler) Matrix(c *gin.Context) {
	matrix, err := h.roles.Matrix(c.Request.Context())
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, matrix)
}

// UpdatePermissions 更新角色权限域（PUT /api/v1/admin/roles/:role/permissions，
// body: {permissions: [domains]}；superadmin 不可编辑；写 settings 持久化 + 即时生效）。
func (h *RoleHandler) UpdatePermissions(c *gin.Context) {
	role := c.Param("role")
	var req struct {
		Permissions []string `json:"permissions"` // 权限域列表
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.roles.UpdateRolePermissions(c.Request.Context(), role, req.Permissions,
		middleware.GetUserID(c), c.ClientIP(), c.Request.UserAgent()); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"role": role, "permissions": req.Permissions})
}
