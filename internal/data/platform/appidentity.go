package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// AppIdentityClient 平台应用用户（C 端）身份客户端：用「App 本人的 app-access token」
// 调平台 GET /api/v1/app-auth/profile 做自省。App 登录发生在 App 直连平台
// （token 双用于平台与本系统），本系统不碰 C 端账密、不签发、不换签。
type AppIdentityClient struct {
	baseURL string
	hc      *http.Client
}

// NewAppIdentityClient 构造（wire provider；与主服务同源，复用 platform.baseUrl）
func NewAppIdentityClient(cfg *conf.Bootstrap) *AppIdentityClient {
	timeout := time.Duration(cfg.Platform.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &AppIdentityClient{
		baseURL: cfg.Platform.BaseURL,
		hc:      &http.Client{Timeout: timeout},
	}
}

// AppProfile 平台侧应用用户视图（只取闸门所需字段；平台 AppJWT 按请求查库校验
// 启用状态，禁用即时 401——本侧无需重复判定，仅在缓存窗口内感知延迟）
type AppProfile struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Status    int    `json:"status"`
	TenantIDs []uint `json:"tenant_ids"`
}

// AppProfile 自省：200 返回主体；401 映射 ErrInvalidToken；网络错误映射 ErrUnavailable
func (c *AppIdentityClient) AppProfile(ctx context.Context, token string) (*AppProfile, error) {
	var out AppProfile
	err := do(ctx, c.hc, http.MethodGet, c.baseURL+"/api/v1/app-auth/profile", token, nil, &out)
	if err == nil {
		if out.ID == 0 {
			return nil, ErrInvalidToken
		}
		if out.TenantIDs == nil {
			out.TenantIDs = []uint{}
		}
		return &out, nil
	}
	var perr *Error
	if errors.As(err, &perr) {
		if perr.HTTPStatus == http.StatusUnauthorized {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// ChangePassword 代理平台「应用用户本人修改密码」（校验旧密码；
// 平台侧成功后吊销其他端会话）
func (c *AppIdentityClient) ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	body := map[string]string{"old_password": oldPassword, "new_password": newPassword}
	return do(ctx, c.hc, http.MethodPut, c.baseURL+"/api/v1/app-auth/password", token, body, nil)
}
