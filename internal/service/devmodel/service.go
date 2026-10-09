// Package devmodel 型号管理应用服务（平台开放面代理，商户 HMAC）
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

// ---- 物模型读面扩展 + 写面（2026-10-09；DTO 复用 biz 契约类型）----

type TMNodeCreateRequest = bizdevmodel.TMNodeCreate
type TMNodeUpdateRequest = bizdevmodel.TMNodeUpdate
type TMSchemaUpdateRequest = bizdevmodel.TMSchemaUpdate
type TMRollbackRequest = bizdevmodel.TMRollback

func (s *Service) TMNode(ctx context.Context, nodeID uint) (*bizdevmodel.TMNode, error) {
	return s.uc.TMNode(ctx, nodeID)
}

func (s *Service) CreateTMNode(ctx context.Context, req TMNodeCreateRequest) (*bizdevmodel.TMNode, error) {
	return s.uc.CreateTMNode(ctx, req)
}

func (s *Service) UpdateTMNode(ctx context.Context, nodeID uint, req TMNodeUpdateRequest) error {
	return s.uc.UpdateTMNode(ctx, nodeID, req)
}

func (s *Service) DeleteTMNode(ctx context.Context, nodeID uint) error {
	return s.uc.DeleteTMNode(ctx, nodeID)
}

func (s *Service) CreateTMDraft(ctx context.Context, nodeID uint) (*bizdevmodel.TMVersion, error) {
	return s.uc.CreateTMDraft(ctx, nodeID)
}

func (s *Service) UpdateTMDraft(ctx context.Context, versionID uint, req TMSchemaUpdateRequest) error {
	return s.uc.UpdateTMDraft(ctx, versionID, req.Schema)
}

func (s *Service) PublishTMVersion(ctx context.Context, versionID uint) (*bizdevmodel.TMVersion, error) {
	return s.uc.PublishTMVersion(ctx, versionID)
}

func (s *Service) RollbackTMVersion(ctx context.Context, versionID uint, req TMRollbackRequest) (*bizdevmodel.TMVersion, error) {
	return s.uc.RollbackTMVersion(ctx, versionID, req.Publish)
}

func (s *Service) DeleteTMVersion(ctx context.Context, versionID uint) error {
	return s.uc.DeleteTMVersion(ctx, versionID)
}

// ResolveTM 合并解析节点完整 Schema（pinnedVersion=0 取基线版本）
func (s *Service) ResolveTM(ctx context.Context, nodeID, pinnedVersion uint) (*bizdevmodel.TMResolveResult, error) {
	return s.uc.ResolveTM(ctx, nodeID, pinnedVersion)
}

func (s *Service) TMInheritanceStatus(ctx context.Context, nodeID uint) (*bizdevmodel.TMInheritanceStatus, error) {
	return s.uc.TMInheritanceStatus(ctx, nodeID)
}
