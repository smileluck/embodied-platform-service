package tenant

import (
	"context"

	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// Usecase 租户领域用例。
// 同步语义（强一致）：本地租户与 embodied-platform 一一对应——
// 平台先行、本地跟随；平台失败则本地整体失败/回滚，保证两侧不产生半写状态。
type Usecase struct {
	repo     Repo
	platform PlatformSyncer
}

func NewUsecase(repo Repo, platform PlatformSyncer) *Usecase {
	return &Usecase{repo: repo, platform: platform}
}

// Create 创建租户：平台创建（含商户绑定）→ 本地落库；本地失败回滚平台侧
func (uc *Usecase) Create(ctx context.Context, name, code, contactName, contactPhone, remark string) (*Tenant, error) {
	t := &Tenant{
		Name: name, Code: code,
		ContactName: contactName, ContactPhone: contactPhone,
		Remark: remark, Status: StatusEnabled,
	}
	pid, err := uc.platform.CreateOnPlatform(ctx, t)
	if err != nil {
		return nil, err
	}
	t.PlatformID = pid
	if err := uc.repo.Create(ctx, t); err != nil {
		// 本地失败（如本地重名）回滚平台侧，避免平台残留孤儿租户；回滚失败仅告警
		if derr := uc.platform.DeleteFromPlatform(ctx, pid); derr != nil {
			logger.Warn("tenant platform rollback failed", zap.Uint("platform_id", pid), zap.Error(derr))
		}
		return nil, err
	}
	return t, nil
}

// Update 更新租户基础资料（code 创建后不可改）：平台先行，成功后本地跟随
func (uc *Usecase) Update(ctx context.Context, id uint, name, contactName, contactPhone, remark string) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	t.Name = name
	t.ContactName = contactName
	t.ContactPhone = contactPhone
	t.Remark = remark
	if err := uc.platform.UpdateOnPlatform(ctx, t); err != nil {
		return err
	}
	return uc.repo.Update(ctx, t)
}

// Delete 删除租户：平台先行（平台侧存在关联时冲突透传），本地按引用检查删除
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Synced() {
		if err := uc.platform.DeleteFromPlatform(ctx, t.PlatformID); err != nil {
			return err
		}
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Tenant, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Tenant, pagination.Page, error) {
	tenants, total, err := uc.repo.List(ctx, q, page, pageSize)
	return tenants, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// SetStatus 启用/禁用租户：平台先行，成功后本地跟随
func (uc *Usecase) SetStatus(ctx context.Context, id uint, status Status) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Synced() {
		if err := uc.platform.SetStatusOnPlatform(ctx, t.PlatformID, status == StatusEnabled); err != nil {
			return err
		}
	}
	t.Status = status
	return uc.repo.Update(ctx, t)
}

// SyncExisting 存量补链：未同步的租户在平台查找同 code 租户并绑定本服务商户，
// 没有则创建；已同步的租户刷新平台侧资料与绑定。返回同步后的租户。
func (uc *Usecase) SyncExisting(ctx context.Context, id uint) (*Tenant, error) {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	pid, err := uc.platform.LinkOrCreateOnPlatform(ctx, t)
	if err != nil {
		return nil, err
	}
	t.PlatformID = pid
	if err := uc.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
