package tenant

import (
	"context"
	"errors"
	"net/http"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
)

// Syncer 平台租户同步实现（开放面商户 HMAC，经平台 SDK）。
// 2026-09-16 起平台开放面提供租户域（scope tenant:*）：创建时 merchant 归属由平台
// 服务端注入调用方商户（无需本地解析 merchant_id），读写按商户绑定收敛——
// 本同步器创建的租户自动进入本商户租户集，开放面设备注册随即覆盖。
type Syncer struct {
	client *platformsdk.Client
}

// NewSyncer 构造（wire provider，绑定 biztenant.PlatformSyncer）
func NewSyncer(client *platformsdk.Client) *Syncer {
	return &Syncer{client: client}
}

// toBiz SDK 租户 → 领域实体
func toBiz(t *platformsdk.Tenant) *biztenant.Tenant {
	return &biztenant.Tenant{
		ID: t.ID, Name: t.Name, Code: t.Code,
		ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark, Status: biztenant.Status(t.Status),
	}
}

// ListOnPlatform 分页拉取本商户的平台租户（对账回流）：平台 ID 落 PlatformID，
// 本地 ID 置零（本地行由对账用例按 PlatformID 定位）
func (s *Syncer) ListOnPlatform(ctx context.Context, page, pageSize int) ([]*biztenant.Tenant, int64, error) {
	list, pg, err := s.client.ListTenants(ctx, page, pageSize, platformsdk.TenantFilter{})
	if err != nil {
		return nil, 0, err
	}
	out := make([]*biztenant.Tenant, 0, len(list))
	for _, t := range list {
		b := toBiz(t)
		b.PlatformID = t.ID
		b.ID = 0
		out = append(out, b)
	}
	if pg == nil {
		pg = &platformsdk.Page{}
	}
	return out, pg.Total, nil
}

func (s *Syncer) CreateOnPlatform(ctx context.Context, t *biztenant.Tenant) (uint, error) {
	pt, err := s.client.CreateTenant(ctx, platformsdk.TenantCreateRequest{
		Name: t.Name, Code: t.Code, ContactName: t.ContactName,
		ContactPhone: t.ContactPhone, Remark: t.Remark,
	})
	if err != nil {
		return 0, err
	}
	return pt.ID, nil
}

func (s *Syncer) UpdateOnPlatform(ctx context.Context, t *biztenant.Tenant) error {
	return platformGone(s.client.UpdateTenant(ctx, t.PlatformID, platformsdk.TenantUpdateRequest{
		Name: t.Name, ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark,
	}))
}

func (s *Syncer) DeleteFromPlatform(ctx context.Context, platformID uint) error {
	return platformGone(s.client.DeleteTenant(ctx, platformID))
}

func (s *Syncer) SetStatusOnPlatform(ctx context.Context, platformID uint, enabled bool) error {
	return platformGone(s.client.SetTenantStatus(ctx, platformID, enabled))
}

// platformGone 平台侧 404（开放面对不存在与越权统一 404）映射为 ErrPlatformTenantGone，
// 供用例层做幂等删除（按已删处理）与补链重建自愈；其余错误原样返回
func platformGone(err error) error {
	var se *platformsdk.Error
	if errors.As(err, &se) && se.HTTPStatus == http.StatusNotFound {
		return biztenant.ErrPlatformTenantGone
	}
	return err
}

// LinkOrCreateOnPlatform 存量补链：本商户绑定集内按 code 精确查找——
// 找到则更新资料（绑定已属本商户，无需再绑）；没有则创建。
// 平台租户唯一性=商户内（2026-09-18 起）：他商户同 code 不构成冲突；
// 仅本商户内已有同 code 才 409（该场景已被上面的查找先行命中，正常不会走到）。
func (s *Syncer) LinkOrCreateOnPlatform(ctx context.Context, t *biztenant.Tenant) (uint, error) {
	if pt, err := s.client.FindTenantByCode(ctx, t.Code); err != nil {
		return 0, err
	} else if pt != nil {
		if uerr := s.client.UpdateTenant(ctx, pt.ID, platformsdk.TenantUpdateRequest{
			Name: t.Name, ContactName: t.ContactName, ContactPhone: t.ContactPhone,
			Remark: t.Remark,
		}); uerr != nil {
			return 0, uerr
		}
		return pt.ID, nil
	}
	return s.CreateOnPlatform(ctx, t)
}
