// 应用用户平台网关端口：本上下文自 2026-10-05 起不再持有本地数据面/登录面，
// 平台（embodied-platform 开放面 /open-api/v1/app-users）是唯一事实源。
// 本文件只保留：业务哨兵 + 网关接口 + 开放面视图类型（data/platform 实现）。
package appuser

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

var (
	// ErrAppUserNotFound 应用用户不存在或不在本商户可见范围（开放面统一 404）
	ErrAppUserNotFound = errors.New("应用用户不存在")
	// ErrDuplicateUsername 应用用户名重复（开放面 409）
	ErrDuplicateUsername = errors.New("用户名已存在，请更换")
	// ErrTenantNotInScope tenant_ids 越界（非本商户绑定租户；开放面 403）
	ErrTenantNotInScope = errors.New("租户不在商户绑定范围内")
	// ErrCrossMerchantDelete 仍挂靠其他商户租户，删除被拒（开放面 409）
	ErrCrossMerchantDelete = errors.New("应用用户仍挂靠其他商户的租户，请先解除其在本商户外的归属")
)

// AppUserView 开放面应用用户视图（tenant_ids 为平台租户 ID，已按商户租户集收敛为交集）
type AppUserView struct {
	ID        uint
	Username  string
	Nickname  string
	Phone     string
	Email     string
	Status    int
	TenantIDs []uint
	CreatedAt string
	UpdatedAt string
}

// ListParams 列表查询（TenantID 为平台租户 ID，可选）
type ListParams struct {
	Keyword  string
	Phone    string
	Status   *int
	TenantID *uint
}

// CreateParams 创建入参（TenantIDs 须⊆商户租户集）
type CreateParams struct {
	Username  string
	Password  string
	Nickname  string
	Phone     string
	Email     string
	TenantIDs []uint
}

// UpdateParams 更新入参（指针可选，nil=不修改；TenantIDs 全量替换本商户范围内归属）
type UpdateParams struct {
	Nickname  *string
	Phone     *string
	Email     *string
	Status    *int
	TenantIDs *[]uint
}

// Gateway 平台开放面应用用户网关（data/platform.AppUserGateway 实现，依赖倒置）。
// ID 一律为平台 app_user ID；tenant_ids 一律为平台租户 ID（全链路唯一口径）。
type Gateway interface {
	List(ctx context.Context, p ListParams, page, pageSize int) ([]*AppUserView, pagination.Page, error)
	Create(ctx context.Context, p CreateParams) (*AppUserView, error)
	Update(ctx context.Context, id uint, p UpdateParams) error
	SetStatus(ctx context.Context, id uint, status int) error
	ResetPassword(ctx context.Context, id uint, password string) error
	Delete(ctx context.Context, id uint) error
}
