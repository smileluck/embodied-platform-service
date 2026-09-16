package tenant

import (
	"context"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// Syncer 平台租户同步实现（开放面商户 HMAC，经平台 SDK）。
// 2026-09-16 起平台开放面提供租户域（scope tenant:*）：创建时 merchant 归属由平台
// 服务端注入调用方商户（无需本地解析 merchant_id），读写按商户绑定收敛——
// 本同步器创建的租户自动进入本商户租户集，开放面设备注册随即覆盖。
type Syncer struct {
	client *sdk.Client
}

// NewSyncer 构造（wire provider，绑定 biztenant.PlatformSyncer）
func NewSyncer(client *sdk.Client) *Syncer {
	return &Syncer{client: client}
}

// toBiz SDK 租户 → 领域实体
func toBiz(t *sdk.Tenant) *biztenant.Tenant {
	return &biztenant.Tenant{
		ID: t.ID, Name: t.Name, Code: t.Code,
		ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark, Status: biztenant.Status(t.Status),
	}
}

func (s *Syncer) CreateOnPlatform(ctx context.Context, t *biztenant.Tenant) (uint, error) {
	pt, err := s.client.CreateTenant(ctx, sdk.TenantCreateRequest{
		Name: t.Name, Code: t.Code, ContactName: t.ContactName,
		ContactPhone: t.ContactPhone, Remark: t.Remark,
	})
	if err != nil {
		return 0, err
	}
	return pt.ID, nil
}

func (s *Syncer) UpdateOnPlatform(ctx context.Context, t *biztenant.Tenant) error {
	return s.client.UpdateTenant(ctx, t.PlatformID, sdk.TenantUpdateRequest{
		Name: t.Name, ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark,
	})
}

func (s *Syncer) DeleteFromPlatform(ctx context.Context, platformID uint) error {
	return s.client.DeleteTenant(ctx, platformID)
}

func (s *Syncer) SetStatusOnPlatform(ctx context.Context, platformID uint, enabled bool) error {
	return s.client.SetTenantStatus(ctx, platformID, enabled)
}

// LinkOrCreateOnPlatform 存量补链：本商户绑定集内按 code 精确查找——
// 找到则更新资料（绑定已属本商户，无需再绑）；没有则创建。
// 注意：他商户的同 code 租户在开放面不可见，若平台上已被占用，创建将 409（提示换码）。
func (s *Syncer) LinkOrCreateOnPlatform(ctx context.Context, t *biztenant.Tenant) (uint, error) {
	if pt, err := s.client.FindTenantByCode(ctx, t.Code); err != nil {
		return 0, err
	} else if pt != nil {
		if uerr := s.client.UpdateTenant(ctx, pt.ID, sdk.TenantUpdateRequest{
			Name: t.Name, ContactName: t.ContactName, ContactPhone: t.ContactPhone,
			Remark: t.Remark,
		}); uerr != nil {
			return 0, uerr
		}
		return pt.ID, nil
	}
	return s.CreateOnPlatform(ctx, t)
}
