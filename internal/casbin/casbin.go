// internal/casbin/casbin.go
// 权限策略（Casbin RBAC，M5：完整五级角色矩阵，设计稿《后台角色》）。
//
// 设计说明：
//   - 策略存内存（p = 角色, 资源域, "access"；g = 用户名, 角色）
//   - users.role 是角色唯一数据源，启动时 SyncRoles 全量加载；登录时角色快照进 JWT
//   - 权限矩阵（角色 → 域）默认硬编码 defaultMatrix，后台可编辑（settings.role_permissions
//     JSON 覆盖），重启由 server 读取 settings 后 InitPolicies 恢复
//   - 自定义角色 / 策略落库（gorm adapter）后置（差异记录），对外接口不变
package casbin

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// 角色常量（与 JWT claims.role 对应，设计稿《后台角色》五角色）。
const (
	RoleSuperAdmin = "superadmin" // 超级管理员：全部权限（内置·不可删除）
	RoleEditor     = "editor"     // 编辑：内容·评论·媒体·审核（内容审核与发布）
	RoleAuthor     = "author"     // 作者：发布·媒体上传（后台仅自己内容）
	RoleVisitor    = "visitor"    // 访客：前台阅读·评论（默认注册角色）
	RoleRestricted = "restricted" // 受限访客：只读（禁言/限流）
)

// 资源域常量（后台功能区；act 统一 "access"，域级粒度，域内读写细分后置）。
const (
	DomainDashboard  = "dashboard"  // 仪表盘
	DomainPosts      = "posts"      // 内容管理
	DomainComments   = "comments"   // 评论管理
	DomainUsers      = "users"      // 用户管理
	DomainMedia      = "media"      // 媒体库
	DomainTags       = "tags"       // 标签分类
	DomainSettings   = "settings"   // 站点设置
	DomainRoles      = "roles"      // 角色权限
	DomainModeration = "moderation" // 内容治理（审核队列/敏感词/封禁）
	DomainPlugins    = "plugins"    // 插件
	DomainSeo        = "seo"        // SEO
	DomainAi         = "ai"         // AI 设置
	DomainReports    = "reports"    // 数据报表
	DomainBackups    = "backups"    // 备份导出
)

// AllDomains 全部资源域（superadmin 通配矩阵用）。
var AllDomains = []string{
	DomainDashboard, DomainPosts, DomainComments, DomainUsers, DomainMedia,
	DomainTags, DomainSettings, DomainRoles, DomainModeration, DomainPlugins,
	DomainSeo, DomainAi, DomainReports, DomainBackups,
}

// defaultMatrix 默认权限矩阵（角色 → 可访问域；设计稿权限范围对齐）。
var defaultMatrix = map[string][]string{
	RoleSuperAdmin: AllDomains, // 全部权限
	RoleEditor: { // 内容·评论·媒体·审核
		DomainDashboard, DomainPosts, DomainComments, DomainMedia, DomainTags,
		DomainModeration, DomainSeo, DomainAi, DomainReports,
	},
	RoleAuthor: { // 发布·媒体上传（posts 域仅本人内容，service 层数据隔离）
		DomainDashboard, DomainPosts, DomainMedia,
	},
	RoleVisitor:    {}, // 无后台
	RoleRestricted: {}, // 无后台（前台只读由 RequireNotRestricted 拦截）
}

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

// NewEnforcer 创建权限执行器并加载默认矩阵策略。
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
	e := &Enforcer{engine: engine}
	// 加载默认矩阵（p 策略）
	if err := e.InitPolicies(nil); err != nil {
		return nil, err
	}
	// 兜底策略：用户名 admin 的用户归属超级管理员（DB 角色加载失败时仍可进入后台；
	// 迁移 011 后无 admin 用户，影响为零）
	if _, err := engine.AddGroupingPolicy("admin", RoleSuperAdmin); err != nil {
		return nil, err
	}
	return e, nil
}

// InitPolicies 重建权限矩阵策略（p：角色 → 域）。
// 参数：custom 自定义矩阵（角色 → 域数组；来自 settings.role_permissions），
//       覆盖默认矩阵（superadmin 强制全量，不可收缩）；nil 时仅用默认矩阵。
func (e *Enforcer) InitPolicies(custom map[string][]string) error {
	// 清空既有 p 策略（以矩阵为准重建）
	if _, err := e.engine.RemoveFilteredPolicy(0, "", "", ""); err != nil {
		return err
	}
	// 合并矩阵：默认 + 自定义覆盖（superadmin 始终全量）
	matrix := make(map[string][]string, len(defaultMatrix))
	for role, domains := range defaultMatrix {
		matrix[role] = domains
	}
	for role, domains := range custom {
		if role == RoleSuperAdmin {
			continue // 超级管理员不可收缩
		}
		matrix[role] = domains
	}
	// 加载 p 策略（act 统一 access；superadmin 用通配 obj）
	for role, domains := range matrix {
		if role == RoleSuperAdmin {
			if _, err := e.engine.AddPolicy(role, "*", "access"); err != nil {
				return err
			}
			continue
		}
		for _, domain := range domains {
			if _, err := e.engine.AddPolicy(role, domain, "access"); err != nil {
				return err
			}
		}
	}
	return nil
}

// SyncRoles 从数据库全量重建用户角色分组（服务启动时调用）。
// 参数：roles 全量用户角色（users.role 唯一数据源；所有角色均需显式策略）。
func (e *Enforcer) SyncRoles(roles []UserRole) error {
	// 清空既有分组策略（含兜底 admin，以 DB 为准）
	if _, err := e.engine.RemoveFilteredGroupingPolicy(0, ""); err != nil {
		return err
	}
	for _, role := range roles {
		if !IsBuiltinRole(role.Role) {
			continue // 未知角色跳过（数据异常兜底）
		}
		if _, err := e.engine.AddGroupingPolicy(role.Username, role.Role); err != nil {
			return err
		}
	}
	return nil
}

// SetRole 设置用户角色（M5：五级角色即时生效；先移除旧分组再加新分组）。
// 说明：内存策略即时生效（登录时按 username 查角色）；重启后由 SyncRoles 从 DB 重建。
func (e *Enforcer) SetRole(username string, role string) error {
	if !IsBuiltinRole(role) {
		return errInvalidRole
	}
	// 移除该用户全部角色分组（防残留旧角色）
	if _, err := e.engine.RemoveFilteredGroupingPolicy(0, username); err != nil {
		return err
	}
	_, err := e.engine.AddGroupingPolicy(username, role)
	return err
}

// RemoveRole 移除用户全部分组（账号注销后清理内存策略，重启前不残留已注销用户的角色映射）。
func (e *Enforcer) RemoveRole(username string) error {
	_, err := e.engine.RemoveFilteredGroupingPolicy(0, username)
	return err
}

// GetRole 查询用户角色（默认 visitor）。
func (e *Enforcer) GetRole(username string) string {
	roles, err := e.engine.GetRolesForUser(username)
	if err != nil {
		return RoleVisitor
	}
	for _, role := range roles {
		if IsBuiltinRole(role) {
			return role
		}
	}
	return RoleVisitor
}

// Enforce 校验权限（sub, obj, act；M5 中间件 RequirePermission 使用）。
func (e *Enforcer) Enforce(sub string, obj string, act string) (bool, error) {
	return e.engine.Enforce(sub, obj, act)
}

// Permissions 查询角色当前权限域（矩阵页展示；superadmin 返回全量域）。
// 说明：按 AllDomains 固定顺序输出（casbin 策略顺序受 map 遍历影响不稳定）。
func (e *Enforcer) Permissions(role string) []string {
	if role == RoleSuperAdmin {
		return AllDomains
	}
	policies, err := e.engine.GetFilteredPolicy(0, role)
	if err != nil {
		return nil
	}
	has := make(map[string]bool, len(policies))
	for _, p := range policies {
		if len(p) >= 2 && p[1] != "*" {
			has[p[1]] = true
		}
	}
	domains := make([]string, 0, len(AllDomains))
	for _, d := range AllDomains {
		if has[d] {
			domains = append(domains, d)
		}
	}
	return domains
}

// UserRole 用户角色（SyncRoles 全量加载输入，与 repository.UserRoleRow 对应）。
type UserRole struct {
	Username string // 账号名（策略主体）
	Role     string // 角色：superadmin / editor / author / visitor / restricted
}

// errInvalidRole 非法角色（SetRole 白名单校验失败）。
var errInvalidRole = &roleError{"角色仅支持 superadmin / editor / author / visitor / restricted"}

// roleError 角色错误（简单包装，供上层识别）。
type roleError struct {
	msg string
}

// Error 实现 error 接口。
func (e *roleError) Error() string {
	return e.msg
}

// IsBuiltinRole 是否为内置角色（纯函数）。
func IsBuiltinRole(role string) bool {
	switch role {
	case RoleSuperAdmin, RoleEditor, RoleAuthor, RoleVisitor, RoleRestricted:
		return true
	}
	return false
}

// DefaultMatrix 返回默认权限矩阵副本（对外只读，纯函数）。
func DefaultMatrix() map[string][]string {
	clone := make(map[string][]string, len(defaultMatrix))
	for role, domains := range defaultMatrix {
		clone[role] = append([]string{}, domains...)
	}
	return clone
}
