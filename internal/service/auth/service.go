// Package auth 认证应用服务（薄用例编排）。
// 登录不在本服务：前端直调平台 POST /api/v1/auth/login（token 双用），
// 本服务只做 token 自省（introspection）+ 本地准入判定 + 自身数据代理。
package auth

import (
	"context"

	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
)

type Service struct {
	uc *bizauth.Usecase
}

func NewService(uc *bizauth.Usecase) *Service { return &Service{uc: uc} }

// Introspect 平台 token 自省（中间件用；缓存由传输层的二级缓存承担）
func (s *Service) Introspect(ctx context.Context, token string) (*bizauth.Subject, error) {
	return s.uc.Introspect(ctx, token)
}

// Authorize 供 RBAC 中间件调用（未准入直接拒绝）
func (s *Service) Authorize(ctx context.Context, platformUserID uint, method, path string) bool {
	return s.uc.Authorize(ctx, platformUserID, method, path)
}

// ProfileUserVO 个人信息用户视图（平台身份 + 准入状态）
type ProfileUserVO struct {
	ID        uint     `json:"id"` // 平台用户 ID
	Username  string   `json:"username"`
	Nickname  string   `json:"nickname"`
	Email     string   `json:"email"`
	Admitted  bool     `json:"admitted"` // 本系统准入状态
	RoleNames []string `json:"role_names"`
}

// ProfileVO 个人信息视图
type ProfileVO struct {
	User        *ProfileUserVO           `json:"user"`
	Permissions []*permission.Permission `json:"permissions"`
}

func (s *Service) Profile(ctx context.Context, sub *bizauth.Subject) (*ProfileVO, error) {
	p, err := s.uc.Profile(ctx, sub)
	if err != nil {
		return nil, err
	}
	vo := &ProfileVO{
		User: &ProfileUserVO{
			ID: p.Subject.UserID, Username: p.Subject.Username,
			Nickname: p.Subject.Nickname, Email: p.Subject.Email,
			Admitted: p.Admitted, RoleNames: p.RoleNames,
		},
		Permissions: p.Permissions,
	}
	if vo.User.RoleNames == nil {
		vo.User.RoleNames = []string{}
	}
	if vo.Permissions == nil {
		vo.Permissions = []*permission.Permission{}
	}
	return vo, nil
}

// UpdateProfileRequest 本人更新资料入参（代理到平台）
type UpdateProfileRequest struct {
	Nickname string `json:"nickname" binding:"max=20"`
	Email    string `json:"email" binding:"omitempty,max=128,email"`
}

// UpdateProfile 代理平台「本人更新昵称/邮箱」（用户本人平台 token）
func (s *Service) UpdateProfile(ctx context.Context, token string, req UpdateProfileRequest) error {
	return s.uc.UpdateProfile(ctx, token, req.Nickname, req.Email)
}

// ChangePasswordRequest 本人修改密码入参（代理到平台；平台侧校验旧密码）
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=64"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
}

// ChangePassword 代理平台「本人修改密码」
func (s *Service) ChangePassword(ctx context.Context, token string, req ChangePasswordRequest) error {
	return s.uc.ChangePassword(ctx, token, req.OldPassword, req.NewPassword)
}
