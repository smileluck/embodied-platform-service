// Package auth 认证上下文数据层：平台身份源自省适配器。
package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/smilex/smilex-admin-gin/internal/biz/admission"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
)

// IdentityAdapter 把平台 IdentityClient 适配为 biz 的 IdentitySource
// （Subject 类型转换 + 哨兵错误映射：平台 401 → ErrInvalidToken，网络故障 → ErrPlatformUnavailable）
type IdentityAdapter struct {
	c *platform.IdentityClient
}

// NewIdentityAdapter 构造（wire provider，绑定 bizauth.IdentitySource）
func NewIdentityAdapter(c *platform.IdentityClient) *IdentityAdapter {
	return &IdentityAdapter{c: c}
}

func (a *IdentityAdapter) Profile(ctx context.Context, token string) (*bizauth.Subject, error) {
	s, err := a.c.Profile(ctx, token)
	if err != nil {
		var perr *platform.Error
		if errors.Is(err, platform.ErrInvalidToken) {
			return nil, bizauth.ErrInvalidToken
		}
		if errors.As(err, &perr) && perr.HTTPStatus == 401 {
			return nil, bizauth.ErrInvalidToken
		}
		if errors.Is(err, platform.ErrUnavailable) {
			return nil, bizauth.ErrPlatformUnavailable
		}
		return nil, err
	}
	out := &bizauth.Subject{
		UserID: s.UserID, Username: s.Username, Nickname: s.Nickname, Email: s.Email,
	}
	// 已准入商户（含商户管理员标记）：穿透 pid: 缓存随 Subject 序列化
	if len(s.Merchants) > 0 {
		out.Merchants = make([]admission.MerchantRef, 0, len(s.Merchants))
		for _, m := range s.Merchants {
			out.Merchants = append(out.Merchants, admission.MerchantRef{ID: m.ID, Code: m.Code, IsAdmin: m.IsAdmin, Admitted: m.Admitted})
		}
	}
	return out, nil
}

func (a *IdentityAdapter) UpdateProfile(ctx context.Context, token, nickname, email string) error {
	return a.c.UpdateProfile(ctx, token, nickname, email)
}

func (a *IdentityAdapter) ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	return a.c.ChangePassword(ctx, token, oldPassword, newPassword)
}

// Login 代理平台登录：平台业务错误（401 密码错等）以 *platform.Error 原样透传
// （msg 供登录日志与前端回显），不映射为本地哨兵；网络故障 → ErrPlatformUnavailable
func (a *IdentityAdapter) Login(ctx context.Context, username, password, captchaID, captchaCode string) (*bizauth.TokenPair, error) {
	pair, err := a.c.Login(ctx, username, password, captchaID, captchaCode)
	if err != nil {
		return nil, mapIdentityErr(err)
	}
	return &bizauth.TokenPair{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt}, nil
}

// Refresh 代理平台刷新令牌（错误映射同 Login）
func (a *IdentityAdapter) Refresh(ctx context.Context, refreshToken string) (*bizauth.TokenPair, error) {
	pair, err := a.c.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, mapIdentityErr(err)
	}
	return &bizauth.TokenPair{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt}, nil
}

// Logout 代理平台登出（错误映射同 Login）
func (a *IdentityAdapter) Logout(ctx context.Context, token string) error {
	return mapIdentityErr(a.c.Logout(ctx, token))
}

// Captcha 取平台登录验证码（错误映射同 Login）
func (a *IdentityAdapter) Captcha(ctx context.Context) (*bizauth.CaptchaInfo, error) {
	info, err := a.c.Captcha(ctx)
	if err != nil {
		return nil, mapIdentityErr(err)
	}
	return &bizauth.CaptchaInfo{Enabled: info.Enabled, CaptchaID: info.CaptchaID, CaptchaImage: info.CaptchaImage}, nil
}

// mapIdentityErr 登录链路的错误归一：ErrUnavailable → ErrPlatformUnavailable；
// 平台信封错误（*platform.Error）原样透传（保留 HTTP 状态与 msg）
func mapIdentityErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platform.ErrUnavailable) {
		return bizauth.ErrPlatformUnavailable
	}
	return err
}

// AppIdentityAdapter 把平台 AppIdentityClient 适配为 biz 的 AppIdentitySource
// （应用用户 C 端自省；哨兵错误映射与 IdentityAdapter 同款）
type AppIdentityAdapter struct {
	c *platform.AppIdentityClient
}

// NewAppIdentityAdapter 构造（wire provider，绑定 bizauth.AppIdentitySource）
func NewAppIdentityAdapter(c *platform.AppIdentityClient) *AppIdentityAdapter {
	return &AppIdentityAdapter{c: c}
}

func (a *AppIdentityAdapter) AppProfile(ctx context.Context, token string) (*bizauth.AppSubject, error) {
	s, err := a.c.AppProfile(ctx, token)
	if err != nil {
		if errors.Is(err, platform.ErrInvalidToken) {
			return nil, bizauth.ErrInvalidToken
		}
		if errors.Is(err, platform.ErrUnavailable) {
			return nil, bizauth.ErrPlatformUnavailable
		}
		var perr *platform.Error
		if errors.As(err, &perr) && perr.HTTPStatus == http.StatusUnauthorized {
			return nil, bizauth.ErrInvalidToken
		}
		return nil, err
	}
	return &bizauth.AppSubject{
		UserID: s.ID, Username: s.Username, Nickname: s.Nickname,
		Status: s.Status, TenantIDs: s.TenantIDs,
	}, nil
}

// AppChangePassword 本人改密代理（哨兵映射与 AppProfile 同款；旧密码错误等
// 平台 4xx 以 *platform.Error 透传给 handler 按状态码回显）
func (a *AppIdentityAdapter) AppChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	err := a.c.ChangePassword(ctx, token, oldPassword, newPassword)
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
