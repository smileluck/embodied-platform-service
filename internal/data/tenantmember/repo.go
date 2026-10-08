// Package tenantmember 租户成员角色绑定仓储（GORM 实现）。
// 绑定表极小（主键双列点查），暂不引入缓存；/app-api/v1 的 per-uid 限流已兜住查询频率。
package tenantmember

import (
	"context"
	"errors"

	biztenantmember "github.com/smilex/smilex-admin-gin/internal/biz/tenantmember"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repo struct {
	data *data.Data
}

// NewRepo 创建绑定仓储
func NewRepo(d *data.Data) biztenantmember.Repo { return &repo{data: d} }

func (r *repo) RoleOf(ctx context.Context, appUserID, tenantPlatformID uint) (biztenantmember.Role, error) {
	var po model.TenantUserRolePO
	err := r.data.DB.WithContext(ctx).
		Where("app_user_id = ? AND tenant_platform_id = ?", appUserID, tenantPlatformID).
		First(&po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil // 无绑定：由 biz 层归一为 member
		}
		return "", err
	}
	return biztenantmember.Role(po.Role), nil
}

func (r *repo) RolesOf(ctx context.Context, appUserIDs []uint, tenantPlatformID uint) (map[uint]biztenantmember.Role, error) {
	out := make(map[uint]biztenantmember.Role, len(appUserIDs))
	if len(appUserIDs) == 0 {
		return out, nil
	}
	var pos []model.TenantUserRolePO
	if err := r.data.DB.WithContext(ctx).
		Where("app_user_id IN ? AND tenant_platform_id = ?", appUserIDs, tenantPlatformID).
		Find(&pos).Error; err != nil {
		return nil, err
	}
	for _, po := range pos {
		out[po.AppUserID] = biztenantmember.Role(po.Role)
	}
	return out, nil
}

func (r *repo) SetRole(ctx context.Context, appUserID, tenantPlatformID uint, role biztenantmember.Role) error {
	po := model.TenantUserRolePO{
		AppUserID: appUserID, TenantPlatformID: tenantPlatformID, Role: string(role),
	}
	return r.data.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_user_id"}, {Name: "tenant_platform_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role", "updated_at"}),
	}).Create(&po).Error
}

func (r *repo) Delete(ctx context.Context, appUserID, tenantPlatformID uint) error {
	return r.data.DB.WithContext(ctx).
		Where("app_user_id = ? AND tenant_platform_id = ?", appUserID, tenantPlatformID).
		Delete(&model.TenantUserRolePO{}).Error
}

func (r *repo) DeleteByTenant(ctx context.Context, tenantPlatformID uint) error {
	return r.data.DB.WithContext(ctx).
		Where("tenant_platform_id = ?", tenantPlatformID).
		Delete(&model.TenantUserRolePO{}).Error
}

func (r *repo) TenantAdminIDs(ctx context.Context, tenantPlatformID uint) ([]uint, error) {
	var ids []uint
	err := r.data.DB.WithContext(ctx).Model(&model.TenantUserRolePO{}).
		Where("tenant_platform_id = ? AND role = ?", tenantPlatformID, string(biztenantmember.RoleTenantAdmin)).
		Pluck("app_user_id", &ids).Error
	return ids, err
}
