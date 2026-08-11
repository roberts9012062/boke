// internal/service/role.go
// 角色权限业务（M5，设计稿《后台角色》#96/#101）：
// 角色矩阵查询（角色/人数/权限域/状态）+ 权限域编辑（settings 持久化 + casbin 即时生效）。
//
// 设计：5 内置角色固定（superadmin/editor/author/visitor/restricted）；
//       权限矩阵默认 casbin.defaultMatrix，后台编辑写入 settings（role_permissions JSON），
//       重启由 server 读取后 InitPolicies 恢复；自定义角色后置（差异记录）。
package service

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/roberts9012062/boke/internal/casbin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// settings 键：自定义权限矩阵（角色 → 域数组，JSON）。
const rolePermissionsKey = "role_permissions"

// RoleMatrixItem 角色矩阵行（设计稿表格列：角色/人数/权限范围/状态/创建/操作）。
type RoleMatrixItem struct {
	Role        string   `json:"role"`        // 角色标识
	Count       int64    `json:"count"`       // 人数
	Permissions []string `json:"permissions"` // 权限域列表
	Status      string   `json:"status"`      // enabled 启用 / restricted 限制（受限访客）
	Builtin     bool     `json:"builtin"`     // 系统内置（不可删除）
}

// RoleMatrix 角色矩阵（设计稿统计条：角色数 5/管理员 N/编辑 N/访客 —）。
type RoleMatrix struct {
	Roles     []RoleMatrixItem `json:"roles"`     // 角色行（按内置顺序）
	RoleCount int              `json:"role_count"` // 角色数（内置 5）
	Total     int64            `json:"total"`      // 全部用户数
}

// RoleService 角色权限服务（连接器类）。
type RoleService struct {
	enforcer *casbin.Enforcer      // 权限执行器（矩阵策略）
	admin    *repository.AdminRepo // 后台聚合（角色人数统计）
	settings *repository.SettingRepo // 站点设置（权限矩阵持久化）
	audit    *repository.AuditRepo // 审计日志（权限变更留痕）
}

// NewRoleService 创建角色权限服务。
func NewRoleService(enforcer *casbin.Enforcer, admin *repository.AdminRepo, settings *repository.SettingRepo, audit *repository.AuditRepo) *RoleService {
	return &RoleService{enforcer: enforcer, admin: admin, settings: settings, audit: audit}
}

// Matrix 角色矩阵（5 内置角色 + 人数 + 当前权限域 + 状态）。
func (s *RoleService) Matrix(ctx context.Context) (*RoleMatrix, error) {
	counts, err := s.admin.CountUsersByRole(ctx)
	if err != nil {
		return nil, err
	}
	// 内置角色顺序（设计稿表格行序：超管/编辑/作者/访客/受限访客）
	order := []string{casbin.RoleSuperAdmin, casbin.RoleEditor, casbin.RoleAuthor, casbin.RoleVisitor, casbin.RoleRestricted}
	roles := make([]RoleMatrixItem, 0, len(order))
	for _, role := range order {
		item := RoleMatrixItem{
			Role:        role,
			Count:       counts[role],
			Permissions: s.enforcer.Permissions(role),
			Status:      "enabled",
			Builtin:     true,
		}
		if role == casbin.RoleRestricted {
			item.Status = "restricted" // 设计稿状态列：受限访客「限制」
		}
		roles = append(roles, item)
	}
	return &RoleMatrix{Roles: roles, RoleCount: len(order), Total: counts[casbin.RoleSuperAdmin] + counts[casbin.RoleEditor] + counts[casbin.RoleAuthor] + counts[casbin.RoleVisitor] + counts[casbin.RoleRestricted]}, nil
}

// UpdateRolePermissions 更新角色权限域（M5 矩阵「权限」入口）。
// 说明：superadmin 不可编辑（全量域）；权限域白名单校验；写 settings 持久化 +
//       casbin 重建策略即时生效；权限变更写审计（actor 由调用方传入）。
func (s *RoleService) UpdateRolePermissions(ctx context.Context, role string, domains []string, actorID int64, ip string, ua string) error {
	// 白名单校验
	if !casbin.IsBuiltinRole(role) {
		return errs.New(errs.CodeBadRequest, "角色仅支持 superadmin / editor / author / visitor / restricted")
	}
	if role == casbin.RoleSuperAdmin {
		return errs.New(errs.CodeStateConflict, "超级管理员拥有全部权限，不可编辑")
	}
	// 域白名单（去重 + 非法值拒绝）
	valid := make(map[string]bool, len(casbin.AllDomains))
	for _, d := range casbin.AllDomains {
		valid[d] = true
	}
	seen := make(map[string]bool, len(domains))
	clean := make([]string, 0, len(domains))
	for _, d := range domains {
		if valid[d] && !seen[d] {
			seen[d] = true
			clean = append(clean, d)
		}
	}
	sort.Strings(clean)

	// 读取当前自定义矩阵 → 更新该角色 → 写回 settings
	custom, err := s.loadCustomMatrix(ctx)
	if err != nil {
		return err
	}
	custom[role] = clean
	payload, err := json.Marshal(custom)
	if err != nil {
		return errs.New(errs.CodeInternal, "权限矩阵序列化失败")
	}
	// 写 settings 持久化（重启后恢复）→ casbin 重建策略即时生效。
	// 说明：先落库后重建；InitPolicies 为纯内存操作失败概率极低，失败窗口可忽略。
	if err := s.settings.SetJSON(ctx, rolePermissionsKey, string(payload)); err != nil {
		return err
	}
	if err := s.enforcer.InitPolicies(custom); err != nil {
		return errs.New(errs.CodeInternal, "权限矩阵应用失败")
	}

	// 审计（权限变更入 audit_logs，架构 9.2）
	before, _ := json.Marshal(casbin.DefaultMatrix()[role])
	after, _ := json.Marshal(clean)
	_ = s.audit.Insert(ctx, repository.AuditEntry{
		ActorID: actorID, Action: "update_role_permissions", ResourceType: "role",
		ResourceID: 0, BeforeData: string(before), AfterData: string(after),
		IP: ip, UserAgent: ua,
	})
	return nil
}

// loadCustomMatrix 读取自定义权限矩阵（settings role_permissions JSON；无记录返回空 map）。
func (s *RoleService) loadCustomMatrix(ctx context.Context) (map[string][]string, error) {
	raw, ok, err := s.settings.Get(ctx, rolePermissionsKey)
	if err != nil {
		return nil, err
	}
	custom := make(map[string][]string)
	if ok && raw != "" {
		// 解析失败降级为空矩阵（不阻断权限编辑）
		_ = json.Unmarshal([]byte(raw), &custom)
	}
	return custom, nil
}

// CustomMatrix 当前自定义矩阵（server 启动时读取以恢复权限编辑）。
func (s *RoleService) CustomMatrix(ctx context.Context) (map[string][]string, error) {
	return s.loadCustomMatrix(ctx)
}
