// Package appuser 应用用户应用服务。
// 2026-10-05 起管理面（列表/创建/更新/删除/重置密码）经平台开放面实时消费——
// 平台（/open-api/v1/app-users）为唯一事实源，本地不再落 app_users 数据，
// tenant_ids 全链路为平台租户 ID（tenant_names 由本地租户按 platform_id 映射补齐）。
// App 登录/刷新/改密一律直连平台 /api/v1/app-auth（token 双用）；App 直调本系统
// 走 /app-api/v1（AppAuth 中间件自省 + 租户闸门）。
// 2026-10-08 起租户门户运营账号收敛为平台 tenant_users（见 /tenant-api/v1 与
// 租户用户管理页），应用用户不再承担门户管理员语义（本地租户端角色已退役）。
package appuser

import (
	"context"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// TenantNameResolver 本地租户名解析（按平台租户 ID；*biztenant.Usecase 满足）。
// 列表走批量方法（整页一次查询消除 N+1），单个定位用于 AppAuth 闸门等场景
type TenantNameResolver interface {
	GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error)
	GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*biztenant.Tenant, error)
}

type Service struct {
	gw      bizappuser.Gateway // 平台开放面应用用户网关（唯一事实源）
	tenants TenantNameResolver // tenant_names 本地映射（展示用，解析失败仅缺名不阻断）
}

func NewService(gw bizappuser.Gateway, tenants TenantNameResolver) *Service {
	return &Service{gw: gw, tenants: tenants}
}

// ---- 管理面（经平台开放面实时消费） ----

// CreateRequest 创建应用用户入参
type CreateRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=64"`
	Password  string `json:"password" binding:"required,min=6,max=20"`
	Nickname  string `json:"nickname" binding:"max=64"`
	Phone     string `json:"phone" binding:"max=32"`
	Email     string `json:"email" binding:"omitempty,max=128,email"`
	TenantIDs []uint `json:"tenant_ids"` // 平台租户 ID，须⊆本商户绑定租户集
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
// id 为平台 app_user ID，tenant_ids 为平台租户 ID）
type VO struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	Nickname    string   `json:"nickname"`
	Phone       string   `json:"phone"`
	Email       string   `json:"email"`
	Status      int      `json:"status"`
	TenantIDs   []uint   `json:"tenant_ids"`
	TenantNames []string `json:"tenant_names"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// toVO 平台视图 → 对外 VO（tenant_names 由 nameMap 补齐；未同步的租户缺名不阻断）
func (s *Service) toVO(u *bizappuser.AppUserView, nameMap map[uint]string) *VO {
	vo := &VO{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		TenantIDs: u.TenantIDs, TenantNames: []string{},
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
	return s.toVO(u, s.tenantNameMap(ctx, []*bizappuser.AppUserView{u})), nil
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
