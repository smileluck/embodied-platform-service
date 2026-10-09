// Package tenantmember 租户门户成员应用服务（/tenant-api/v1/members）。
// 成员=平台租户用户（账号身份事实源在平台，经开放面实时消费）；角色与绑定自 2026-10-09
// 本地化（RoleUsecase）。tenant_id 一律锁定为认证主体的 tid（DTO 不接受客户端指定）。
// 防失管守卫（本人/最后管理员）在本层实现：启停状态按平台账号、权限按本地绑定判定。
package tenantmember

import (
	"context"
	"errors"

	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

var (
	// ErrMemberNotInTenant 成员不属于当前租户（含已被移除/平台侧越界）
	ErrMemberNotInTenant = errors.New("成员不属于当前租户")
	// ErrLastTenantAdmin 租户内最后一名可管成员的账号不可移除/降级/禁用（防租户失管；
	// 商户管理端仍可代管恢复，但门户应保持自治能力）
	ErrLastTenantAdmin = errors.New("租户内最后一名管理员不可执行该操作")
	// ErrCannotModifySelf 不可移除/禁用/变更自己（留任交由其他管理员操作）
	ErrCannotModifySelf = errors.New("不可对本人执行该操作")
)

// memberPermPrefix 成员自治域权限码前缀（持有任一 member:* 权限=可参与成员自治，
// 最后管理员守卫按此判定「失管」）
const memberPermPrefix = "member:"

type Service struct {
	gw    biztenantuser.Gateway      // 平台开放面租户用户网关（账号身份；tenant_id 由本层锁定）
	roles *biztenantuser.RoleUsecase // 本地租户角色用例（角色/绑定/权限）
}

func NewService(gw biztenantuser.Gateway, roles *biztenantuser.RoleUsecase) *Service {
	return &Service{gw: gw, roles: roles}
}

// CreateRequest 新增成员入参（role_ids 为本租户下本地租户角色，缺省无角色）
type CreateRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=20"`
	Nickname string `json:"nickname" binding:"max=64"`
	Phone    string `json:"phone" binding:"max=32"`
	Email    string `json:"email" binding:"omitempty,max=128,email"`
	RoleIDs  []uint `json:"role_ids"`
}

// UpdateRequest 成员资料更新入参（tenant_id 不可改：跨租户归属是商户管理端职责）
type UpdateRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,max=64"`
	Phone    *string `json:"phone" binding:"omitempty,max=32"`
	Email    *string `json:"email" binding:"omitempty,max=128,email"`
}

// SetStatusRequest 启停入参
type SetStatusRequest struct {
	Status int `json:"status" binding:"required,oneof=0 1"`
}

// ResetPasswordRequest 重置密码入参（旧密码立即失效）
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// SetRolesRequest 角色全量替换入参
type SetRolesRequest struct {
	RoleIDs []uint `json:"role_ids"`
}

// VO 成员视图（id/role_ids 均为平台/本地口径；tenant_id 为平台租户 ID）
type VO struct {
	ID        uint   `json:"id"`
	TenantID  uint   `json:"tenant_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    int    `json:"status"`
	RoleIDs   []uint `json:"role_ids"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toVO(u *biztenantuser.TenantUserView, roleIDs []uint) *VO {
	vo := &VO{
		ID: u.ID, TenantID: u.TenantID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
	if roleIDs == nil {
		roleIDs = []uint{}
	}
	vo.RoleIDs = roleIDs
	return vo
}

// listAllUsers 拉取本租户全部成员（分页循环；启停状态事实源在平台——最后管理员守卫用；
// 单租户运营账号规模有限，20 页×100 兜底）
func (s *Service) listAllUsers(ctx context.Context, tid uint) ([]*biztenantuser.TenantUserView, error) {
	var all []*biztenantuser.TenantUserView
	for page := 1; page <= 20; page++ {
		list, pg, err := s.gw.ListUsers(ctx, biztenantuser.UserListParams{TenantID: &tid}, page, 100)
		if err != nil {
			return nil, err
		}
		all = append(all, list...)
		if len(list) == 0 || pg.Page*pg.PageSize >= int(pg.Total) {
			break
		}
	}
	return all, nil
}

// locateMember 在本租户内定位成员（平台按 id 单查；越界/不存在统一 ErrMemberNotInTenant）
func (s *Service) locateMember(ctx context.Context, tid, id uint) (*biztenantuser.TenantUserView, error) {
	u, err := s.gw.GetUser(ctx, id)
	if err != nil {
		return nil, ErrMemberNotInTenant
	}
	if u.TenantID != tid {
		return nil, ErrMemberNotInTenant
	}
	return u, nil
}

// rolePerms 本租户角色 ID → 权限码集合（本地库直查；角色列表加载失败按空集保守处理）
func (s *Service) rolePerms(ctx context.Context, tid uint) (map[uint][]string, error) {
	roles, _, err := s.roles.ListRoles(ctx, biztenantuser.RoleListParams{TenantID: &tid}, 1, 0)
	if err != nil {
		return nil, err
	}
	m := make(map[uint][]string, len(roles))
	for _, r := range roles {
		m[r.ID] = r.PermCodes
	}
	return m, nil
}

// hasMemberPerms 用户是否持有成员自治域权限（角色 ∩ member:* 前缀）
func hasMemberPerms(roleIDs []uint, rolePerms map[uint][]string) bool {
	for _, rid := range roleIDs {
		for _, p := range rolePerms[rid] {
			if len(p) >= len(memberPermPrefix) && p[:len(memberPermPrefix)] == memberPermPrefix {
				return true
			}
		}
	}
	return false
}

// guardNotLastAdmin 防失管守卫：目标成员是本租户最后一名启用中的成员自治管理员，
// 且本次操作会使其失去该能力时拒绝（禁用/移除/角色替换为无 member 权限）。
// 启停状态按平台账号（事实源），权限映射按本地角色（一条查询）。
func (s *Service) guardNotLastAdmin(ctx context.Context, tid, targetID uint, newRoleIDs *[]uint) error {
	list, err := s.listAllUsers(ctx, tid)
	if err != nil {
		return err
	}
	rolePerms, err := s.rolePerms(ctx, tid)
	if err != nil {
		return err
	}
	roleMap, err := s.roles.UserRoleIDs(ctx, memberIDs(list))
	if err != nil {
		return err
	}
	var targetFound bool
	others := 0
	for _, u := range list {
		if !hasMemberPerms(roleMap[u.ID], rolePerms) || u.Status != 1 {
			continue
		}
		if u.ID == targetID {
			targetFound = true
			continue
		}
		others++
	}
	if !targetFound || others > 0 {
		return nil // 目标本就不持成员权限，或仍有其他管理员
	}
	if newRoleIDs != nil && hasMemberPerms(*newRoleIDs, rolePerms) {
		return nil // 替换后仍持成员权限
	}
	return ErrLastTenantAdmin
}

// memberIDs 提取成员 id 集（本地绑定批量查询用）
func memberIDs(list []*biztenantuser.TenantUserView) []uint {
	ids := make([]uint, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	return ids
}

// List 本租户成员列表（账号来自平台；role_ids 本地绑定补齐）
func (s *Service) List(ctx context.Context, tid uint, kw string, page, pageSize int) ([]*VO, pagination.Page, error) {
	list, pg, err := s.gw.ListUsers(ctx, biztenantuser.UserListParams{Keyword: kw, TenantID: &tid}, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	roleMap, err := s.roles.UserRoleIDs(ctx, memberIDs(list))
	if err != nil {
		return nil, pagination.Page{}, err
	}
	out := make([]*VO, 0, len(list))
	for _, u := range list {
		out = append(out, toVO(u, roleMap[u.ID]))
	}
	return out, pg, nil
}

// Create 在本租户新增成员（tenant_id 锁定 tid）：平台建号 + 本地落角色绑定
func (s *Service) Create(ctx context.Context, tid uint, req CreateRequest) (*VO, error) {
	u, err := s.gw.CreateUser(ctx, biztenantuser.UserCreateParams{
		TenantID: tid, Username: req.Username, Password: req.Password,
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email,
	})
	if err != nil {
		return nil, err
	}
	if len(req.RoleIDs) > 0 {
		if err := s.roles.SetUserRoles(ctx, tid, u.ID, req.RoleIDs); err != nil {
			return nil, err
		}
	}
	return toVO(u, req.RoleIDs), nil
}

// Update 更新成员资料
func (s *Service) Update(ctx context.Context, tid, id uint, req UpdateRequest) error {
	if _, err := s.locateMember(ctx, tid, id); err != nil {
		return err
	}
	return s.gw.UpdateUser(ctx, id, biztenantuser.UserUpdateParams{
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email,
	})
}

// SetStatus 启停成员（不可对本人操作；禁用受最后管理员守卫）
func (s *Service) SetStatus(ctx context.Context, tid, id, actorID uint, status int) error {
	if _, err := s.locateMember(ctx, tid, id); err != nil {
		return err
	}
	if id == actorID {
		return ErrCannotModifySelf
	}
	if status != 1 {
		if err := s.guardNotLastAdmin(ctx, tid, id, nil); err != nil {
			return err
		}
	}
	return s.gw.SetUserStatus(ctx, id, status)
}

// ResetPassword 重置成员密码
func (s *Service) ResetPassword(ctx context.Context, tid, id uint, password string) error {
	if _, err := s.locateMember(ctx, tid, id); err != nil {
		return err
	}
	return s.gw.ResetUserPassword(ctx, id, password)
}

// Remove 删除成员（平台软删账号；不可对本人操作，受最后管理员守卫。
// 本地角色绑定随账号失能自然失义，零投影原则不做对账清理）
func (s *Service) Remove(ctx context.Context, tid, id, actorID uint) error {
	if _, err := s.locateMember(ctx, tid, id); err != nil {
		return err
	}
	if id == actorID {
		return ErrCannotModifySelf
	}
	if err := s.guardNotLastAdmin(ctx, tid, id, nil); err != nil {
		return err
	}
	return s.gw.DeleteUser(ctx, id)
}

// SetRoles 全量替换成员角色（本地绑定；不可对本人操作，受最后管理员守卫）
func (s *Service) SetRoles(ctx context.Context, tid, id, actorID uint, roleIDs []uint) error {
	if _, err := s.locateMember(ctx, tid, id); err != nil {
		return err
	}
	if id == actorID {
		return ErrCannotModifySelf
	}
	if err := s.guardNotLastAdmin(ctx, tid, id, &roleIDs); err != nil {
		return err
	}
	return s.roles.SetUserRoles(ctx, tid, id, roleIDs)
}

// RoleVO 角色选项视图（门户成员表单数据源；snake_case 与管理面 RoleVO 口径一致）
type RoleVO struct {
	ID        uint     `json:"id"`
	TenantID  uint     `json:"tenant_id"`
	Name      string   `json:"name"`
	Code      string   `json:"code"`
	Remark    string   `json:"remark"`
	PermCodes []string `json:"perm_codes"`
}

// ListRoles 本租户角色列表（门户成员表单数据源；本地库直查）
func (s *Service) ListRoles(ctx context.Context, tid uint) ([]*RoleVO, error) {
	roles, _, err := s.roles.ListRoles(ctx, biztenantuser.RoleListParams{TenantID: &tid}, 1, 0)
	if err != nil {
		return nil, err
	}
	out := make([]*RoleVO, 0, len(roles))
	for _, r := range roles {
		vo := &RoleVO{
			ID: r.ID, TenantID: r.TenantID, Name: r.Name, Code: r.Code,
			Remark: r.Remark, PermCodes: r.PermCodes,
		}
		if vo.PermCodes == nil {
			vo.PermCodes = []string{}
		}
		out = append(out, vo)
	}
	return out, nil
}
