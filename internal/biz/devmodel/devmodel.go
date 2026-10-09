// Package devmodel 设备型号限界上下文 —— 领域层。
//
// 纯代理：型号与物模型真源在 embodied-platform 开放面（商户 HMAC，SDK 类型经 data 层
// 镜像转换）。2026-10-08 起平台侧型号按商户归属收敛（merchant_id：0=通用，
// >0=商户独立）——读=通用+本商户，创建 merchant_id 由平台注入调用方，
// 更新/删除仅本商户独立型号（通用与他商户型号统一 404）。
// 物模型 2026-10-09 起同口径开放写面（节点 CRUD + 版本草稿/发布/回滚/删除，
// 版本操作经所属节点归属继承校验）；数据映射为同上下文子域（见 mapping.go）。
package devmodel

import (
	"context"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Model 设备型号（镜像平台 biz/devmodel.Model；MerchantID 0=通用 / >0=商户独立归属）
type Model struct {
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
	Transport    string `json:"transport"` // "" 跟随平台默认 | socket | mqtt
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ModelCreate 创建型号入参
type ModelCreate struct {
	Code         string `json:"code" binding:"required,max=64"`
	Name         string `json:"name" binding:"required,max=64"`
	TMNodeID     uint   `json:"tm_node_id" binding:"required"`
	TMVersionID  uint   `json:"tm_version_id" binding:"required"`
	Manufacturer string `json:"manufacturer"`
	Description  string `json:"description" binding:"max=255"`
	Transport    string `json:"transport" binding:"omitempty,oneof=socket mqtt"`
}

// ModelUpdate 更新型号入参
type ModelUpdate struct {
	Name         string `json:"name" binding:"required,max=64"`
	TMVersionID  uint   `json:"tm_version_id"`
	Status       int    `json:"status" binding:"required,oneof=1 2"`
	Manufacturer string `json:"manufacturer"`
	Description  string `json:"description" binding:"max=255"`
	Transport    string `json:"transport" binding:"omitempty,oneof=socket mqtt"`
}

// ModelQuery 型号列表过滤
type ModelQuery struct {
	Keyword string
	Status  *int
}

// TMNode 物模型节点（只读选择器数据源；开放面按 通用+本商户 收敛）
type TMNode struct {
	ID         uint   `json:"id"`
	Layer      string `json:"layer"` // base/category/model/instance
	ParentID   uint   `json:"parent_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Status     int    `json:"status"`
	MerchantID uint   `json:"merchant_id"` // 0=通用，>0=商户独立
}

// TMVersion 物模型版本（完整版本视图：列表选择器仅 published 且不含 schema；
// 写面动作（建草稿/发布/回滚）返回含 draft 状态与 schema 的完整版本）
type TMVersion struct {
	ID             uint      `json:"id"`
	NodeID         uint      `json:"node_id"`
	Version        int       `json:"version"`
	Schema         *TMSchema `json:"schema,omitempty"`
	Status         string    `json:"status"` // draft | published
	ParentVersions string    `json:"parent_versions,omitempty"`
	RevertOf       *uint     `json:"revert_of,omitempty"`
	PublishedAt    string    `json:"published_at"`
	CreatedAt      string    `json:"created_at,omitempty"`
	UpdatedAt      string    `json:"updated_at,omitempty"`
}

// Gateway 平台型号网关接口（data 层经管理面服务账号实现，依赖倒置）
type Gateway interface {
	ListModels(ctx context.Context, q ModelQuery, page, pageSize int) ([]*Model, *pagination.Page, error)
	GetModel(ctx context.Context, id uint) (*Model, error)
	CreateModel(ctx context.Context, req ModelCreate) (*Model, error)
	UpdateModel(ctx context.Context, id uint, req ModelUpdate) error
	DeleteModel(ctx context.Context, id uint) error
	ListTMNodes(ctx context.Context, layer, kw string) ([]*TMNode, error)
	ListTMVersions(ctx context.Context, nodeID uint) ([]*TMVersion, error)
	// ---- 物模型读面扩展 + 写面（2026-10-09；收敛同型号域，平台保证）----
	GetTMNode(ctx context.Context, nodeID uint) (*TMNode, error)
	CreateTMNode(ctx context.Context, req TMNodeCreate) (*TMNode, error)
	UpdateTMNode(ctx context.Context, nodeID uint, req TMNodeUpdate) error
	DeleteTMNode(ctx context.Context, nodeID uint) error
	CreateTMDraft(ctx context.Context, nodeID uint) (*TMVersion, error)
	UpdateTMDraft(ctx context.Context, versionID uint, schema *TMSchema) error
	PublishTMVersion(ctx context.Context, versionID uint) (*TMVersion, error)
	RollbackTMVersion(ctx context.Context, versionID uint, publish bool) (*TMVersion, error)
	DeleteTMVersion(ctx context.Context, versionID uint) error
	ResolveTM(ctx context.Context, nodeID, pinnedVersion uint) (*TMResolveResult, error)
	GetTMInheritanceStatus(ctx context.Context, nodeID uint) (*TMInheritanceStatus, error)
}

// Usecase 型号领域用例
type Usecase struct {
	gw Gateway
}

func NewUsecase(gw Gateway) *Usecase { return &Usecase{gw: gw} }

func (uc *Usecase) List(ctx context.Context, q ModelQuery, page, pageSize int) ([]*Model, *pagination.Page, error) {
	return uc.gw.ListModels(ctx, q, page, pageSize)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Model, error) {
	return uc.gw.GetModel(ctx, id)
}

func (uc *Usecase) Create(ctx context.Context, req ModelCreate) (*Model, error) {
	return uc.gw.CreateModel(ctx, req)
}

func (uc *Usecase) Update(ctx context.Context, id uint, req ModelUpdate) error {
	return uc.gw.UpdateModel(ctx, id, req)
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	return uc.gw.DeleteModel(ctx, id)
}

// TMNodes 物模型节点选择器（layer=model 即型号可绑定的层）
func (uc *Usecase) TMNodes(ctx context.Context, layer, kw string) ([]*TMNode, error) {
	return uc.gw.ListTMNodes(ctx, layer, kw)
}

// TMPublishedVersions 节点的已发布版本（型号绑定仅允许发布态）
func (uc *Usecase) TMPublishedVersions(ctx context.Context, nodeID uint) ([]*TMVersion, error) {
	vs, err := uc.gw.ListTMVersions(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]*TMVersion, 0, len(vs))
	for _, v := range vs {
		if v.Status == "published" {
			out = append(out, v)
		}
	}
	return out, nil
}
