// 租户用户平台网关端口：平台（embodied-platform）开放面 /open-api/v1/tenant-users*
// 与 /open-api/v1/tenant-roles* 是唯一事实源（2026-10-08 第四套身份：租户门户运营账号，
// 单租户绑定 + 租户作用域 RBAC），本上下文不落任何本地投影。
// 本文件只保留：业务哨兵 + 网关接口 + 开放面视图类型（data/platform 实现）。
package tenantuser

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

var (
	// ErrTenantUserNotFound 租户用户/角色不存在或不在本商户可见范围（开放面统一 404）
	ErrTenantUserNotFound = errors.New("租户用户不存在")
	// ErrTenantRoleNotFound 租户角色不存在或不可见（开放面统一 404）
	ErrTenantRoleNotFound = errors.New("租户角色不存在")
	// ErrDuplicateUsername 租户用户名重复（开放面 409）
	ErrDuplicateUsername = errors.New("用户名已存在，请更换")
	// ErrDuplicateRoleCode 租户角色编码重复（开放面 409）
	ErrDuplicateRoleCode = errors.New("角色编码已存在，请更换")
	// ErrRoleInUse 角色已分配用户（开放面 409）
	ErrRoleInUse = errors.New("角色已分配用户，须先解除绑定")
	// ErrTenantNotInScope tenant_id 越界（非本商户绑定租户；开放面 403）
	ErrTenantNotInScope = errors.New("租户不在商户绑定范围内")
	// ErrInvalidPerm 权限点不在平台目录内或超上限
	ErrInvalidPerm = errors.New("权限点无效")
)

// TenantUserView 开放面租户用户视图（单租户绑定；TenantID 为平台租户 ID）
type TenantUserView struct {
	ID        uint
	TenantID  uint
	Username  string
	Nickname  string
	Phone     string
	Email     string
	Status    int
	RoleIDs   []uint
	CreatedAt string
	UpdatedAt string
}

// TenantRoleView 开放面租户角色视图（PermCodes 为租户门户权限码）
type TenantRoleView struct {
	ID        uint
	TenantID  uint
	Name      string
	Code      string
	Remark    string
	PermCodes []string
	CreatedAt string
	UpdatedAt string
}

// PermDefView 租户门户权限点目录项（Group 为资源域）
type PermDefView struct {
	Code  string
	Group string
}

// UserListParams 租户用户列表查询（TenantID 为平台租户 ID，可选）
type UserListParams struct {
	Keyword  string
	Phone    string
	Status   *int
	TenantID *uint
}

// UserCreateParams 创建租户用户入参（TenantID 须∈商户租户集）
type UserCreateParams struct {
	TenantID uint
	Username string
	Password string
	Nickname string
	Phone    string
	Email    string
	RoleIDs  []uint
}

// UserUpdateParams 更新入参（指针可选，nil=不修改）
type UserUpdateParams struct {
	Nickname *string
	Phone    *string
	Email    *string
	Status   *int
}

// RoleListParams 租户角色列表查询（TenantID 为平台租户 ID，可选）
type RoleListParams struct {
	Keyword  string
	TenantID *uint
}

// RoleCreateParams 创建租户角色入参
type RoleCreateParams struct {
	TenantID  uint
	Name      string
	Code      string
	Remark    string
	PermCodes []string
}

// RoleUpdateParams 更新入参（指针可选，nil=不修改；code 不可改）
type RoleUpdateParams struct {
	Name      *string
	Remark    *string
	PermCodes *[]string
}

// Gateway 平台开放面租户用户/角色网关（data/platform.TenantUserGateway 实现，依赖倒置）。
// 用户/角色 ID 一律为平台 ID；TenantID 一律为平台租户 ID（全链路唯一口径）。
type Gateway interface {
	ListUsers(ctx context.Context, p UserListParams, page, pageSize int) ([]*TenantUserView, pagination.Page, error)
	CreateUser(ctx context.Context, p UserCreateParams) (*TenantUserView, error)
	UpdateUser(ctx context.Context, id uint, p UserUpdateParams) error
	SetUserStatus(ctx context.Context, id uint, status int) error
	ResetUserPassword(ctx context.Context, id uint, password string) error
	SetUserRoles(ctx context.Context, id uint, roleIDs []uint) error
	DeleteUser(ctx context.Context, id uint) error

	ListRoles(ctx context.Context, p RoleListParams, page, pageSize int) ([]*TenantRoleView, pagination.Page, error)
	CreateRole(ctx context.Context, p RoleCreateParams) (*TenantRoleView, error)
	UpdateRole(ctx context.Context, id uint, p RoleUpdateParams) error
	SetRolePerms(ctx context.Context, id uint, permCodes []string) error
	DeleteRole(ctx context.Context, id uint) error

	// ListPermCatalog 租户门户权限点目录（角色配权 UI 数据源；平台 base ∪ 本商户注册码）
	ListPermCatalog(ctx context.Context) ([]*PermDefView, error)
}
