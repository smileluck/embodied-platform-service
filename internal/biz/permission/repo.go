package permission

import (
	"context"
	"errors"
)

// ErrPermissionNotFound 权限不存在
var ErrPermissionNotFound = errors.New("权限不存在")

// ErrHasChildren 存在子节点，须先删除子级
var ErrHasChildren = errors.New("该节点下存在子级，请先删除子级节点")

// ErrDuplicateCode 权限编码已存在
var ErrDuplicateCode = errors.New("权限编码已存在，请更换")

// Query 权限列表查询条件
type Query struct {
	Type string // menu | api，空为全部
}

// Repo 权限仓储接口
type Repo interface {
	Create(ctx context.Context, p *Permission) error
	Update(ctx context.Context, p *Permission) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*Permission, error)
	// FindByCode 按编码查询（创建时查重用）
	FindByCode(ctx context.Context, code string) (*Permission, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Permission, int64, error)
	// CountByParentID 统计指定父节点的直接子级数量（删除保护用）
	CountByParentID(ctx context.Context, parentID uint) (int64, error)
	// FindByUserID 查询用户经角色关联到的全部权限（data 层做 join）
	FindByUserID(ctx context.Context, userID uint) ([]*Permission, error)
}
