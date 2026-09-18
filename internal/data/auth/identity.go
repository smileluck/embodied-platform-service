// Package auth 认证上下文数据层：平台身份源自省适配器。
package auth

import (
	"context"
	"errors"

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
