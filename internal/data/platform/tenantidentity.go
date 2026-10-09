package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// TenantIdentityClient 平台租户门户身份客户端（第四套身份 tenant_users）：
// 门户登录/刷新经本服务后端代理平台公开 API（POST /tenant-api/v1/auth/*，无 HMAC；
// 服务端可挂 LoginIPGuard/限流），另用「用户本人的 tenant-access token」调
// GET /tenant-api/v1/auth/profile 做自省。本系统不碰租户端账密、不签发、不换签。
type TenantIdentityClient struct {
	baseURL string
	hc      *http.Client
}

// NewTenantIdentityClient 构造（wire provider；与主服务同源，复用 platform.baseUrl）
func NewTenantIdentityClient(cfg *conf.Bootstrap) *TenantIdentityClient {
	timeout := time.Duration(cfg.Platform.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &TenantIdentityClient{
		baseURL: cfg.Platform.BaseURL,
		hc:      &http.Client{Timeout: timeout},
	}
}

// TenantProfile 平台侧门户 profile 视图（只取身份自省所需字段；平台 TenantAuth 按请求
// 查库校验用户/租户双启用，禁用即时 401——本侧仅缓存窗口内感知延迟。租户 RBAC 自
// 2026-10-09 本地化：平台不再下发 perm_codes，权限由 TenantAuth 装载时本地三表解析）
type TenantProfile struct {
	User *struct {
		ID       uint   `json:"id"`
		TenantID uint   `json:"tenant_id"`
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Status   int    `json:"status"`
	} `json:"user"`
	Tenant *struct {
		ID      uint   `json:"id"`
		Name    string `json:"name"`
		Code    string `json:"code"`
		Enabled bool   `json:"enabled"`
	} `json:"tenant"`
}

// TenantLogin 代理平台门户登录（公开 API）：平台业务错误以 *Error 透传 msg；
// 网络错误映射 ErrUnavailable
func (c *TenantIdentityClient) TenantLogin(ctx context.Context, username, password string) (*TokenPair, error) {
	body := map[string]string{"username": username, "password": password}
	var out TokenPair
	if err := do(ctx, c.hc, http.MethodPost, c.baseURL+"/tenant-api/v1/auth/login", "", body, &out); err != nil {
		return nil, mapPublicErr(err)
	}
	return &out, nil
}

// TenantRefresh 代理平台门户刷新令牌（typ 隔离：只接受 tenant-refresh）
func (c *TenantIdentityClient) TenantRefresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	body := map[string]string{"refresh_token": refreshToken}
	var out TokenPair
	if err := do(ctx, c.hc, http.MethodPost, c.baseURL+"/tenant-api/v1/auth/refresh", "", body, &out); err != nil {
		return nil, mapPublicErr(err)
	}
	return &out, nil
}

// TenantProfileByToken 自省：200 返回视图；401 映射 ErrInvalidToken；网络错误映射 ErrUnavailable
func (c *TenantIdentityClient) TenantProfileByToken(ctx context.Context, token string) (*TenantProfile, error) {
	var out TenantProfile
	err := do(ctx, c.hc, http.MethodGet, c.baseURL+"/tenant-api/v1/profile", token, nil, &out)
	if err == nil {
		if out.User == nil || out.User.ID == 0 || out.User.TenantID == 0 {
			return nil, ErrInvalidToken
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

// TenantChangePassword 代理平台「租户门户用户本人修改密码」（校验旧密码）
func (c *TenantIdentityClient) TenantChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	body := map[string]string{"old_password": oldPassword, "new_password": newPassword}
	return do(ctx, c.hc, http.MethodPut, c.baseURL+"/tenant-api/v1/password", token, body, nil)
}
