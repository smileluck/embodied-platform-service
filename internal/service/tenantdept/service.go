// Package tenantdept 租户部门应用服务（管理面）：部门树 CRUD + 用户多部门归属绑定。
// 部门为本地域（2026-10-10 租户内部数据隔离基础能力，对齐 tenant_roles 下沉先例）；
// tenant_id 全链路为平台租户 ID（部门页自带租户下拉，视图不回填 tenant_name）。
package tenantdept

import (
	"context"

	biztenantdept "github.com/smilex/smilex-admin-gin/internal/biz/tenantdept"
)

type Service struct {
	depts *biztenantdept.Usecase
}

func NewService(depts *biztenantdept.Usecase) *Service {
	return &Service{depts: depts}
}

// ---- 部门 CRUD ----

// DeptCreateRequest 创建租户部门入参（parent_id=0 为根；code 同租户内唯一）
type DeptCreateRequest struct {
	TenantID uint   `json:"tenant_id" binding:"required"`
	ParentID uint   `json:"parent_id"`
	Name     string `json:"name" binding:"required,max=64"`
	Code     string `json:"code" binding:"required,min=2,max=64"`
	Sort     int    `json:"sort"`
	Remark   string `json:"remark" binding:"max=255"`
}

// DeptUpdateRequest 更新入参（指针可选，省略即不修改；tenant_id/code 不可改）
type DeptUpdateRequest struct {
	ParentID *uint   `json:"parent_id"`
	Name     *string `json:"name" binding:"omitempty,max=64"`
	Sort     *int    `json:"sort"`
	Remark   *string `json:"remark" binding:"omitempty,max=255"`
}

// SetUserDeptsRequest 用户部门全量替换入参（多部门；部门须与用户同租户）
type SetUserDeptsRequest struct {
	DeptIDs []uint `json:"dept_ids"`
}

func (s *Service) ListDepts(ctx context.Context, tenantID uint) ([]*biztenantdept.DeptView, error) {
	return s.depts.ListDepts(ctx, tenantID)
}

func (s *Service) CreateDept(ctx context.Context, req DeptCreateRequest) (*biztenantdept.DeptView, error) {
	return s.depts.CreateDept(ctx, biztenantdept.DeptCreateParams{
		TenantID: req.TenantID, ParentID: req.ParentID, Name: req.Name,
		Code: req.Code, Sort: req.Sort, Remark: req.Remark,
	})
}

func (s *Service) UpdateDept(ctx context.Context, id uint, req DeptUpdateRequest) error {
	return s.depts.UpdateDept(ctx, id, biztenantdept.DeptUpdateParams{
		ParentID: req.ParentID, Name: req.Name, Sort: req.Sort, Remark: req.Remark,
	})
}

func (s *Service) DeleteDept(ctx context.Context, id uint) error {
	return s.depts.DeleteDept(ctx, id)
}

// SetUserDepts 全量替换本地部门绑定：dept_ids 为 nil 时按空集处理（全清）。
// 用户的租户归属由调用方（tenantuser service 经平台账号）确定
func (s *Service) SetUserDepts(ctx context.Context, tenantID, userID uint, deptIDs []uint) error {
	if deptIDs == nil {
		deptIDs = []uint{}
	}
	return s.depts.SetUserDepts(ctx, tenantID, userID, deptIDs)
}
