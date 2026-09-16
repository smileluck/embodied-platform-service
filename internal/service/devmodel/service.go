// Package devmodel 型号管理应用服务（平台管理面代理）
package devmodel

import (
	"context"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
)

type Service struct {
	uc *bizdevmodel.Usecase
}

func NewService(uc *bizdevmodel.Usecase) *Service { return &Service{uc: uc} }

// Create/Update 入参直接复用 biz 契约类型（镜像平台 API）
type CreateRequest = bizdevmodel.ModelCreate
type UpdateRequest = bizdevmodel.ModelUpdate

// ListRequest 型号列表查询
type ListRequest struct {
	Keyword string `form:"kw"`
	Status  *int   `form:"status" binding:"omitempty,oneof=1 2"`
}

func (s *Service) List(ctx context.Context, req ListRequest, page, pageSize int) ([]*bizdevmodel.Model, interface{}, error) {
	return s.uc.List(ctx, bizdevmodel.ModelQuery{Keyword: req.Keyword, Status: req.Status}, page, pageSize)
}

func (s *Service) Get(ctx context.Context, id uint) (*bizdevmodel.Model, error) {
	return s.uc.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, req bizdevmodel.ModelCreate) (*bizdevmodel.Model, error) {
	return s.uc.Create(ctx, req)
}

func (s *Service) Update(ctx context.Context, id uint, req bizdevmodel.ModelUpdate) error {
	return s.uc.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.uc.Delete(ctx, id)
}

// TMNodesRequest 物模型节点选择器查询（layer=model 为型号可绑定层）
type TMNodesRequest struct {
	Layer string `form:"layer"`
	Kw    string `form:"kw"`
}

func (s *Service) TMNodes(ctx context.Context, req TMNodesRequest) ([]*bizdevmodel.TMNode, error) {
	return s.uc.TMNodes(ctx, req.Layer, req.Kw)
}

// TMPublishedVersions 节点的已发布版本（型号绑定仅允许发布态）
func (s *Service) TMPublishedVersions(ctx context.Context, nodeID uint) ([]*bizdevmodel.TMVersion, error) {
	return s.uc.TMPublishedVersions(ctx, nodeID)
}
