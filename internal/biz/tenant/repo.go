package tenant

import (
	"context"
	"errors"
)

// ErrTenantNotFound 租户不存在
var ErrTenantNotFound = errors.New("租户不存在")

// ErrDuplicateTenantName 租户名称重复
var ErrDuplicateTenantName = errors.New("租户名称已存在，请更换")

// ErrDuplicateTenantCode 租户编码重复
var ErrDuplicateTenantCode = errors.New("租户编码已存在，请更换")

// ErrTenantInUse 租户下存在应用用户，禁止删除
var ErrTenantInUse = errors.New("该租户下存在应用用户，请先移除关联")

// Query 租户列表查询条件（名称/编码按字段拆分，各自独立全模糊匹配，可叠加）
type Query struct {
	Name   string
	Code   string
	Status *int
}

// Repo 租户仓储接口（由 data 层实现，依赖倒置）。
// Update 仅更新基础资料，code 创建后不可改；
// Delete 在存在关联应用用户时拒绝（ErrTenantInUse）。
type Repo interface {
	Create(ctx context.Context, t *Tenant) error
	Update(ctx context.Context, t *Tenant) error
	Delete(ctx context.Context, id uint) error
	Get(ctx context.Context, id uint) (*Tenant, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Tenant, int64, error)
	// ListUnsynced 列出尚未与平台同步（platform_id=0）的租户（存量补链用）
	ListUnsynced(ctx context.Context) ([]*Tenant, error)
}

// PlatformSyncer 平台租户同步接口（data 层经管理面服务账号实现，依赖倒置）。
// 平台是设备侧租户真源：同步时统一绑定本服务商户（merchant_id），
// 绑入商户租户集后开放面设备注册才可用。
type PlatformSyncer interface {
	// CreateOnPlatform 平台创建租户（平台侧重名等冲突原样返回），返回平台租户 ID
	CreateOnPlatform(ctx context.Context, t *Tenant) (uint, error)
	// UpdateOnPlatform 同步平台租户基础资料（含商户绑定保持）
	UpdateOnPlatform(ctx context.Context, t *Tenant) error
	// DeleteFromPlatform 删除平台租户（平台侧存在关联时返回冲突）
	DeleteFromPlatform(ctx context.Context, platformID uint) error
	// SetStatusOnPlatform 同步平台租户启停状态
	SetStatusOnPlatform(ctx context.Context, platformID uint, enabled bool) error
	// LinkOrCreateOnPlatform 存量补链：平台已有同 code 租户则（重新）绑定本服务商户并
	// 同步资料、返回其 ID；没有则创建。返回平台租户 ID。
	LinkOrCreateOnPlatform(ctx context.Context, t *Tenant) (uint, error)
}
