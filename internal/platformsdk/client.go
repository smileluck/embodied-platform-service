package platformsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client 开放 API 客户端。BaseURL 形如 https://platform.example.com（直连）
// 或 https://gw.example.com/gw/open-api/v1 注意：经 /gw 访问时 BaseURL 含网关前缀，
// 签名按实际发送的完整 path 计算，Client 已内聚处理）。
type Client struct {
	BaseURL string // 形如 https://host[:port] 或 https://host/gw/open-api（见 NewClient 说明）
	Signer  *Signer
	HTTP    *http.Client
}

// NewClient 构造客户端。
//
// baseURL 直连填 "http://host:27080"（内部拼 /open-api/v1 前缀）；
// 经 /gw 网关填 "http://gw:27081/gw/open-api"（路径签名口径与平台契约一致：
// 直连签 /open-api/v1/...，经网关签 /gw/open-api/v1/...）。
func NewClient(baseURL, appKey, appSecret string) *Client {
	return &Client{
		BaseURL: baseURL,
		Signer:  NewSigner(appKey, appSecret),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// pathPrefix 完整请求 path 前缀（即签名 path 口径）：直连 /open-api/v1，经 /gw 网关 /gw/open-api/v1
func (c *Client) pathPrefix() string {
	if strings.HasSuffix(c.BaseURL, "/gw/open-api") {
		return "/gw/open-api/v1"
	}
	return "/open-api/v1"
}

// endpointBase 取 BaseURL 的 scheme://host（其 path 部分仅用于识别 /gw 形态，不参与拼接，
// 避免「BaseURL 带 path + 显式钉死签名 path」的双重叠加）
func (c *Client) endpointBase() string {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return c.BaseURL
	}
	return u.Scheme + "://" + u.Host
}

// do 发送签名请求并解包信封；out 非 nil 时把 data 反序列化进去。
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any, statusOut *int) error {
	fullPath := c.pathPrefix() + path
	endpoint := c.endpointBase() + fullPath

	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("sdk: marshal body: %w", err)
		}
		raw = b
	}

	u := endpoint
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("sdk: build request: %w", err)
	}
	req.URL.Path = fullPath // 签名按完整 path（u 解析可能因 BaseURL 带路径而重组，这里显式钉死）
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.Signer.Sign(req, raw); err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("sdk: do request: %w", err)
	}
	defer resp.Body.Close()
	if statusOut != nil {
		*statusOut = resp.StatusCode
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("sdk: read response: %w", err)
	}
	var env envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return &Error{HTTPStatus: resp.StatusCode, Code: -1, Msg: string(respBody)}
	}
	if env.Code != 0 {
		return &Error{HTTPStatus: resp.StatusCode, Code: env.Code, Msg: env.Msg}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("sdk: unmarshal data: %w", err)
		}
	}
	return nil
}

// doList 列表请求（解 list + page）
func (c *Client) doList(ctx context.Context, path string, query url.Values, listOut any) (*Page, error) {
	fullPath := c.pathPrefix() + path
	endpoint := c.endpointBase() + fullPath
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("sdk: build request: %w", err)
	}
	req.URL.Path = fullPath
	if err := c.Signer.Sign(req, nil); err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sdk: do request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("sdk: read response: %w", err)
	}
	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			List json.RawMessage `json:"list"`
			Page Page            `json:"page"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, &Error{HTTPStatus: resp.StatusCode, Code: -1, Msg: string(respBody)}
	}
	if env.Code != 0 {
		return nil, &Error{HTTPStatus: resp.StatusCode, Code: env.Code, Msg: env.Msg}
	}
	if len(env.Data.List) > 0 {
		if err := json.Unmarshal(env.Data.List, listOut); err != nil {
			return nil, fmt.Errorf("sdk: unmarshal list: %w", err)
		}
	}
	return &env.Data.Page, nil
}

// ---- 业务方法 ----

// Ping 凭证自检（返回商户脱敏信息）
func (c *Client) Ping(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/ping", nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return out, nil
}

// ListDevices 设备列表（页从 1 起；过滤项与服务端 scope 收敛叠加）
func (c *Client) ListDevices(ctx context.Context, page, pageSize int, filter DeviceFilter) ([]*Device, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if filter.Keyword != "" {
		q.Set("kw", filter.Keyword)
	}
	if filter.ModelID != 0 {
		q.Set("model_id", strconv.FormatUint(uint64(filter.ModelID), 10))
	}
	if filter.Status != "" {
		q.Set("status", filter.Status)
	}
	if filter.Online != nil {
		q.Set("online", strconv.FormatBool(*filter.Online))
	}
	if filter.Transport != "" {
		q.Set("transport", filter.Transport)
	}
	var list []*Device
	pg, err := c.doList(ctx, "/devices", q, &list)
	return list, pg, err
}

// DeviceFilter 设备列表过滤项（租户过滤不可指定——由服务端按商户绑定注入）
type DeviceFilter struct {
	Keyword   string // sn/名称模糊
	ModelID   uint
	Status    string
	Online    *bool
	Transport string // socket | mqtt | default
}

// GetDevice 设备详情
func (c *Client) GetDevice(ctx context.Context, id uint) (*Device, error) {
	var out Device
	if err := c.do(ctx, http.MethodGet, "/devices/"+strconv.FormatUint(uint64(id), 10), nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// RegisterDevice 注册设备（tenant_id 须在商户绑定租户集内，否则 403）
func (c *Client) RegisterDevice(ctx context.Context, req RegisterDeviceRequest) (*Device, error) {
	var out Device
	if err := c.do(ctx, http.MethodPost, "/devices", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetShadow 设备影子
func (c *Client) GetShadow(ctx context.Context, deviceID uint) (*Shadow, error) {
	var out Shadow
	if err := c.do(ctx, http.MethodGet, "/devices/"+strconv.FormatUint(uint64(deviceID), 10)+"/shadow", nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// IssueCommand 下发命令（受理即返回；结果轮询 GetCommand）。IdempotencyKey 非空时幂等重放。
func (c *Client) IssueCommand(ctx context.Context, deviceID uint, req IssueCommandRequest) (*Command, error) {
	var out Command
	path := "/devices/" + strconv.FormatUint(uint64(deviceID), 10) + "/commands"
	if err := c.do(ctx, http.MethodPost, path, nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDeviceCommands 本商户下发过的命令（status 可选过滤）
func (c *Client) ListDeviceCommands(ctx context.Context, deviceID uint, status string, page, pageSize int) ([]*Command, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if status != "" {
		q.Set("status", status)
	}
	var list []*Command
	pg, err := c.doList(ctx, "/devices/"+strconv.FormatUint(uint64(deviceID), 10)+"/commands", q, &list)
	return list, pg, err
}

// GetCommand 单条命令（只可见本商户下发）
func (c *Client) GetCommand(ctx context.Context, id uint) (*Command, error) {
	var out Command
	if err := c.do(ctx, http.MethodGet, "/device-commands/"+strconv.FormatUint(uint64(id), 10), nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// TelemetryQuery 时序历史查询参数
type TelemetryQuery struct {
	Metric          string
	From, To        time.Time // RFC3339
	IntervalSeconds int       // 降采样粒度（秒），0=原始点
	Marker          string    // 上页 NextMarker
	Limit           int       // 默认 50，上限 200
}

// QueryTelemetry 时序遥测历史（ClickHouse）
func (c *Client) QueryTelemetry(ctx context.Context, deviceID uint, q TelemetryQuery) (*TelemetryHistory, error) {
	vals := url.Values{}
	if q.Metric != "" {
		vals.Set("metric", q.Metric)
	}
	if !q.From.IsZero() {
		vals.Set("from", q.From.Format(time.RFC3339))
	}
	if !q.To.IsZero() {
		vals.Set("to", q.To.Format(time.RFC3339))
	}
	if q.IntervalSeconds > 0 {
		vals.Set("interval_seconds", strconv.Itoa(q.IntervalSeconds))
	}
	if q.Marker != "" {
		vals.Set("marker", q.Marker)
	}
	if q.Limit > 0 {
		vals.Set("limit", strconv.Itoa(q.Limit))
	}
	var out TelemetryHistory
	path := "/devices/" + strconv.FormatUint(uint64(deviceID), 10) + "/telemetry"
	if err := c.do(ctx, http.MethodGet, path, vals, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDataEvents 离散事件游标拉取（升序增量；返回本轮最大 ID 供下轮 sinceID）
func (c *Client) ListDataEvents(ctx context.Context, deviceID, sinceID uint, limit int) ([]*DataEvent, error) {
	q := url.Values{}
	q.Set("since_id", strconv.FormatUint(uint64(sinceID), 10))
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	// data-events 响应为 {list}（无分页结构）
	var raw struct {
		List []*DataEvent `json:"list"`
	}
	path := "/devices/" + strconv.FormatUint(uint64(deviceID), 10) + "/data-events"
	if err := c.do(ctx, http.MethodGet, path, q, nil, &raw, nil); err != nil {
		return nil, err
	}
	return raw.List, nil
}

// ---- 租户域（scope tenant:*；商户自管租户，创建归属服务端注入） ----

// TenantFilter 租户列表过滤（结果天然限定本商户绑定集）
type TenantFilter struct {
	Name   string
	Code   string
	Status *int
}

// ListTenants 本商户绑定租户列表（name/code 模糊 + status 精确；页从 1 起）
func (c *Client) ListTenants(ctx context.Context, page, pageSize int, filter TenantFilter) ([]*Tenant, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if filter.Name != "" {
		q.Set("name", filter.Name)
	}
	if filter.Code != "" {
		q.Set("code", filter.Code)
	}
	if filter.Status != nil {
		q.Set("status", strconv.Itoa(*filter.Status))
	}
	var list []*Tenant
	pg, err := c.doList(ctx, "/tenants", q, &list)
	return list, pg, err
}

// CreateTenant 创建租户（merchant 归属=调用方商户；code 全局唯一，冲突 409）
func (c *Client) CreateTenant(ctx context.Context, req TenantCreateRequest) (*Tenant, error) {
	var out Tenant
	if err := c.do(ctx, http.MethodPost, "/tenants", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTenant 更新租户基础资料（非本商户绑定租户 404）
func (c *Client) UpdateTenant(ctx context.Context, id uint, req TenantUpdateRequest) error {
	return c.do(ctx, http.MethodPut, "/tenants/"+strconv.FormatUint(uint64(id), 10), nil, req, nil, nil)
}

// SetTenantStatus 启用/禁用租户
func (c *Client) SetTenantStatus(ctx context.Context, id uint, enabled bool) error {
	body := map[string]bool{"enabled": enabled}
	return c.do(ctx, http.MethodPut, "/tenants/"+strconv.FormatUint(uint64(id), 10)+"/status", nil, body, nil, nil)
}

// DeleteTenant 删除租户（平台侧存在关联应用用户时 409）
func (c *Client) DeleteTenant(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/tenants/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil, nil)
}

// FindTenantByCode 在本商户绑定集内按 code 精确查找（找不到返回 nil）。
// 租户 name/code 唯一性=商户内（2026-09-18 起）：他商户同 code 租户不可见、也不构成创建冲突
func (c *Client) FindTenantByCode(ctx context.Context, code string) (*Tenant, error) {
	for page := 1; page <= 50; page++ {
		list, pg, err := c.ListTenants(ctx, page, 50, TenantFilter{Code: code})
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

// ---- 型号域（scope model:*；全局资源） ----

// ModelFilter 型号列表过滤
type ModelFilter struct {
	Keyword string
	Status  *int
}

// ListDeviceModels 型号列表
func (c *Client) ListDeviceModels(ctx context.Context, page, pageSize int, filter ModelFilter) ([]*DeviceModel, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if filter.Keyword != "" {
		q.Set("kw", filter.Keyword)
	}
	if filter.Status != nil {
		q.Set("status", strconv.Itoa(*filter.Status))
	}
	var list []*DeviceModel
	pg, err := c.doList(ctx, "/device-models", q, &list)
	return list, pg, err
}

// GetDeviceModel 型号详情
func (c *Client) GetDeviceModel(ctx context.Context, id uint) (*DeviceModel, error) {
	var out DeviceModel
	if err := c.do(ctx, http.MethodGet, "/device-models/"+strconv.FormatUint(uint64(id), 10), nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateDeviceModel 创建型号（须绑物模型 model 层节点 + 发布态版本）
func (c *Client) CreateDeviceModel(ctx context.Context, req ModelCreateRequest) (*DeviceModel, error) {
	var out DeviceModel
	if err := c.do(ctx, http.MethodPost, "/device-models", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDeviceModel 更新型号
func (c *Client) UpdateDeviceModel(ctx context.Context, id uint, req ModelUpdateRequest) error {
	return c.do(ctx, http.MethodPut, "/device-models/"+strconv.FormatUint(uint64(id), 10), nil, req, nil, nil)
}

// DeleteDeviceModel 删除型号（型号下存在设备时 409）
func (c *Client) DeleteDeviceModel(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/device-models/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil, nil)
}

// ---- 物模型只读选择器（scope thing-model:read） ----

// ListThingModelNodes 物模型节点列表（layer 可选过滤 base/category/model/instance；
// pageSize=0 全量，供选择器整表构建）
func (c *Client) ListThingModelNodes(ctx context.Context, layer, kw string, pageSize int) ([]*TMNode, *Page, error) {
	q := url.Values{}
	if layer != "" {
		q.Set("layer", layer)
	}
	if kw != "" {
		q.Set("kw", kw)
	}
	q.Set("page", "1")
	q.Set("page_size", strconv.Itoa(pageSize))
	var list []*TMNode
	pg, err := c.doList(ctx, "/thing-models", q, &list)
	return list, pg, err
}

// ListTMPublishedVersions 节点的已发布版本（开放面只返回 published；型号绑定只允许发布态）
func (c *Client) ListTMPublishedVersions(ctx context.Context, nodeID uint) ([]*TMVersion, error) {
	var raw struct {
		List []*TMVersion `json:"list"`
	}
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10) + "/versions"
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &raw, nil); err != nil {
		return nil, err
	}
	return raw.List, nil
}

// ---- 用户域（scope user:*；商户成员：即建即绑/解绑，账号本体全局） ----

// UserFilter 商户成员列表过滤
type UserFilter struct {
	Keyword string // 用户名/昵称模糊
}

// ListUsers 本商户绑定成员列表（kw 模糊；页从 1 起；账号本体全局，只返回本商户绑定成员）
func (c *Client) ListUsers(ctx context.Context, page, pageSize int, filter UserFilter) ([]*MerchantUser, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if filter.Keyword != "" {
		q.Set("kw", filter.Keyword)
	}
	var list []*MerchantUser
	pg, err := c.doList(ctx, "/users", q, &list)
	return list, pg, err
}

// CreateOrBindUser 新增成员：平台无此账号则创建并绑定，有则仅绑定（幂等）。
// Password 仅新建时生效（初始密码），已存在账号绝不触碰密码。
func (c *Client) CreateOrBindUser(ctx context.Context, req UserCreateOrBindRequest) (*UserCreateOrBindResult, error) {
	var out UserCreateOrBindResult
	if err := c.do(ctx, http.MethodPost, "/users", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UnbindUser 移除成员：仅解除与本商户的绑定（平台账号保留）；未绑定/不存在 404
func (c *Client) UnbindUser(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/users/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil, nil)
}

// SetUserAdmission 业务端开/关成员准入（写回平台事实源；未绑定/不存在返回 404）
func (c *Client) SetUserAdmission(ctx context.Context, userID uint, admitted bool) error {
	return c.do(ctx, http.MethodPut, "/users/"+strconv.FormatUint(uint64(userID), 10)+"/admission",
		nil, &UserSetAdmissionRequest{Admitted: admitted}, nil, nil)
}
