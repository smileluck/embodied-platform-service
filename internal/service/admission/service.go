// Package admission 准入管理应用服务。
// 2026-09-17 起成员来源=平台开放面（本商户绑定成员）：列表实时拉平台、
// 新增/删除推送平台（新增=平台无此账号则创建有则绑定；删除=仅解绑），
// 本地只维护准入投影（开关/角色）。
package admission

import (
	"context"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

type Service struct {
	uc *bizadmission.Usecase
}

func NewService(uc *bizadmission.Usecase) *Service { return &Service{uc: uc} }

// ListRequest 成员列表查询（kw 在平台侧过滤用户名/昵称）
type ListRequest struct {
	Keyword string `form:"kw"`
}

// List 成员列表（平台绑定成员 + 本地准入投影合并；kw/分页在平台侧）
func (s *Service) List(ctx context.Context, req ListRequest, page, pageSize int) ([]*bizadmission.MemberRow, pagination.Page, error) {
	return s.uc.ListFromPlatform(ctx, req.Keyword, page, pageSize)
}

// Get 单条投影（按平台用户 ID；未准入 404）
func (s *Service) Get(ctx context.Context, platformUserID uint) (*bizadmission.Projection, error) {
	return s.uc.GetByPlatformUser(ctx, platformUserID)
}

// AddRequest 新增成员：推送平台（无此账号则创建并绑定，有则仅绑定）并建本地准入投影。
// Password 仅平台不存在该用户名时必填（作为平台账号初始密码）。
type AddRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Nickname string `json:"nickname" binding:"max=64"`
	Password string `json:"password" binding:"max=64"`
	// Enabled 准入开关（缺省 true：添加即准入）
	Enabled *bool  `json:"enabled"`
	RoleIDs []uint `json:"role_ids"`
}

// Add 新增成员；返回（投影, 平台侧账号是否已存在仅绑定）
func (s *Service) Add(ctx context.Context, req AddRequest) (*bizadmission.Projection, bool, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return s.uc.AddMember(ctx, req.Username, req.Nickname, req.Password, enabled, req.RoleIDs)
}

// SetEnabledRequest 准入开关
type SetEnabledRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// SetEnabled 开/关准入（关闭即同步吊销该用户对本系统的访问授权）
func (s *Service) SetEnabled(ctx context.Context, platformUserID uint, enabled bool) error {
	return s.uc.SetEnabledByPlatformUser(ctx, platformUserID, enabled)
}

// SetRolesRequest 角色绑定
type SetRolesRequest struct {
	RoleIDs []uint `json:"role_ids"`
}

// SetRoles 调整本地角色
func (s *Service) SetRoles(ctx context.Context, platformUserID uint, roleIDs []uint) error {
	return s.uc.SetRolesByPlatformUser(ctx, platformUserID, roleIDs)
}

// Delete 移除成员：解除平台侧绑定（账号本体保留）并删除本地投影
func (s *Service) Delete(ctx context.Context, platformUserID uint) error {
	return s.uc.DeleteMember(ctx, platformUserID)
}

// SyncFromPlatform 从平台拉取本商户绑定成员：补建缺失投影（成员语义=准入开启）、刷新快照
func (s *Service) SyncFromPlatform(ctx context.Context) (created, refreshed int, err error) {
	return s.uc.SyncFromPlatform(ctx)
}
