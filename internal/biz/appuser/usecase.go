package appuser

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 应用用户领域用例
type Usecase struct {
	repo   Repo
	tokens TokenIssuer
}

func NewUsecase(repo Repo, tokens TokenIssuer) *Usecase {
	return &Usecase{repo: repo, tokens: tokens}
}

// Create 创建应用用户：bcrypt 落哈希，同步绑定租户关联。
// username 唯一性由数据库唯一索引兜底（冲突在仓储映射为 ErrDuplicateUsername）。
func (uc *Usecase) Create(ctx context.Context, username, password, nickname, phone, email string, tenantIDs []uint) (*AppUser, error) {
	u := &AppUser{
		Username: username, Nickname: nickname, Phone: phone, Email: email,
		Status: StatusEnabled, TenantIDs: tenantIDs,
	}
	if err := u.SetPassword(password); err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Update 更新应用用户基础资料与租户关联（username 创建后不可改，不触碰密码）
func (uc *Usecase) Update(ctx context.Context, id uint, nickname, phone, email string, status *Status, tenantIDs []uint) error {
	u, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	u.Nickname = nickname
	u.Phone = phone
	u.Email = email
	if status != nil {
		u.Status = *status
	}
	u.TenantIDs = tenantIDs
	return uc.repo.Update(ctx, u)
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*AppUser, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*AppUser, pagination.Page, error) {
	users, total, err := uc.repo.List(ctx, q, page, pageSize)
	return users, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// ResetPassword 管理员重置密码：bcrypt 哈希落库（旧密码立即失效）
func (uc *Usecase) ResetPassword(ctx context.Context, id uint, newPlain string) error {
	u, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := u.SetPassword(newPlain); err != nil {
		return err
	}
	return uc.repo.UpdatePassword(ctx, id, u.PasswordHash)
}

// SetStatus 启用/禁用应用用户（禁用后 token 校验即时失败）
func (uc *Usecase) SetStatus(ctx context.Context, id uint, status Status) error {
	u, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	u.Status = status
	return uc.repo.Update(ctx, u)
}

// Login 应用用户登录：用户不存在与密码错误统一返回 ErrBadCredentials（防用户名枚举），
// 禁用返回 ErrAppUserDisabled；成功后签发应用令牌对（typ 与后台账号体系隔离）。
func (uc *Usecase) Login(ctx context.Context, username, password string) (*AppUser, *TokenPair, error) {
	u, err := uc.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, nil, ErrBadCredentials
	}
	if !u.CheckPassword(password) {
		return nil, nil, ErrBadCredentials
	}
	if !u.Enabled() {
		return nil, nil, ErrAppUserDisabled
	}
	tp, err := uc.IssueTokens(u)
	if err != nil {
		return nil, nil, err
	}
	return u, tp, nil
}

// IssueTokens 为应用用户签发访问/刷新令牌对
func (uc *Usecase) IssueTokens(u *AppUser) (*TokenPair, error) {
	access, expiresAt, err := uc.tokens.IssueAppAccessToken(u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	refresh, err := uc.tokens.IssueAppRefreshToken(u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: expiresAt}, nil
}

// Refresh 刷新令牌：校验 refresh token（typ 隔离）→ 用户必须存在且启用 → 重新签发
func (uc *Usecase) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	uid, username, err := uc.tokens.ParseAppRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	u, err := uc.repo.Get(ctx, uid)
	if err != nil || u.Username != username || !u.Enabled() {
		return nil, ErrBadCredentials
	}
	return uc.IssueTokens(u)
}

// Profile 当前应用用户信息（含租户关联；中间件已校验启用状态）
func (uc *Usecase) Profile(ctx context.Context, id uint) (*AppUser, error) {
	return uc.repo.Get(ctx, id)
}

// ChangePassword 本人修改密码：先按用户名取哈希校验旧密码，再落库新哈希
// （username 创建后不可改，与 token 主体一致；旧密码错误返回 ErrBadCredentials）
func (uc *Usecase) ChangePassword(ctx context.Context, username, oldPlain, newPlain string) error {
	u, err := uc.repo.GetByUsername(ctx, username)
	if err != nil {
		return err
	}
	if !u.CheckPassword(oldPlain) {
		return ErrBadCredentials
	}
	if err := u.SetPassword(newPlain); err != nil {
		return err
	}
	return uc.repo.UpdatePassword(ctx, u.ID, u.PasswordHash)
}
