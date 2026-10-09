// 租户 RBAC 本地域（2026-10-09 自平台下沉）：角色聚合、仓储端口与视图类型。
// tenant_id 一律为平台租户 ID、user_id 一律为平台 tenant_user ID（账号身份在平台，
// 本地只持授权；角色 (tenant_id, code) 唯一，删除走软删+墓碑释放槽位）。
package tenantuser

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrTenantRoleNotFound 租户角色不存在（含不属于该租户的越权引用）
	ErrTenantRoleNotFound = errors.New("租户角色不存在")
	// ErrDuplicateRoleCode 租户角色编码重复（(tenant_id, code) 唯一）
	ErrDuplicateRoleCode = errors.New("角色编码已存在，请更换")
	// ErrRoleInUse 角色已分配用户（本地 tenant_user_role_binds），禁止删除
	ErrRoleInUse = errors.New("角色已分配用户，须先解除绑定")
)

// TenantRole 租户角色聚合根；PermCodes 来自本地权限码注册表（permcatalog.go 唯一真源）
type TenantRole struct {
	ID        uint      `json:"id"`
	TenantID  uint      `json:"tenant_id"` // 平台租户 ID
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Remark    string    `json:"remark"`
	PermCodes []string  `json:"perm_codes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RoleRepo 本地租户角色仓储端口（由 data/tenantrole 实现，依赖倒置）。
// Create/Update 同步替换权限点（先删后插）；code/tenant_id 创建后不可改；
// Delete 在角色仍有用户绑定时拒绝（ErrRoleInUse）。
type RoleRepo interface {
	Create(ctx context.Context, r *TenantRole) error
	Update(ctx context.Context, r *TenantRole) error
	Delete(ctx context.Context, id uint) error
	Get(ctx context.Context, id uint) (*TenantRole, error)
	List(ctx context.Context, q RoleListParams, page, pageSize int) ([]*TenantRole, int64, error)
	// ReplaceUserRoles 事务内全量替换用户角色绑定（先删后插；roleIDs 已由用例层校验同租户）
	ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error
	// UserRoleIDs 批量取用户角色绑定（user_id -> role_ids；列表补齐 RoleIDs 用）
	UserRoleIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error)
	// ResolvePerms 用户权限码集合（binds → roles → role_perms 去重，TenantAuth 缓存装载用）
	ResolvePerms(ctx context.Context, userID, tenantID uint) ([]string, error)
}

// PermResolver 租户门户权限解析最小接口（RoleRepo 满足；TenantAuth 中间件依赖）
type PermResolver interface {
	ResolvePerms(ctx context.Context, userID, tenantID uint) ([]string, error)
}
