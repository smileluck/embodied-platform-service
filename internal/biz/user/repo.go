package user

import (
	"context"
	"errors"
)

// ErrUserNotFound 用户不存在
var ErrUserNotFound = errors.New("用户不存在")

// ErrDuplicateUsername 用户名重复
var ErrDuplicateUsername = errors.New("用户名已存在，请更换")

// Query 用户列表查询条件
type Query struct {
	Username string
	Status   *int
}

// Repo 用户仓储接口（由 data 层实现，依赖倒置）。
// 查询类接口默认不返回密码哈希（Omit password）；需要校验密码的场景
// 使用 FindByUsername（登录）或 FindByIDWithPassword（改密校验旧密码）。
type Repo interface {
	Create(ctx context.Context, u *User) error
	// Update 仅更新 nickname/email/status，绝不触碰 password
	Update(ctx context.Context, u *User) error
	// UpdatePassword 单独更新密码哈希（管理员重置 / 本人修改密码）
	UpdatePassword(ctx context.Context, id uint, passwordHash string) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*User, error)
	// FindByIDWithPassword 携带密码哈希返回（校验旧密码场景专用）
	FindByIDWithPassword(ctx context.Context, id uint) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*User, int64, error)
	SetRoles(ctx context.Context, userID uint, roleIDs []uint) error
}
