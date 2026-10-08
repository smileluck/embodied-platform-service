// Package appuser 应用用户应用服务。
// 2026-10-05 起管理面（列表/创建/更新/删除/重置密码）经平台开放面实时消费——
// 平台（/open-api/v1/app-users）为唯一事实源，本地不再落 app_users 数据，
// tenant_ids 全链路为平台租户 ID（tenant_names 由本地租户按 platform_id 映射补齐）。
// App 登录/刷新/改密一律直连平台 /api/v1/app-auth（token 双用）；App 直调本系统
// 走 /app-api/v1（AppAuth 中间件自省 + 租户闸门）。
package appuser

import (
	"context"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	biztenantmember "github.com/smilex/smilex-admin-gin/internal/biz/tenantmember"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// TenantNameResolver 本地租户名解析（按平台租户 ID；*biztenant.Usecase 满足）。
// 列表走批量方法（整页一次查询消除 N+1），单个定位用于 AppAuth 闸门等场景
type TenantNameResolver interface {
	GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error)
	GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*biztenant.Tenant, error)
}

// TenantRoleReader 租户端角色批量读取（*biztenantmember.Usecase 满足）：
// 管理面列表补齐应用用户的租户端角色标注
type TenantRoleReader interface {
	RolesOf(ctx context.Context, appUserIDs []uint, tenantPlatformID uint) (map[uint]biztenantmember.Role, error)
	SetMemberRole(ctx context.Context, tenantPlatformID, appUserID, actorID uint, role biztenantmember.Role) error
}

type Service struct {
	gw      bizappuser.Gateway       // 平台开放面应用用户网关（唯一事实源）
	tenants TenantNameResolver       // tenant_names 本地映射（展示用，解析失败仅缺名不阻断）
	roles   TenantRoleReader         // 租户端角色（列表标注/创建时指定/单独设置）
}

func NewService(gw bizappuser.Gateway, tenants TenantNameResolver, roles TenantRoleReader) *Service {
	return &Service{gw: gw, tenants: tenants, roles: roles}
}

// ---- 管理面（经平台开放面实时消费） ----

// CreateRequest 创建应用用户入参（tenant_roles 为租户端角色初始绑定，
// 逐条落在 tenant_ids 内的租户上；缺省 member）
type CreateRequest struct {
	Username   string            `json:"username" binding:"required,min=3,max=64"`
	Password   string            `json:"password" binding:"required,min=6,max=20"`
	Nickname   string            `json:"nickname" binding:"max=64"`
	Phone      string            `json:"phone" binding:"max=32"`
	Email      string            `json:"email" binding:"omitempty,max=128,email"`
	TenantIDs  []uint            `json:"tenant_ids"` // 平台租户 ID，须⊆本商户绑定租户集
	TenantRole []TenantRoleInput `json:"tenant_roles"`
}

// TenantRoleInput 租户端角色设置（tenant_id 为平台租户 ID，须在用户归属集内）
type TenantRoleInput struct {
	TenantID uint   `json:"tenant_id" binding:"required"`
	Role     string `json:"role" binding:"required,oneof=tenant_admin member"`
}

// UpdateRequest 更新应用用户入参（username 创建后不可改；tenant_ids 全量替换本商户范围内归属）。
// 字段可选：省略即不修改，状态切换等局部更新不会清空其余资料。
type UpdateRequest struct {
	Nickname  *string `json:"nickname" binding:"omitempty,max=64"`
	Phone     *string `json:"phone" binding:"omitempty,max=32"`
	Email     *string `json:"email" binding:"omitempty,max=128,email"`
	Status    *int    `json:"status" binding:"omitempty,oneof=0 1"`
	TenantIDs *[]uint `json:"tenant_ids"` // 平台租户 ID
}

// ResetPasswordRequest 重置密码入参
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// VO 应用用户视图（对外形状与本地时代一致，前端无感；
// id 为平台 app_user ID，tenant_ids 为平台租户 ID，tenant_roles 为租户端角色标注）
type VO struct {
	ID          uint            `json:"id"`
	Username    string          `json:"username"`
	Nickname    string          `json:"nickname"`
	Phone       string          `json:"phone"`
	Email       string          `json:"email"`
	Status      int             `json:"status"`
	TenantIDs   []uint          `json:"tenant_ids"`
	TenantNames []string        `json:"tenant_names"`
	TenantRoles []TenantRoleVO  `json:"tenant_roles"` // [{tenant_id, role}]（读取失败仅缺标注）
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// TenantRoleVO 租户端角色标注
type TenantRoleVO struct {
	TenantID uint   `json:"tenant_id"`
	Role     string `json:"role"`
}

// toVO 平台视图 → 对外 VO（tenant_names 由 nameMap 补齐；未同步的租户缺名不阻断）
func (s *Service) toVO(u *bizappuser.AppUserView, nameMap map[uint]string) *VO {
	vo := &VO{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		TenantIDs: u.TenantIDs, TenantNames: []string{}, TenantRoles: []TenantRoleVO{},
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
	if vo.TenantIDs == nil {
		vo.TenantIDs = []uint{}
	}
	for _, pid := range u.TenantIDs {
		if name, ok := nameMap[pid]; ok {
			vo.TenantNames = append(vo.TenantNames, name)
		}
	}
	return vo
}

// attachTenantRoles 列表角色标注：按整页出现过的租户逐个批量查绑定（每租户一次
// 查询），只落在用户当前归属集内的租户上（孤儿绑定不展示）；失败仅缺标注不阻断
func (s *Service) attachTenantRoles(ctx context.Context, vos []*VO) {
	if s.roles == nil || len(vos) == 0 {
		return
	}
	seen := make(map[uint]struct{})
	for _, vo := range vos {
		for _, pid := range vo.TenantIDs {
			seen[pid] = struct{}{}
		}
	}
	ids := make([]uint, 0, len(vos))
	for _, vo := range vos {
		ids = append(ids, vo.ID)
	}
	byUser := make(map[uint]map[uint]string, len(vos))
	for pid := range seen {
		roleMap, err := s.roles.RolesOf(ctx, ids, pid)
		if err != nil {
			continue
		}
		for uid, r := range roleMap {
			if byUser[uid] == nil {
				byUser[uid] = map[uint]string{}
			}
			byUser[uid][pid] = string(biztenantmember.NormalizeRole(r))
		}
	}
	for _, vo := range vos {
		for _, pid := range vo.TenantIDs {
			if r, ok := byUser[vo.ID][pid]; ok {
				vo.TenantRoles = append(vo.TenantRoles, TenantRoleVO{TenantID: pid, Role: r})
			}
		}
	}
}

// tenantNameMap 批量解析平台租户 ID → 本地租户名（一次查询；空集不发查询）
func (s *Service) tenantNameMap(ctx context.Context, views []*bizappuser.AppUserView) map[uint]string {
	seen := make(map[uint]struct{})
	ids := make([]uint, 0, len(views)*2)
	for _, u := range views {
		for _, pid := range u.TenantIDs {
			if _, ok := seen[pid]; !ok {
				seen[pid] = struct{}{}
				ids = append(ids, pid)
			}
		}
	}
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

// List 本商户可见应用用户列表（实时来自平台开放面；本地无数据面）
func (s *Service) List(ctx context.Context, q bizappuser.ListParams, page, pageSize int) ([]*VO, pagination.Page, error) {
	list, pg, err := s.gw.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	nameMap := s.tenantNameMap(ctx, list)
	out := make([]*VO, 0, len(list))
	for _, u := range list {
		out = append(out, s.toVO(u, nameMap))
	}
	s.attachTenantRoles(ctx, out)
	return out, pg, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*VO, error) {
	u, err := s.gw.Create(ctx, bizappuser.CreateParams{
		Username: req.Username, Password: req.Password, Nickname: req.Nickname,
		Phone: req.Phone, Email: req.Email, TenantIDs: req.TenantIDs,
	})
	if err != nil {
		return nil, err
	}
	// 租户端角色初始绑定（actor=0：商户管理端操作，不受本人守卫限制；
	// 最后管理员守卫仍生效——新任命不触碰既有管理员，天然不冲突）
	for _, tr := range req.TenantRole {
		_ = s.roles.SetMemberRole(ctx, tr.TenantID, u.ID, 0, biztenantmember.Role(tr.Role))
	}
	return s.toVO(u, s.tenantNameMap(ctx, []*bizappuser.AppUserView{u})), nil
}

// SetTenantRole 单独设置应用用户在某租户的租户端角色（管理面修复/调整入口）
func (s *Service) SetTenantRole(ctx context.Context, appUserID uint, req TenantRoleInput) error {
	return s.roles.SetMemberRole(ctx, req.TenantID, appUserID, 0, biztenantmember.Role(req.Role))
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	return s.gw.Update(ctx, id, bizappuser.UpdateParams{
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email,
		Status: req.Status, TenantIDs: req.TenantIDs,
	})
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.gw.Delete(ctx, id) }

func (s *Service) ResetPassword(ctx context.Context, id uint, req ResetPasswordRequest) error {
	return s.gw.ResetPassword(ctx, id, req.Password)
}
