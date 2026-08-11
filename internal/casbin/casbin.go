// internal/casbin/casbin.go
// 权限策略（Casbin RBAC，MVP 两级角色：admin / user，架构文档 9.1）。
//
// 设计说明：
//   - 策略存内存（g, admin, admin：用户名 admin 的用户为管理员角色）
//   - 登录时由 auth service 查询角色并写入 JWT claims（鉴权中间件直接读 claims）
//   - M2 完整 RBAC 时切换 gorm adapter（策略落库、后台可编辑），对外接口不变
package casbin

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// 角色常量（与 JWT claims.role 对应）。
const (
	RoleAdmin = "admin" // 管理员
	RoleUser  = "user"  // 普通用户
)

// modelText RBAC 模型（标准 model.conf 内容，内嵌避免外部文件依赖）。
const modelText = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (r.obj == p.obj || p.obj == "*") && r.act == p.act
`

// Enforcer 权限执行器（连接器类，持有 Casbin 实例）。
type Enforcer struct {
	engine *casbin.Enforcer
}

// NewEnforcer 创建权限执行器并加载初始策略。
// 返回：执行器；初始化失败时返回错误。
func NewEnforcer() (*Enforcer, error) {
	// 从文本构建 RBAC 模型（NewEnforcer 直接传字符串会被当作文件路径）
	rbacModel, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, err
	}
	engine, err := casbin.NewEnforcer(rbacModel)
	if err != nil {
		return nil, err
	}
	// 兜底策略：用户名 admin 的用户归属管理员角色（DB 角色加载失败时仍可进入后台）
	if _, err := engine.AddGroupingPolicy("admin", RoleAdmin); err != nil {
		return nil, err
	}
	return &Enforcer{engine: engine}, nil
}

// SyncRoles 从数据库全量重建角色策略（服务启动时调用，替代硬编码）。
// 参数：roles 管理员名单（users.role = 'admin' 的用户；非管理员默认 user 角色，无需显式策略）。
func (e *Enforcer) SyncRoles(roles []UserRole) error {
	// 清空既有分组策略（含兜底 admin，以 DB 为准）
	if _, err := e.engine.RemoveFilteredGroupingPolicy(0, ""); err != nil {
		return err
	}
	for _, role := range roles {
		if role.Role != RoleAdmin {
			continue
		}
		if _, err := e.engine.AddGroupingPolicy(role.Username, RoleAdmin); err != nil {
			return err
		}
	}
	return nil
}

// SetRole 设置用户角色（M2 角色调整 UI：admin ↔ user）。
// 说明：内存策略即时生效（登录时按 username 查角色）；重启后由 SyncRoles 从 DB 重建。
func (e *Enforcer) SetRole(username string, role string) error {
	if role == RoleAdmin {
		_, err := e.engine.AddGroupingPolicy(username, RoleAdmin)
		return err
	}
	_, err := e.engine.RemoveGroupingPolicy(username, RoleAdmin)
	return err
}

// GetRole 查询用户角色（admin / user）。
// 参数：username 用户名。
func (e *Enforcer) GetRole(username string) string {
	roles, err := e.engine.GetRolesForUser(username)
	if err != nil {
		return RoleUser
	}
	for _, role := range roles {
		if role == RoleAdmin {
			return RoleAdmin
		}
	}
	return RoleUser
}

// Enforce 校验权限（M2 扩展使用：sub, obj, act）。
func (e *Enforcer) Enforce(sub string, obj string, act string) (bool, error) {
	return e.engine.Enforce(sub, obj, act)
}

// UserRole 用户角色（SyncRoles 全量加载输入，与 repository.UserRoleRow 对应）。
type UserRole struct {
	Username string // 账号名（策略主体）
	Role     string // 角色：admin / user
}
