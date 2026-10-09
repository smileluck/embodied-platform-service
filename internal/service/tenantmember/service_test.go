package tenantmember

import (
	"context"
	"errors"
	"testing"

	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// fakeGateway 开放面网关替身：内存态租户用户/角色，支撑守卫分支
type fakeGateway struct {
	users []*biztenantuser.TenantUserView
	roles []*biztenantuser.TenantRoleView
}

func (f *fakeGateway) ListUsers(ctx context.Context, p biztenantuser.UserListParams, page, pageSize int) ([]*biztenantuser.TenantUserView, pagination.Page, error) {
	out := []*biztenantuser.TenantUserView{}
	for _, u := range f.users {
		if p.TenantID == nil || u.TenantID == *p.TenantID {
			out = append(out, u)
		}
	}
	return out, pagination.Page{Page: page, PageSize: pageSize, Total: int64(len(out))}, nil
}
func (f *fakeGateway) CreateUser(ctx context.Context, p biztenantuser.UserCreateParams) (*biztenantuser.TenantUserView, error) {
	return nil, errors.New("unused")
}
func (f *fakeGateway) UpdateUser(ctx context.Context, id uint, p biztenantuser.UserUpdateParams) error {
	return nil
}
func (f *fakeGateway) SetUserStatus(ctx context.Context, id uint, status int) error { return nil }
func (f *fakeGateway) ResetUserPassword(ctx context.Context, id uint, password string) error {
	return nil
}
func (f *fakeGateway) SetUserRoles(ctx context.Context, id uint, roleIDs []uint) error { return nil }
func (f *fakeGateway) DeleteUser(ctx context.Context, id uint) error                   { return nil }
func (f *fakeGateway) ListRoles(ctx context.Context, p biztenantuser.RoleListParams, page, pageSize int) ([]*biztenantuser.TenantRoleView, pagination.Page, error) {
	out := []*biztenantuser.TenantRoleView{}
	for _, r := range f.roles {
		if p.TenantID == nil || r.TenantID == *p.TenantID {
			out = append(out, r)
		}
	}
	return out, pagination.Page{Page: page, PageSize: pageSize, Total: int64(len(out))}, nil
}
func (f *fakeGateway) CreateRole(ctx context.Context, p biztenantuser.RoleCreateParams) (*biztenantuser.TenantRoleView, error) {
	return nil, errors.New("unused")
}
func (f *fakeGateway) UpdateRole(ctx context.Context, id uint, p biztenantuser.RoleUpdateParams) error {
	return nil
}
func (f *fakeGateway) SetRolePerms(ctx context.Context, id uint, permCodes []string) error {
	return nil
}
func (f *fakeGateway) DeleteRole(ctx context.Context, id uint) error { return nil }
func (f *fakeGateway) ListPermCatalog(ctx context.Context) ([]*biztenantuser.PermDefView, error) {
	return nil, nil
}

// adminRole 含成员域权限的角色；plainRole 无成员域权限
var (
	adminRole = &biztenantuser.TenantRoleView{ID: 1, TenantID: 71, PermCodes: []string{"member:user:list", "device:list"}}
	plainRole = &biztenantuser.TenantRoleView{ID: 2, TenantID: 71, PermCodes: []string{"device:list"}}
)

func newFixture() *fakeGateway {
	return &fakeGateway{
		roles: []*biztenantuser.TenantRoleView{adminRole, plainRole},
		users: []*biztenantuser.TenantUserView{
			{ID: 1, TenantID: 71, Status: 1, RoleIDs: []uint{1}}, // 唯一管理员
			{ID: 2, TenantID: 71, Status: 1, RoleIDs: []uint{2}}, // 普通成员
			{ID: 3, TenantID: 99, Status: 1, RoleIDs: []uint{1}}, // 他租户（不可见）
		},
	}
}

// 最后管理员守卫：目标为唯一启用中的成员自治管理员时，禁用/移除/角色替换失格均拒绝；
// 替换后仍持成员权限放行；存在其他管理员放行
func TestGuardNotLastAdmin(t *testing.T) {
	svc := NewService(newFixture())
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
	// 第二管理员在场：对任一管理员的操作放行
	admin2 := &biztenantuser.TenantUserView{ID: 4, TenantID: 71, Status: 1, RoleIDs: []uint{1}}
	svc.gw.(*fakeGateway).users = append(svc.gw.(*fakeGateway).users, admin2)
	if err := svc.guardNotLastAdmin(ctx, 71, 1, nil); err != nil {
		t.Fatalf("存在其他管理员应放行, got %v", err)
	}
	// 被禁用的其他管理员不计入
	svc.gw.(*fakeGateway).users[3].Status = 0
	if err := svc.guardNotLastAdmin(ctx, 71, 1, nil); !errors.Is(err, ErrLastTenantAdmin) {
		t.Fatalf("其他管理员均已禁用应拒绝, got %v", err)
	}
}

// 本人守卫：移除/启停/改角色对本人拒绝；更新资料与重置密码不受限
func TestCannotModifySelf(t *testing.T) {
	svc := NewService(newFixture())
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
	// 对他人操作走守卫（2 为非管理员，放行到网关）
	if err := svc.SetRoles(ctx, 71, 2, 1, []uint{1}); err != nil {
		t.Fatalf("对他人改角色应放行, got %v", err)
	}
}

// 越界定位：目标不在本租户统一 ErrMemberNotInTenant（不泄露存在性）
func TestLocateMemberOutOfTenant(t *testing.T) {
	svc := NewService(newFixture())
	if _, err := svc.locateMember(context.Background(), 71, 3); !errors.Is(err, ErrMemberNotInTenant) {
		t.Fatalf("他租户用户应按不存在处理, got %v", err)
	}
	if _, err := svc.locateMember(context.Background(), 71, 999); !errors.Is(err, ErrMemberNotInTenant) {
		t.Fatalf("不存在用户应返回 ErrMemberNotInTenant, got %v", err)
	}
}
