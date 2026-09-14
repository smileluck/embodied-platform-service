package auth

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/session"
	"github.com/smilex/smilex-admin-gin/internal/biz/user"
)

var (
	// ErrInvalidCredentials 用户名或密码错误 / 用户被禁用
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrDisabledAccount 账号被禁用
	ErrDisabledAccount = errors.New("account disabled")
	// ErrCaptcha 验证码缺失/过期/错误
	ErrCaptcha = errors.New("captcha code invalid")
)

// UserStore 认证上下文依赖的用户存取接口（只依赖所需最小方法，而非整个 user.Repo）
type UserStore interface {
	FindByUsername(ctx context.Context, username string) (*user.User, error)
	FindByID(ctx context.Context, id uint) (*user.User, error)
	// FindByIDWithPassword 携带密码哈希返回（本人改密校验旧密码用）
	FindByIDWithPassword(ctx context.Context, id uint) (*user.User, error)
	// Update 本人更新基础资料（仓储层仅写昵称/邮箱/状态）
	Update(ctx context.Context, u *user.User) error
	// UpdatePassword 单独落库密码哈希
	UpdatePassword(ctx context.Context, id uint, passwordHash string) error
}

// RoleNameReader 角色名读取接口（个人中心展示角色名）
type RoleNameReader interface {
	FindNamesByIDs(ctx context.Context, ids []uint) ([]string, error)
}

// PermissionReader 权限读取接口（跨上下文走接口）
type PermissionReader interface {
	FindByUserID(ctx context.Context, userID uint) ([]*permission.Permission, error)
}

// CaptchaVerifier 登录验证码校验接口（由 captcha 上下文实现，跨上下文走接口）
type CaptchaVerifier interface {
	Verify(id, answer string) bool
}

// SessionManager 会话管理接口（由 session 上下文实现，跨上下文走接口）
type SessionManager interface {
	Create(ctx context.Context, userID uint, username, nickname, device, ip, userAgent string) (*session.Session, error)
	Validate(ctx context.Context, sid string) bool
	Extend(ctx context.Context, sid string) error
	Revoke(ctx context.Context, sid string) error
	RevokeByUser(ctx context.Context, userID uint) (int, error)
	RevokeByUserExcept(ctx context.Context, userID uint, keepSid string) (int, error)
}

// LoginMeta 登录环境信息（由传输层注入，用于建立会话）
type LoginMeta struct {
	Device    string // 设备端：web / app（空默认 web）
	IP        string
	UserAgent string
}

// Usecase 认证领域用例
type Usecase struct {
	users    UserStore
	roles    RoleNameReader
	perms    PermissionReader
	tokens   TokenIssuer
	captcha  CaptchaVerifier
	sessions SessionManager
}

func NewUsecase(users UserStore, roles RoleNameReader, perms PermissionReader, tokens TokenIssuer, captcha CaptchaVerifier, sessions SessionManager) *Usecase {
	return &Usecase{users: users, roles: roles, perms: perms, tokens: tokens, captcha: captcha, sessions: sessions}
}

// Login 登录签发令牌（先校验一次性图形验证码）。
// 每次登录建立一个会话并写入 token（sid）：同端旧会话被互斥吊销，不同端并存。
func (uc *Usecase) Login(ctx context.Context, username, password, captchaID, captchaCode string, meta LoginMeta) (*TokenPair, error) {
	if !uc.captcha.Verify(captchaID, captchaCode) {
		return nil, ErrCaptcha
	}
	u, err := uc.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !u.CheckPassword(password) {
		return nil, ErrInvalidCredentials
	}
	if !u.Enabled() {
		return nil, ErrDisabledAccount
	}
	sess, err := uc.sessions.Create(ctx, u.ID, u.Username, u.Nickname, meta.Device, meta.IP, meta.UserAgent)
	if err != nil {
		return nil, err
	}
	subject := Subject{UserID: u.ID, Username: u.Username, SessionID: sess.ID}
	access, expiresAt, err := uc.tokens.IssueAccessToken(subject)
	if err != nil {
		return nil, err
	}
	refresh, err := uc.tokens.IssueRefreshToken(subject)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: expiresAt}, nil
}

// Refresh 刷新令牌：会话必须存活，轮转后同 sid 续期
func (uc *Usecase) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	s, err := uc.tokens.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	if s.SessionID == "" || !uc.sessions.Validate(ctx, s.SessionID) {
		return nil, errors.New("session revoked")
	}
	u, err := uc.users.FindByID(ctx, s.UserID)
	if err != nil || !u.Enabled() {
		return nil, ErrInvalidCredentials
	}
	if err := uc.sessions.Extend(ctx, s.SessionID); err != nil {
		return nil, err
	}
	subject := Subject{UserID: u.ID, Username: u.Username, SessionID: s.SessionID}
	access, expiresAt, err := uc.tokens.IssueAccessToken(subject)
	if err != nil {
		return nil, err
	}
	refresh, err := uc.tokens.IssueRefreshToken(subject)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: expiresAt}, nil
}

// Logout 登出：吊销当前会话，token 立即失效
func (uc *Usecase) Logout(ctx context.Context, sid string) error {
	if sid == "" {
		return nil
	}
	return uc.sessions.Revoke(ctx, sid)
}

// Profile 当前用户信息（含角色名与权限）
func (uc *Usecase) Profile(ctx context.Context, userID uint) (*Profile, error) {
	u, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	ps, err := uc.perms.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// 角色名仅用于展示，读取失败时降级为空列表，不阻断个人中心
	roleNames, err := uc.roles.FindNamesByIDs(ctx, u.RoleIDs)
	if err != nil {
		roleNames = nil
	}
	return &Profile{User: u, RoleNames: roleNames, Permissions: ps}, nil
}

// UpdateProfile 本人更新基础资料（昵称/邮箱；不经过管理员接口的超管保护）
func (uc *Usecase) UpdateProfile(ctx context.Context, userID uint, nickname, email string) (*user.User, error) {
	u, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if nickname != "" {
		u.Nickname = nickname
	}
	if email != "" {
		u.Email = email
	}
	if err := uc.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// ChangePassword 本人修改密码：先校验旧密码，再落库新哈希；
// 成功后吊销除当前会话外的其他端会话（当前端不掉线）
func (uc *Usecase) ChangePassword(ctx context.Context, userID uint, oldPlain, newPlain, currentSessionID string) error {
	u, err := uc.users.FindByIDWithPassword(ctx, userID)
	if err != nil {
		return err
	}
	if !u.CheckPassword(oldPlain) {
		return ErrInvalidCredentials
	}
	if err := u.SetPassword(newPlain); err != nil {
		return err
	}
	if err := uc.users.UpdatePassword(ctx, userID, string(u.Password)); err != nil {
		return err
	}
	_, _ = uc.sessions.RevokeByUserExcept(ctx, userID, currentSessionID)
	return nil
}

// ParseSubject 解析 access token
func (uc *Usecase) ParseSubject(token string) (*Subject, error) {
	return uc.tokens.ParseAccessToken(token)
}

// ValidateSession 会话是否存活（中间件用；顺带节流刷新最近活跃）
func (uc *Usecase) ValidateSession(ctx context.Context, sid string) bool {
	return uc.sessions.Validate(ctx, sid)
}

// Authorize RBAC 接口鉴权
func (uc *Usecase) Authorize(ctx context.Context, userID uint, method, path string) bool {
	ps, err := uc.perms.FindByUserID(ctx, userID)
	if err != nil {
		return false
	}
	for _, p := range ps {
		if p.Match(method, path) {
			return true
		}
	}
	return false
}
