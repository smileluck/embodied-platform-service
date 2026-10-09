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

// ListDevices 设备列表（页从 1 起；过滤项与服务端 scope 收敛叠加；
// TenantID 可选单租户过滤，须在商户绑定租户集内否则 403）
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
	if filter.TenantID != nil && *filter.TenantID != 0 {
		q.Set("tenant_id", strconv.FormatUint(uint64(*filter.TenantID), 10))
	}
	var list []*Device
	pg, err := c.doList(ctx, "/devices", q, &list)
	return list, pg, err
}

// DeviceFilter 设备列表过滤项（TenantID 可选单租户过滤=平台租户 ID，
// 未指定时返回商户绑定租户集全量；越界 403）
type DeviceFilter struct {
	Keyword   string // sn/名称模糊
	ModelID   uint
	Status    string
	Online    *bool
	Transport string // socket | mqtt | default
	TenantID  *uint  // 平台租户 ID（可选）
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

// ---- 物模型写面 + 读面扩展（2026-10-09，scope thing-model:write / thing-model:read；
// 创建 merchant_id 服务端注入调用方，写仅本商户独立节点，通用/他商户统一 404） ----

// GetTMNode 物模型节点详情（节点须对调用商户可见）
func (c *Client) GetTMNode(ctx context.Context, nodeID uint) (*TMNode, error) {
	var out TMNode
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateTMNode 创建物模型节点（merchant_id 服务端注入调用方商户；父链层级/归属校验由平台兜底）
func (c *Client) CreateTMNode(ctx context.Context, req *TMNodeCreateRequest) (*TMNode, error) {
	var out TMNode
	if err := c.do(ctx, http.MethodPost, "/thing-models", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTMNode 更新物模型节点（仅 name/status；code 与 merchant_id 不可改）
func (c *Client) UpdateTMNode(ctx context.Context, nodeID uint, req *TMNodeUpdateRequest) error {
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10)
	return c.do(ctx, http.MethodPut, path, nil, req, nil, nil)
}

// DeleteTMNode 删除物模型节点（有子节点/被型号绑定或设备引用时 409）
func (c *Client) DeleteTMNode(ctx context.Context, nodeID uint) error {
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil, nil)
}

// CreateTMDraft 节点下新建草稿版本（单草稿制：已有草稿 409）
func (c *Client) CreateTMDraft(ctx context.Context, nodeID uint) (*TMVersion, error) {
	var out TMVersion
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10) + "/versions"
	if err := c.do(ctx, http.MethodPost, path, nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTMDraft 修改草稿版本 Schema（已发布版本 409）
func (c *Client) UpdateTMDraft(ctx context.Context, versionID uint, schema *TMSchema) error {
	body := struct {
		Schema *TMSchema `json:"schema"`
	}{Schema: schema}
	path := "/thing-models/versions/" + strconv.FormatUint(uint64(versionID), 10)
	return c.do(ctx, http.MethodPut, path, nil, body, nil, nil)
}

// PublishTMVersion 发布版本（发布后不可变；ADR-012 只扩展不删减校验由平台兜底）
func (c *Client) PublishTMVersion(ctx context.Context, versionID uint) (*TMVersion, error) {
	var out TMVersion
	path := "/thing-models/versions/" + strconv.FormatUint(uint64(versionID), 10) + "/publish"
	if err := c.do(ctx, http.MethodPost, path, nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// RollbackTMVersion 回退到指定已发布版本（复制其 Schema 与父链快照创建新草稿；
// publish=true 时建草稿后直接发布，一步完成回退）
func (c *Client) RollbackTMVersion(ctx context.Context, versionID uint, publish bool) (*TMVersion, error) {
	var out TMVersion
	body := struct {
		Publish bool `json:"publish"`
	}{Publish: publish}
	path := "/thing-models/versions/" + strconv.FormatUint(uint64(versionID), 10) + "/rollback"
	if err := c.do(ctx, http.MethodPost, path, nil, body, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTMVersion 删除版本（草稿可直接删；已发布版本被型号引用时 409）
func (c *Client) DeleteTMVersion(ctx context.Context, versionID uint) error {
	path := "/thing-models/versions/" + strconv.FormatUint(uint64(versionID), 10)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil, nil)
}

// ResolveTM 合并解析节点完整 Schema（pinnedVersion≠0 时须属于该节点）
func (c *Client) ResolveTM(ctx context.Context, nodeID, pinnedVersion uint) (*TMResolveResult, error) {
	q := url.Values{}
	if pinnedVersion != 0 {
		q.Set("version_id", strconv.FormatUint(uint64(pinnedVersion), 10))
	}
	var out TMResolveResult
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10) + "/resolve"
	if err := c.do(ctx, http.MethodGet, path, q, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTMInheritanceStatus 节点继承状态（父类是否已发布快照之后的新版本）
func (c *Client) GetTMInheritanceStatus(ctx context.Context, nodeID uint) (*TMInheritanceStatus, error) {
	var out TMInheritanceStatus
	path := "/thing-models/" + strconv.FormatUint(uint64(nodeID), 10) + "/inheritance-status"
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
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

// ---- 应用用户域（scope app-user:*；C 端多租户终端用户，2026-10-05） ----

// AppUserFilter 应用用户列表过滤（tenant_id 为平台租户 ID，结果天然限定商户租户集交集）
type AppUserFilter struct {
	Keyword  string // 用户名/昵称模糊
	Phone    string // 手机号精确
	Status   *int   // 1 启用 0 禁用
	TenantID uint   // 平台租户 ID（0=不过滤）
}

// ListAppUsers 本商户可见应用用户列表（归属∩商户租户集≠∅；页从 1 起）
func (c *Client) ListAppUsers(ctx context.Context, page, pageSize int, filter AppUserFilter) ([]*AppUser, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if filter.Keyword != "" {
		q.Set("kw", filter.Keyword)
	}
	if filter.Phone != "" {
		q.Set("phone", filter.Phone)
	}
	if filter.Status != nil {
		q.Set("status", strconv.Itoa(*filter.Status))
	}
	if filter.TenantID != 0 {
		q.Set("tenant_id", strconv.FormatUint(uint64(filter.TenantID), 10))
	}
	var list []*AppUser
	pg, err := c.doList(ctx, "/app-users", q, &list)
	return list, pg, err
}

// CreateAppUser 创建应用用户（tenant_ids 须⊆商户租户集否则 403；username 冲突 409）
func (c *Client) CreateAppUser(ctx context.Context, req AppUserCreateRequest) (*AppUser, error) {
	var out AppUser
	if err := c.do(ctx, http.MethodPost, "/app-users", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAppUser 更新应用用户（不可见/不存在 404；tenant_ids 越界 403）
func (c *Client) UpdateAppUser(ctx context.Context, id uint, req AppUserUpdateRequest) error {
	return c.do(ctx, http.MethodPut, "/app-users/"+strconv.FormatUint(uint64(id), 10), nil, req, nil, nil)
}

// SetAppUserStatus 启用/禁用应用用户（禁用后 App 登录/token 校验即时失败）
func (c *Client) SetAppUserStatus(ctx context.Context, id uint, enabled bool) error {
	status := 0
	if enabled {
		status = 1
	}
	return c.do(ctx, http.MethodPut, "/app-users/"+strconv.FormatUint(uint64(id), 10)+"/status",
		nil, &struct {
			Status int `json:"status"`
		}{Status: status}, nil, nil)
}

// ResetAppUserPassword 管理员重置密码（旧密码立即失效；不可见/不存在 404）
func (c *Client) ResetAppUserPassword(ctx context.Context, id uint, password string) error {
	return c.do(ctx, http.MethodPut, "/app-users/"+strconv.FormatUint(uint64(id), 10)+"/password",
		nil, &AppUserResetPasswordRequest{Password: password}, nil, nil)
}

// DeleteAppUser 删除应用用户（平台软删+墓碑；仍有他商户归属时 409，须归属商户各自先解除）
func (c *Client) DeleteAppUser(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/app-users/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil, nil)
}

// ---- 租户用户域（scope tenant-user:*；账号身份面。租户 RBAC 自 2026-10-09 本地化到本仓） ----

// TenantUserFilter 租户用户列表过滤（tenant_id 为平台租户 ID，越界=空集而非报错）
type TenantUserFilter struct {
	Keyword  string // 用户名/昵称模糊
	Phone    string // 手机号精确
	Status   *int   // 1 启用 0 禁用
	TenantID uint   // 平台租户 ID（0=不过滤）
}

// ListTenantUsers 本商户可见租户用户列表（归属租户∈商户租户集；页从 1 起）
func (c *Client) ListTenantUsers(ctx context.Context, page, pageSize int, filter TenantUserFilter) ([]*TenantUser, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if filter.Keyword != "" {
		q.Set("kw", filter.Keyword)
	}
	if filter.Phone != "" {
		q.Set("phone", filter.Phone)
	}
	if filter.Status != nil {
		q.Set("status", strconv.Itoa(*filter.Status))
	}
	if filter.TenantID != 0 {
		q.Set("tenant_id", strconv.FormatUint(uint64(filter.TenantID), 10))
	}
	var list []*TenantUser
	pg, err := c.doList(ctx, "/tenant-users", q, &list)
	return list, pg, err
}

// CreateTenantUser 创建租户门户运营账号（tenant_id 须∈商户租户集否则 403；username 冲突 409）
func (c *Client) CreateTenantUser(ctx context.Context, req TenantUserCreateRequest) (*TenantUser, error) {
	var out TenantUser
	if err := c.do(ctx, http.MethodPost, "/tenant-users", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateTenantUser 更新租户用户资料（本商户不可见/不存在统一 404；tenant_id 不可改）
func (c *Client) UpdateTenantUser(ctx context.Context, id uint, req TenantUserUpdateRequest) error {
	return c.do(ctx, http.MethodPut, "/tenant-users/"+strconv.FormatUint(uint64(id), 10), nil, req, nil, nil)
}

// SetTenantUserStatus 启用/禁用租户用户（禁用后门户登录/token 校验即时失败）
func (c *Client) SetTenantUserStatus(ctx context.Context, id uint, enabled bool) error {
	status := 0
	if enabled {
		status = 1
	}
	return c.do(ctx, http.MethodPut, "/tenant-users/"+strconv.FormatUint(uint64(id), 10)+"/status",
		nil, &struct {
			Status int `json:"status"`
		}{Status: status}, nil, nil)
}

// ResetTenantUserPassword 管理员重置密码（旧密码立即失效；不可见/不存在 404）
func (c *Client) ResetTenantUserPassword(ctx context.Context, id uint, password string) error {
	return c.do(ctx, http.MethodPut, "/tenant-users/"+strconv.FormatUint(uint64(id), 10)+"/password",
		nil, &struct {
			Password string `json:"password"`
		}{Password: password}, nil, nil)
}

// GetTenantUser 按 id 取单个账号（身份字段；不可见统一 404。scope tenant-user:list。
// 本地校验角色同租户/门户定位成员用）
func (c *Client) GetTenantUser(ctx context.Context, id uint) (*TenantUser, error) {
	var out TenantUser
	if err := c.do(ctx, http.MethodGet, "/tenant-users/"+strconv.FormatUint(uint64(id), 10), nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteTenantUser 删除租户用户（平台软删+墓碑释放 username 槽位）
func (c *Client) DeleteTenantUser(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/tenant-users/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil, nil)
}

// ---- 数据映射域（scope mapping:*；2026-10-09）----
// 收敛口径：读=通用+本商户、写=仅本商户独立资源（通用/他商户统一 404 不泄露存在性）、
// 创建 merchant_id 服务端注入调用方（入参不收）。版本管理=单草稿制 + 发布不可变 + 追加式回滚。

// ListMappingDefs 映射资源列表（kw 模糊 + 分页；范围=通用 + 本商户独立）
func (c *Client) ListMappingDefs(ctx context.Context, kw string, page, pageSize int) ([]*DataMappingDef, *Page, error) {
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	if kw != "" {
		q.Set("kw", kw)
	}
	var list []*DataMappingDef
	pg, err := c.doList(ctx, "/data-mappings", q, &list)
	return list, pg, err
}

// GetMappingDef 映射资源详情（不可见/不存在统一 404）
func (c *Client) GetMappingDef(ctx context.Context, id uint) (*DataMappingDef, error) {
	var out DataMappingDef
	if err := c.do(ctx, http.MethodGet, "/data-mappings/"+strconv.FormatUint(uint64(id), 10), nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateMappingDef 创建映射资源 + 首版草稿（一步完成；merchant_id=调用方商户）
func (c *Client) CreateMappingDef(ctx context.Context, req MappingCreateRequest) (*MappingCreateResult, error) {
	var out MappingCreateResult
	if err := c.do(ctx, http.MethodPost, "/data-mappings", nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMappingDef 删除映射资源（仍有版本或型号绑定 409；仅本商户独立资源可删）
func (c *Client) DeleteMappingDef(ctx context.Context, id uint) error {
	return c.do(ctx, http.MethodDelete, "/data-mappings/"+strconv.FormatUint(uint64(id), 10), nil, nil, nil, nil)
}

// ListMappingVersions 资源版本列表（含草稿；def 不可见 404；响应 {list} 无分页）
func (c *Client) ListMappingVersions(ctx context.Context, defID uint) ([]*DataMappingVersion, error) {
	var raw struct {
		List []*DataMappingVersion `json:"list"`
	}
	path := "/data-mappings/" + strconv.FormatUint(uint64(defID), 10) + "/versions"
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &raw, nil); err != nil {
		return nil, err
	}
	return raw.List, nil
}

// CreateMappingDraft 资源下新建草稿（单草稿制：已有草稿 409；def 须本商户独立）
func (c *Client) CreateMappingDraft(ctx context.Context, defID uint, req MappingDraftRequest) (*DataMappingVersion, error) {
	var out DataMappingVersion
	path := "/data-mappings/" + strconv.FormatUint(uint64(defID), 10) + "/versions"
	if err := c.do(ctx, http.MethodPost, path, nil, req, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMappingVersion 版本详情（所属 def 不可见 404）
func (c *Client) GetMappingVersion(ctx context.Context, versionID uint) (*DataMappingVersion, error) {
	var out DataMappingVersion
	if err := c.do(ctx, http.MethodGet, "/data-mappings/versions/"+strconv.FormatUint(uint64(versionID), 10), nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateMappingDraft 更新草稿（仅 draft 可改，已发布 409）
func (c *Client) UpdateMappingDraft(ctx context.Context, versionID uint, req MappingDraftUpdateRequest) error {
	return c.do(ctx, http.MethodPut, "/data-mappings/versions/"+strconv.FormatUint(uint64(versionID), 10), nil, req, nil, nil)
}

// PublishMapping 发布版本（对已绑定型号逐一做物模型一致性校验，不符 400/409）
func (c *Client) PublishMapping(ctx context.Context, versionID uint) (*DataMappingVersion, error) {
	var out DataMappingVersion
	path := "/data-mappings/versions/" + strconv.FormatUint(uint64(versionID), 10) + "/publish"
	if err := c.do(ctx, http.MethodPost, path, nil, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// RollbackMapping 回退到指定已发布版本（publish=true 建回滚草稿后直接发布，一步完成）
func (c *Client) RollbackMapping(ctx context.Context, versionID uint, publish bool) (*DataMappingVersion, error) {
	var out DataMappingVersion
	path := "/data-mappings/versions/" + strconv.FormatUint(uint64(versionID), 10) + "/rollback"
	if err := c.do(ctx, http.MethodPost, path, nil, &MappingRollbackRequest{Publish: publish}, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMappingVersion 删除版本（当前生效版本 409；仅可删草稿与历史已发布版本）
func (c *Client) DeleteMappingVersion(ctx context.Context, versionID uint) error {
	return c.do(ctx, http.MethodDelete, "/data-mappings/versions/"+strconv.FormatUint(uint64(versionID), 10), nil, nil, nil, nil)
}

// ListMappingBoundModels 资源已绑定的型号列表（def 不可见 404；响应 {list} 无分页）
func (c *Client) ListMappingBoundModels(ctx context.Context, defID uint) ([]*DeviceModel, error) {
	var raw struct {
		List []*DeviceModel `json:"list"`
	}
	path := "/data-mappings/" + strconv.FormatUint(uint64(defID), 10) + "/bindings"
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &raw, nil); err != nil {
		return nil, err
	}
	return raw.List, nil
}

// BindMappingModel 绑定型号（目标型号须对调用商户可见否则 404；商户归属不一致 409；重复绑定幂等）
func (c *Client) BindMappingModel(ctx context.Context, defID, modelID uint) error {
	path := "/data-mappings/" + strconv.FormatUint(uint64(defID), 10) + "/bindings"
	return c.do(ctx, http.MethodPost, path, nil, &MappingBindRequest{ModelID: modelID}, nil, nil)
}

// UnbindMappingModel 解绑型号（未绑定幂等成功）
func (c *Client) UnbindMappingModel(ctx context.Context, defID, modelID uint) error {
	path := "/data-mappings/" + strconv.FormatUint(uint64(defID), 10) + "/bindings/" + strconv.FormatUint(uint64(modelID), 10)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil, nil)
}

// GetPublishedMapping 型号当前生效映射版本（型号须对调用商户可见否则 404；无已发布版本返回 nil, nil）
func (c *Client) GetPublishedMapping(ctx context.Context, modelID uint) (*DataMappingVersion, error) {
	var out *DataMappingVersion
	q := url.Values{"model_id": []string{strconv.FormatUint(uint64(modelID), 10)}}
	if err := c.do(ctx, http.MethodGet, "/data-mappings/effective", q, nil, &out, nil); err != nil {
		return nil, err
	}
	return out, nil
}
