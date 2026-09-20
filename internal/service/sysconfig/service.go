// Package sysconfig 系统参数应用服务
package sysconfig

import (
	"context"

	bizsys "github.com/smilex/smilex-admin-gin/internal/biz/sysconfig"
)

type Service struct {
	uc *bizsys.Usecase
}

func NewService(uc *bizsys.Usecase) *Service { return &Service{uc: uc} }

// CreateRequest 新增参数
type CreateRequest struct {
	Key         string `json:"key" binding:"required,max=64"`
	Value       string `json:"value" binding:"required,max=512"`
	Type        string `json:"type" binding:"required,oneof=string number bool"`
	Description string `json:"description" binding:"max=200"`
}

// UpdateRequest 修改参数值（键与类型不可变）
type UpdateRequest struct {
	Value string `json:"value" binding:"required,max=512"`
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*bizsys.Config, error) {
	return s.uc.Create(ctx, &bizsys.Config{
		Key: req.Key, Value: req.Value, Type: bizsys.ValueType(req.Type), Description: req.Description,
	})
}

func (s *Service) Update(ctx context.Context, key string, req UpdateRequest) (*bizsys.Config, error) {
	return s.uc.Update(ctx, key, req.Value)
}

func (s *Service) Delete(ctx context.Context, key string) error { return s.uc.Delete(ctx, key) }
func (s *Service) List(ctx context.Context, keyword string) ([]*bizsys.Config, error) {
	return s.uc.List(ctx, keyword)
}

// EnsureBuiltin 启动播种内置参数（幂等）
func (s *Service) EnsureBuiltin() error { return s.uc.EnsureBuiltin(context.Background()) }

// IntDefault 整型参数读取（传输层分页上限等消费点）
func (s *Service) IntDefault(ctx context.Context, key string, def int) int {
	return s.uc.GetIntDefault(ctx, key, def)
}
