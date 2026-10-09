// 数据映射领域（devmodel 上下文内的子域）—— 纯代理平台开放面 /data-mappings。
// 真源在 embodied-platform（无本地表）；商户收敛（读=通用+本商户、写仅本商户独立资源
// 404 不泄露、创建 merchant_id 服务端注入）由平台开放面保证，本服务不重复判断。
// 版本管理=单草稿制 + 发布不可变 + 追加式回滚（publish=true 一步完成）。
package devmodel

import (
	"context"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Mapping 单条数据映射规则（镜像平台 Mapping；source_topic → 数据分类 + 字段提取/触发，
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

// MappingDef 数据映射资源（MerchantID 0=通用 / >0=商户独立归属）
type MappingDef struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	MerchantID uint   `json:"merchant_id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// MappingVersion 映射版本（资源内版本号自增；draft→published 发布不可变；
// RevertOf 记录回滚来源版本 ID）
type MappingVersion struct {
	ID          uint      `json:"id"`
	DefID       uint      `json:"def_id"`
	Version     int       `json:"version"`
	Label       string    `json:"label,omitempty"`
	Mappings    []Mapping `json:"mappings"`
	Status      string    `json:"status"` // draft | published
	RevertOf    *uint     `json:"revert_of,omitempty"`
	PublishedAt string    `json:"published_at"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}

// MappingCreateResult 创建应答：资源 + 首版草稿
type MappingCreateResult struct {
	Def     *MappingDef     `json:"def"`
	Version *MappingVersion `json:"version"`
}

// MappingCreate 创建映射资源入参（资源 + 首版草稿一步完成；校验口径对齐平台开放面）
type MappingCreate struct {
	Name     string    `json:"name" binding:"required,max=64"`
	Label    string    `json:"label" binding:"max=64"`
	Mappings []Mapping `json:"mappings" binding:"required,min=1,dive"`
}

// MappingDraft 资源下新建草稿入参（单草稿制：已有草稿平台返回 409）
type MappingDraft struct {
	Label    string    `json:"label" binding:"max=64"`
	Mappings []Mapping `json:"mappings" binding:"required,min=1,dive"`
}

// MappingDraftUpdate 更新草稿入参（Label 指针可选，nil=不修改；仅 draft 可改）
type MappingDraftUpdate struct {
	Label    *string   `json:"label" binding:"omitempty,max=64"`
	Mappings []Mapping `json:"mappings" binding:"required,min=1,dive"`
}

// MappingBind 绑定型号入参（目标型号须对本商户可见；商户一致性由平台校验，违反 409）
type MappingBind struct {
	ModelID uint `json:"model_id" binding:"required"`
}

// MappingRollback 回滚入参（Publish=true 建回滚草稿后直接发布，一步完成回退）
type MappingRollback struct {
	Publish bool `json:"publish"`
}

// MappingGateway 平台数据映射网关接口（data 层经开放面 SDK 实现，依赖倒置）
type MappingGateway interface {
	ListMappings(ctx context.Context, kw string, page, pageSize int) ([]*MappingDef, *pagination.Page, error)
	GetMapping(ctx context.Context, id uint) (*MappingDef, error)
	CreateMapping(ctx context.Context, req MappingCreate) (*MappingCreateResult, error)
	DeleteMapping(ctx context.Context, id uint) error
	ListMappingVersions(ctx context.Context, defID uint) ([]*MappingVersion, error)
	CreateMappingDraft(ctx context.Context, defID uint, req MappingDraft) (*MappingVersion, error)
	GetMappingVersion(ctx context.Context, versionID uint) (*MappingVersion, error)
	UpdateMappingDraft(ctx context.Context, versionID uint, req MappingDraftUpdate) error
	PublishMapping(ctx context.Context, versionID uint) (*MappingVersion, error)
	RollbackMapping(ctx context.Context, versionID uint, publish bool) (*MappingVersion, error)
	DeleteMappingVersion(ctx context.Context, versionID uint) error
	ListMappingBoundModels(ctx context.Context, defID uint) ([]*Model, error)
	BindMappingModel(ctx context.Context, defID, modelID uint) error
	UnbindMappingModel(ctx context.Context, defID, modelID uint) error
	// GetPublishedMapping 型号当前生效映射版本；无已发布版本返回 nil, nil（非错误）
	GetPublishedMapping(ctx context.Context, modelID uint) (*MappingVersion, error)
}

// MappingUsecase 数据映射领域用例（纯透传，收敛与状态机校验都在平台侧）
type MappingUsecase struct {
	gw MappingGateway
}

func NewMappingUsecase(gw MappingGateway) *MappingUsecase { return &MappingUsecase{gw: gw} }

func (uc *MappingUsecase) List(ctx context.Context, kw string, page, pageSize int) ([]*MappingDef, *pagination.Page, error) {
	return uc.gw.ListMappings(ctx, kw, page, pageSize)
}

func (uc *MappingUsecase) Get(ctx context.Context, id uint) (*MappingDef, error) {
	return uc.gw.GetMapping(ctx, id)
}

func (uc *MappingUsecase) Create(ctx context.Context, req MappingCreate) (*MappingCreateResult, error) {
	return uc.gw.CreateMapping(ctx, req)
}

func (uc *MappingUsecase) Delete(ctx context.Context, id uint) error {
	return uc.gw.DeleteMapping(ctx, id)
}

func (uc *MappingUsecase) Versions(ctx context.Context, defID uint) ([]*MappingVersion, error) {
	return uc.gw.ListMappingVersions(ctx, defID)
}

func (uc *MappingUsecase) CreateDraft(ctx context.Context, defID uint, req MappingDraft) (*MappingVersion, error) {
	return uc.gw.CreateMappingDraft(ctx, defID, req)
}

func (uc *MappingUsecase) GetVersion(ctx context.Context, versionID uint) (*MappingVersion, error) {
	return uc.gw.GetMappingVersion(ctx, versionID)
}

func (uc *MappingUsecase) UpdateDraft(ctx context.Context, versionID uint, req MappingDraftUpdate) error {
	return uc.gw.UpdateMappingDraft(ctx, versionID, req)
}

func (uc *MappingUsecase) Publish(ctx context.Context, versionID uint) (*MappingVersion, error) {
	return uc.gw.PublishMapping(ctx, versionID)
}

func (uc *MappingUsecase) Rollback(ctx context.Context, versionID uint, publish bool) (*MappingVersion, error) {
	return uc.gw.RollbackMapping(ctx, versionID, publish)
}

func (uc *MappingUsecase) DeleteVersion(ctx context.Context, versionID uint) error {
	return uc.gw.DeleteMappingVersion(ctx, versionID)
}

func (uc *MappingUsecase) BoundModels(ctx context.Context, defID uint) ([]*Model, error) {
	return uc.gw.ListMappingBoundModels(ctx, defID)
}

func (uc *MappingUsecase) BindModel(ctx context.Context, defID, modelID uint) error {
	return uc.gw.BindMappingModel(ctx, defID, modelID)
}

func (uc *MappingUsecase) UnbindModel(ctx context.Context, defID, modelID uint) error {
	return uc.gw.UnbindMappingModel(ctx, defID, modelID)
}

func (uc *MappingUsecase) PublishedForModel(ctx context.Context, modelID uint) (*MappingVersion, error) {
	return uc.gw.GetPublishedMapping(ctx, modelID)
}
