package tenant

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// Usecase 租户领域用例。
// 同步语义（强一致）：本地租户与 embodied-platform 一一对应——
// 平台先行、本地跟随；平台失败则本地整体失败/回滚，保证两侧不产生半写状态。
// 平台侧租户被删（ErrPlatformTenantGone，2026-09-18 起治理上已不该发生）的兜底：
// 删除按「已删」幂等放行，更新/启停先补链重建（同 code 在平台重建并回填新 platform_id）再重试。
type Usecase struct {
	repo     Repo
	platform PlatformSyncer
}

func NewUsecase(repo Repo, platform PlatformSyncer) *Usecase {
	return &Usecase{repo: repo, platform: platform}
}

// tenantCodePattern 租户业务标识字符集（与前端 min/max 规则对齐；
// code 是平台侧命名空间键，中文/空格等字符会导致同步与 URL 场景歧义）
var tenantCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,64}$`)

// Create 创建租户：平台创建（含商户绑定）→ 本地落库；本地失败回滚平台侧
func (uc *Usecase) Create(ctx context.Context, name, code, contactName, contactPhone, remark string) (*Tenant, error) {
	if !tenantCodePattern.MatchString(code) {
		return nil, ErrInvalidTenantCode
	}
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

// Update 更新租户基础资料（code 创建后不可改）：平台先行，成功后本地跟随。
// 平台侧已删 → 补链重建（重建即携带最新资料，无需重试更新），回填新 platform_id
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
		if !errors.Is(err, ErrPlatformTenantGone) {
			return err
		}
		if err := uc.relink(ctx, t); err != nil {
			return err
		}
	}
	return uc.repo.Update(ctx, t)
}

// Delete 删除租户：本地关联检查（存在关联应用用户拒绝）→ 平台先行（平台侧存在关联时冲突透传）
// → 本地删除。平台侧已删（ErrPlatformTenantGone）按已删放行（幂等，兜底治理外的手工删除）
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Synced() {
		if err := uc.platform.DeleteFromPlatform(ctx, t.PlatformID); err != nil &&
			!errors.Is(err, ErrPlatformTenantGone) {
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

// SetStatus 启用/禁用租户：平台先行，成功后本地跟随。
// 平台侧已删 → 补链重建后对新平台租户重试一次（平台创建默认启用，禁用必须显式补设）
func (uc *Usecase) SetStatus(ctx context.Context, id uint, status Status) error {
	t, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.Synced() {
		if err := uc.platform.SetStatusOnPlatform(ctx, t.PlatformID, status == StatusEnabled); err != nil {
			if !errors.Is(err, ErrPlatformTenantGone) {
				return err
			}
			if err := uc.relink(ctx, t); err != nil {
				return err
			}
			if err := uc.platform.SetStatusOnPlatform(ctx, t.PlatformID, status == StatusEnabled); err != nil {
				return err
			}
		}
	}
	t.Status = status
	return uc.repo.Update(ctx, t)
}

// relink 平台侧租户丢失时补链重建：按当前 code/资料在本商户命名空间重建
// （同商户唯一性+软删槽位已释放，同 code 重建可行），回填新 platform_id
func (uc *Usecase) relink(ctx context.Context, t *Tenant) error {
	pid, err := uc.platform.LinkOrCreateOnPlatform(ctx, t)
	if err != nil {
		return err
	}
	t.PlatformID = pid
	return nil
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

// GetByPlatformID 按平台租户 ID 只读定位本地租户（AppAuth 闸门用；不做补链/重建）
func (uc *Usecase) GetByPlatformID(ctx context.Context, platformID uint) (*Tenant, error) {
	return uc.repo.GetByPlatformID(ctx, platformID)
}

// GetByPlatformIDs 批量版（租户名映射等列表场景，消除 N+1）
func (uc *Usecase) GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*Tenant, error) {
	return uc.repo.GetByPlatformIDs(ctx, platformIDs)
}

// RelinkByPlatformID 按平台租户 ID 定位本地租户并补链重建（设备注册自愈用，
// 实现 bizdevice.TenantRelinker）。LinkOrCreate 幂等：平台侧活租户按 code 命中即更新，
// 已删则同 code 重建；返回（可能更新的）平台租户 ID
func (uc *Usecase) RelinkByPlatformID(ctx context.Context, platformTenantID uint) (uint, error) {
	t, err := uc.repo.GetByPlatformID(ctx, platformTenantID)
	if err != nil {
		return 0, err
	}
	if err := uc.relink(ctx, t); err != nil {
		return 0, err
	}
	if err := uc.repo.Update(ctx, t); err != nil {
		return 0, err
	}
	return t.PlatformID, nil
}

// ReconcileFromPlatform 对账回流：分页拉取平台商户租户全量，本地投影跟随平台真相
// （平台为事实端；平台侧直接变更经此回流，漂移上界=任务周期）。语义：
//   - 平台有、本地无（含平台侧直接新建）→ 本地补建（platform_id 直填）；
//   - 两侧都有但资料/启停不一致 → 本地覆盖为平台值；
//   - 本地已同步（platform_id>0）但平台已无 → 本地软删+墓碑（直接 repo.Delete，
//     不再调平台——平台侧本就不存在）；
//   - 本地未同步（platform_id=0，存量补链域）不参与对账，保持原状。
//
// 单项失败记日志继续（尽力而为），平台拉取失败整体失败（下轮重试）。
func (uc *Usecase) ReconcileFromPlatform(ctx context.Context) (string, error) {
	const pageSize = 100
	platformTenants := map[uint]*Tenant{}
	total := int64(-1)
	for page := 1; page <= 200; page++ {
		list, t, err := uc.platform.ListOnPlatform(ctx, page, pageSize)
		if err != nil {
			return "", fmt.Errorf("拉取平台租户第 %d 页: %w", page, err)
		}
		for _, pt := range list {
			platformTenants[pt.PlatformID] = pt
		}
		total = t
		if int64(page*pageSize) >= total || len(list) == 0 {
			break
		}
	}

	var created, updated, deleted, unchanged int
	for pid, pt := range platformTenants {
		local, err := uc.repo.GetByPlatformID(ctx, pid)
		if err != nil && !errors.Is(err, ErrTenantNotFound) {
			logger.Warn("tenant reconcile: 定位本地租户失败", zap.Uint("platform_id", pid), zap.Error(err))
			continue
		}
		if local == nil {
			nt := &Tenant{
				PlatformID: pt.PlatformID, Name: pt.Name, Code: pt.Code,
				ContactName: pt.ContactName, ContactPhone: pt.ContactPhone,
				Remark: pt.Remark, Status: pt.Status,
			}
			if err := uc.repo.Create(ctx, nt); err != nil {
				logger.Warn("tenant reconcile: 补建本地租户失败", zap.Uint("platform_id", pid), zap.Error(err))
				continue
			}
			created++
			continue
		}
		if local.Name == pt.Name && local.Code == pt.Code && local.ContactName == pt.ContactName &&
			local.ContactPhone == pt.ContactPhone && local.Remark == pt.Remark && local.Status == pt.Status {
			unchanged++
			continue
		}
		local.Name, local.Code = pt.Name, pt.Code
		local.ContactName, local.ContactPhone, local.Remark = pt.ContactName, pt.ContactPhone, pt.Remark
		local.Status = pt.Status
		if err := uc.repo.Update(ctx, local); err != nil {
			logger.Warn("tenant reconcile: 更新本地租户失败", zap.Uint("platform_id", pid), zap.Error(err))
			continue
		}
		updated++
	}

	// 平台已无的已同步本地租户 → 跟随下线（软删+墓碑释放唯一槽位）。
	// 护栏：平台拉取成功但结果为空时跳过下线——空结果无法区分「平台真清空」与
	// 「列表接口异常返回空」，批量下线是不可逆操作，宁可下一轮带上非空真相再下线
	locals, _, err := uc.repo.List(ctx, Query{}, 1, 0)
	if err != nil {
		logger.Warn("tenant reconcile: 拉取本地租户失败", zap.Error(err))
	} else {
		synced := 0
		for _, lt := range locals {
			if !lt.Synced() {
				continue // 未同步存量（补链域）不参与对账
			}
			synced++
		}
		if synced > 0 && len(platformTenants) == 0 {
			logger.Warn("tenant reconcile: 平台租户列表为空，跳过下线阶段（防误删，待下轮非空真相）",
				zap.Int("local_synced", synced))
		} else {
			for _, lt := range locals {
				if !lt.Synced() {
					continue
				}
				if _, ok := platformTenants[lt.PlatformID]; ok {
					continue
				}
				if err := uc.repo.Delete(ctx, lt.ID); err != nil {
					logger.Warn("tenant reconcile: 下线本地租户失败", zap.Uint("id", lt.ID), zap.Uint("platform_id", lt.PlatformID), zap.Error(err))
					continue
				}
				deleted++
			}
		}
	}
	return fmt.Sprintf("租户对账完成：平台 %d，新增 %d、更新 %d、下线 %d、未变 %d", total, created, updated, deleted, unchanged), nil
}
