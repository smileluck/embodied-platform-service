package platformsdk

import (
	"encoding/json"
	"fmt"
)

// 开放 API scope 常量（目录唯一真源在平台 internal/biz/merchant/scopes.go；
// 申请授权时以此常量与平台管理端对齐）。支持精确 / 域通配（device:*）/ 全局（*）。
const (
	ScopeDeviceList         = "device:list"
	ScopeDeviceGet          = "device:get"
	ScopeDeviceRegister     = "device:register"
	ScopeDeviceShadowRead   = "device:shadow:read"
	ScopeDeviceCommandList  = "device:command:list"
	ScopeDeviceCommandIssue = "device:command:issue"
	ScopeTelemetryRead      = "telemetry:read"
	ScopeDataEventRead      = "data-event:read"
	// 租户域（2026-09-16）：商户自管租户；创建时归属取调用方，读写按商户绑定收敛
	ScopeTenantList   = "tenant:list"
	ScopeTenantCreate = "tenant:create"
	ScopeTenantUpdate = "tenant:update"
	ScopeTenantStatus = "tenant:status"
	ScopeTenantDelete = "tenant:delete"
	// 型号域（2026-09-16）：全局资源，scope 控权
	ScopeModelList   = "model:list"
	ScopeModelGet    = "model:get"
	ScopeModelCreate = "model:create"
	ScopeModelUpdate = "model:update"
	ScopeModelDelete = "model:delete"
	// 物模型（2026-09-16 只读选择器：版本仅 published；2026-10-09 起放写面）：
	// read=节点/版本/resolve/继承状态读；write=节点 CRUD + 版本草稿/发布/回滚/删除
	// （写仅本商户独立节点，创建 merchant_id 服务端注入调用方）
	ScopeThingModelRead  = "thing-model:read"
	ScopeThingModelWrite = "thing-model:write"
	// 数据映射域（2026-10-09）：读=通用+本商户，写=仅本商户独立资源（通用/他商户统一 404）；
	// 创建 merchant_id 服务端注入调用方（入参不收）；publish 覆盖发布与回滚
	ScopeMappingList    = "mapping:list"
	ScopeMappingGet     = "mapping:get"
	ScopeMappingCreate  = "mapping:create"
	ScopeMappingUpdate  = "mapping:update"
	ScopeMappingDelete  = "mapping:delete"
	ScopeMappingPublish = "mapping:publish"
)

// Device 设备（开放面视图；时间为 RFC3339）
type Device struct {
	ID              uint              `json:"id"`
	SN              string            `json:"sn"`
	Name            string            `json:"name"`
	ModelID         uint              `json:"model_id"`
	TenantID        uint              `json:"tenant_id"`
	Status          string            `json:"status"` // inactive/active/disabled/retired
	Online          bool              `json:"online"`
	Transport       string            `json:"transport"` // ""跟随平台默认 | socket | mqtt
	FirmwareVersion string            `json:"firmware_version,omitempty"`
	HardwareVersion string            `json:"hardware_version,omitempty"`
	ServiceVersions map[string]string `json:"service_versions,omitempty"`
	LastSeenAt      string            `json:"last_seen_at"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

// RegisterDeviceRequest 设备注册入参（tenant_id 须在商户绑定租户集内）
type RegisterDeviceRequest struct {
	SN              string            `json:"sn"`
	Name            string            `json:"name"`
	ModelID         uint              `json:"model_id"`
	TenantID        uint              `json:"tenant_id"`
	FirmwareVersion string            `json:"firmware_version,omitempty"`
	HardwareVersion string            `json:"hardware_version,omitempty"`
	ServiceVersions map[string]string `json:"service_versions,omitempty"`
	Transport       string            `json:"transport,omitempty"` // socket | mqtt（空=跟随平台默认）
}

// Shadow 设备影子（desired 平台下发 / reported 设备回报，均为 JSON 文本）
type Shadow struct {
	DeviceID  uint   `json:"device_id"`
	Desired   string `json:"desired"`
	Reported  string `json:"reported"`
	UpdatedAt string `json:"updated_at"`
}

// Command 设备命令（Priority 数值越小越高：0 急停/1 运维/2 Agent/3 业务）
type Command struct {
	ID             uint   `json:"id"`
	DeviceID       uint   `json:"device_id"`
	CommandType    string `json:"command_type"`
	Params         string `json:"params"`
	Priority       int    `json:"priority"`
	Status         string `json:"status"` // pending/dispatched/acked/succeeded/failed/cancelling/cancelled/preempted/timeout
	Caller         string `json:"caller"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Result         string `json:"result,omitempty"`
	TraceID        string `json:"trace_id,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// IssueCommandRequest 命令下发入参（caller 由服务端固定为 merchant:{code}）。
// Priority 必须显式赋值：0=急停（高危，必抢占）…3=业务；服务端对缺省直接 400
// （int 零值即 0，无法区分「未填」与「显式急停」，故务必按语义填）。
type IssueCommandRequest struct {
	CommandType    string `json:"command_type"`
	Params         any    `json:"params,omitempty"`
	Priority       int    `json:"priority"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// DataEvent 离散数据事件（数据映射 event 分类落地；游标轮询：上轮最大 ID 作下轮 SinceID）
type DataEvent struct {
	ID          uint   `json:"id"`
	DeviceID    uint   `json:"device_id"`
	EventType   string `json:"event_type"`
	Payload     string `json:"payload"`
	SourceTopic string `json:"source_topic"`
	OccurredAt  string `json:"occurred_at"`
}

// TelemetrySample 遥测样本（IntervalSeconds>0 时为区间均值）
type TelemetrySample struct {
	Metric   string  `json:"metric"`
	Value    float64 `json:"value"`
	StrValue string  `json:"str_value,omitempty"`
	Ts       string  `json:"ts"`
}

// TelemetryHistory 遥测历史（NextMarker 非空表示还有更多，作下轮查询 Marker）
type TelemetryHistory struct {
	Items      []TelemetrySample `json:"items"`
	NextMarker string            `json:"next_marker,omitempty"`
}

// ---- 租户域（2026-09-16，scope tenant:*）----

// Tenant 租户（开放面视图：本商户绑定集；不含 merchant_id 等内部治理字段）
type Tenant struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Remark       string `json:"remark"`
	Status       int    `json:"status"` // 1 启用 0 禁用
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// TenantCreateRequest 创建租户入参（merchant 归属服务端注入调用方，不可指定）
type TenantCreateRequest struct {
	Name         string `json:"name"`
	Code         string `json:"code"`
	ContactName  string `json:"contact_name,omitempty"`
	ContactPhone string `json:"contact_phone,omitempty"`
	Remark       string `json:"remark,omitempty"`
}

// TenantUpdateRequest 更新租户入参（code 创建后不可改；绑定保持不变）
type TenantUpdateRequest struct {
	Name         string `json:"name"`
	ContactName  string `json:"contact_name,omitempty"`
	ContactPhone string `json:"contact_phone,omitempty"`
	Remark       string `json:"remark,omitempty"`
}

// ---- 型号域（2026-09-16，scope model:*）----

// DeviceModel 设备型号（merchant_id=0 通用 / >0 商户独立；开放面读=通用+本商户，写仅本商户独立——通用与他商户型号更新/删除统一 404）
type DeviceModel struct {
	ID           uint   `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	TMNodeID     uint   `json:"tm_node_id"`
	TMVersionID  uint   `json:"tm_version_id"`
	TMNodeName   string `json:"tm_node_name,omitempty"`
	TMVersionNo  int    `json:"tm_version_no,omitempty"`
	Status       int    `json:"status"` // 1 启用 2 禁用
	MerchantID   uint   `json:"merchant_id"`
	Manufacturer string `json:"manufacturer"`
	Description  string `json:"description"`
	Transport    string `json:"transport"` // ""跟随平台默认 | socket | mqtt
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ModelCreateRequest 创建型号入参（须绑物模型 model 层节点 + 发布态版本）
type ModelCreateRequest struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	TMNodeID     uint   `json:"tm_node_id"`
	TMVersionID  uint   `json:"tm_version_id"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Description  string `json:"description,omitempty"`
	Transport    string `json:"transport,omitempty"`
}

// ModelUpdateRequest 更新型号入参（code 创建后不可改）
type ModelUpdateRequest struct {
	Name         string `json:"name"`
	TMVersionID  uint   `json:"tm_version_id"`
	Status       int    `json:"status"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Description  string `json:"description,omitempty"`
	Transport    string `json:"transport,omitempty"`
}

// ---- 物模型（2026-09-16 只读选择器；2026-10-09 起含写面，scope thing-model:read/write）----

// TMNode 物模型节点（开放面节点列表按 通用+本商户 收敛）
type TMNode struct {
	ID         uint   `json:"id"`
	Layer      string `json:"layer"` // base/category/model/instance
	ParentID   uint   `json:"parent_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     int    `json:"status"`
	MerchantID uint   `json:"merchant_id"` // 0=通用，>0=商户独立
}

// TMProperty 物模型属性要素
type TMProperty struct {
	DataType    string   `json:"data_type"` // int / float / bool / string / enum / json
	Unit        string   `json:"unit,omitempty"`
	Writable    bool     `json:"writable"`
	Enum        []string `json:"enum,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Description string   `json:"description,omitempty"`
}

// TMParam 服务入参/事件出参定义
type TMParam struct {
	DataType    string   `json:"data_type"`
	Required    bool     `json:"required,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description,omitempty"`
}

// TMService 服务要素
type TMService struct {
	CallType    string             `json:"call_type"` // sync / async
	Params      map[string]TMParam `json:"params,omitempty"`
	Description string             `json:"description,omitempty"`
}

// TMEvent 事件要素
type TMEvent struct {
	Params      map[string]TMParam `json:"params,omitempty"`
	Description string             `json:"description,omitempty"`
}

// TMSchema 物模型 Schema（三类要素以名称为主键）
type TMSchema struct {
	Properties map[string]TMProperty `json:"properties,omitempty"`
	Services   map[string]TMService  `json:"services,omitempty"`
	Events     map[string]TMEvent    `json:"events,omitempty"`
}

// TMVersion 物模型版本（GET /thing-models/:id/versions 仅返回 published 的选择器视图；
// 写面动作（建草稿/发布/回滚）返回完整版本含 draft 状态与 schema）
type TMVersion struct {
	ID             uint      `json:"id"`
	NodeID         uint      `json:"node_id"`
	Version        int       `json:"version"`
	Schema         *TMSchema `json:"schema,omitempty"`
	Status         string    `json:"status"` // draft | published（选择器列表仅 published）
	ParentVersions string    `json:"parent_versions,omitempty"`
	RevertOf       *uint     `json:"revert_of,omitempty"`
	PublishedAt    string    `json:"published_at"`
	CreatedAt      string    `json:"created_at,omitempty"`
	UpdatedAt      string    `json:"updated_at,omitempty"`
}

// TMNodeCreateRequest 创建物模型节点（不收 merchant_id——归属服务端注入调用方商户）
type TMNodeCreateRequest struct {
	Layer    string `json:"layer"` // base/category/model/instance
	ParentID uint   `json:"parent_id,omitempty"`
	Code     string `json:"code"`
	Name     string `json:"name"`
}

// TMNodeUpdateRequest 更新物模型节点（code 与 merchant_id 创建后开放面不可改）
type TMNodeUpdateRequest struct {
	Name   string `json:"name"`
	Status int    `json:"status"` // 1=启用 2=禁用
}

// TMChainLink 继承链单元（节点与其参与合并的版本）
type TMChainLink struct {
	Node    *TMNode    `json:"node"`
	Version *TMVersion `json:"version"`
}

// TMResolveResult 合并解析结果（schema + 参与合并的链路版本）
type TMResolveResult struct {
	Schema *TMSchema      `json:"schema"`
	Chain  []*TMChainLink `json:"chain"`
}

// TMInheritanceUpdate 父层版本差异明细（快照基线 vs 当前最新已发布）
type TMInheritanceUpdate struct {
	Node            *TMNode    `json:"node"`
	SnapshotVersion *TMVersion `json:"snapshot_version"`
	LatestVersion   *TMVersion `json:"latest_version"`
}

// TMInheritanceStatus 继承状态（基线的父链快照是否落后于各父层最新已发布版本）
type TMInheritanceStatus struct {
	UpToDate bool                   `json:"up_to_date"`
	Baseline string                 `json:"baseline"` // draft | published | none
	Updates  []*TMInheritanceUpdate `json:"updates,omitempty"`
}

// ---- 数据映射域（2026-10-09，scope mapping:*）----

// Mapping 单条数据映射规则（source_topic → 数据分类 + 字段提取/触发；
// source_topic 支持设备占位符 {sn}/{device_id}/{name}/{model_id}）
type Mapping struct {
	SourceTopic  string            `json:"source_topic"`          // 通道内主题（socket 帧 topic / MQTT topic）
	TopicMatch   string            `json:"topic_match,omitempty"` // exact（默认）| glob（预留）
	DataCategory string            `json:"data_category"`         // shadow | telemetry | event | alarm | media
	DataType     string            `json:"data_type"`             // shadow=物模型属性名；event=物模型事件名；telemetry/alarm=自定义
	SampleRateHz float64           `json:"sample_rate_hz,omitempty"`
	FieldExtract map[string]string `json:"field_extract,omitempty"`
	Fields       []string          `json:"fields,omitempty"`
	Trigger      string            `json:"trigger,omitempty"`
}

// DataMappingDef 数据映射资源（merchant_id=0 通用 / >0 商户独立；开放面读=通用+本商户，写仅本商户独立）
type DataMappingDef struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	MerchantID uint   `json:"merchant_id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// DataMappingVersion 映射版本（资源内版本号自增；draft→published 发布不可变；回滚=复制来源版本追加新草稿）
type DataMappingVersion struct {
	ID          uint      `json:"id"`
	DefID       uint      `json:"def_id"`
	Version     int       `json:"version"`
	Label       string    `json:"label,omitempty"`
	Mappings    []Mapping `json:"mappings"`
	Status      string    `json:"status"`              // draft | published
	RevertOf    *uint     `json:"revert_of,omitempty"` // 回滚来源版本 ID
	PublishedAt string    `json:"published_at"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// MappingCreateRequest 创建映射资源入参（资源 + 首版草稿一步完成；
// merchant_id 服务端注入调用方商户，入参不收）
type MappingCreateRequest struct {
	Name     string    `json:"name"`
	Label    string    `json:"label,omitempty"`
	Mappings []Mapping `json:"mappings"`
}

// MappingDraftRequest 资源下新建草稿入参（单草稿制：已有草稿 409）
type MappingDraftRequest struct {
	Label    string    `json:"label,omitempty"`
	Mappings []Mapping `json:"mappings"`
}

// MappingDraftUpdateRequest 更新草稿入参（label 指针可选，nil=不修改；仅 draft 可改）
type MappingDraftUpdateRequest struct {
	Label    *string   `json:"label,omitempty"`
	Mappings []Mapping `json:"mappings"`
}

// MappingBindRequest 绑定型号入参（目标型号须对调用商户可见；
// 商户一致性：通用型号只绑通用映射，商户独立型号只绑通用或同商户映射，违反 409）
type MappingBindRequest struct {
	ModelID uint `json:"model_id"`
}

// MappingRollbackRequest 回滚入参（publish=true 建回滚草稿后直接发布，一步完成回退）
type MappingRollbackRequest struct {
	Publish bool `json:"publish"`
}

// MappingCreateResult 创建应答：资源 + 首版草稿
type MappingCreateResult struct {
	Def     *DataMappingDef     `json:"def"`
	Version *DataMappingVersion `json:"version"`
}

// Page 分页信息（列表响应 data.page）
type Page struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// envelope 统一响应信封 {code,msg,data}
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// listEnvelope 列表响应 data {list,page}
type listEnvelope struct {
	List json.RawMessage `json:"list"`
	Page Page            `json:"page"`
}

// Error 平台返回的非 0 业务错误（携带 HTTP 状态与信封 msg）
type Error struct {
	HTTPStatus int
	Code       int
	Msg        string
}

func (e *Error) Error() string {
	return fmt.Sprintf("openapi: http %d code %d: %s", e.HTTPStatus, e.Code, e.Msg)
}

// ---- 用户域（scope user:*；商户成员管理） ----

const (
	ScopeUserList   = "user:list"
	ScopeUserCreate = "user:create"
	ScopeUserDelete = "user:delete"
)

// MerchantUser 商户成员（平台账号本体全局；视图只含开放面允许暴露的字段，PII 不出开放面）
type MerchantUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Status   int    `json:"status"`   // 平台侧账号状态：1 启用 0 禁用（禁用=平台全局，无法登录）
	IsAdmin  bool   `json:"is_admin"` // 商户管理员标记（业务端据此投影为本地管理员角色）
	Admitted bool   `json:"admitted"` // 业务端准入状态（业务端开/关准入写回平台）
	BoundAt  string `json:"bound_at"`
}

// UserCreateOrBindRequest 新增成员入参。
// Password 仅平台不存在该用户名时必填（作为平台账号初始密码，6-64 位）；已存在时忽略。
// UserSetAdmissionRequest 业务端成员准入开关入参（写回平台事实源）
type UserSetAdmissionRequest struct {
	Admitted bool `json:"admitted"`
}

type UserCreateOrBindRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname,omitempty"`
	Password string `json:"password,omitempty"`
}

// UserCreateOrBindResult 新增成员结果
type UserCreateOrBindResult struct {
	User *MerchantUser `json:"user"`
	// Existed true=账号已存在、仅建立了本商户关联（未创建、未改密）
	Existed bool `json:"existed"`
}

// ---- 应用用户域（scope app-user:*；C 端多租户终端用户，2026-10-05） ----

const (
	ScopeAppUserList          = "app-user:list"
	ScopeAppUserCreate        = "app-user:create"
	ScopeAppUserUpdate        = "app-user:update"
	ScopeAppUserDelete        = "app-user:delete"
	ScopeAppUserSetStatus     = "app-user:setStatus"
	ScopeAppUserResetPassword = "app-user:resetPassword"
)

// AppUser 应用用户（tenant_ids 已按商户租户集收敛为交集视图；跨商户归属不可见）
type AppUser struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    int    `json:"status"`     // 1 启用 0 禁用（禁用=平台全局，App 登录/token 校验即时失败）
	TenantIDs []uint `json:"tenant_ids"` // 平台租户 ID（与商户租户集的交集）
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// AppUserCreateRequest 创建应用用户入参（tenant_ids 须⊆商户租户集，否则 403；
// username 冲突 409）
type AppUserCreateRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Nickname  string `json:"nickname,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	TenantIDs []uint `json:"tenant_ids"`
}

// AppUserUpdateRequest 更新入参（指针可选，nil/省略=不修改；tenant_ids 只改写本商户
// 租户集内归属，集外（他商户）归属平台侧保留；须⊆商户租户集，否则 403）
type AppUserUpdateRequest struct {
	Nickname  *string `json:"nickname,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Email     *string `json:"email,omitempty"`
	Status    *int    `json:"status,omitempty"`
	TenantIDs *[]uint `json:"tenant_ids,omitempty"`
}

// AppUserResetPasswordRequest 重置密码入参（旧密码立即失效）
type AppUserResetPasswordRequest struct {
	Password string `json:"password"`
}

// ---- 租户用户域（scope tenant-user:*；租户门户运营账号身份面，2026-10-08。
// 租户 RBAC 自 2026-10-09 本地化：tenant-role:* 域与 tenant-user:setRoles 已退役） ----

const (
	ScopeTenantUserList          = "tenant-user:list"
	ScopeTenantUserCreate        = "tenant-user:create"
	ScopeTenantUserUpdate        = "tenant-user:update"
	ScopeTenantUserDelete        = "tenant-user:delete"
	ScopeTenantUserSetStatus     = "tenant-user:setStatus"
	ScopeTenantUserResetPassword = "tenant-user:resetPassword"
)

// TenantUser 租户门户运营账号（单租户绑定，tenant_id 天然在商户租户集内；
// 角色绑定本地落库，平台身份面不涉）
type TenantUser struct {
	ID        uint   `json:"id"`
	TenantID  uint   `json:"tenant_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    int    `json:"status"` // 1 启用 0 禁用（禁用后门户登录/token 校验即时失败）
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TenantUserCreateRequest 创建租户用户入参（tenant_id 须∈商户租户集否则 403；username 冲突 409）
type TenantUserCreateRequest struct {
	TenantID uint   `json:"tenant_id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
}

// TenantUserUpdateRequest 更新入参（指针可选，nil/省略=不修改；tenant_id 不可改）
type TenantUserUpdateRequest struct {
	Nickname *string `json:"nickname,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Status   *int    `json:"status,omitempty"`
}

