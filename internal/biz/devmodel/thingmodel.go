// 物模型管理（devmodel 上下文内的子域）—— 纯代理平台开放面 /thing-models。
// 真源在平台（无本地表）；商户收敛（读=通用+本商户、写仅本商户独立节点 404 不泄露、
// 创建 merchant_id 服务端注入、版本操作经所属节点归属继承）由平台开放面保证，
// 本服务不重复判断。版本管理=单草稿制 + 发布不可变 + 追加式回滚。
package devmodel

import (
	"context"
)

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

// TMSchema 物模型 Schema（三类要素以名称为主键；要素级校验由平台兜底）
type TMSchema struct {
	Properties map[string]TMProperty `json:"properties,omitempty"`
	Services   map[string]TMService  `json:"services,omitempty"`
	Events     map[string]TMEvent    `json:"events,omitempty"`
}

// TMNodeCreate 创建物模型节点入参（校验口径对齐平台开放面；不收 merchant_id，平台注入调用方）
type TMNodeCreate struct {
	Layer    string `json:"layer" binding:"required,oneof=base category model instance"`
	ParentID uint   `json:"parent_id"`
	Code     string `json:"code" binding:"required,max=64"`
	Name     string `json:"name" binding:"required,max=64"`
}

// TMNodeUpdate 更新物模型节点入参（code 与 merchant_id 创建后不可改）
type TMNodeUpdate struct {
	Name   string `json:"name" binding:"required,max=64"`
	Status int    `json:"status" binding:"required,oneof=1 2"`
}

// TMSchemaUpdate 草稿 Schema 更新入参
type TMSchemaUpdate struct {
	Schema *TMSchema `json:"schema" binding:"required"`
}

// TMRollback 版本回滚入参（Publish=true 建回滚草稿后直接发布，一步完成回退）
type TMRollback struct {
	Publish bool `json:"publish"`
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

// ---- 用例（纯透传 Gateway；收敛与状态机校验都在平台侧）----

func (uc *Usecase) TMNode(ctx context.Context, nodeID uint) (*TMNode, error) {
	return uc.gw.GetTMNode(ctx, nodeID)
}

func (uc *Usecase) CreateTMNode(ctx context.Context, req TMNodeCreate) (*TMNode, error) {
	return uc.gw.CreateTMNode(ctx, req)
}

func (uc *Usecase) UpdateTMNode(ctx context.Context, nodeID uint, req TMNodeUpdate) error {
	return uc.gw.UpdateTMNode(ctx, nodeID, req)
}

func (uc *Usecase) DeleteTMNode(ctx context.Context, nodeID uint) error {
	return uc.gw.DeleteTMNode(ctx, nodeID)
}

func (uc *Usecase) CreateTMDraft(ctx context.Context, nodeID uint) (*TMVersion, error) {
	return uc.gw.CreateTMDraft(ctx, nodeID)
}

func (uc *Usecase) UpdateTMDraft(ctx context.Context, versionID uint, schema *TMSchema) error {
	return uc.gw.UpdateTMDraft(ctx, versionID, schema)
}

func (uc *Usecase) PublishTMVersion(ctx context.Context, versionID uint) (*TMVersion, error) {
	return uc.gw.PublishTMVersion(ctx, versionID)
}

func (uc *Usecase) RollbackTMVersion(ctx context.Context, versionID uint, publish bool) (*TMVersion, error) {
	return uc.gw.RollbackTMVersion(ctx, versionID, publish)
}

func (uc *Usecase) DeleteTMVersion(ctx context.Context, versionID uint) error {
	return uc.gw.DeleteTMVersion(ctx, versionID)
}

// ResolveTM 合并解析节点完整 Schema（pinnedVersion=0 取基线版本）
func (uc *Usecase) ResolveTM(ctx context.Context, nodeID, pinnedVersion uint) (*TMResolveResult, error) {
	return uc.gw.ResolveTM(ctx, nodeID, pinnedVersion)
}

func (uc *Usecase) TMInheritanceStatus(ctx context.Context, nodeID uint) (*TMInheritanceStatus, error) {
	return uc.gw.GetTMInheritanceStatus(ctx, nodeID)
}
