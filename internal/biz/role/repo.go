package role

import (
	"context"
	"errors"
)

// ErrRoleNotFound 角色不存在
var ErrRoleNotFound = errors.New("角色不存在")

// ErrRoleHasUsers 角色下仍有用户，须先移除
var ErrRoleHasUsers = errors.New("该角色下存在用户，请先移除用户与该角色的关联")

// ErrSuperRoleLocked 超管角色为系统内置，禁止修改/删除
var ErrSuperRoleLocked = errors.New("超级管理员角色为系统内置，禁止修改和操作")

// Query 角色列表查询条件
type Query struct {
	Name string
}

// Repo 角色仓储接口
type Repo interface {
	Create(ctx context.Context, r *Role) error
	Update(ctx context.Context, r *Role) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*Role, error)
	// FindNamesByIDs 按角色 ID 列表查角色名（个人中心展示用，按 id 排序返回）
	FindNamesByIDs(ctx context.Context, ids []uint) ([]string, error)
	// CountUsers 统计角色下的用户数量（删除保护用）
	CountUsers(ctx context.Context, roleID uint) (int64, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Role, int64, error)
	SetPermissions(ctx context.Context, roleID uint, permissionIDs []uint) error
}
