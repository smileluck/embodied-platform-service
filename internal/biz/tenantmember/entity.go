// Package tenantmember 租户成员限界上下文 —— 领域层。
//
// 租户端用户 = 平台应用用户（AppUser）：身份/账密/多租户归属全部在平台
// （唯一身份源），登录由租户端前端直调平台 /app-auth（token 双用），
// AppAuth 中间件已按「归属即准入」做租户闸门。本上下文只补租户级授权层：
// 「平台 app_user × 平台租户 ID」的本地角色绑定（内置双角色，不建角色表），
// 成员资料经 appuser.Gateway 实时消费平台开放面，本地不落成员数据面。
// 与 B 端 admission 投影同一哲学：身份在平台，授权在本地。
package tenantmember

import (
	"context"
	"errors"
)

// Role 内置租户端角色（固定两档，不与 B 端全局 RBAC 混用；无绑定视为 member）
type Role string

const (
	// RoleTenantAdmin 租户管理员：可管理本租户成员与角色
	RoleTenantAdmin Role = "tenant_admin"
	// RoleMember 普通成员：只读本租户资源（绑定缺失的默认档）
	RoleMember Role = "member"
)

// ValidRole 是否合法内置角色
func ValidRole(r Role) bool { return r == RoleTenantAdmin || r == RoleMember }

// NormalizeRole 绑定缺失/未知值归一为 member（展示与判定共用）
func NormalizeRole(r Role) Role {
	if r == RoleTenantAdmin {
		return RoleTenantAdmin
	}
	return RoleMember
}

var (
	// ErrMemberNotInTenant 成员不属于当前租户（含已被移除/平台侧越界）
	ErrMemberNotInTenant = errors.New("成员不属于当前租户")
	// ErrLastTenantAdmin 租户内最后一名管理员不可移除/降级/禁用（防租户失管）
	ErrLastTenantAdmin = errors.New("租户内最后一名管理员不可执行该操作")
	// ErrCannotModifySelf 管理员不可移除/变更自己（留任交由其他管理员操作）
	ErrCannotModifySelf = errors.New("不可对本人执行该操作")
)

// Member 租户成员视图（平台 AppUser 视图 + 本租户内角色）
type Member struct {
	AppUserID uint
	Username  string
	Nickname  string
	Phone     string
	Email     string
	Status    int
	Role      Role
	CreatedAt string
	UpdatedAt string
}

// ListQuery 成员列表查询
type ListQuery struct {
	Keyword string
}

// CreateParams 新增成员入参（TenantIDs 由用例固定为当前租户，不接受指定）
type CreateParams struct {
	Username string
	Password string
	Nickname string
	Phone    string
	Email    string
	Role     Role // 缺省 member
}

// UpdateParams 成员资料更新入参（指针可选；不含 tenant_ids——跨租户归属是商户管理端职责）
type UpdateParams struct {
	Nickname *string
	Phone    *string
	Email    *string
}

// Repo 租户成员角色绑定仓储接口（data 层实现，依赖倒置）。
// 绑定即授权：无软删，解除即物理删除；孤儿绑定（成员已不在租户）在
// AppAuth 闸门处天然失效，属卫生问题，由移除路径与低频任务清理。
type Repo interface {
	// RoleOf 查成员在指定租户的角色（无绑定为空串）
	RoleOf(ctx context.Context, appUserID, tenantPlatformID uint) (Role, error)
	// RolesOf 批量查（成员列表合并用；无绑定者不在返回 map 中）
	RolesOf(ctx context.Context, appUserIDs []uint, tenantPlatformID uint) (map[uint]Role, error)
	// SetRole 写绑定（存在即覆盖）
	SetRole(ctx context.Context, appUserID, tenantPlatformID uint, role Role) error
	// Delete 解除单条绑定（幂等）
	Delete(ctx context.Context, appUserID, tenantPlatformID uint) error
	// DeleteByTenant 清空指定租户的全部绑定（租户停用/软删后的卫生清理）
	DeleteByTenant(ctx context.Context, tenantPlatformID uint) error
	// TenantAdminIDs 租户内管理员 app_user ID 集（最后管理员守卫）
	TenantAdminIDs(ctx context.Context, tenantPlatformID uint) ([]uint, error)
}
