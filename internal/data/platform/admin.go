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

// ErrNotConfigured 平台接入未配置（baseUrl / 服务账号缺失）
var ErrNotConfigured = errors.New("platform integration not configured")

// AdminClient 平台管理面客户端（/api/v1，服务账号 JWT）。
// 用于租户同步、设备型号管理、物模型只读选择器、平台用户列表（准入同步）、商户 ID 解析。
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

// Ping 连通性自检（登录 + 拉一页租户）
func (c *AdminClient) Ping(ctx context.Context) error {
	var out []json.RawMessage
	_, err := c.list(ctx, "/api/v1/tenants", url.Values{"page": {"1"}, "page_size": {"1"}}, &out)
	return err
}

func (c *AdminClient) list(ctx context.Context, path string, query url.Values, listOut any) (*Page, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	token, err := c.ensureToken(ctx)
	if err != nil {
		return nil, err
	}
	return doList(ctx, c.hc, endpoint, token, listOut)
}

// ---- 租户（同步；平台是设备侧租户的唯一真源，本系统租户需与平台保持一致） ----

// TenantCreate 平台创建租户入参（MerchantID=本服务商户，绑入商户租户集后开放面设备注册才可用）
type TenantCreate struct {
	Name         string `json:"name"`
	Code         string `json:"code"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Remark       string `json:"remark"`
	MerchantID   uint   `json:"merchant_id"`
}

// TenantUpdate 平台更新租户入参（code 创建后不可改）
type TenantUpdate struct {
	Name         string `json:"name"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Remark       string `json:"remark"`
	MerchantID   uint   `json:"merchant_id"`
}

// Tenant 平台租户视图
type Tenant struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Remark       string `json:"remark"`
	MerchantID   uint   `json:"merchant_id"`
	Status       int    `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (c *AdminClient) CreateTenant(ctx context.Context, req TenantCreate) (*Tenant, error) {
	var out Tenant
	if err := c.do(ctx, http.MethodPost, "/api/v1/tenants", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AdminClient) UpdateTenant(ctx context.Context, id uint, req TenantUpdate) error {
	return c.do(ctx, http.MethodPut, "/api/v1/tenants/"+strconv.FormatUint(uint64(id), 10), nil, req, nil)
}

func (c *AdminClient) DeleteTenant(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/tenants/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil)
}

func (c *AdminClient) SetTenantStatus(ctx context.Context, id uint, status int) error {
	body := map[string]*int{"status": &status}
	return c.do(ctx, http.MethodPut, "/api/v1/tenants/"+strconv.FormatUint(uint64(id), 10)+"/status", nil, body, nil)
}

// FindTenantByCode 按 code 精确查找平台租户（存量补链用；未找到返回 nil）
func (c *AdminClient) FindTenantByCode(ctx context.Context, code string) (*Tenant, error) {
	var list []*Tenant
	q := url.Values{"page": {"1"}, "page_size": {"50"}, "code": {code}}
	// 平台列表 code 过滤为精确匹配；分页遍历前几页足够覆盖存量补链场景
	for page := 1; page <= 20; page++ {
		q.Set("page", strconv.Itoa(page))
		pg, err := c.list(ctx, "/api/v1/tenants", q, &list)
		if err != nil {
			return nil, err
		}
		for _, t := range list {
			if t.Code == code {
				return t, nil
			}
		}
		if pg == nil || len(list) == 0 || pg.Page*pg.PageSize >= int(pg.Total) {
			break
		}
	}
	return nil, nil
}

// ---- 设备型号（管理面 CRUD；创建须绑物模型 model 层节点的已发布版本） ----

// DeviceModel 平台设备型号视图（与平台 biz/devmodel.Model json 对齐）
type DeviceModel struct {
	ID           uint   `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	TMNodeID     uint   `json:"tm_node_id"`
	TMVersionID  uint   `json:"tm_version_id"`
	TMNodeName   string `json:"tm_node_name,omitempty"`
	TMVersionNo  int    `json:"tm_version_no,omitempty"`
	Status       int    `json:"status"`
	Manufacturer string `json:"manufacturer"`
	Description  string `json:"description"`
	Transport    string `json:"transport"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ModelCreate 平台创建型号入参
type ModelCreate struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	TMNodeID     uint   `json:"tm_node_id"`
	TMVersionID  uint   `json:"tm_version_id"`
	Manufacturer string `json:"manufacturer"`
	Description  string `json:"description"`
	Transport    string `json:"transport,omitempty"`
}

// ModelUpdate 平台更新型号入参
type ModelUpdate struct {
	Name         string `json:"name"`
	TMVersionID  uint   `json:"tm_version_id"`
	Status       int    `json:"status"`
	Manufacturer string `json:"manufacturer"`
	Description  string `json:"description"`
	Transport    string `json:"transport,omitempty"`
}

// ModelQuery 型号列表过滤
type ModelQuery struct {
	Keyword string
	Status  *int
}

func (c *AdminClient) ListDeviceModels(ctx context.Context, q ModelQuery, page, pageSize int) ([]*DeviceModel, *Page, error) {
	vals := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if q.Keyword != "" {
		vals.Set("kw", q.Keyword)
	}
	if q.Status != nil {
		vals.Set("status", strconv.Itoa(*q.Status))
	}
	var list []*DeviceModel
	pg, err := c.list(ctx, "/api/v1/device-models", vals, &list)
	return list, pg, err
}

func (c *AdminClient) CreateDeviceModel(ctx context.Context, req ModelCreate) (*DeviceModel, error) {
	var out DeviceModel
	if err := c.do(ctx, http.MethodPost, "/api/v1/device-models", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AdminClient) GetDeviceModel(ctx context.Context, id uint) (*DeviceModel, error) {
	var out DeviceModel
	if err := c.do(ctx, http.MethodGet, "/api/v1/device-models/"+strconv.FormatUint(uint64(id), 10), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AdminClient) UpdateDeviceModel(ctx context.Context, id uint, req ModelUpdate) error {
	return c.do(ctx, http.MethodPut, "/api/v1/device-models/"+strconv.FormatUint(uint64(id), 10), nil, req, nil)
}

func (c *AdminClient) DeleteDeviceModel(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/device-models/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil)
}

// ---- 物模型（只读，型号创建表单的选择器数据源） ----

// ThingModelNode 物模型节点
type ThingModelNode struct {
	ID       uint   `json:"id"`
	Layer    string `json:"layer"` // base/category/model/instance
	ParentID uint   `json:"parent_id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Status   int    `json:"status"`
}

// ThingModelVersion 物模型版本（型号绑定须选 status=published）
type ThingModelVersion struct {
	ID          uint   `json:"id"`
	NodeID      uint   `json:"node_id"`
	Version     int    `json:"version"`
	Status      string `json:"status"` // draft / published
	PublishedAt string `json:"published_at"`
	CreatedAt   string `json:"created_at"`
}

// ListThingModelNodes 物模型节点列表（page_size=0 全量；layer 可选过滤）
func (c *AdminClient) ListThingModelNodes(ctx context.Context, layer, kw string) ([]*ThingModelNode, error) {
	vals := url.Values{"page_size": {"0"}}
	if layer != "" {
		vals.Set("layer", layer)
	}
	if kw != "" {
		vals.Set("kw", kw)
	}
	var list []*ThingModelNode
	if _, err := c.list(ctx, "/api/v1/thing-models", vals, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// ListThingModelVersions 节点全部版本（含草稿；调用方自行过滤 published）
func (c *AdminClient) ListThingModelVersions(ctx context.Context, nodeID uint) ([]*ThingModelVersion, error) {
	var list []*ThingModelVersion
	path := "/api/v1/thing-models/" + strconv.FormatUint(uint64(nodeID), 10) + "/versions"
	if _, err := c.list(ctx, path, nil, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// ---- 平台用户（准入投影同步的数据源） ----

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
	var list []*PlatformUser
	pg, err := c.list(ctx, "/api/v1/users", vals, &list)
	return list, pg, err
}

// ---- 商户 ID 解析（租户同步时绑定 merchant_id 用） ----

// FindMerchantIDByAppKey 按 AppKey 查平台商户 ID（本服务商户唯一）
func (c *AdminClient) FindMerchantIDByAppKey(ctx context.Context, appKey string) (uint, error) {
	var list []*struct {
		ID     uint   `json:"id"`
		AppKey string `json:"app_key"`
	}
	q := url.Values{"page": {"1"}, "page_size": {"20"}, "app_key": {appKey}}
	pg, err := c.list(ctx, "/api/v1/merchants", q, &list)
	if err != nil {
		return 0, err
	}
	for _, m := range list {
		if m.AppKey == appKey {
			return m.ID, nil
		}
	}
	_ = pg
	return 0, fmt.Errorf("platform merchant not found for app_key %q", appKey)
}
