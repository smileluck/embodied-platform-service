// 租户用户平台网关端口：平台（embodied-platform）开放面 /open-api/v1/tenant-users*
// 是租户门户账号身份的事实源（2026-10-08 第四套身份：单租户绑定运营账号）；
// 租户 RBAC 自 2026-10-09 本地化（见 rolerepo.go/roleusecase.go），本端口只承载账号生命周期。
// 本文件只保留：业务哨兵 + 网关接口 + 开放面视图类型（data/platform 实现）。
package tenantuser

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

var (
	// ErrTenantUserNotFound 租户用户不存在或不在本商户可见范围（开放面统一 404）
	ErrTenantUserNotFound = errors.New("租户用户不存在")
	// ErrDuplicateUsername 租户用户名重复（开放面 409）
	ErrDuplicateUsername = errors.New("用户名已存在，请更换")
	// ErrTenantNotInScope tenant_id 越界（非本商户绑定租户；开放面 403）
	ErrTenantNotInScope = errors.New("租户不在商户绑定范围内")
)

// TenantUserView 开放面租户用户视图（单租户绑定；TenantID 为平台租户 ID；
// RoleIDs 由本地绑定补齐——平台身份面不涉角色）
type TenantUserView struct {
	ID        uint
	TenantID  uint
	Username  string
	Nickname  string
	Phone     string
	Email     string
	Status    int
	CreatedAt string
	UpdatedAt string
}

// PermDefView 租户门户权限点目录项（Group 为资源域）
type PermDefView struct {
	Code  string `json:"code"`
	Group string `json:"group"`
}

// UserListParams 租户用户列表查询（TenantID 为平台租户 ID，可选）
type UserListParams struct {
	Keyword  string
	Phone    string
	Status   *int
	TenantID *uint
}

// UserCreateParams 创建租户用户入参（TenantID 须∈商户租户集；角色绑定本地另行落库）
type UserCreateParams struct {
	TenantID uint
	Username string
	Password string
	Nickname string
	Phone    string
	Email    string
}

// UserUpdateParams 更新入参（指针可选，nil=不修改）
type UserUpdateParams struct {
	Nickname *string
	Phone    *string
	Email    *string
	Status   *int
}

// Gateway 平台开放面租户用户网关（data/platform.TenantUserGateway 实现，依赖倒置）。
// 用户 ID 一律为平台 ID；TenantID 一律为平台租户 ID（全链路唯一口径）。
type Gateway interface {
	ListUsers(ctx context.Context, p UserListParams, page, pageSize int) ([]*TenantUserView, pagination.Page, error)
	GetUser(ctx context.Context, id uint) (*TenantUserView, error)
	CreateUser(ctx context.Context, p UserCreateParams) (*TenantUserView, error)
	UpdateUser(ctx context.Context, id uint, p UserUpdateParams) error
	SetUserStatus(ctx context.Context, id uint, status int) error
	ResetUserPassword(ctx context.Context, id uint, password string) error
	DeleteUser(ctx context.Context, id uint) error
}
