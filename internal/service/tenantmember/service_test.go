package tenantmember

import (
	"context"
	"errors"
	"testing"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	biztenantdept "github.com/smilex/smilex-admin-gin/internal/biz/tenantdept"
	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// fakeGateway 开放面账号网关替身（身份在平台；角色绑定本地）
type fakeGateway struct {
	users map[uint]*biztenantuser.TenantUserView
}

func (f *fakeGateway) ListUsers(ctx context.Context, p biztenantuser.UserListParams, page, pageSize int) ([]*biztenantuser.TenantUserView, pagination.Page, error) {
	out := []*biztenantuser.TenantUserView{}
	for _, u := range f.users {
		if p.TenantID == nil || u.TenantID == *p.TenantID {
			if p.Keyword == "" || u.Username == p.Keyword {
				out = append(out, u)
			}
		}
	}
	return out, pagination.Page{Page: page, PageSize: pageSize, Total: int64(len(out))}, nil
}
func (f *fakeGateway) GetUser(ctx context.Context, id uint) (*biztenantuser.TenantUserView, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, biztenantuser.ErrTenantUserNotFound
}
func (f *fakeGateway) CreateUser(ctx context.Context, p biztenantuser.UserCreateParams) (*biztenantuser.TenantUserView, error) {
	return nil, errors.New("unused")
}
func (f *fakeGateway) UpdateUser(ctx context.Context, id uint, p biztenantuser.UserUpdateParams) error {
	return nil
}
func (f *fakeGateway) SetUserStatus(ctx context.Context, id uint, status int) error {
	f.users[id].Status = status
	return nil
}
func (f *fakeGateway) ResetUserPassword(ctx context.Context, id uint, password string) error {
	return nil
}
func (f *fakeGateway) DeleteUser(ctx context.Context, id uint) error { return nil }

// fakeRoleRepo 本地角色仓储替身（内存态角色 + 用户绑定）
type fakeRoleRepo struct {
	roles map[uint]*biztenantuser.TenantRole
	binds map[uint][]uint // userID -> roleIDs
}

func (f *fakeRoleRepo) Create(ctx context.Context, r *biztenantuser.TenantRole) error {
	f.roles[r.ID] = r
	return nil
}
func (f *fakeRoleRepo) Update(ctx context.Context, r *biztenantuser.TenantRole) error {
	f.roles[r.ID] = r
	return nil
}
func (f *fakeRoleRepo) Delete(ctx context.Context, id uint) error {
	delete(f.roles, id)
	return nil
}
func (f *fakeRoleRepo) Get(ctx context.Context, id uint) (*biztenantuser.TenantRole, error) {
	if r, ok := f.roles[id]; ok {
		return r, nil
	}
	return nil, biztenantuser.ErrTenantRoleNotFound
}
func (f *fakeRoleRepo) List(ctx context.Context, q biztenantuser.RoleListParams, page, pageSize int) ([]*biztenantuser.TenantRole, int64, error) {
	out := []*biztenantuser.TenantRole{}
	for _, r := range f.roles {
		if q.TenantID == nil || r.TenantID == *q.TenantID {
			out = append(out, r)
		}
	}
	return out, int64(len(out)), nil
}
func (f *fakeRoleRepo) ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	f.binds[userID] = roleIDs
	return nil
}
func (f *fakeRoleRepo) UserRoleIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error) {
	out := map[uint][]uint{}
	for _, id := range userIDs {
		if ids, ok := f.binds[id]; ok {
			out[id] = ids
		}
	}
	return out, nil
}
func (f *fakeRoleRepo) ResolvePerms(ctx context.Context, userID, tenantID uint) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, rid := range f.binds[userID] {
		if r, ok := f.roles[rid]; ok && r.TenantID == tenantID {
			for _, p := range r.PermCodes {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
	}
	return out, nil
}

// fakeTenantChecker 本地租户投影替身（存在即通过；biztenantuser/biztenantdept
// 两处 TenantChecker 同构，同一替身满足两者）
type fakeTenantChecker struct{}

func (fakeTenantChecker) GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error) {
	if platformID == 0 {
		return nil, biztenant.ErrTenantNotFound
	}
	return &biztenant.Tenant{ID: platformID, PlatformID: platformID}, nil
}

// fakeDeptRepo 部门仓储替身（仅记录删号级联清理调用；其余方法测试路径不触达）
type fakeDeptRepo struct {
	clearedUsers []uint
}

func (f *fakeDeptRepo) Create(ctx context.Context, d *biztenantdept.TenantDept) error { return nil }
func (f *fakeDeptRepo) Update(ctx context.Context, d *biztenantdept.TenantDept) error { return nil }
func (f *fakeDeptRepo) Delete(ctx context.Context, id uint) error                     { return nil }
func (f *fakeDeptRepo) Get(ctx context.Context, id uint) (*biztenantdept.TenantDept, error) {
	return nil, biztenantdept.ErrDeptNotFound
}
func (f *fakeDeptRepo) List(ctx context.Context, tenantID uint) ([]*biztenantdept.TenantDept, error) {
	return nil, nil
}
func (f *fakeDeptRepo) CountChildren(ctx context.Context, parentID uint) (int64, error) {
	return 0, nil
}
func (f *fakeDeptRepo) ReplaceUserDepts(ctx context.Context, userID uint, deptIDs []uint) error {
	return nil
}
func (f *fakeDeptRepo) DeleteUserDepts(ctx context.Context, userID uint) error {
	f.clearedUsers = append(f.clearedUsers, userID)
	return nil
}
func (f *fakeDeptRepo) UserDeptIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error) {
	return map[uint][]uint{}, nil
}
func (f *fakeDeptRepo) UserIDsByDepts(ctx context.Context, deptIDs []uint) ([]uint, error) {
	return nil, nil
}
func (f *fakeDeptRepo) MemberCounts(ctx context.Context, tenantID uint) (map[uint]int64, error) {
	return map[uint]int64{}, nil
}
func (f *fakeDeptRepo) DeptNamesByIDs(ctx context.Context, deptIDs []uint) (map[uint]string, error) {
	return map[uint]string{}, nil
}

func newFixture() (*Service, *fakeGateway, *fakeRoleRepo, *fakeDeptRepo) {
	gw := &fakeGateway{
		users: map[uint]*biztenantuser.TenantUserView{
			1: {ID: 1, TenantID: 71, Status: 1, Username: "admin"}, // 唯一管理员
			2: {ID: 2, TenantID: 71, Status: 1, Username: "plain"}, // 普通成员
			3: {ID: 3, TenantID: 99, Status: 1, Username: "other"}, // 他租户（不可见）
		},
	}
	repo := &fakeRoleRepo{
		roles: map[uint]*biztenantuser.TenantRole{
			1: {ID: 1, TenantID: 71, PermCodes: []string{"member:user:list", "device:list"}},
			2: {ID: 2, TenantID: 71, PermCodes: []string{"device:list"}},
		},
		binds: map[uint][]uint{1: {1}, 2: {2}, 3: {1}},
	}
	deptRepo := &fakeDeptRepo{}
	svc := NewService(gw, biztenantuser.NewRoleUsecase(repo, fakeTenantChecker{}),
		biztenantdept.NewUsecase(deptRepo, fakeTenantChecker{}))
	return svc, gw, repo, deptRepo
}

// 最后管理员守卫：目标为唯一启用中的成员自治管理员时，禁用/移除/角色替换失格均拒绝；
// 替换后仍持成员权限放行；存在其他管理员放行（启停按平台账号、权限按本地绑定）
func TestGuardNotLastAdmin(t *testing.T) {
	svc, gw, _, _ := newFixture()
	ctx := context.Background()

	if err := svc.guardNotLastAdmin(ctx, 71, 1, nil); !errors.Is(err, ErrLastTenantAdmin) {
		t.Fatalf("唯一管理员移除应拒绝, got %v", err)
	}
	noPerms := []uint{2}
	if err := svc.guardNotLastAdmin(ctx, 71, 1, &noPerms); !errors.Is(err, ErrLastTenantAdmin) {
		t.Fatalf("唯一管理员角色替换为无成员权限应拒绝, got %v", err)
	}
	keepPerms := []uint{1}
	if err := svc.guardNotLastAdmin(ctx, 71, 1, &keepPerms); err != nil {
		t.Fatalf("替换后仍持成员权限应放行, got %v", err)
	}
	if err := svc.guardNotLastAdmin(ctx, 71, 2, nil); err != nil {
		t.Fatalf("非管理员目标不受守卫, got %v", err)
	}
	// 第二管理员在场：对任一管理员的操作放行（本地绑定补管理员角色）
	gw.users[4] = &biztenantuser.TenantUserView{ID: 4, TenantID: 71, Status: 1, Username: "admin2"}
	if err := svc.guardNotLastAdmin(ctx, 71, 1, nil); err == nil {
		// 未绑角色：按本地绑定 user4 无角色，仍应拒绝
		t.Fatalf("未持成员权限的在场成员不构成管理员, 应仍拒绝")
	}
	if err := svc.roles.SetUserRoles(ctx, 71, 4, []uint{1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.guardNotLastAdmin(ctx, 71, 1, nil); err != nil {
		t.Fatalf("存在其他管理员应放行, got %v", err)
	}
	// 被禁用的其他管理员不计入（状态事实源在平台）
	gw.users[4].Status = 0
	if err := svc.guardNotLastAdmin(ctx, 71, 1, nil); !errors.Is(err, ErrLastTenantAdmin) {
		t.Fatalf("其他管理员均已禁用应拒绝, got %v", err)
	}
}

// 本人守卫：移除/启停/改角色对本人拒绝；更新资料与重置密码不受限
func TestCannotModifySelf(t *testing.T) {
	svc, _, _, _ := newFixture()
	ctx := context.Background()

	if err := svc.SetStatus(ctx, 71, 1, 1, 1); !errors.Is(err, ErrCannotModifySelf) {
		t.Fatalf("自启停应拒绝, got %v", err)
	}
	if err := svc.Remove(ctx, 71, 1, 1); !errors.Is(err, ErrCannotModifySelf) {
		t.Fatalf("自移除应拒绝, got %v", err)
	}
	if err := svc.SetRoles(ctx, 71, 1, 1, []uint{2}); !errors.Is(err, ErrCannotModifySelf) {
		t.Fatalf("自改角色应拒绝, got %v", err)
	}
	// 对他人操作走守卫（2 为非管理员，放行到本地绑定替换）
	if err := svc.SetRoles(ctx, 71, 2, 1, []uint{1}); err != nil {
		t.Fatalf("对他人改角色应放行, got %v", err)
	}
}

// 越界定位：目标不在本租户统一 ErrMemberNotInTenant（不泄露存在性；按 id 单查）
func TestLocateMemberOutOfTenant(t *testing.T) {
	svc, _, _, _ := newFixture()
	if _, err := svc.locateMember(context.Background(), 71, 3); !errors.Is(err, ErrMemberNotInTenant) {
		t.Fatalf("他租户用户应按不存在处理, got %v", err)
	}
	if _, err := svc.locateMember(context.Background(), 71, 999); !errors.Is(err, ErrMemberNotInTenant) {
		t.Fatalf("不存在用户应返回 ErrMemberNotInTenant, got %v", err)
	}
}

// 成员列表 role_ids 由本地绑定补齐（平台身份面不返回角色）
func TestListEnrichesRoleIDs(t *testing.T) {
	svc, _, _, _ := newFixture()
	list, _, err := svc.List(context.Background(), 71, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("租户 71 应有 2 名成员, got %d", len(list))
	}
	byID := map[uint][]uint{}
	for _, vo := range list {
		byID[vo.ID] = vo.RoleIDs
	}
	if len(byID[1]) != 1 || byID[1][0] != 1 {
		t.Fatalf("user1 绑定角色应为 [1], got %v", byID[1])
	}
	if len(byID[2]) != 1 || byID[2][0] != 2 {
		t.Fatalf("user2 绑定角色应为 [2], got %v", byID[2])
	}
}

// 移除成员：平台删号成功后级联清理本地部门绑定（成员计数依赖 binds，残留虚增 member_count）
func TestRemoveClearsDeptBinds(t *testing.T) {
	svc, gw, _, deptRepo := newFixture()
	ctx := context.Background()
	// user2 为普通成员（非唯一管理员），移除放行
	if err := svc.Remove(ctx, 71, 2, 1); err != nil {
		t.Fatal(err)
	}
	if len(deptRepo.clearedUsers) != 1 || deptRepo.clearedUsers[0] != 2 {
		t.Fatalf("移除成员应级联清理其部门绑定, got %v", deptRepo.clearedUsers)
	}
	// 守卫拒绝的移除不触发清理
	if err := svc.Remove(ctx, 71, 1, 1); !errors.Is(err, ErrCannotModifySelf) {
		t.Fatalf("自移除应拒绝, got %v", err)
	}
	if len(deptRepo.clearedUsers) != 1 {
		t.Fatalf("守卫拒绝路径不应触发清理, got %v", deptRepo.clearedUsers)
	}
	_ = gw
}
