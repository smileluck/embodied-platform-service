// Package devmodel 设备型号限界上下文 —— 领域层。
//
// 纯代理：型号真源在 embodied-platform 管理面（/api/v1/device-models，服务账号 JWT）。
// 型号创建必须绑定物模型 model 层节点 + 已发布版本——本系统只提供只读选择器
// （物模型管理本身列入后续规划）。类型镜像平台 API 契约（snake_case）。
package devmodel

import (
	"context"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Model 设备型号（镜像平台 biz/devmodel.Model）
type Model struct {
	ID           uint   `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	TMNodeID     uint   `json:"tm_node_id"`
	TMVersionID  uint   `json:"tm_version_id"`
	TMNodeName   string `json:"tm_node_name,omitempty"`
	TMVersionNo  int    `json:"tm_version_no,omitempty"`
	Status       int    `json:"status"` // 1 启用 2 禁用
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

// TMNode 物模型节点（只读选择器数据源）
type TMNode struct {
	ID       uint   `json:"id"`
	Layer    string `json:"layer"` // base/category/model/instance
	ParentID uint   `json:"parent_id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Status   int    `json:"status"`
}

// TMVersion 物模型版本（型号绑定须选 status=published）
type TMVersion struct {
	ID          uint   `json:"id"`
	NodeID      uint   `json:"node_id"`
	Version     int    `json:"version"`
	Status      string `json:"status"` // draft / published
	PublishedAt string `json:"published_at"`
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
