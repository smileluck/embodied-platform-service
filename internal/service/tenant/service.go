// Package tenant 租户应用服务
package tenant

import (
	"context"
	"time"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
)

type Service struct {
	uc *biztenant.Usecase
}

func NewService(uc *biztenant.Usecase) *Service { return &Service{uc: uc} }

// CreateRequest 创建租户入参
type CreateRequest struct {
	Name         string `json:"name" binding:"required,max=64"`
	Code         string `json:"code" binding:"required,min=2,max=64"`
	ContactName  string `json:"contact_name" binding:"max=64"`
	ContactPhone string `json:"contact_phone" binding:"max=32"`
	Remark       string `json:"remark" binding:"max=255"`
}

// UpdateRequest 更新租户入参（code 创建后不可改，故不含 code）
type UpdateRequest struct {
	Name         string `json:"name" binding:"required,max=64"`
	ContactName  string `json:"contact_name" binding:"max=64"`
	ContactPhone string `json:"contact_phone" binding:"max=32"`
	Remark       string `json:"remark" binding:"max=255"`
}

// SetStatusRequest 修改租户状态入参（1 启用 0 禁用；指针接收避免 required 拒绝零值）
type SetStatusRequest struct {
	Status *int `json:"status" binding:"required,oneof=0 1"`
}

// VO 租户视图
type VO struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Remark       string `json:"remark"`
	Status       int    `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ToVO 租户实体转视图
func ToVO(t *biztenant.Tenant) *VO {
	return &VO{
		ID: t.ID, Name: t.Name, Code: t.Code,
		ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark, Status: int(t.Status),
		CreatedAt: formatTime(t.CreatedAt), UpdatedAt: formatTime(t.UpdatedAt),
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*VO, error) {
	t, err := s.uc.Create(ctx, req.Name, req.Code, req.ContactName, req.ContactPhone, req.Remark)
	if err != nil {
		return nil, err
	}
	return ToVO(t), nil
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	return s.uc.Update(ctx, id, req.Name, req.ContactName, req.ContactPhone, req.Remark)
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }

func (s *Service) Get(ctx context.Context, id uint) (*VO, error) {
	t, err := s.uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToVO(t), nil
}

func (s *Service) List(ctx context.Context, q biztenant.Query, page, pageSize int) ([]*VO, interface{}, error) {
	tenants, pg, err := s.uc.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*VO, 0, len(tenants))
	for _, t := range tenants {
		out = append(out, ToVO(t))
	}
	return out, pg, nil
}

func (s *Service) SetStatus(ctx context.Context, id uint, req SetStatusRequest) error {
	return s.uc.SetStatus(ctx, id, biztenant.Status(*req.Status))
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
