// pkg/plugin-sdk/caller.go
// 插件 API 调用者身份（P1 加固）：宿主代理经契约字段（contract.APICall 的
// CallerID/CallerRole/CallerSystem）内联透传调用者，插件侧在自定义 API handler
// 中查询身份做 per-endpoint 鉴权。
// 两种调用来源：
//   - 用户态（System=false）：/api/v1/plugins/{id}/** 代理，携带登录用户 ID 与角色；
//   - 系统态（System=true）：宿主核心内部桥接调用（如公开音乐播放代理），
//     非外部用户发起，插件可信任（宿主自身已做产品级公开决策）。
package sdk

import (
	"context"
)

// CallerIdentity 调用者身份（宿主代理透传；系统调用 System=true）。
type CallerIdentity struct {
	UserID int64  // 用户 ID（0=匿名/未知）
	Role   string // 角色名（superadmin/admin/editor/author/visitor；空=未知）
	System bool   // 系统调用（宿主内部桥接，非外部用户）
}

// callerIdentityKey context 键类型（避免与其他包冲突）。
type callerIdentityKey struct{}

// WithCallerIdentity 注入调用者身份到 ctx（server 侧解析 metadata 后调用；插件作者不调用）。
func WithCallerIdentity(ctx context.Context, id CallerIdentity) context.Context {
	return context.WithValue(ctx, callerIdentityKey{}, id)
}

// CallerFrom 取调用者身份（未透传返回零值——按最小权限处理：非系统、无角色）。
func CallerFrom(ctx context.Context) CallerIdentity {
	if ctx == nil {
		return CallerIdentity{}
	}
	id, ok := ctx.Value(callerIdentityKey{}).(CallerIdentity)
	if !ok {
		return CallerIdentity{}
	}
	return id
}

// CallerID 取调用者用户 ID（未透传返回 0）。
func CallerID(ctx context.Context) int64 {
	return CallerFrom(ctx).UserID
}

// CallerRole 取调用者角色名（未透传返回空串）。
func CallerRole(ctx context.Context) string {
	return CallerFrom(ctx).Role
}

// CallerIsSystem 调用是否来自宿主系统桥接（公开播放代理等）。
func CallerIsSystem(ctx context.Context) bool {
	return CallerFrom(ctx).System
}

// CallerIsAdmin 调用者是否管理员角色（superadmin/admin；系统调用不算——语义仅描述用户角色）。
func CallerIsAdmin(ctx context.Context) bool {
	role := CallerFrom(ctx).Role
	return role == "superadmin" || role == "admin"
}

// TrustedCaller 调用者是否可执行管理操作（系统桥接或管理员用户）。
// 插件管理端点（登录导入/登出/配置写入等站点级操作）建议统一用它拦截。
func TrustedCaller(ctx context.Context) bool {
	id := CallerFrom(ctx)
	return id.System || id.Role == "superadmin" || id.Role == "admin"
}
