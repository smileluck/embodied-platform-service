// 租户门户身份适配器：把平台 TenantIdentityClient 适配为 biz 的 TenantIdentitySource
// （主体类型转换 + 哨兵错误映射：平台 401 → ErrInvalidToken，网络故障 → ErrPlatformUnavailable）。
package auth

import (
	"context"
	"errors"
	"net/http"

	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
)

// TenantIdentityAdapter wire 绑定 bizauth.TenantIdentitySource
type TenantIdentityAdapter struct {
	c *platform.TenantIdentityClient
}

// NewTenantIdentityAdapter 构造（wire provider）
func NewTenantIdentityAdapter(c *platform.TenantIdentityClient) *TenantIdentityAdapter {
	return &TenantIdentityAdapter{c: c}
}

func (a *TenantIdentityAdapter) TenantProfile(ctx context.Context, token string) (*bizauth.TenantSubject, error) {
	p, err := a.c.TenantProfileByToken(ctx, token)
	if err != nil {
		var perr *platform.Error
		switch {
		case errors.Is(err, platform.ErrInvalidToken):
			return nil, bizauth.ErrInvalidToken
		case errors.Is(err, platform.ErrUnavailable):
			return nil, bizauth.ErrPlatformUnavailable
		case errors.As(err, &perr) && perr.HTTPStatus == http.StatusUnauthorized:
			return nil, bizauth.ErrInvalidToken
		}
		return nil, err
	}
	// PermCodes 由中间件装载时本地解析填充（RBAC 本地化），此处只组装身份字段
	return &bizauth.TenantSubject{
		UserID: p.User.ID, Username: p.User.Username, Nickname: p.User.Nickname,
		TenantID: p.User.TenantID,
	}, nil
}

// TenantChangePassword 本人改密代理（旧密码错误等平台 4xx 以 *platform.Error 透传
// 给 handler 按状态码回显）
func (a *TenantIdentityAdapter) TenantChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	err := a.c.TenantChangePassword(ctx, token, oldPassword, newPassword)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, platform.ErrInvalidToken):
		return bizauth.ErrInvalidToken
	case errors.Is(err, platform.ErrUnavailable):
		return bizauth.ErrPlatformUnavailable
	}
	var perr *platform.Error
	if errors.As(err, &perr) && perr.HTTPStatus == http.StatusUnauthorized {
		return bizauth.ErrInvalidToken
	}
	return err
}

// TenantLogin 代理平台门户登录：平台业务错误（401 密码错等）以 *platform.Error
// 原样透传（msg 供前端回显），不映射为本地哨兵；网络故障 → ErrPlatformUnavailable
func (a *TenantIdentityAdapter) TenantLogin(ctx context.Context, username, password string) (*bizauth.TokenPair, error) {
	pair, err := a.c.TenantLogin(ctx, username, password)
	if err != nil {
		return nil, mapIdentityErr(err)
	}
	return &bizauth.TokenPair{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt}, nil
}

// TenantRefresh 代理平台门户刷新令牌（错误映射同 TenantLogin）
func (a *TenantIdentityAdapter) TenantRefresh(ctx context.Context, refreshToken string) (*bizauth.TokenPair, error) {
	pair, err := a.c.TenantRefresh(ctx, refreshToken)
	if err != nil {
		return nil, mapIdentityErr(err)
	}
	return &bizauth.TokenPair{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt}, nil
}
