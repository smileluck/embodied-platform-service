package tenantmember

import (
	"context"
	"errors"
	"testing"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// ---- 替身 ----

// fakeGateway 平台开放面网关假实现：users 为「本商户可见」应用用户真相，
// 记录 Update 的 tenant_ids 差集更新结果
type fakeGateway struct {
	users     []*bizappuser.AppUserView
	updated   map[uint][]uint // id -> 最后一次 Update 的 TenantIDs
	createErr error
}

func (f *fakeGateway) List(ctx context.Context, p bizappuser.ListParams, page, pageSize int) ([]*bizappuser.AppUserView, pagination.Page, error) {
	var hits []*bizappuser.AppUserView
	for _, u := range f.users {
		if p.TenantID == nil {
			hits = append(hits, u)
			continue
		}
		for _, id := range u.TenantIDs {
			if id == *p.TenantID {
				hits = append(hits, u)
				break
			}
		}
	}
	if f.updated != nil { // 已被移除的（Update 过 tenant_ids）按真相过滤
		var live []*bizappuser.AppUserView
		for _, u := range hits {
			if ids, ok := f.updated[u.ID]; ok {
				cp := *u
				cp.TenantIDs = ids
				in := false
				for _, id := range ids {
					if p.TenantID != nil && id == *p.TenantID {
						in = true
					}
				}
				if p.TenantID == nil || in {
					live = append(live, &cp)
				}
			} else {
				live = append(live, u)
			}
		}
		hits = live
	}
	return hits, pagination.Page{Page: page, PageSize: pageSize, Total: int64(len(hits))}, nil
}

func (f *fakeGateway) Create(ctx context.Context, p bizappuser.CreateParams) (*bizappuser.AppUserView, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	v := &bizappuser.AppUserView{ID: uint(len(f.users) + 100), Username: p.Username, Status: 1, TenantIDs: p.TenantIDs}
	f.users = append(f.users, v)
	return v, nil
}

func (f *fakeGateway) Update(ctx context.Context, id uint, p bizappuser.UpdateParams) error {
	if p.TenantIDs != nil {
		if f.updated == nil {
			f.updated = map[uint][]uint{}
		}
		f.updated[id] = *p.TenantIDs
	}
	return nil
}

func (f *fakeGateway) SetStatus(ctx context.Context, id uint, status int) error    { return nil }
func (f *fakeGateway) ResetPassword(ctx context.Context, id uint, pw string) error { return nil }
func (f *fakeGateway) Delete(ctx context.Context, id uint) error                   { return nil }

// memRepo 绑定仓储内存替身
type memRepo struct {
	bindings map[uint]map[uint]Role // appUserID -> tenantPlatformID -> role
}

func (m *memRepo) RoleOf(ctx context.Context, appUserID, tenantPlatformID uint) (Role, error) {
	if r, ok := m.bindings[appUserID][tenantPlatformID]; ok {
		return r, nil
	}
	return "", nil
}

func (m *memRepo) RolesOf(ctx context.Context, ids []uint, tenantPlatformID uint) (map[uint]Role, error) {
	out := map[uint]Role{}
	for _, id := range ids {
		if r, ok := m.bindings[id][tenantPlatformID]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func (m *memRepo) SetRole(ctx context.Context, appUserID, tenantPlatformID uint, role Role) error {
	if m.bindings[appUserID] == nil {
		m.bindings[appUserID] = map[uint]Role{}
	}
	m.bindings[appUserID][tenantPlatformID] = role
	return nil
}

func (m *memRepo) Delete(ctx context.Context, appUserID, tenantPlatformID uint) error {
	delete(m.bindings[appUserID], tenantPlatformID)
	return nil
}

func (m *memRepo) DeleteByTenant(ctx context.Context, tenantPlatformID uint) error {
	for _, byTenant := range m.bindings {
		delete(byTenant, tenantPlatformID)
	}
	return nil
}

func (m *memRepo) TenantAdminIDs(ctx context.Context, tenantPlatformID uint) ([]uint, error) {
	var ids []uint
	for uid, byTenant := range m.bindings {
		if byTenant[tenantPlatformID] == RoleTenantAdmin {
			ids = append(ids, uid)
		}
	}
	return ids, nil
}

func newFixture() (*Usecase, *fakeGateway, *memRepo) {
	gw := &fakeGateway{users: []*bizappuser.AppUserView{
		{ID: 1, Username: "admin1", Status: 1, TenantIDs: []uint{10}},
		{ID: 2, Username: "admin2", Status: 1, TenantIDs: []uint{10}},
		{ID: 3, Username: "member", Status: 1, TenantIDs: []uint{10}},
		{ID: 4, Username: "other", Status: 1, TenantIDs: []uint{20}},
	}}
	repo := &memRepo{bindings: map[uint]map[uint]Role{
		1: {10: RoleTenantAdmin},
		2: {10: RoleTenantAdmin},
		3: {10: RoleMember},
	}}
	return NewUsecase(repo, gw), gw, repo
}

// ---- 守卫用例 ----

// TestRemoveMember_LastAdminGuard 最后管理员守卫：仅剩一名管理员时移除被拒；
// 两 名管理员时移除成功且 tenant_ids 差集更新 + 绑定解除
func TestRemoveMember_LastAdminGuard(t *testing.T) {
	uc, gw, repo := newFixture()

	// 场景一：admin2 先被移除后，admin1 成为最后管理员，不可再被移除
	if err := uc.RemoveMember(context.Background(), 10, 2, 1); err != nil {
		t.Fatalf("移除非最后管理员应成功, got %v", err)
	}
	if got := gw.updated[2]; len(got) != 0 {
		t.Errorf("admin2 移除后 tenant_ids 应为空集, got %v", got)
	}
	if _, ok := repo.bindings[2][10]; ok {
		t.Error("移除后绑定应解除")
	}
	if err := uc.RemoveMember(context.Background(), 10, 1, 3); !errors.Is(err, ErrLastTenantAdmin) {
		t.Errorf("移除最后管理员应返回 ErrLastTenantAdmin, got %v", err)
	}

	// 场景二：普通成员可直接移除
	if err := uc.RemoveMember(context.Background(), 10, 3, 1); err != nil {
		t.Fatalf("移除普通成员应成功, got %v", err)
	}
}

// TestRemoveMember_SelfForbidden 本人不可被自己移除
func TestRemoveMember_SelfForbidden(t *testing.T) {
	uc, _, _ := newFixture()
	if err := uc.RemoveMember(context.Background(), 10, 1, 1); !errors.Is(err, ErrCannotModifySelf) {
		t.Errorf("自移除应返回 ErrCannotModifySelf, got %v", err)
	}
}

// TestRemoveMember_CrossTenantRejected 跨租户目标拒绝（成员不在当前租户）
func TestRemoveMember_CrossTenantRejected(t *testing.T) {
	uc, _, _ := newFixture()
	// user 4 只属于租户 20
	if err := uc.RemoveMember(context.Background(), 10, 4, 1); !errors.Is(err, ErrMemberNotInTenant) {
		t.Errorf("跨租户移除应返回 ErrMemberNotInTenant, got %v", err)
	}
}

// TestSetMemberRole_LastAdminGuard 最后管理员不可被降级
func TestSetMemberRole_LastAdminGuard(t *testing.T) {
	uc, _, repo := newFixture()
	// 先移除 admin2（剩下 admin1 唯一管理员）
	_ = uc.RemoveMember(context.Background(), 10, 2, 1)
	if err := uc.SetMemberRole(context.Background(), 10, 1, 2, RoleMember); !errors.Is(err, ErrLastTenantAdmin) {
		t.Errorf("降级最后管理员应返回 ErrLastTenantAdmin, got %v", err)
	}
	// 任命第二名管理员后，原管理员可被降级
	if err := uc.SetMemberRole(context.Background(), 10, 3, 1, RoleTenantAdmin); err != nil {
		t.Fatalf("任命管理员应成功, got %v", err)
	}
	if repo.bindings[3][10] != RoleTenantAdmin {
		t.Error("任命后绑定应为 tenant_admin")
	}
	if err := uc.SetMemberRole(context.Background(), 10, 1, 3, RoleMember); err != nil {
		t.Fatalf("存在其他管理员时降级应成功, got %v", err)
	}
}

// TestSetMemberRole_SelfForbidden 本人不可变更自己角色
func TestSetMemberRole_SelfForbidden(t *testing.T) {
	uc, _, _ := newFixture()
	if err := uc.SetMemberRole(context.Background(), 10, 1, 1, RoleMember); !errors.Is(err, ErrCannotModifySelf) {
		t.Errorf("自降级应返回 ErrCannotModifySelf, got %v", err)
	}
}

// TestSetMemberStatus_LastAdminGuard 最后管理员不可被禁用
func TestSetMemberStatus_LastAdminGuard(t *testing.T) {
	uc, _, _ := newFixture()
	_ = uc.RemoveMember(context.Background(), 10, 2, 1)
	if err := uc.SetMemberStatus(context.Background(), 10, 1, 3, 0); !errors.Is(err, ErrLastTenantAdmin) {
		t.Errorf("禁用最后管理员应返回 ErrLastTenantAdmin, got %v", err)
	}
	if err := uc.SetMemberStatus(context.Background(), 10, 3, 3, 0); !errors.Is(err, ErrCannotModifySelf) {
		t.Errorf("禁用本人应返回 ErrCannotModifySelf, got %v", err)
	}
}

// TestCreateMember 新增成员固定当前租户 + 角色绑定（含缺省 member）
func TestCreateMember(t *testing.T) {
	uc, gw, repo := newFixture()
	m, err := uc.CreateMember(context.Background(), 10, CreateParams{Username: "newbie", Password: "secret1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(gw.users[len(gw.users)-1].TenantIDs) != 1 || gw.users[len(gw.users)-1].TenantIDs[0] != 10 {
		t.Errorf("新增成员 tenant_ids 应固定为 [10], got %v", gw.users[len(gw.users)-1].TenantIDs)
	}
	if m.Role != RoleMember {
		t.Errorf("缺省角色应为 member, got %s", m.Role)
	}
	if repo.bindings[m.AppUserID][10] != RoleMember {
		t.Error("新增后应落 member 绑定")
	}
	// 指定管理员角色
	m2, err := uc.CreateMember(context.Background(), 10, CreateParams{Username: "boss", Password: "secret1", Role: RoleTenantAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if m2.Role != RoleTenantAdmin || repo.bindings[m2.AppUserID][10] != RoleTenantAdmin {
		t.Error("指定 tenant_admin 应生效")
	}
}

// TestListMembers 列表合并本地角色（无绑定归一 member）+ 跨租户成员不出现
func TestListMembers(t *testing.T) {
	uc, _, _ := newFixture()
	list, pg, err := uc.ListMembers(context.Background(), 10, ListQuery{}, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || pg.Total != 3 {
		t.Fatalf("租户 10 应有 3 名成员, got %d (total %d)", len(list), pg.Total)
	}
	roles := map[uint]Role{}
	for _, m := range list {
		roles[m.AppUserID] = m.Role
	}
	if roles[1] != RoleTenantAdmin || roles[3] != RoleMember {
		t.Errorf("角色合并错误: %v", roles)
	}
}

// TestUpdateMember_CrossTenantRejected 资料更新前置归属校验
func TestUpdateMember_CrossTenantRejected(t *testing.T) {
	uc, _, _ := newFixture()
	nick := "x"
	if err := uc.UpdateMember(context.Background(), 10, 4, UpdateParams{Nickname: &nick}); !errors.Is(err, ErrMemberNotInTenant) {
		t.Errorf("跨租户更新应返回 ErrMemberNotInTenant, got %v", err)
	}
	if err := uc.UpdateMember(context.Background(), 10, 3, UpdateParams{Nickname: &nick}); err != nil {
		t.Fatalf("本租户更新应成功, got %v", err)
	}
}

// TestRemoveMember_KeepsOtherTenants 移除保留本商户其他租户归属
func TestRemoveMember_KeepsOtherTenants(t *testing.T) {
	uc, gw, _ := newFixture()
	// member(3) 同时属于 10 与 20
	gw.users[2].TenantIDs = []uint{10, 20}
	if err := uc.RemoveMember(context.Background(), 10, 3, 1); err != nil {
		t.Fatal(err)
	}
	got := gw.updated[3]
	if len(got) != 1 || got[0] != 20 {
		t.Errorf("移除租户 10 后应保留 [20], got %v", got)
	}
}
