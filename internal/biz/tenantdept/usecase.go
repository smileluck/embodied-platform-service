// 租户部门本地用例（2026-10-10）：部门树 CRUD（防环、删除守卫）+ 用户多部门归属绑定。
// 部门归属租户经本地租户投影校验（tenants.platform_id，对齐 tenantuser.RoleUsecase 模式）。
package tenantdept

import (
	"context"
	"time"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
)

// TenantChecker 本地租户投影存在性检查最小接口（*biztenant.Usecase 满足，依赖倒置；
// 与 tenantuser.TenantChecker 同构，避免跨上下文直接依赖）
type TenantChecker interface {
	GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error)
}

// Usecase 租户部门领域用例（本地组织架构）
type Usecase struct {
	depts   DeptRepo
	tenants TenantChecker
}

func NewUsecase(depts DeptRepo, tenants TenantChecker) *Usecase {
	return &Usecase{depts: depts, tenants: tenants}
}

// DeptCreateParams 创建部门入参（ParentID=0 为根；code 同租户内唯一）
type DeptCreateParams struct {
	TenantID uint
	ParentID uint
	Name     string
	Code     string
	Sort     int
	Remark   string
}

// DeptUpdateParams 更新入参（指针可选，nil=不修改；tenant_id 创建后不可改）
type DeptUpdateParams struct {
	ParentID *uint
	Name     *string
	Sort     *int
	Remark   *string
}

func toView(d *TenantDept, memberCount int64) *DeptView {
	return &DeptView{
		ID: d.ID, TenantID: d.TenantID, ParentID: d.ParentID, Name: d.Name, Code: d.Code,
		Sort: d.Sort, Remark: d.Remark, MemberCount: memberCount,
		CreatedAt: formatTime(d.CreatedAt), UpdatedAt: formatTime(d.UpdatedAt),
	}
}

// formatTime 视图时间格式（与管理台口径一致）
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// ListDepts 部门平表全量（含直属成员数；树形组装在前端按 parent_id 完成）
func (uc *Usecase) ListDepts(ctx context.Context, tenantID uint) ([]*DeptView, error) {
	list, err := uc.depts.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	counts, err := uc.depts.MemberCounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]*DeptView, 0, len(list))
	for _, d := range list {
		out = append(out, toView(d, counts[d.ID]))
	}
	return out, nil
}

// checkParent 父部门校验：存在、同租户（防跨租户挂载提权）
func (uc *Usecase) checkParent(ctx context.Context, tenantID, parentID uint) error {
	parent, err := uc.depts.Get(ctx, parentID)
	if err != nil {
		return ErrParentInvalid
	}
	if parent.TenantID != tenantID {
		return ErrParentInvalid
	}
	return nil
}

// checkDeptsInTenant 部门归属校验：全部部门必须存在且属于该租户（防跨租户引用）
func (uc *Usecase) checkDeptsInTenant(ctx context.Context, tenantID uint, deptIDs []uint) error {
	if len(deptIDs) == 0 {
		return nil
	}
	for _, id := range deptIDs {
		d, err := uc.depts.Get(ctx, id)
		if err != nil {
			return err
		}
		if d.TenantID != tenantID {
			return ErrDeptNotFound
		}
	}
	return nil
}

// CreateDept 创建部门：归属租户须在本地投影内；父部门（非根时）须存在且同租户
func (uc *Usecase) CreateDept(ctx context.Context, p DeptCreateParams) (*DeptView, error) {
	if _, err := uc.tenants.GetByPlatformID(ctx, p.TenantID); err != nil {
		return nil, err
	}
	if p.ParentID != 0 {
		if err := uc.checkParent(ctx, p.TenantID, p.ParentID); err != nil {
			return nil, err
		}
	}
	d := &TenantDept{TenantID: p.TenantID, ParentID: p.ParentID, Name: p.Name, Code: p.Code, Sort: p.Sort, Remark: p.Remark}
	if err := uc.depts.Create(ctx, d); err != nil {
		return nil, err
	}
	return toView(d, 0), nil
}

// UpdateDept 更新部门（tenant_id/code 创建后不可改）。父部门变更时防环：
// 新父不得为自身，且其祖先链不得经过自身（否则部门子树会被挂到自己下面成环）
func (uc *Usecase) UpdateDept(ctx context.Context, id uint, p DeptUpdateParams) error {
	d, err := uc.depts.Get(ctx, id)
	if err != nil {
		return err
	}
	if p.ParentID != nil && *p.ParentID != d.ParentID {
		if *p.ParentID == id {
			return ErrParentInvalid
		}
		if *p.ParentID != 0 {
			if err := uc.checkParent(ctx, d.TenantID, *p.ParentID); err != nil {
				return err
			}
			// 沿新父祖先链上溯（全量拉平后内存遍历，单租户部门量级小）
			all, err := uc.depts.List(ctx, d.TenantID)
			if err != nil {
				return err
			}
			byID := make(map[uint]*TenantDept, len(all))
			for _, n := range all {
				byID[n.ID] = n
			}
			for cur := byID[*p.ParentID]; cur != nil; cur = byID[cur.ParentID] {
				if cur.ID == id {
					return ErrParentInvalid
				}
			}
		}
		d.ParentID = *p.ParentID
	}
	if p.Name != nil {
		d.Name = *p.Name
	}
	if p.Sort != nil {
		d.Sort = *p.Sort
	}
	if p.Remark != nil {
		d.Remark = *p.Remark
	}
	return uc.depts.Update(ctx, d)
}

// DeleteDept 删除部门：存在子部门时拒绝（ErrDeptHasChildren）；直属成员绑定事务内级联解除
func (uc *Usecase) DeleteDept(ctx context.Context, id uint) error {
	if _, err := uc.depts.Get(ctx, id); err != nil {
		return err
	}
	cnt, err := uc.depts.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return ErrDeptHasChildren
	}
	return uc.depts.Delete(ctx, id)
}

// SetUserDepts 全量替换用户部门绑定（多部门；userID 为平台 tenant_user ID，tenantID
// 须与用户归属一致——由调用方从平台账号确定；部门必须属于该租户）
func (uc *Usecase) SetUserDepts(ctx context.Context, tenantID, userID uint, deptIDs []uint) error {
	if err := uc.checkDeptsInTenant(ctx, tenantID, deptIDs); err != nil {
		return err
	}
	return uc.depts.ReplaceUserDepts(ctx, userID, deptIDs)
}

// UserDeptIDs 批量取用户部门绑定（管理台列表补齐 dept_ids 用）
func (uc *Usecase) UserDeptIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error) {
	return uc.depts.UserDeptIDs(ctx, userIDs)
}

// UserIDsUnderDept 取部门（含全部后代）成员用户 ID 集合：全量拉平内存展开后代 +
// 仓储按部门集合查绑定。返回部门归属租户 ID（调用方据此向平台拉取用户）
func (uc *Usecase) UserIDsUnderDept(ctx context.Context, deptID uint) ([]uint, uint, error) {
	dept, err := uc.depts.Get(ctx, deptID)
	if err != nil {
		return nil, 0, err
	}
	all, err := uc.depts.List(ctx, dept.TenantID)
	if err != nil {
		return nil, 0, err
	}
	children := make(map[uint][]uint, len(all))
	for _, n := range all {
		children[n.ParentID] = append(children[n.ParentID], n.ID)
	}
	ids := []uint{deptID}
	for i := 0; i < len(ids); i++ {
		ids = append(ids, children[ids[i]]...)
	}
	userIDs, err := uc.depts.UserIDsByDepts(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	return userIDs, dept.TenantID, nil
}

// DeptNamesByIDs 批量取部门名（用户列表回填部门名用）
func (uc *Usecase) DeptNamesByIDs(ctx context.Context, deptIDs []uint) (map[uint]string, error) {
	return uc.depts.DeptNamesByIDs(ctx, deptIDs)
}
