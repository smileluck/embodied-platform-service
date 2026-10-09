// 租户角色本地用例（2026-10-09 自平台下沉）：角色 CRUD + 用户角色绑定 + 权限码目录下发。
// 权限码唯一真源 = 本包 permcatalog.go 注册表（不再向平台同步）；角色归属租户经本地
// 租户投影校验（tenants.platform_id，tenant_reconcile 回流平台真相）。
package tenantuser

import (
	"context"
	"time"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// TenantChecker 本地租户投影存在性检查最小接口（*biztenant.Usecase 满足，依赖倒置）
type TenantChecker interface {
	GetByPlatformID(ctx context.Context, platformID uint) (*biztenant.Tenant, error)
}

// RoleUsecase 租户角色领域用例（本地 RBAC）
type RoleUsecase struct {
	roles   RoleRepo
	tenants TenantChecker
}

func NewRoleUsecase(roles RoleRepo, tenants TenantChecker) *RoleUsecase {
	return &RoleUsecase{roles: roles, tenants: tenants}
}

// TenantRoleView 租户角色视图（管理台/门户共用；TenantID 为平台租户 ID）
type TenantRoleView struct {
	ID        uint
	TenantID  uint
	Name      string
	Code      string
	Remark    string
	PermCodes []string
	CreatedAt string
	UpdatedAt string
}

// RoleListParams 租户角色列表查询（TenantID 为平台租户 ID，可选）
type RoleListParams struct {
	Keyword  string
	TenantID *uint
}

// RoleCreateParams 创建租户角色入参
type RoleCreateParams struct {
	TenantID  uint
	Name      string
	Code      string
	Remark    string
	PermCodes []string
}

// RoleUpdateParams 更新入参（指针可选，nil=不修改；code 不可改）
type RoleUpdateParams struct {
	Name      *string
	Remark    *string
	PermCodes *[]string
}

func (uc *RoleUsecase) toView(r *TenantRole) *TenantRoleView {
	vo := &TenantRoleView{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Code: r.Code, Remark: r.Remark,
		PermCodes: r.PermCodes,
		CreatedAt: formatTime(r.CreatedAt), UpdatedAt: formatTime(r.UpdatedAt),
	}
	if vo.PermCodes == nil {
		vo.PermCodes = []string{}
	}
	return vo
}

// checkRolesInTenant 角色归属校验：全部角色必须存在且属于该租户（防跨租户引用提权）
func (uc *RoleUsecase) checkRolesInTenant(ctx context.Context, tenantID uint, roleIDs []uint) error {
	if len(roleIDs) == 0 {
		return nil
	}
	byID := make(map[uint]*TenantRole, len(roleIDs))
	for _, id := range roleIDs {
		r, err := uc.roles.Get(ctx, id)
		if err != nil {
			return err
		}
		byID[id] = r
	}
	for _, id := range roleIDs {
		if r := byID[id]; r.TenantID != tenantID {
			return ErrTenantRoleNotFound
		}
	}
	return nil
}

// ListRoles 角色列表（pageSize<=0 表示全量；本地库直查）
func (uc *RoleUsecase) ListRoles(ctx context.Context, q RoleListParams, page, pageSize int) ([]*TenantRoleView, pagination.Page, error) {
	list, total, err := uc.roles.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	out := make([]*TenantRoleView, 0, len(list))
	for _, r := range list {
		out = append(out, uc.toView(r))
	}
	return out, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// CreateRole 创建角色：归属租户须在本地投影内；perm_codes 须全部在本地注册表目录内
// （NormalizePerms 校验归一化）；(tenant_id, code) 唯一冲突映射 ErrDuplicateRoleCode
func (uc *RoleUsecase) CreateRole(ctx context.Context, p RoleCreateParams) (*TenantRoleView, error) {
	if _, err := uc.tenants.GetByPlatformID(ctx, p.TenantID); err != nil {
		return nil, err
	}
	perms, err := NormalizePerms(p.PermCodes)
	if err != nil {
		return nil, err
	}
	r := &TenantRole{TenantID: p.TenantID, Name: p.Name, Code: p.Code, Remark: p.Remark, PermCodes: perms}
	if err := uc.roles.Create(ctx, r); err != nil {
		return nil, err
	}
	return uc.toView(r), nil
}

// UpdateRole 更新角色（code/tenant_id 创建后不可改）。字段可选：nil 表示不修改——
// 只改名称/备注的请求不会清空权限点
func (uc *RoleUsecase) UpdateRole(ctx context.Context, id uint, p RoleUpdateParams) error {
	r, err := uc.roles.Get(ctx, id)
	if err != nil {
		return err
	}
	if p.Name != nil {
		r.Name = *p.Name
	}
	if p.Remark != nil {
		r.Remark = *p.Remark
	}
	if p.PermCodes != nil {
		perms, err := NormalizePerms(*p.PermCodes)
		if err != nil {
			return err
		}
		r.PermCodes = perms
	}
	return uc.roles.Update(ctx, r)
}

// SetRolePerms 权限点全量替换（独立入口便于管理台按钮权限码粒度控制）
func (uc *RoleUsecase) SetRolePerms(ctx context.Context, id uint, permCodes []string) error {
	r, err := uc.roles.Get(ctx, id)
	if err != nil {
		return err
	}
	perms, err := NormalizePerms(permCodes)
	if err != nil {
		return err
	}
	r.PermCodes = perms
	return uc.roles.Update(ctx, r)
}

// DeleteRole 删除角色（仍有用户绑定时仓储返回 ErrRoleInUse）
func (uc *RoleUsecase) DeleteRole(ctx context.Context, id uint) error {
	return uc.roles.Delete(ctx, id)
}

// SetUserRoles 全量替换用户角色绑定（userID 为平台 tenant_user ID，tenantID 须与
// 用户归属一致——由调用方从平台账号确定；角色必须属于该租户）
func (uc *RoleUsecase) SetUserRoles(ctx context.Context, tenantID, userID uint, roleIDs []uint) error {
	if err := uc.checkRolesInTenant(ctx, tenantID, roleIDs); err != nil {
		return err
	}
	return uc.roles.ReplaceUserRoles(ctx, userID, roleIDs)
}

// UserRoleIDs 批量取用户角色绑定（管理台列表补齐 role_ids 用）
func (uc *RoleUsecase) UserRoleIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error) {
	return uc.roles.UserRoleIDs(ctx, userIDs)
}

// PermCatalog 本地权限码目录（角色配权 UI 数据源；permcatalog.go 唯一真源副本）
func (uc *RoleUsecase) PermCatalog() []*PermDefView {
	out := make([]*PermDefView, 0, len(Catalog))
	for _, d := range Catalog {
		out = append(out, &PermDefView{Code: d.Code, Group: d.Group})
	}
	return out
}

// formatTime 视图时间格式（与管理台口径一致）
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
