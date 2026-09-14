package tenant

import (
	"context"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 租户领域用例
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase {
	return &Usecase{repo: repo}
}

// Create 创建租户（code 唯一性由数据库唯一索引兜底，冲突在仓储映射为 ErrDuplicateTenantCode）
func (uc *Usecase) Create(ctx context.Context, name, code, contactName, contactPhone, remark string) (*Tenant, error) {
	t := &Tenant{
		Name: name, Code: code,
		ContactName: contactName, ContactPhone: contactPhone,
		Remark: remark, Status: StatusEnabled,
	}
	if err := uc.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Update 更新租户基础资料（code 创建后不可改）
func (uc *Usecase) Update(ctx context.Context, id uint, name, contactName, contactPhone, remark string) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	t.Name = name
	t.ContactName = contactName
	t.ContactPhone = contactPhone
	t.Remark = remark
	return uc.repo.Update(ctx, t)
}

// Delete 删除租户（存在关联应用用户时仓储返回 ErrTenantInUse）
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Tenant, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Tenant, pagination.Page, error) {
	tenants, total, err := uc.repo.List(ctx, q, page, pageSize)
	return tenants, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// SetStatus 启用/禁用租户
func (uc *Usecase) SetStatus(ctx context.Context, id uint, status Status) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	t.Status = status
	return uc.repo.Update(ctx, t)
}
