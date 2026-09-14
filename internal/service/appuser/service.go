// Package appuser 应用用户应用服务（管理端 CRUD + 应用用户独立认证）
package appuser

import (
	"context"
	"time"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
)

type Service struct {
	uc *bizappuser.Usecase
}

func NewService(uc *bizappuser.Usecase) *Service { return &Service{uc: uc} }

// ---- 管理端 ----

// CreateRequest 创建应用用户入参
type CreateRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=64"`
	Password  string `json:"password" binding:"required,min=6,max=20"`
	Nickname  string `json:"nickname" binding:"max=64"`
	Phone     string `json:"phone" binding:"max=32"`
	Email     string `json:"email" binding:"omitempty,max=128,email"`
	TenantIDs []uint `json:"tenant_ids"`
}

// UpdateRequest 更新应用用户入参（username 创建后不可改；tenant_ids 全量替换）
type UpdateRequest struct {
	Nickname  string `json:"nickname" binding:"max=64"`
	Phone     string `json:"phone" binding:"max=32"`
	Email     string `json:"email" binding:"omitempty,max=128,email"`
	Status    *int   `json:"status" binding:"omitempty,oneof=0 1"`
	TenantIDs []uint `json:"tenant_ids"`
}

// ResetPasswordRequest 重置密码入参
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// VO 应用用户视图（含租户关联，不含密码哈希）
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

// ToVO 应用用户实体转视图
func ToVO(u *bizappuser.AppUser) *VO {
	vo := &VO{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: int(u.Status),
		TenantIDs: u.TenantIDs, TenantNames: u.TenantNames,
		CreatedAt: formatTime(u.CreatedAt), UpdatedAt: formatTime(u.UpdatedAt),
	}
	if vo.TenantIDs == nil {
		vo.TenantIDs = []uint{}
	}
	if vo.TenantNames == nil {
		vo.TenantNames = []string{}
	}
	return vo
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*VO, error) {
	u, err := s.uc.Create(ctx, req.Username, req.Password, req.Nickname, req.Phone, req.Email, req.TenantIDs)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, u.ID)
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	var st *bizappuser.Status
	if req.Status != nil {
		sv := bizappuser.Status(*req.Status)
		st = &sv
	}
	return s.uc.Update(ctx, id, req.Nickname, req.Phone, req.Email, st, req.TenantIDs)
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }

func (s *Service) Get(ctx context.Context, id uint) (*VO, error) {
	u, err := s.uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToVO(u), nil
}

func (s *Service) List(ctx context.Context, q bizappuser.Query, page, pageSize int) ([]*VO, interface{}, error) {
	users, pg, err := s.uc.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*VO, 0, len(users))
	for _, u := range users {
		out = append(out, ToVO(u))
	}
	return out, pg, nil
}

func (s *Service) ResetPassword(ctx context.Context, id uint, req ResetPasswordRequest) error {
	return s.uc.ResetPassword(ctx, id, req.Password)
}

// ---- 应用用户独立认证（app-auth） ----

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
	return &LoginVO{
		AccessToken: tp.AccessToken, RefreshToken: tp.RefreshToken, ExpiresAt: tp.ExpiresAt,
		User: ToVO(u),
	}, nil
}

// Refresh 刷新令牌（typ 隔离：只接受 app-refresh token）
func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*bizappuser.TokenPair, error) {
	return s.uc.Refresh(ctx, req.RefreshToken)
}

// Profile 当前应用用户信息（含租户关联）
func (s *Service) Profile(ctx context.Context, id uint) (*VO, error) {
	return s.Get(ctx, id)
}

// ChangePassword 本人修改密码（校验旧密码）
func (s *Service) ChangePassword(ctx context.Context, username string, req ChangePasswordRequest) error {
	return s.uc.ChangePassword(ctx, username, req.OldPassword, req.NewPassword)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
