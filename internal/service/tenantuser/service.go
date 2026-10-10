// Package tenantuser 租户用户应用服务（管理面）。
// 账号身份经平台开放面实时消费（/open-api/v1/tenant-users*，本地不落数据）；
// 租户 RBAC 自 2026-10-09 本地化：角色与用户角色绑定走本地 RoleUsecase。
// tenant_id 全链路为平台租户 ID（tenant_name 由本地租户按 platform_id 映射补齐）。
// 租户门户认证/成员自助管理见 /tenant-api/v1 代理组，与本管理面共用网关与角色用例。
package tenantuser

import (
	"context"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	biztenantdept "github.com/smilex/smilex-admin-gin/internal/biz/tenantdept"
	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// TenantNameResolver 本地租户名解析（按平台租户 ID；*biztenant.Usecase 满足）。
// 列表走批量方法（整页一次查询消除 N+1），解析失败仅缺名不阻断
type TenantNameResolver interface {
	GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*biztenant.Tenant, error)
}

type Service struct {
	gw      biztenantuser.Gateway      // 平台开放面租户用户网关（账号身份事实源）
	roles   *biztenantuser.RoleUsecase // 本地租户角色用例（RBAC 本地化）
	depts   *biztenantdept.Usecase     // 本地租户部门用例（多部门归属，2026-10-10）
	tenants TenantNameResolver         // tenant_names 本地映射（展示用）
}

func NewService(gw biztenantuser.Gateway, roles *biztenantuser.RoleUsecase, depts *biztenantdept.Usecase, tenants TenantNameResolver) *Service {
	return &Service{gw: gw, roles: roles, depts: depts, tenants: tenants}
}

// ---- 租户用户（账号在平台，角色绑定在本地） ----

// UserCreateRequest 创建租户用户入参（校验口径与平台开放面一致；role_ids 为本地角色）
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

// SetRolesRequest 角色全量替换入参（本地绑定；角色须与用户同租户）
type SetRolesRequest struct {
	RoleIDs []uint `json:"role_ids"`
}

// SetDeptsRequest 部门全量替换入参（本地绑定，多部门；部门须与用户同租户）
type SetDeptsRequest struct {
	DeptIDs []uint `json:"dept_ids"`
}

// UserDeptItem 用户所属部门项（列表回填 dept_ids + 名称）
type UserDeptItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// UserVO 租户用户视图（tenant_names 由本地租户映射补齐；role_ids/depts 由本地绑定补齐，
// 角色名称展示由前端按角色列表解析）
type UserVO struct {
	ID         uint           `json:"id"`
	TenantID   uint           `json:"tenant_id"`
	TenantName string         `json:"tenant_name"`
	Username   string         `json:"username"`
	Nickname   string         `json:"nickname"`
	Phone      string         `json:"phone"`
	Email      string         `json:"email"`
	Status     int            `json:"status"`
	RoleIDs    []uint         `json:"role_ids"`
	Depts      []UserDeptItem `json:"depts"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
}

func (s *Service) toUserVO(u *biztenantuser.TenantUserView, roleIDs []uint, depts []UserDeptItem, nameMap map[uint]string) *UserVO {
	vo := &UserVO{
		ID: u.ID, TenantID: u.TenantID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		RoleIDs: roleIDs, Depts: depts, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
	if vo.RoleIDs == nil {
		vo.RoleIDs = []uint{}
	}
	if vo.Depts == nil {
		vo.Depts = []UserDeptItem{}
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

// ListUsers 租户用户列表（账号实时来自平台开放面；role_ids/depts 本地绑定批量补齐）。
// dept_id 有值时切换为本地交集路径（见 listUsersByDept），返回结构与常规路径一致
func (s *Service) ListUsers(ctx context.Context, q biztenantuser.UserListParams, page, pageSize int) ([]*UserVO, pagination.Page, error) {
	if q.DeptID != nil {
		return s.listUsersByDept(ctx, q, page, pageSize)
	}
	list, pg, err := s.gw.ListUsers(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	ids := make([]uint, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.TenantID)
	}
	nameMap := s.tenantNameMap(ctx, ids)
	roleMap := s.userRoleMap(ctx, list)
	deptMap := s.userDeptMap(ctx, list)
	out := make([]*UserVO, 0, len(list))
	for _, u := range list {
		out = append(out, s.toUserVO(u, roleMap[u.ID], deptMap[u.ID], nameMap))
	}
	return out, pg, nil
}

// listUsersByDept 部门筛选路径：部门（含全部后代）成员绑定在本地，账号在平台且开放面
// 不支持按 id 集合过滤——本地反查成员 user_ids 后，按部门租户翻页拉取平台全量做内存交集，
// 再本地分页。已知限制：拉取按 100/页翻页，超大租户（万级用户）性能受限；平台侧支持
// 部门维度过滤后应切换下推（见 aiDoc/contracts/boundary.md）
func (s *Service) listUsersByDept(ctx context.Context, q biztenantuser.UserListParams, page, pageSize int) ([]*UserVO, pagination.Page, error) {
	userIDs, deptTenantID, err := s.depts.UserIDsUnderDept(ctx, *q.DeptID)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	// 部门租户与显式租户筛选不符时按空集返回（防御前端跨租户拼参数）
	if q.TenantID != nil && *q.TenantID != deptTenantID {
		return []*UserVO{}, pagination.Page{Page: page, PageSize: pageSize, Total: 0}, nil
	}
	// 翻页拉取部门租户全量（kw/phone/status 交由平台过滤），与本地成员集求交集
	fetchSize := 100
	member := make(map[uint]struct{}, len(userIDs))
	for _, id := range userIDs {
		member[id] = struct{}{}
	}
	var matched []*biztenantuser.TenantUserView
	fetchQ := biztenantuser.UserListParams{Keyword: q.Keyword, Phone: q.Phone, Status: q.Status, TenantID: &deptTenantID}
	for p := 1; ; p++ {
		list, pg, err := s.gw.ListUsers(ctx, fetchQ, p, fetchSize)
		if err != nil {
			return nil, pagination.Page{}, err
		}
		for _, u := range list {
			if _, ok := member[u.ID]; ok {
				matched = append(matched, u)
			}
		}
		if int64(p*fetchSize) >= pg.Total || len(list) == 0 {
			break
		}
	}
	// 本地分页（保持平台返回顺序）
	total := int64(len(matched))
	start := (page - 1) * pageSize
	if start >= len(matched) {
		matched = nil
	} else {
		end := start + pageSize
		if end > len(matched) {
			end = len(matched)
		}
		matched = matched[start:end]
	}
	ids := make([]uint, 0, len(matched))
	for _, u := range matched {
		ids = append(ids, u.TenantID)
	}
	nameMap := s.tenantNameMap(ctx, ids)
	roleMap := s.userRoleMap(ctx, matched)
	deptMap := s.userDeptMap(ctx, matched)
	out := make([]*UserVO, 0, len(matched))
	for _, u := range matched {
		out = append(out, s.toUserVO(u, roleMap[u.ID], deptMap[u.ID], nameMap))
	}
	return out, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// userRoleMap 批量取本地角色绑定（user_id -> role_ids；失败按空集处理不阻断列表）
func (s *Service) userRoleMap(ctx context.Context, list []*biztenantuser.TenantUserView) map[uint][]uint {
	ids := make([]uint, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	m, err := s.roles.UserRoleIDs(ctx, ids)
	if err != nil {
		return map[uint][]uint{}
	}
	return m
}

// userDeptMap 批量取本地部门绑定并解析名称（user_id -> [{id,name}]；失败按空集不阻断）
func (s *Service) userDeptMap(ctx context.Context, list []*biztenantuser.TenantUserView) map[uint][]UserDeptItem {
	ids := make([]uint, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	binds, err := s.depts.UserDeptIDs(ctx, ids)
	if err != nil {
		return map[uint][]UserDeptItem{}
	}
	deptIDs := make(map[uint]struct{})
	for _, ds := range binds {
		for _, d := range ds {
			deptIDs[d] = struct{}{}
		}
	}
	flat := make([]uint, 0, len(deptIDs))
	for d := range deptIDs {
		flat = append(flat, d)
	}
	names, err := s.depts.DeptNamesByIDs(ctx, flat)
	if err != nil {
		names = map[uint]string{}
	}
	out := make(map[uint][]UserDeptItem, len(binds))
	for uid, ds := range binds {
		items := make([]UserDeptItem, 0, len(ds))
		for _, d := range ds {
			items = append(items, UserDeptItem{ID: d, Name: names[d]})
		}
		out[uid] = items
	}
	return out
}

// CreateUser 创建租户用户：平台建号 + 本地落角色绑定
func (s *Service) CreateUser(ctx context.Context, req UserCreateRequest) (*UserVO, error) {
	u, err := s.gw.CreateUser(ctx, biztenantuser.UserCreateParams{
		TenantID: req.TenantID, Username: req.Username, Password: req.Password,
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email,
	})
	if err != nil {
		return nil, err
	}
	if len(req.RoleIDs) > 0 {
		if err := s.roles.SetUserRoles(ctx, u.TenantID, u.ID, req.RoleIDs); err != nil {
			return nil, err
		}
	}
	return s.toUserVO(u, req.RoleIDs, nil, s.tenantNameMap(ctx, []uint{u.TenantID})), nil
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

// SetUserRoles 全量替换本地角色绑定：先经平台定位用户（防 id 漂移/越权），
// 角色归属校验（同租户）在 RoleUsecase
func (s *Service) SetUserRoles(ctx context.Context, id uint, req SetRolesRequest) error {
	u, err := s.gw.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if req.RoleIDs == nil {
		req.RoleIDs = []uint{}
	}
	return s.roles.SetUserRoles(ctx, u.TenantID, id, req.RoleIDs)
}

// SetUserDepts 全量替换本地部门绑定（多部门）：先经平台定位用户（防 id 漂移/越权），
// 部门归属校验（同租户）在 tenantdept.Usecase
func (s *Service) SetUserDepts(ctx context.Context, id uint, req SetDeptsRequest) error {
	u, err := s.gw.GetUser(ctx, id)
	if err != nil {
		return err
	}
	if req.DeptIDs == nil {
		req.DeptIDs = []uint{}
	}
	return s.depts.SetUserDepts(ctx, u.TenantID, id, req.DeptIDs)
}

// DeleteUser 删除用户：平台软删账号；本地部门绑定级联清理（成员计数依赖 binds，残留会
// 虚增 member_count——与角色绑定不同，后者残留无害仅影响回填，零投影原则不做清理）。
// 平台侧直接删号无回调通道，残留为已知边界（见 boundary.md）
func (s *Service) DeleteUser(ctx context.Context, id uint) error {
	if err := s.gw.DeleteUser(ctx, id); err != nil {
		return err
	}
	if err := s.depts.ClearUserDepts(ctx, id); err != nil {
		// 账号已删成定局，清理失败仅计数虚高——告警不阻断，避免误报「删除失败」
		logger.Warn("tenant user dept binds cleanup failed after delete", zap.Uint("user_id", id), zap.Error(err))
	}
	return nil
}

// ---- 租户角色（本地 RBAC） ----

// RoleCreateRequest 创建租户角色入参（perm_codes 须在本地注册表目录内）
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

// ListRoles 租户角色列表（本地库直查）
func (s *Service) ListRoles(ctx context.Context, q biztenantuser.RoleListParams, page, pageSize int) ([]*RoleVO, pagination.Page, error) {
	list, pg, err := s.roles.ListRoles(ctx, q, page, pageSize)
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
	r, err := s.roles.CreateRole(ctx, biztenantuser.RoleCreateParams{
		TenantID: req.TenantID, Name: req.Name, Code: req.Code,
		Remark: req.Remark, PermCodes: req.PermCodes,
	})
	if err != nil {
		return nil, err
	}
	return s.toRoleVO(r, s.tenantNameMap(ctx, []uint{r.TenantID})), nil
}

func (s *Service) UpdateRole(ctx context.Context, id uint, req RoleUpdateRequest) error {
	return s.roles.UpdateRole(ctx, id, biztenantuser.RoleUpdateParams{
		Name: req.Name, Remark: req.Remark, PermCodes: req.PermCodes,
	})
}

func (s *Service) SetRolePerms(ctx context.Context, id uint, req SetPermsRequest) error {
	if req.PermCodes == nil {
		req.PermCodes = []string{}
	}
	return s.roles.SetRolePerms(ctx, id, req.PermCodes)
}

func (s *Service) DeleteRole(ctx context.Context, id uint) error {
	return s.roles.DeleteRole(ctx, id)
}

// ListPermCatalog 租户门户权限点目录（角色配权 UI 数据源；本地注册表唯一下发）
func (s *Service) ListPermCatalog() []*biztenantuser.PermDefView {
	return s.roles.PermCatalog()
}
