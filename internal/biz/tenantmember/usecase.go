package tenantmember

import (
	"context"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 租户成员用例：全部操作强制限定在当前租户域内（tenantPlatformID
// 来自 AppAuth 闸门解析的 X-Tenant-ID，不信任调用方传参），成员资料实时
// 读写平台开放面，本地只维护角色绑定。
type Usecase struct {
	repo Repo
	gw   bizappuser.Gateway // 平台开放面应用用户网关（唯一事实源）
}

func NewUsecase(repo Repo, gw bizappuser.Gateway) *Usecase {
	return &Usecase{repo: repo, gw: gw}
}

// RoleOf 查成员在租户内的角色（无绑定归一为 member；profile/TenantAdmin 中间件用）
func (uc *Usecase) RoleOf(ctx context.Context, appUserID, tenantPlatformID uint) (Role, error) {
	r, err := uc.repo.RoleOf(ctx, appUserID, tenantPlatformID)
	if err != nil {
		return RoleMember, err
	}
	return NormalizeRole(r), nil
}

// RolesOf 批量查成员角色（管理面列表标注用；无绑定者不在返回 map 中）
func (uc *Usecase) RolesOf(ctx context.Context, appUserIDs []uint, tenantPlatformID uint) (map[uint]Role, error) {
	return uc.repo.RolesOf(ctx, appUserIDs, tenantPlatformID)
}

// ListMembers 本租户成员列表（开放面按租户过滤 + 本地角色批量合并）
func (uc *Usecase) ListMembers(ctx context.Context, tenantPlatformID uint, q ListQuery, page, pageSize int) ([]*Member, pagination.Page, error) {
	views, pg, err := uc.gw.List(ctx, bizappuser.ListParams{
		Keyword: q.Keyword, TenantID: &tenantPlatformID,
	}, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	ids := make([]uint, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	roleMap := map[uint]Role{}
	if len(ids) > 0 {
		if roleMap, err = uc.repo.RolesOf(ctx, ids, tenantPlatformID); err != nil {
			return nil, pagination.Page{}, err
		}
	}
	out := make([]*Member, 0, len(views))
	for _, v := range views {
		out = append(out, &Member{
			AppUserID: v.ID, Username: v.Username, Nickname: v.Nickname,
			Phone: v.Phone, Email: v.Email, Status: v.Status,
			Role: NormalizeRole(roleMap[v.ID]),
			CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
		})
	}
	return out, pg, nil
}

// CreateMember 在本租户新增成员（tenant_ids 固定为当前租户；平台建号成功后落角色绑定）
func (uc *Usecase) CreateMember(ctx context.Context, tenantPlatformID uint, p CreateParams) (*Member, error) {
	role := NormalizeRole(p.Role)
	v, err := uc.gw.Create(ctx, bizappuser.CreateParams{
		Username: p.Username, Password: p.Password, Nickname: p.Nickname,
		Phone: p.Phone, Email: p.Email,
		TenantIDs: []uint{tenantPlatformID},
	})
	if err != nil {
		return nil, err
	}
	if err := uc.repo.SetRole(ctx, v.ID, tenantPlatformID, role); err != nil {
		return nil, err
	}
	return &Member{
		AppUserID: v.ID, Username: v.Username, Nickname: v.Nickname,
		Phone: v.Phone, Email: v.Email, Status: v.Status, Role: role,
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}, nil
}

// locateMember 定位本租户内的成员（开放面无单查端点：按租户过滤分页扫描；
// 租户成员量级小，首页未命中继续翻页直至尽头）
func (uc *Usecase) locateMember(ctx context.Context, tenantPlatformID, appUserID uint) (*bizappuser.AppUserView, error) {
	const size = 100
	for page := 1; ; page++ {
		views, pg, err := uc.gw.List(ctx, bizappuser.ListParams{TenantID: &tenantPlatformID}, page, size)
		if err != nil {
			return nil, err
		}
		for _, v := range views {
			if v.ID == appUserID {
				return v, nil
			}
		}
		if int64(page*size) >= pg.Total {
			return nil, ErrMemberNotInTenant
		}
	}
}

// UpdateMember 更新成员资料（前置归属校验；不含跨租户归属变更）
func (uc *Usecase) UpdateMember(ctx context.Context, tenantPlatformID, appUserID uint, p UpdateParams) error {
	if _, err := uc.locateMember(ctx, tenantPlatformID, appUserID); err != nil {
		return err
	}
	return uc.gw.Update(ctx, appUserID, bizappuser.UpdateParams{
		Nickname: p.Nickname, Phone: p.Phone, Email: p.Email,
	})
}

// SetMemberStatus 启停成员（平台账号级生效：禁用后登录/token 即时失败；
// 最后管理员与本人不可禁用）
func (uc *Usecase) SetMemberStatus(ctx context.Context, tenantPlatformID, appUserID, actorID uint, status int) error {
	if appUserID == actorID {
		return ErrCannotModifySelf
	}
	if err := uc.guardNotLastAdmin(ctx, tenantPlatformID, appUserID, status == 0); err != nil {
		return err
	}
	if _, err := uc.locateMember(ctx, tenantPlatformID, appUserID); err != nil {
		return err
	}
	return uc.gw.SetStatus(ctx, appUserID, status)
}

// ResetMemberPassword 重置成员密码（旧密码立即失效；前置归属校验）
func (uc *Usecase) ResetMemberPassword(ctx context.Context, tenantPlatformID, appUserID uint, password string) error {
	if _, err := uc.locateMember(ctx, tenantPlatformID, appUserID); err != nil {
		return err
	}
	return uc.gw.ResetPassword(ctx, appUserID, password)
}

// RemoveMember 移除成员出本租户：Update tenant_ids 差集更新（保留本商户其他
// 租户归属与他商户归属），不是删除账号；同步解除本地角色绑定
func (uc *Usecase) RemoveMember(ctx context.Context, tenantPlatformID, appUserID, actorID uint) error {
	if appUserID == actorID {
		return ErrCannotModifySelf
	}
	if err := uc.guardNotLastAdmin(ctx, tenantPlatformID, appUserID, true); err != nil {
		return err
	}
	v, err := uc.locateMember(ctx, tenantPlatformID, appUserID)
	if err != nil {
		return err
	}
	remain := make([]uint, 0, len(v.TenantIDs))
	for _, id := range v.TenantIDs {
		if id != tenantPlatformID {
			remain = append(remain, id)
		}
	}
	if err := uc.gw.Update(ctx, appUserID, bizappuser.UpdateParams{TenantIDs: &remain}); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, appUserID, tenantPlatformID)
}

// SetMemberRole 设置成员在本租户的角色（最后管理员不可降级、本人不可变更；
// 非法角色值归一为 member，不落到绑定表）
func (uc *Usecase) SetMemberRole(ctx context.Context, tenantPlatformID, appUserID, actorID uint, role Role) error {
	role = NormalizeRole(role)
	if appUserID == actorID {
		return ErrCannotModifySelf
	}
	if err := uc.guardNotLastAdmin(ctx, tenantPlatformID, appUserID, role != RoleTenantAdmin); err != nil {
		return err
	}
	if _, err := uc.locateMember(ctx, tenantPlatformID, appUserID); err != nil {
		return err
	}
	return uc.repo.SetRole(ctx, appUserID, tenantPlatformID, role)
}

// guardNotLastAdmin 最后管理员守卫：目标为本租户管理员且管理员仅剩其一人时，
// 移除/降级/禁用类操作（affecting=true）拒绝
func (uc *Usecase) guardNotLastAdmin(ctx context.Context, tenantPlatformID, appUserID uint, affecting bool) error {
	if !affecting {
		return nil
	}
	role, err := uc.repo.RoleOf(ctx, appUserID, tenantPlatformID)
	if err != nil {
		return err
	}
	if role != RoleTenantAdmin {
		return nil
	}
	admins, err := uc.repo.TenantAdminIDs(ctx, tenantPlatformID)
	if err != nil {
		return err
	}
	if len(admins) <= 1 {
		return ErrLastTenantAdmin
	}
	return nil
}

// PurgeTenant 清空租户全部绑定（租户停用/软删后的卫生清理；闸门已挡流量，
// 绑定清除只是收敛数据面）
func (uc *Usecase) PurgeTenant(ctx context.Context, tenantPlatformID uint) error {
	return uc.repo.DeleteByTenant(ctx, tenantPlatformID)
}
