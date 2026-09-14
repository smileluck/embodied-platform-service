// Package permission 权限应用服务
package permission

import (
	"context"

	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
)

type Service struct {
	uc *bizperm.Usecase
}

func NewService(uc *bizperm.Usecase) *Service { return &Service{uc: uc} }

type CreateRequest struct {
	Name     string       `json:"name" binding:"required,max=20"`
	Code     string       `json:"code" binding:"required,max=64"`
	Type     bizperm.Type `json:"type" binding:"required,oneof=dir menu button"`
	Method   string       `json:"method" binding:"omitempty,max=16"`
	Path     string       `json:"path" binding:"omitempty,max=255"`
	ParentID uint         `json:"parent_id"`
	Icon     string       `json:"icon" binding:"omitempty,max=512"`
	Sort     int          `json:"sort"`
}

type UpdateRequest struct {
	Name     string  `json:"name" binding:"omitempty,max=20"` // 空=保持原名
	Method   string  `json:"method" binding:"omitempty,max=16"`
	Path     string  `json:"path" binding:"omitempty,max=255"`
	Icon     *string `json:"icon"`      // 指针：区分未传与清空
	Sort     *int    `json:"sort"`      // 指针：区分未传与归零
	ParentID *uint   `json:"parent_id"` // 指针：区分未传与挪到顶级(0)
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*bizperm.Permission, error) {
	return s.uc.Create(ctx, req.Name, req.Code, req.Type, req.Method, req.Path, req.ParentID, req.Icon, req.Sort)
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	sort := 0
	if req.Sort != nil {
		sort = *req.Sort
	}
	return s.uc.Update(ctx, id, req.Name, req.Method, req.Path, req.Icon, sort, req.ParentID)
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }

func (s *Service) Get(ctx context.Context, id uint) (*bizperm.Permission, error) { return s.uc.Get(ctx, id) }

func (s *Service) List(ctx context.Context, q bizperm.Query, page, pageSize int) ([]*bizperm.Permission, interface{}, error) {
	return s.uc.List(ctx, q, page, pageSize)
}

// UserMenuTree 当前用户可见菜单树
func (s *Service) UserMenuTree(ctx context.Context, userID uint) ([]*bizperm.MenuNode, error) {
	return s.uc.UserMenuTree(ctx, userID)
}

// SearchUserMenus 当前用户可见菜单关键词搜索（顶栏命令面板用，上限 10 条）
func (s *Service) SearchUserMenus(ctx context.Context, userID uint, kw string) ([]*bizperm.MenuHit, error) {
	return s.uc.SearchUserMenus(ctx, userID, kw, 10)
}
