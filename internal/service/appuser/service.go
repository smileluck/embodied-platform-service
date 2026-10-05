// Package appuser 应用用户应用服务。
// 2026-10-05 起管理面（列表/创建/更新/删除/重置密码）经平台开放面实时消费——
// 平台（/open-api/v1/app-users）为唯一事实源，本地不再落 app_users 数据，
// tenant_ids 全链路为平台租户 ID（tenant_names 由本地租户按 platform_id 映射补齐）。
// app-auth（Login/Refresh/Profile/ChangePassword）为本地遗留登录面，随统一收尾删除，
// 届时 App 直连平台 /api/v1/app-auth（token 双用 + service AppAuth 自省）。
package appuser

import (
	"context"
	"time"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// TenantNameResolver 本地租户名解析（按平台租户 ID 只读定位；*biztenant.Usecase 满足）
type TenantNameResolver interface {
	GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error)
}

type Service struct {
	gw      bizappuser.Gateway  // 平台开放面应用用户网关（唯一事实源）
	tenants TenantNameResolver  // tenant_names 本地映射（展示用，解析失败仅缺名不阻断）
	uc      *bizappuser.Usecase // 遗留本地登录面（app-auth；统一收尾时移除）
}

func NewService(gw bizappuser.Gateway, tenants TenantNameResolver, uc *bizappuser.Usecase) *Service {
	return &Service{gw: gw, tenants: tenants, uc: uc}
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

// toVO 平台视图 → 对外 VO（tenant_names 本地映射补齐）
func (s *Service) toVO(ctx context.Context, u *bizappuser.AppUserView) *VO {
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
		if tn, err := s.tenants.GetByPlatformID(ctx, pid); err == nil && tn != nil {
			vo.TenantNames = append(vo.TenantNames, tn.Name)
		}
	}
	return vo
}

// List 本商户可见应用用户列表（实时来自平台开放面；本地无数据面）
func (s *Service) List(ctx context.Context, q bizappuser.ListParams, page, pageSize int) ([]*VO, pagination.Page, error) {
	list, pg, err := s.gw.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	out := make([]*VO, 0, len(list))
	for _, u := range list {
		out = append(out, s.toVO(ctx, u))
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
	return s.toVO(ctx, u), nil
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

// ---- 应用用户独立认证（本地遗留登录面；统一收尾删除，App 改直连平台 /app-auth） ----

// LoginRequest 应用用户登录入参（无验证码、无设备端会话概念）
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest 刷新入参
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ChangePasswordRequest 本人修改密码入参
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=64"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
}

// LoginVO 登录响应：令牌对 + 用户信息（字段命名与后台登录响应风格一致）
type LoginVO struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         *VO       `json:"user"`
}

// Login 应用用户登录（防爆破由传输层 LoginIPGuard 中间件负责）
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginVO, error) {
	u, tp, err := s.uc.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	vo := &VO{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: int(u.Status),
		TenantIDs: u.TenantIDs, TenantNames: []string{},
	}
	return &LoginVO{
		AccessToken: tp.AccessToken, RefreshToken: tp.RefreshToken, ExpiresAt: tp.ExpiresAt,
		User: vo,
	}, nil
}

// Refresh 刷新令牌（typ 隔离：只接受 app-refresh token）
func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*bizappuser.TokenPair, error) {
	return s.uc.Refresh(ctx, req.RefreshToken)
}

// Profile 当前应用用户信息（含租户关联）
func (s *Service) Profile(ctx context.Context, id uint) (*VO, error) {
	u, err := s.uc.Profile(ctx, id)
	if err != nil {
		return nil, err
	}
	vo := &VO{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: int(u.Status),
		TenantIDs: u.TenantIDs, TenantNames: u.TenantNames,
		CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: u.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	return vo, nil
}

// ChangePassword 本人修改密码（校验旧密码）
func (s *Service) ChangePassword(ctx context.Context, username string, req ChangePasswordRequest) error {
	return s.uc.ChangePassword(ctx, username, req.OldPassword, req.NewPassword)
}
