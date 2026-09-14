// Package file 文件管理应用服务
package file

import (
	"context"
	"io"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
)

type Service struct {
	uc *bizfile.Usecase
}

func NewService(uc *bizfile.Usecase) *Service { return &Service{uc: uc} }

func (s *Service) Upload(ctx context.Context, name string, r io.Reader, size int64, uploaderID uint, uploaderName string) (*bizfile.File, error) {
	return s.uc.Upload(ctx, name, r, size, uploaderID, uploaderName)
}

func (s *Service) ResolveDownload(ctx context.Context, id uint) (*bizfile.Download, error) {
	return s.uc.ResolveDownload(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }

func (s *Service) Get(ctx context.Context, id uint) (*bizfile.File, error) { return s.uc.Get(ctx, id) }

func (s *Service) List(ctx context.Context, q bizfile.Query, page, pageSize int) ([]*bizfile.File, interface{}, error) {
	return s.uc.List(ctx, q, page, pageSize)
}
