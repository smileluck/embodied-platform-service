// Package dashboard 仪表盘应用服务
package dashboard

import (
	"context"

	bizdash "github.com/smilex/smilex-admin-gin/internal/biz/dashboard"
)

type Service struct {
	uc *bizdash.Usecase
}

func NewService(uc *bizdash.Usecase) *Service { return &Service{uc: uc} }

// Stats 首页聚合数据
func (s *Service) Stats(ctx context.Context) (*bizdash.Stats, error) {
	return s.uc.Stats(ctx)
}
