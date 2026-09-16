// Package admission 准入管理应用服务
package admission

import (
	"context"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
)

type Service struct {
	uc *bizadmission.Usecase
}

func NewService(uc *bizadmission.Usecase) *Service { return &Service{uc: uc} }

// ListRequest 列表查询
type ListRequest struct {
	Username string `form:"username"`
	Status   *int   `form:"status" binding:"omitempty,oneof=0 1"`
}

func (s *Service) List(ctx context.Context, req ListRequest, page, pageSize int) ([]*bizadmission.Projection, interface{}, error) {
	return s.uc.List(ctx, bizadmission.Query{Username: req.Username, Status: req.Status}, page, pageSize)
}

func (s *Service) Get(ctx context.Context, id uint) (*bizadmission.Projection, error) {
	return s.uc.Get(ctx, id)
}

// SetEnabledRequest 准入开关
type SetEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// SetEnabled 开/关准入（关闭即同步吊销该用户对本系统的访问授权）
func (s *Service) SetEnabled(ctx context.Context, id uint, enabled bool) error {
	return s.uc.SetEnabled(ctx, id, enabled)
}

// SetRolesRequest 角色绑定
type SetRolesRequest struct {
	RoleIDs []uint `json:"role_ids"`
}

func (s *Service) SetRoles(ctx context.Context, id uint, roleIDs []uint) error {
	return s.uc.SetRoles(ctx, id, roleIDs)
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.uc.Delete(ctx, id)
}

// SyncFromPlatform 从平台拉取用户列表预建/刷新投影（默认停用，待管理员开启）
func (s *Service) SyncFromPlatform(ctx context.Context) (created, refreshed int, err error) {
	return s.uc.SyncFromPlatform(ctx)
}
