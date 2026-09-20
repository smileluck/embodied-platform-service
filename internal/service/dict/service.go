// Package dict 数据字典应用服务
package dict

import (
	"context"

	bizdict "github.com/smilex/smilex-admin-gin/internal/biz/dict"
)

type Service struct {
	uc *bizdict.Usecase
}

func NewService(uc *bizdict.Usecase) *Service { return &Service{uc: uc} }

// ---- 字典类型 ----

type TypeCreateRequest struct {
	Name   string `json:"name" binding:"required,max=20"`
	Code   string `json:"code" binding:"required,max=64"`
	Remark string `json:"remark" binding:"max=200"`
	Status *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

type TypeUpdateRequest struct {
	Name   string `json:"name" binding:"omitempty,max=20"`
	Code   string `json:"code" binding:"omitempty,max=64"`
	Remark string `json:"remark" binding:"max=200"`
	Status *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

func (s *Service) CreateType(ctx context.Context, req TypeCreateRequest) (*bizdict.DictType, error) {
	return s.uc.CreateType(ctx, bizdict.TypeInput{
		Name: req.Name, Code: req.Code, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) UpdateType(ctx context.Context, id uint, req TypeUpdateRequest) error {
	return s.uc.UpdateType(ctx, id, bizdict.TypeInput{
		Name: req.Name, Code: req.Code, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) DeleteType(ctx context.Context, id uint) error { return s.uc.DeleteType(ctx, id) }
func (s *Service) GetType(ctx context.Context, id uint) (*bizdict.DictType, error) {
	return s.uc.GetType(ctx, id)
}
func (s *Service) ListTypes(ctx context.Context, q bizdict.Query, page, pageSize int) ([]*bizdict.DictType, interface{}, error) {
	return s.uc.ListTypes(ctx, q, page, pageSize)
}

// ---- 字典项 ----

type ItemCreateRequest struct {
	TypeID uint   `json:"type_id" binding:"omitempty,gt=0"` // 归属类型以路径 :id 为准
	Label  string `json:"label" binding:"required,max=20"`
	Value  string `json:"value" binding:"required,max=64"`
	Sort   int    `json:"sort" binding:"gte=0,lte=9999"`
	Remark string `json:"remark" binding:"max=200"`
	Status *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

type ItemUpdateRequest struct {
	TypeID uint   `json:"type_id" binding:"omitempty,gt=0"`
	Label  string `json:"label" binding:"required,max=20"`
	Value  string `json:"value" binding:"required,max=64"`
	Sort   int    `json:"sort" binding:"gte=0,lte=9999"`
	Remark string `json:"remark" binding:"max=200"`
	Status *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

func (s *Service) CreateItem(ctx context.Context, req ItemCreateRequest) (*bizdict.DictItem, error) {
	return s.uc.CreateItem(ctx, bizdict.ItemInput{
		TypeID: req.TypeID, Label: req.Label, Value: req.Value,
		Sort: req.Sort, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) UpdateItem(ctx context.Context, id uint, req ItemUpdateRequest) error {
	return s.uc.UpdateItem(ctx, id, bizdict.ItemInput{
		TypeID: req.TypeID, Label: req.Label, Value: req.Value,
		Sort: req.Sort, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) DeleteItem(ctx context.Context, id uint) error { return s.uc.DeleteItem(ctx, id) }
func (s *Service) GetItem(ctx context.Context, id uint) (*bizdict.DictItem, error) {
	return s.uc.GetItem(ctx, id)
}
func (s *Service) ListItems(ctx context.Context, typeID uint, page, pageSize int) ([]*bizdict.DictItem, interface{}, error) {
	return s.uc.ListItems(ctx, typeID, page, pageSize)
}

// ItemsByCode 消费入口（登录即可用，无管理权限要求）
func (s *Service) ItemsByCode(ctx context.Context, code string) ([]*bizdict.DictItem, error) {
	return s.uc.ItemsByCode(ctx, code)
}

func statusOf(s *int) bizdict.Status {
	if s != nil && *s == 0 {
		return bizdict.StatusDisabled
	}
	return bizdict.StatusEnabled
}
