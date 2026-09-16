package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// ErrNotConfigured 平台管理面服务账号未配置（platform.admin 缺失）
var ErrNotConfigured = errors.New("platform admin account not configured")

// AdminClient 平台管理面客户端（/api/v1，管理账号 JWT）。
//
// 2026-09-16 起，租户同步与型号管理已切换到开放面商户 HMAC（见 data/tenant/syncer.go、
// data/devmodel/repo.go），本客户端只保留一个用途：**从平台同步用户列表**（准入投影的
// 便利功能——管理账号列表是跨项目全局数据，按治理决策不上开放面，见平台决策记录
// 2026-09-16-openapi-tenant-model.md 第 4 条）。未配置时该功能 503，核心流程
// （登录/准入/租户/设备/型号/文件）不受影响。
//
// token 内存缓存，过期前自动重登；请求 401 时重登一次重试。
type AdminClient struct {
	baseURL  string
	username string
	password string
	hc       *http.Client

	mu       sync.Mutex
	token    string
	expireAt time.Time
}

// NewAdminClient 构造管理面客户端（wire provider）
func NewAdminClient(cfg *conf.Bootstrap) *AdminClient {
	timeout := time.Duration(cfg.Platform.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &AdminClient{
		baseURL:  cfg.Platform.BaseURL,
		username: cfg.Platform.Admin.Username,
		password: cfg.Platform.Admin.Password,
		hc:       &http.Client{Timeout: timeout},
	}
}

func (c *AdminClient) configured() bool {
	return c.baseURL != "" && c.username != "" && c.password != ""
}

// ensureToken 返回可用的服务账号 access token（过期前 60s 提前重登）
func (c *AdminClient) ensureToken(ctx context.Context) (string, error) {
	if !c.configured() {
		return "", ErrNotConfigured
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expireAt.Add(-60*time.Second)) {
		return c.token, nil
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   string `json:"expires_at"`
	}
	body := map[string]string{"username": c.username, "password": c.password}
	endpoint := c.baseURL + "/api/v1/auth/login"
	var raw []byte
	if err := doRaw(ctx, c.hc, http.MethodPost, endpoint, "", body, &raw); err != nil {
		return "", fmt.Errorf("platform admin login failed: %w", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("platform admin login: unmarshal: %w", err)
	}
	exp := time.Now().Add(2 * time.Hour) // access token 平台侧 2h；expires_at 解析失败时保守取 2h
	if t, err := time.Parse(time.RFC3339, out.ExpiresAt); err == nil {
		exp = t
	}
	c.token, c.expireAt = out.AccessToken, exp
	return c.token, nil
}

// do 带服务账号鉴权的请求；401（会话被吊销/过期）时重登一次重试
func (c *AdminClient) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	token, err := c.ensureToken(ctx)
	if err != nil {
		return err
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	err = do(ctx, c.hc, method, endpoint, token, body, out)
	var perr *Error
	if errors.As(err, &perr) && perr.HTTPStatus == http.StatusUnauthorized {
		c.mu.Lock()
		c.token = ""
		c.mu.Unlock()
		if token, err = c.ensureToken(ctx); err != nil {
			return err
		}
		err = do(ctx, c.hc, method, endpoint, token, body, out)
	}
	return err
}

// Ping 连通性自检（登录 + 拉一页平台用户）
func (c *AdminClient) Ping(ctx context.Context) error {
	var out []json.RawMessage
	q := url.Values{"page": {"1"}, "page_size": {"1"}}
	endpoint := c.baseURL + "/api/v1/users?" + q.Encode()
	token, err := c.ensureToken(ctx)
	if err != nil {
		return err
	}
	_, err = doList(ctx, c.hc, endpoint, token, &out)
	return err
}

// ---- 平台用户（准入投影同步的唯一数据源） ----

// PlatformUser 平台管理账号视图
type PlatformUser struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ListPlatformUsers 平台用户列表（username 前缀过滤）
func (c *AdminClient) ListPlatformUsers(ctx context.Context, username string, page, pageSize int) ([]*PlatformUser, *Page, error) {
	vals := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if username != "" {
		vals.Set("username", username)
	}
	endpoint := c.baseURL + "/api/v1/users"
	if len(vals) > 0 {
		endpoint += "?" + vals.Encode()
	}
	token, err := c.ensureToken(ctx)
	if err != nil {
		return nil, nil, err
	}
	var list []*PlatformUser
	pg, err := doList(ctx, c.hc, endpoint, token, &list)
	return list, pg, err
}
