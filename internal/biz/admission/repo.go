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
	// SetRoles 全量替换角色绑定（data 层保证内置锁定角色的现持有绑定不被冲掉）
	SetRoles(ctx context.Context, id uint, roleIDs []uint) error
	// GrantRole 为平台用户追加单个角色绑定（幂等；商户管理员投影自愈用）
	GrantRole(ctx context.Context, platformUserID, roleID uint) error
	// RevokeRole 移除平台用户的单个角色绑定（幂等；撤商户管理员标记回收用）
	RevokeRole(ctx context.Context, platformUserID, roleID uint) error
	// RoleHolderUserIDs 持有指定角色的平台用户 ID 集（同步对账「已持有但平台未标记」回收用）
	RoleHolderUserIDs(ctx context.Context, roleID uint) ([]uint, error)
}
