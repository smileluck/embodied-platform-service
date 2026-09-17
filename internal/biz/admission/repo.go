package admission

import (
	"context"
	"errors"
)

// ErrNotFound 准入记录不存在
var ErrNotFound = errors.New("准入记录不存在")

// ErrDuplicatePlatformUser 该平台用户已有准入记录
var ErrDuplicatePlatformUser = errors.New("该平台用户已存在准入记录")

// Query 准入列表查询条件
type Query struct {
	Username string // 用户名前缀模糊
	Status   *int   // 1 已准入 0 已停用（nil 全部）
}

// Repo 准入投影仓储接口（由 data 层实现，依赖倒置）
type Repo interface {
	Create(ctx context.Context, p *Projection) error
	// Update 仅更新快照字段与准入开关
	Update(ctx context.Context, p *Projection) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*Projection, error)
	// FindByPlatformUserID 按平台用户 ID 查（认证链路每请求调用）
	FindByPlatformUserID(ctx context.Context, platformUserID uint) (*Projection, error)
	// FindByPlatformUserIDs 批量按平台用户 ID 查（成员列表合并投影用；未建投影的不在返回 map 中）
	FindByPlatformUserIDs(ctx context.Context, platformUserIDs []uint) (map[uint]*Projection, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Projection, int64, error)
	SetRoles(ctx context.Context, id uint, roleIDs []uint) error
}
