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
}
