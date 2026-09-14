package role

import (
	"context"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 角色领域用例
type Usecase struct {
	repo Repo
}

// superAdminRoleID 超管角色固定 ID：禁止修改和操作
const superAdminRoleID uint = 1

func NewUsecase(repo Repo) *Usecase { return &Usecase{repo: repo} }

func (uc *Usecase) Create(ctx context.Context, name, remark string) (*Role, error) {
	r := &Role{Name: name, Remark: remark}
	if err := uc.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (uc *Usecase) Update(ctx context.Context, id uint, name, remark string) error {
	if id == superAdminRoleID {
		return ErrSuperRoleLocked
	}
	r, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if name != "" {
		r.Name = name
	}
	if remark != "" {
		r.Remark = remark
	}
	return uc.repo.Update(ctx, r)
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	if id == superAdminRoleID {
		return ErrSuperRoleLocked
	}
	if n, err := uc.repo.CountUsers(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return ErrRoleHasUsers
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Role, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Role, pagination.Page, error) {
	roles, total, err := uc.repo.List(ctx, q, page, pageSize)
	return roles, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// SetPermissions 绑定权限
func (uc *Usecase) SetPermissions(ctx context.Context, roleID uint, permissionIDs []uint) error {
	if roleID == superAdminRoleID {
		return ErrSuperRoleLocked
	}
	return uc.repo.SetPermissions(ctx, roleID, permissionIDs)
}
