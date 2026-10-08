// Package tenantmember 租户端成员应用服务（/app-api/v1 租户面）。
// 当前租户上下文（X-Tenant-ID 解析的平台租户 ID）与操作者（app_user ID）
// 由 handler 从认证上下文注入，DTO 不接受客户端指定。
package tenantmember

import (
	"context"

	biztenantmember "github.com/smilex/smilex-admin-gin/internal/biz/tenantmember"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

type Service struct {
	uc *biztenantmember.Usecase
}

func NewService(uc *biztenantmember.Usecase) *Service { return &Service{uc: uc} }

// CreateRequest 新增成员入参（角色缺省 member）
type CreateRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=20"`
	Nickname string `json:"nickname" binding:"max=64"`
	Phone    string `json:"phone" binding:"max=32"`
	Email    string `json:"email" binding:"omitempty,max=128,email"`
	Role     string `json:"role" binding:"omitempty,oneof=tenant_admin member"`
}

// UpdateRequest 成员资料更新入参（不含 tenant_ids：跨租户归属是商户管理端职责）
type UpdateRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,max=64"`
	Phone    *string `json:"phone" binding:"omitempty,max=32"`
	Email    *string `json:"email" binding:"omitempty,max=128,email"`
}

// SetStatusRequest 启停入参
type SetStatusRequest struct {
	Status int `json:"status" binding:"required,oneof=0 1"`
}

// ResetPasswordRequest 重置密码入参（旧密码立即失效）
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=20"`
}

// SetRoleRequest 角色设置入参
type SetRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=tenant_admin member"`
}

// VO 成员视图（id 为平台 app_user ID，role 为本租户内角色）
type VO struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    int    `json:"status"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toVO(m *biztenantmember.Member) *VO {
	return &VO{
		ID: m.AppUserID, Username: m.Username, Nickname: m.Nickname,
		Phone: m.Phone, Email: m.Email, Status: m.Status,
		Role: string(biztenantmember.NormalizeRole(m.Role)),
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// List 本租户成员列表
func (s *Service) List(ctx context.Context, tenantPlatformID uint, kw string, page, pageSize int) ([]*VO, pagination.Page, error) {
	list, pg, err := s.uc.ListMembers(ctx, tenantPlatformID, biztenantmember.ListQuery{Keyword: kw}, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	out := make([]*VO, 0, len(list))
	for _, m := range list {
		out = append(out, toVO(m))
	}
	return out, pg, nil
}

// Create 在本租户新增成员
func (s *Service) Create(ctx context.Context, tenantPlatformID uint, req CreateRequest) (*VO, error) {
	m, err := s.uc.CreateMember(ctx, tenantPlatformID, biztenantmember.CreateParams{
		Username: req.Username, Password: req.Password, Nickname: req.Nickname,
		Phone: req.Phone, Email: req.Email, Role: biztenantmember.Role(req.Role),
	})
	if err != nil {
		return nil, err
	}
	return toVO(m), nil
}

// Update 更新成员资料
func (s *Service) Update(ctx context.Context, tenantPlatformID, appUserID uint, req UpdateRequest) error {
	return s.uc.UpdateMember(ctx, tenantPlatformID, appUserID, biztenantmember.UpdateParams{
		Nickname: req.Nickname, Phone: req.Phone, Email: req.Email,
	})
}

// SetStatus 启停成员（actor 为操作者 app_user ID，守卫在 biz 层）
func (s *Service) SetStatus(ctx context.Context, tenantPlatformID, appUserID, actorID uint, status int) error {
	return s.uc.SetMemberStatus(ctx, tenantPlatformID, appUserID, actorID, status)
}

// ResetPassword 重置成员密码
func (s *Service) ResetPassword(ctx context.Context, tenantPlatformID, appUserID uint, password string) error {
	return s.uc.ResetMemberPassword(ctx, tenantPlatformID, appUserID, password)
}

// Remove 移除成员出本租户（差集更新 tenant_ids，不删账号）
func (s *Service) Remove(ctx context.Context, tenantPlatformID, appUserID, actorID uint) error {
	return s.uc.RemoveMember(ctx, tenantPlatformID, appUserID, actorID)
}

// SetRole 设置成员角色
func (s *Service) SetRole(ctx context.Context, tenantPlatformID, appUserID, actorID uint, role string) error {
	return s.uc.SetMemberRole(ctx, tenantPlatformID, appUserID, actorID, biztenantmember.Role(role))
}
