package appuser

import (
	"context"
	"errors"
	"testing"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// fakeResolver 租户名解析替身（计数批量调用次数，验证 N+1 已消除）
type fakeResolver struct {
	byPlatform map[uint]*biztenant.Tenant
	batchCalls int
}

func (f *fakeResolver) GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error) {
	if t, ok := f.byPlatform[platformID]; ok {
		return t, nil
	}
	return nil, biztenant.ErrTenantNotFound
}

func (f *fakeResolver) GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*biztenant.Tenant, error) {
	f.batchCalls++
	out := []*biztenant.Tenant{}
	for _, id := range platformIDs {
		if t, ok := f.byPlatform[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// fakeGateway 网关替身：返回固定一页视图
type fakeGateway struct {
	list []*bizappuser.AppUserView
}

func (f *fakeGateway) List(ctx context.Context, p bizappuser.ListParams, page, pageSize int) ([]*bizappuser.AppUserView, pagination.Page, error) {
	return f.list, pagination.Page{Page: page, PageSize: pageSize, Total: int64(len(f.list))}, nil
}
func (f *fakeGateway) Create(ctx context.Context, p bizappuser.CreateParams) (*bizappuser.AppUserView, error) {
	return nil, errors.New("unused")
}
func (f *fakeGateway) Update(ctx context.Context, id uint, p bizappuser.UpdateParams) error {
	return nil
}
func (f *fakeGateway) SetStatus(ctx context.Context, id uint, status int) error          { return nil }
func (f *fakeGateway) ResetPassword(ctx context.Context, id uint, password string) error { return nil }
func (f *fakeGateway) Delete(ctx context.Context, id uint) error                         { return nil }

// TestList_TenantNamesBatched 列表租户名映射：整页一次批量查询（N+1 消除），
// 未同步租户缺名不阻断、名字顺序与 tenant_ids 对齐
func TestList_TenantNamesBatched(t *testing.T) {
	resolver := &fakeResolver{byPlatform: map[uint]*biztenant.Tenant{
		101: {ID: 1, PlatformID: 101, Name: "T101"},
		102: {ID: 2, PlatformID: 102, Name: "T102"},
	}}
	gw := &fakeGateway{list: []*bizappuser.AppUserView{
		{ID: 11, Username: "a", Status: 1, TenantIDs: []uint{101, 999}}, // 999 本地未同步
		{ID: 12, Username: "b", Status: 1, TenantIDs: []uint{102, 101}}, // 跨行复用同一租户
	}}
	svc := NewService(gw, resolver)

	list, pg, err := svc.List(context.Background(), bizappuser.ListParams{}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if pg.Total != 2 || len(list) != 2 {
		t.Fatalf("list = %d, total = %d, want 2/2", len(list), pg.Total)
	}
	if resolver.batchCalls != 1 {
		t.Fatalf("批量解析调用次数 = %d, want 1（N+1 已消除）", resolver.batchCalls)
	}
	if got := list[0].TenantNames; len(got) != 1 || got[0] != "T101" {
		t.Fatalf("未同步租户应缺名不阻断: %v", got)
	}
	if got := list[1].TenantNames; len(got) != 2 || got[0] != "T102" || got[1] != "T101" {
		t.Fatalf("名字应与 tenant_ids 顺序对齐: %v", got)
	}
}
