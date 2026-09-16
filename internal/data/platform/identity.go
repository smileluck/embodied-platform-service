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
	UserID   uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Status   int    `json:"status"`
}

// IdentityClient 平台身份客户端：用「用户本人的平台 token」做自省与自身数据代理。
// 登录发生在本系统前端（直调平台 /auth/login，token 双用），本系统后端不碰密码。
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
