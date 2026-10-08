package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
)

var (
	// ErrInvalidToken 平台判定 token 无效/过期（本系统应回 401，前端走平台刷新流程）
	ErrInvalidToken = errors.New("platform token invalid")
	// ErrUnavailable 平台不可达（fail-closed，本系统应回 503）
	ErrUnavailable = errors.New("platform unavailable")
)

// Subject 平台身份主体（来自平台 GET /auth/profile 的 user 部分）
type Subject struct {
	UserID    uint          `json:"id"`
	Username  string        `json:"username"`
	Nickname  string        `json:"nickname"`
	Email     string        `json:"email"`
	Status    int           `json:"status"`
	Merchants []MerchantRef `json:"merchants"` // 本人已准入商户（含商户管理员标记；旧版平台无此字段为空）
}

// MerchantRef 平台 profile 下发的已准入商户引用
type MerchantRef struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	IsAdmin  bool   `json:"is_admin"`
	Admitted bool   `json:"admitted"`
}

// IdentityClient 平台身份客户端：管理端登录代理由本服务转发平台公开 API
// （POST /auth/login 等，无 HMAC；服务端因此可落登录日志），另用「用户本人的
// 平台 token」做自省与自身数据代理。
type IdentityClient struct {
	baseURL string
	hc      *http.Client
}

// NewIdentityClient 构造（wire provider）
func NewIdentityClient(cfg *conf.Bootstrap) *IdentityClient {
	timeout := time.Duration(cfg.Platform.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &IdentityClient{
		baseURL: cfg.Platform.BaseURL,
		hc:      &http.Client{Timeout: timeout},
	}
}

// profileVO 平台 ProfileVO（只取所需字段）
type profileVO struct {
	User *Subject `json:"user"`
}

// Profile 身份自省：200 返回主体；401 映射 ErrInvalidToken；网络错误映射 ErrUnavailable
func (c *IdentityClient) Profile(ctx context.Context, token string) (*Subject, error) {
	var out profileVO
	err := do(ctx, c.hc, http.MethodGet, c.baseURL+"/api/v1/auth/profile", token, nil, &out)
	if err == nil {
		if out.User == nil || out.User.UserID == 0 {
			return nil, ErrInvalidToken
		}
		return out.User, nil
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

// UpdateProfile 代理平台「本人更新昵称/邮箱」（用户本人 token）
func (c *IdentityClient) UpdateProfile(ctx context.Context, token, nickname, email string) error {
	body := map[string]string{"nickname": nickname, "email": email}
	return do(ctx, c.hc, http.MethodPut, c.baseURL+"/api/v1/auth/profile", token, body, nil)
}

// ChangePassword 代理平台「本人修改密码」（校验旧密码；平台侧成功后吊销其他端会话）
func (c *IdentityClient) ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	body := map[string]string{"old_password": oldPassword, "new_password": newPassword}
	return do(ctx, c.hc, http.MethodPut, c.baseURL+"/api/v1/auth/password", token, body, nil)
}

// TokenPair 平台令牌对（登录/刷新响应 data；expires_at 为 RFC3339）
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// CaptchaInfo 平台登录验证码开关与图文（captcha_image 为 PNG base64，无 data: 前缀）
type CaptchaInfo struct {
	Enabled      bool   `json:"enabled"`
	CaptchaID    string `json:"captcha_id"`
	CaptchaImage string `json:"captcha_image"`
}

// Login 代理平台登录（公开 API，无 HMAC）：平台业务错误（401 密码错等）
// 以 *Error 原样透传 msg（落登录日志与前端回显用）；网络错误映射 ErrUnavailable
func (c *IdentityClient) Login(ctx context.Context, username, password, captchaID, captchaCode string) (*TokenPair, error) {
	body := map[string]string{
		"username": username, "password": password,
		"captcha_id": captchaID, "captcha_code": captchaCode,
		"device_type": "web",
	}
	var out TokenPair
	if err := do(ctx, c.hc, http.MethodPost, c.baseURL+"/api/v1/auth/login", "", body, &out); err != nil {
		return nil, mapPublicErr(err)
	}
	return &out, nil
}

// Refresh 代理平台刷新令牌（refresh_token 放 body；跨域 cookie 不可用）
func (c *IdentityClient) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	body := map[string]string{"refresh_token": refreshToken}
	var out TokenPair
	if err := do(ctx, c.hc, http.MethodPost, c.baseURL+"/api/v1/auth/refresh", "", body, &out); err != nil {
		return nil, mapPublicErr(err)
	}
	return &out, nil
}

// Logout 代理平台登出（带用户本人 token，平台侧吊销会话）
func (c *IdentityClient) Logout(ctx context.Context, token string) error {
	return do(ctx, c.hc, http.MethodPost, c.baseURL+"/api/v1/auth/logout", token, nil, nil)
}

// Captcha 取平台登录验证码（开关关时 captcha_id/image 为空）
func (c *IdentityClient) Captcha(ctx context.Context) (*CaptchaInfo, error) {
	var out CaptchaInfo
	if err := do(ctx, c.hc, http.MethodGet, c.baseURL+"/api/v1/auth/captcha", "", nil, &out); err != nil {
		return nil, mapPublicErr(err)
	}
	return &out, nil
}

// mapPublicErr 公开 API 错误归一：平台信封错误（*Error，含 HTTP 状态与 msg）原样透传，
// 网络/编解码等本地错误一律包装为 ErrUnavailable（fail-closed，上层回 503）
func mapPublicErr(err error) error {
	var perr *Error
	if errors.As(err, &perr) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
