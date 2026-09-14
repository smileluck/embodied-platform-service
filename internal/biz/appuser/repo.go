package appuser

import (
	"context"
	"errors"
)

// ErrAppUserNotFound 应用用户不存在
var ErrAppUserNotFound = errors.New("应用用户不存在")

// ErrDuplicateUsername 应用用户名重复
var ErrDuplicateUsername = errors.New("用户名已存在，请更换")

// ErrAppUserDisabled 应用用户已被禁用（登录/访问时拒绝）
var ErrAppUserDisabled = errors.New("应用用户已被禁用")

// ErrBadCredentials 用户名或密码错误（用户不存在与密码错误统一返回，防用户名枚举）
var ErrBadCredentials = errors.New("用户名或密码错误")

// Query 应用用户列表查询条件（关键词模糊匹配用户名/昵称，手机精确匹配，可按租户筛选）
type Query struct {
	Keyword  string
	Phone    string
	Status   *int
	TenantID *uint
}

// Repo 应用用户仓储接口（由 data 层实现，依赖倒置）。
// Create/Update 同步替换租户关联（先删后插）；Update 不触碰密码哈希。
type Repo interface {
	Create(ctx context.Context, u *AppUser) error
	Update(ctx context.Context, u *AppUser) error
	Delete(ctx context.Context, id uint) error
	Get(ctx context.Context, id uint) (*AppUser, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*AppUser, int64, error)
	// GetByUsername 按用户名查询并携带密码哈希（登录/改密校验专用）
	GetByUsername(ctx context.Context, username string) (*AppUser, error)
	// ReplaceTenants 全量替换用户的租户关联
	ReplaceTenants(ctx context.Context, id uint, tenantIDs []uint) error
	// UpdatePassword 单独落库密码哈希
	UpdatePassword(ctx context.Context, id uint, passwordHash string) error
}
