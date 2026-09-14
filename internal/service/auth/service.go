// Package auth 认证应用服务（薄用例编排，方法签名与 api/auth/v1 契约对应）
package auth

import (
	"context"
	"errors"

	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizcaptcha "github.com/smilex/smilex-admin-gin/internal/biz/captcha"
	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
)

type Service struct {
	uc      *bizauth.Usecase
	captcha *bizcaptcha.Usecase
}

func NewService(uc *bizauth.Usecase, captcha *bizcaptcha.Usecase) *Service {
	return &Service{uc: uc, captcha: captcha}
}

// LoginRequest 登录入参
type LoginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	CaptchaID   string `json:"captcha_id"`                                    // 验证码停用时可空
	CaptchaCode string `json:"captcha_code"`                                  // 验证码停用时可空
	DeviceType  string `json:"device_type" binding:"omitempty,oneof=web app"` // 设备端：同端互斥、异端并存
	IP          string `json:"-"`                                             // 由传输层注入（建立会话用）
	UserAgent   string `json:"-"`                                             // 由传输层注入（建立会话用）
}

// RefreshRequest 刷新入参
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*bizauth.TokenPair, error) {
	meta := bizauth.LoginMeta{Device: req.DeviceType, IP: req.IP, UserAgent: req.UserAgent}
	return s.uc.Login(ctx, req.Username, req.Password, req.CaptchaID, req.CaptchaCode, meta)
}

// CaptchaVO 图形验证码视图（image 为 PNG 的 base64，无 data: 前缀）
type CaptchaVO struct {
	CaptchaID    string `json:"captcha_id"`
	CaptchaImage string `json:"captcha_image"`
	Enabled      bool   `json:"enabled"` // false 时前端隐藏验证码表单
}

// GenerateCaptcha 生成一次性图形验证码；验证码停用时返回 enabled=false 的空视图
func (s *Service) GenerateCaptcha() (*CaptchaVO, error) {
	if !s.captcha.Enabled() {
		return &CaptchaVO{Enabled: false}, nil
	}
	id, b64, err := s.captcha.Generate()
	if err != nil {
		return nil, err
	}
	return &CaptchaVO{CaptchaID: id, CaptchaImage: b64, Enabled: true}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*bizauth.TokenPair, error) {
	return s.uc.Refresh(ctx, req.RefreshToken)
}

// Logout 登出：吊销当前会话
func (s *Service) Logout(ctx context.Context, sid string) error {
	return s.uc.Logout(ctx, sid)
}

// ProfileUserVO 个人信息用户视图（专用 VO，字段显式声明，密码不可能泄露）
type ProfileUserVO struct {
	ID        uint     `json:"id"`
	Username  string   `json:"username"`
	Nickname  string   `json:"nickname"`
	Email     string   `json:"email"`
	Status    int      `json:"status"`
	RoleNames []string `json:"role_names"`
	CreatedAt string   `json:"created_at"`
}

// ProfileVO 个人信息视图
type ProfileVO struct {
	User        *ProfileUserVO           `json:"user"`
	Permissions []*permission.Permission `json:"permissions"`
}

func (s *Service) Profile(ctx context.Context, userID uint) (*ProfileVO, error) {
	p, err := s.uc.Profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.User == nil {
		return nil, errors.New("user not found")
	}
	u := p.User
	vo := &ProfileUserVO{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname, Email: u.Email,
		Status: int(u.Status), RoleNames: p.RoleNames, CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if vo.RoleNames == nil {
		vo.RoleNames = []string{}
	}
	return &ProfileVO{User: vo, Permissions: p.Permissions}, nil
}

// UpdateProfileRequest 本人更新资料入参
type UpdateProfileRequest struct {
	Nickname string `json:"nickname" binding:"max=20"`
	Email    string `json:"email" binding:"omitempty,max=128,email"`
}

// UpdateProfile 本人更新昵称/邮箱，返回更新后的完整个人信息
func (s *Service) UpdateProfile(ctx context.Context, userID uint, req UpdateProfileRequest) (*ProfileVO, error) {
	if _, err := s.uc.UpdateProfile(ctx, userID, req.Nickname, req.Email); err != nil {
		return nil, err
	}
	return s.Profile(ctx, userID)
}

// ChangePasswordRequest 本人修改密码入参
// 新密码限 6-20 位；旧密码保留 64 位上限以兼容历史长密码
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=64"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
}

// ChangePassword 本人修改密码（校验旧密码；成功后吊销其他端会话，当前会话由调用方传入）
func (s *Service) ChangePassword(ctx context.Context, userID uint, req ChangePasswordRequest, currentSessionID string) error {
	return s.uc.ChangePassword(ctx, userID, req.OldPassword, req.NewPassword, currentSessionID)
}

// Authorize 供 RBAC 中间件调用
func (s *Service) Authorize(ctx context.Context, userID uint, method, path string) bool {
	return s.uc.Authorize(ctx, userID, method, path)
}

// ParseSubject 解析 access token
func (s *Service) ParseSubject(token string) (*bizauth.Subject, error) {
	return s.uc.ParseSubject(token)
}

// ValidateSession 会话是否存活（中间件用）
func (s *Service) ValidateSession(ctx context.Context, sid string) bool {
	return s.uc.ValidateSession(ctx, sid)
}
