package tenant

import (
	"context"
	"errors"

	bizdevice "github.com/smilex/smilex-admin-gin/internal/biz/device"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/gorm"
)

// availability 设备注册的租户可用性判定（本地租户表按平台租户 ID 查已同步且启用的租户）
type availability struct {
	data *data.Data
}

// NewTenantAvailability 构造（wire provider，绑定 bizdevice.TenantReader）
func NewTenantAvailability(d *data.Data) bizdevice.TenantReader {
	return &availability{data: d}
}

func (a *availability) TenantAvailable(ctx context.Context, platformTenantID uint) (bool, error) {
	var po model.TenantPO
	err := a.data.DB.WithContext(ctx).
		Where("platform_id = ? AND status = 1", platformTenantID).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
