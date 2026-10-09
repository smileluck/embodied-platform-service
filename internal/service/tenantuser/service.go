// Package tenantuser 租户用户应用服务（管理面）。
// 平台（/open-api/v1/tenant-users*、/open-api/v1/tenant-roles*）为唯一事实源，
// 经开放面实时消费，本地不落数据；tenant_id 全链路为平台租户 ID
// （tenant_name 由本地租户按 platform_id 映射补齐）。
// 租户门户认证/成员自助管理见 /tenant-api/v1 代理组（Phase 3），与本章管理面共用本网关。
package tenantuser

import (
	"context"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// TenantNameResolver 本地租户名解析（按平台租户 ID；*biztenant.Usecase 满足）。
// 列表走批量方法（整页一次查询消除 N+1），解析失败仅缺名不阻断
type TenantNameResolver interface {
	GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*biztenant.Tenant, error)
}

type Service struct {
	gw      biztenantuser.Gateway // 平台开放面租户用户/角色网关（唯一事实源）
	tenants TenantNameResolver    // tenant_names 本地映射（展示用）
}

func NewService(gw biztenantuser.Gateway, tenants TenantNameResolver) *Service {
	return &Service{gw: gw, tenants: tenants}
}

// ---- 租户用户 ----

// UserCreateRequest 创建租户用户入参（校验口径与平台开放面一致）
type UserCreateRequest struct {
	TenantID uint   `json:"tenant_id" binding:"required"`
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=20"`
	Nickname string `json:"nickname" binding:"max=64"`
	Phone    string `json:"phone" binding:"max=32"`
	Email    string `json:"email" binding:"omitempty,max=128,email"`
	RoleIDs  []uint `json:"role_ids"`
}

// UserUpdateRequest 更新入参（指针可选，省略即不修改；tenant_id 不可改）
type UserUpdateRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,max=64"`
	Phone    *string `json:"phone" binding:"omitempty,max=32"`
	Email    *string `json:"email" binding:"omitempty,max=128,email"`
	Status   *int    `json:"status" binding:"omitempty,oneof=0 1"`
}

// ResetPasswordRequest 重置密码入参
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// SetRolesRequest 角色全量替换入参
type SetRolesRequest struct {
	RoleIDs []uint `json:"role_ids"`
}

// UserVO 租户用户视图（tenant_names 由本地租户映射补齐；角色展示由前端按角色列表解析）
type UserVO struct {
	ID         uint   `json:"id"`
	TenantID   uint   `json:"tenant_id"`
	TenantName string `json:"tenant_name"`
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	Status     int    `json:"status"`
	RoleIDs    []uint `json:"role_ids"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func (s *Service) toUserVO(u *biztenantuser.TenantUserView, nameMap map[uint]string) *UserVO {
	vo := &UserVO{
		ID: u.ID, TenantID: u.TenantID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		RoleIDs: u.RoleIDs, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
	if vo.RoleIDs == nil {
		vo.RoleIDs = []uint{}
	}
	vo.TenantName = nameMap[u.TenantID]
	return vo
}

// tenantNameMap 批量解析平台租户 ID → 本地租户名（一次查询；空集不发查询）
func (s *Service) tenantNameMap(ctx context.Context, ids []uint) map[uint]string {
	m := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return m
	}
	tenants, err := s.tenants.GetByPlatformIDs(ctx, ids)
	if err != nil {
		return m // 解析失败仅缺名，不阻断列表
	}
	for _, tn := range tenants {
		m[tn.PlatformID] = tn.Name
	}
	return m
}

// ListUsers 租户用户列表（实时来自平台开放面）
func (s *Service) ListUsers(ctx context.Context, q biztenantuser.UserListParams, page, pageSize int) ([]*UserVO, pagination.Page, error) {
	list, pg, err := s.gw.ListUsers(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	ids := make([]uint, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.TenantID)
	}
	nameMap := s.tenantNameMap(ctx, ids)
	out := make([]*UserVO, 0, len(list))
	for _, u := range list {
		out = append(out, s.toUserVO(u, nameMap))
	}
	return out, pg, nil
}

func (s *Service) CreateUser(ctx context.Context, req UserCreateRequest) (*UserVO, error) {
	u, err := s.gw.CreateUser(ctx, biztenantuser.UserCreateParams{
		TenantID: req.TenantID, Username: req.Username, Password: req.Password,
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email, RoleIDs: req.RoleIDs,
	})
	if err != nil {
		return nil, err
	}
	return s.toUserVO(u, s.tenantNameMap(ctx, []uint{u.TenantID})), nil
}

func (s *Service) UpdateUser(ctx context.Context, id uint, req UserUpdateRequest) error {
	return s.gw.UpdateUser(ctx, id, biztenantuser.UserUpdateParams{
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email, Status: req.Status,
	})
}

func (s *Service) SetUserStatus(ctx context.Context, id uint, status int) error {
	return s.gw.SetUserStatus(ctx, id, status)
}

func (s *Service) ResetUserPassword(ctx context.Context, id uint, req ResetPasswordRequest) error {
	return s.gw.ResetUserPassword(ctx, id, req.Password)
}

func (s *Service) SetUserRoles(ctx context.Context, id uint, req SetRolesRequest) error {
	if req.RoleIDs == nil {
		req.RoleIDs = []uint{}
	}
	return s.gw.SetUserRoles(ctx, id, req.RoleIDs)
}

func (s *Service) DeleteUser(ctx context.Context, id uint) error {
	return s.gw.DeleteUser(ctx, id)
}

// ---- 租户角色 ----

// RoleCreateRequest 创建租户角色入参（perm_codes 须在平台目录内）
type RoleCreateRequest struct {
	TenantID  uint     `json:"tenant_id" binding:"required"`
	Name      string   `json:"name" binding:"required,max=64"`
	Code      string   `json:"code" binding:"required,min=2,max=64"`
	Remark    string   `json:"remark" binding:"max=255"`
	PermCodes []string `json:"perm_codes"`
}

// RoleUpdateRequest 更新入参（code 不可改）
type RoleUpdateRequest struct {
	Name      *string   `json:"name" binding:"omitempty,max=64"`
	Remark    *string   `json:"remark" binding:"omitempty,max=255"`
	PermCodes *[]string `json:"perm_codes"`
}

// SetPermsRequest 权限点全量替换入参
type SetPermsRequest struct {
	PermCodes []string `json:"perm_codes"`
}

// RoleVO 租户角色视图
type RoleVO struct {
	ID         uint     `json:"id"`
	TenantID   uint     `json:"tenant_id"`
	TenantName string   `json:"tenant_name"`
	Name       string   `json:"name"`
	Code       string   `json:"code"`
	Remark     string   `json:"remark"`
	PermCodes  []string `json:"perm_codes"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

func (s *Service) toRoleVO(r *biztenantuser.TenantRoleView, nameMap map[uint]string) *RoleVO {
	vo := &RoleVO{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Code: r.Code, Remark: r.Remark,
		PermCodes: r.PermCodes, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if vo.PermCodes == nil {
		vo.PermCodes = []string{}
	}
	vo.TenantName = nameMap[r.TenantID]
	return vo
}

// ListRoles 租户角色列表（实时来自平台开放面）
func (s *Service) ListRoles(ctx context.Context, q biztenantuser.RoleListParams, page, pageSize int) ([]*RoleVO, pagination.Page, error) {
	list, pg, err := s.gw.ListRoles(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	ids := make([]uint, 0, len(list))
	for _, r := range list {
		ids = append(ids, r.TenantID)
	}
	nameMap := s.tenantNameMap(ctx, ids)
	out := make([]*RoleVO, 0, len(list))
	for _, r := range list {
		out = append(out, s.toRoleVO(r, nameMap))
	}
	return out, pg, nil
}

func (s *Service) CreateRole(ctx context.Context, req RoleCreateRequest) (*RoleVO, error) {
	r, err := s.gw.CreateRole(ctx, biztenantuser.RoleCreateParams{
		TenantID: req.TenantID, Name: req.Name, Code: req.Code,
		Remark: req.Remark, PermCodes: req.PermCodes,
	})
	if err != nil {
		return nil, err
	}
	return s.toRoleVO(r, s.tenantNameMap(ctx, []uint{r.TenantID})), nil
}

func (s *Service) UpdateRole(ctx context.Context, id uint, req RoleUpdateRequest) error {
	return s.gw.UpdateRole(ctx, id, biztenantuser.RoleUpdateParams{
		Name: req.Name, Remark: req.Remark, PermCodes: req.PermCodes,
	})
}

func (s *Service) SetRolePerms(ctx context.Context, id uint, req SetPermsRequest) error {
	if req.PermCodes == nil {
		req.PermCodes = []string{}
	}
	return s.gw.SetRolePerms(ctx, id, req.PermCodes)
}

func (s *Service) DeleteRole(ctx context.Context, id uint) error {
	return s.gw.DeleteRole(ctx, id)
}

// ListPermCatalog 租户门户权限点目录（角色配权 UI 数据源）
func (s *Service) ListPermCatalog(ctx context.Context) ([]*biztenantuser.PermDefView, error) {
	return s.gw.ListPermCatalog(ctx)
}
