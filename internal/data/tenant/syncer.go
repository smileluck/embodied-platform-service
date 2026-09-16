package tenant

import (
	"context"
	"sync"
	"time"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
)

// Syncer 平台租户同步实现（管理面服务账号）。
// 同步的租户统一绑定本服务商户（按 AppKey 解析平台商户 ID，进程内缓存 10 分钟），
// 绑入商户租户集后，开放面（商户 HMAC）的设备注册/查询才覆盖这些租户。
type Syncer struct {
	admin *platform.AdminClient
	cfg   *conf.Bootstrap

	mu         sync.Mutex
	merchantID uint
	resolvedAt time.Time
}

// NewSyncer 构造（wire provider，绑定 biztenant.PlatformSyncer）
func NewSyncer(admin *platform.AdminClient, cfg *conf.Bootstrap) *Syncer {
	return &Syncer{admin: admin, cfg: cfg}
}

// merchantIDCacheTTL 商户 ID 解析缓存时长
const merchantIDCacheTTL = 10 * time.Minute

// resolveMerchantID 解析本服务商户的平台 ID（AppKey → GET /merchants?app_key=）
func (s *Syncer) resolveMerchantID(ctx context.Context) (uint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.merchantID != 0 && time.Since(s.resolvedAt) < merchantIDCacheTTL {
		return s.merchantID, nil
	}
	id, err := s.admin.FindMerchantIDByAppKey(ctx, s.cfg.Platform.AppKey)
	if err != nil {
		return 0, err
	}
	s.merchantID, s.resolvedAt = id, time.Now()
	return id, nil
}

func (s *Syncer) CreateOnPlatform(ctx context.Context, t *biztenant.Tenant) (uint, error) {
	mid, err := s.resolveMerchantID(ctx)
	if err != nil {
		return 0, err
	}
	pt, err := s.admin.CreateTenant(ctx, platform.TenantCreate{
		Name: t.Name, Code: t.Code, ContactName: t.ContactName,
		ContactPhone: t.ContactPhone, Remark: t.Remark, MerchantID: mid,
	})
	if err != nil {
		return 0, err
	}
	return pt.ID, nil
}

func (s *Syncer) UpdateOnPlatform(ctx context.Context, t *biztenant.Tenant) error {
	mid, err := s.resolveMerchantID(ctx)
	if err != nil {
		return err
	}
	return s.admin.UpdateTenant(ctx, t.PlatformID, platform.TenantUpdate{
		Name: t.Name, ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark, MerchantID: mid,
	})
}

func (s *Syncer) DeleteFromPlatform(ctx context.Context, platformID uint) error {
	return s.admin.DeleteTenant(ctx, platformID)
}

func (s *Syncer) SetStatusOnPlatform(ctx context.Context, platformID uint, enabled bool) error {
	status := 0
	if enabled {
		status = 1
	}
	return s.admin.SetTenantStatus(ctx, platformID, status)
}

func (s *Syncer) LinkOrCreateOnPlatform(ctx context.Context, t *biztenant.Tenant) (uint, error) {
	// 平台已有同 code 租户 → 重新绑定本服务商户并同步资料；没有 → 创建
	if pt, err := s.admin.FindTenantByCode(ctx, t.Code); err == nil && pt != nil {
		mid, err := s.resolveMerchantID(ctx)
		if err != nil {
			return 0, err
		}
		if uerr := s.admin.UpdateTenant(ctx, pt.ID, platform.TenantUpdate{
			Name: t.Name, ContactName: t.ContactName, ContactPhone: t.ContactPhone,
			Remark: t.Remark, MerchantID: mid,
		}); uerr != nil {
			return 0, uerr
		}
		return pt.ID, nil
	} else if err != nil {
		return 0, err
	}
	return s.CreateOnPlatform(ctx, t)
}
