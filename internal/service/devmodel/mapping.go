// 数据映射应用服务（平台开放面代理，商户 HMAC）——DTO 复用 biz 契约类型（type alias 模式）
package devmodel

import (
	"context"

	bizdevmodel "github.com/smilex/smilex-admin-gin/internal/biz/devmodel"
)

// MappingService 数据映射应用服务（纯透传 MappingUsecase）
type MappingService struct {
	uc *bizdevmodel.MappingUsecase
}

func NewMappingService(uc *bizdevmodel.MappingUsecase) *MappingService { return &MappingService{uc: uc} }

// 请求 DTO 直接复用 biz 契约类型（镜像平台 API；binding 校验在 biz 类型 tag 上）
type MappingCreateRequest = bizdevmodel.MappingCreate
type MappingDraftRequest = bizdevmodel.MappingDraft
type MappingDraftUpdateRequest = bizdevmodel.MappingDraftUpdate
type MappingBindRequest = bizdevmodel.MappingBind
type MappingRollbackRequest = bizdevmodel.MappingRollback

// MappingListRequest 映射资源列表查询
type MappingListRequest struct {
	Keyword string `form:"kw"`
}

func (s *MappingService) List(ctx context.Context, req MappingListRequest, page, pageSize int) ([]*bizdevmodel.MappingDef, interface{}, error) {
	return s.uc.List(ctx, req.Keyword, page, pageSize)
}

func (s *MappingService) Get(ctx context.Context, id uint) (*bizdevmodel.MappingDef, error) {
	return s.uc.Get(ctx, id)
}

func (s *MappingService) Create(ctx context.Context, req MappingCreateRequest) (*bizdevmodel.MappingCreateResult, error) {
	return s.uc.Create(ctx, req)
}

func (s *MappingService) Delete(ctx context.Context, id uint) error {
	return s.uc.Delete(ctx, id)
}

func (s *MappingService) Versions(ctx context.Context, defID uint) ([]*bizdevmodel.MappingVersion, error) {
	return s.uc.Versions(ctx, defID)
}

func (s *MappingService) CreateDraft(ctx context.Context, defID uint, req MappingDraftRequest) (*bizdevmodel.MappingVersion, error) {
	return s.uc.CreateDraft(ctx, defID, req)
}

func (s *MappingService) GetVersion(ctx context.Context, versionID uint) (*bizdevmodel.MappingVersion, error) {
	return s.uc.GetVersion(ctx, versionID)
}

func (s *MappingService) UpdateDraft(ctx context.Context, versionID uint, req MappingDraftUpdateRequest) error {
	return s.uc.UpdateDraft(ctx, versionID, req)
}

func (s *MappingService) Publish(ctx context.Context, versionID uint) (*bizdevmodel.MappingVersion, error) {
	return s.uc.Publish(ctx, versionID)
}

func (s *MappingService) Rollback(ctx context.Context, versionID uint, req MappingRollbackRequest) (*bizdevmodel.MappingVersion, error) {
	return s.uc.Rollback(ctx, versionID, req.Publish)
}

func (s *MappingService) DeleteVersion(ctx context.Context, versionID uint) error {
	return s.uc.DeleteVersion(ctx, versionID)
}

func (s *MappingService) BoundModels(ctx context.Context, defID uint) ([]*bizdevmodel.Model, error) {
	return s.uc.BoundModels(ctx, defID)
}

func (s *MappingService) BindModel(ctx context.Context, defID uint, req MappingBindRequest) error {
	return s.uc.BindModel(ctx, defID, req.ModelID)
}

func (s *MappingService) UnbindModel(ctx context.Context, defID, modelID uint) error {
	return s.uc.UnbindModel(ctx, defID, modelID)
}

// PublishedForModel 型号当前生效映射版本（无已发布版本返回 nil, nil）
func (s *MappingService) PublishedForModel(ctx context.Context, modelID uint) (*bizdevmodel.MappingVersion, error) {
	return s.uc.PublishedForModel(ctx, modelID)
}
