// 平台开放面租户用户/角色网关：包装 platformsdk 调用并映射业务哨兵
// （404 按操作实体区分用户/角色不泄露存在性；409 按平台哨兵文案区分重名/占用——
// 开放面错误信封无机器可读子码，平台侧文案变更需同步此处，与 mapAppUserErr 同模式）。
package platform

import (
	"context"
	"errors"
	"net/http"
	"strings"

	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// TenantUserGateway 平台开放面租户用户/角色网关（wire 绑定 biztenantuser.Gateway）
type TenantUserGateway struct {
	c *platformsdk.Client
}

// NewTenantUserGateway 构造（wire provider；client 来自 NewOpenAPIClient）
func NewTenantUserGateway(c *platformsdk.Client) *TenantUserGateway {
	return &TenantUserGateway{c: c}
}

// mapUserErr 开放面用户操作错误 → 业务哨兵
func mapTenantUserErr(err error) error {
	if err == nil {
		return nil
	}
	var perr *platformsdk.Error
	if !errors.As(err, &perr) {
		return err
	}
	switch perr.HTTPStatus {
	case http.StatusNotFound:
		return biztenantuser.ErrTenantUserNotFound
	case http.StatusForbidden:
		return biztenantuser.ErrTenantNotInScope
	case http.StatusConflict:
		if strings.Contains(perr.Msg, "用户名已存在") {
			return biztenantuser.ErrDuplicateUsername
		}
		return err
	}
	return err
}

// mapRoleErr 开放面角色操作错误 → 业务哨兵
func mapTenantRoleErr(err error) error {
	if err == nil {
		return nil
	}
	var perr *platformsdk.Error
	if !errors.As(err, &perr) {
		return err
	}
	switch perr.HTTPStatus {
	case http.StatusNotFound:
		return biztenantuser.ErrTenantRoleNotFound
	case http.StatusForbidden:
		return biztenantuser.ErrTenantNotInScope
	case http.StatusConflict:
		switch {
		case strings.Contains(perr.Msg, "角色编码已存在"):
			return biztenantuser.ErrDuplicateRoleCode
		case strings.Contains(perr.Msg, "分配"):
			return biztenantuser.ErrRoleInUse
		}
	}
	return err
}

// mapPermErr 权限点校验错误（400 权限点无效）
func mapTenantPermErr(err error) error {
	if err == nil {
		return nil
	}
	var perr *platformsdk.Error
	if errors.As(err, &perr) && perr.HTTPStatus == http.StatusBadRequest &&
		strings.Contains(perr.Msg, "权限点无效") {
		return biztenantuser.ErrInvalidPerm
	}
	return err
}

func toTenantUserView(u *platformsdk.TenantUser) *biztenantuser.TenantUserView {
	if u == nil {
		return nil
	}
	return &biztenantuser.TenantUserView{
		ID: u.ID, TenantID: u.TenantID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status, RoleIDs: u.RoleIDs,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func toTenantRoleView(r *platformsdk.TenantRole) *biztenantuser.TenantRoleView {
	if r == nil {
		return nil
	}
	return &biztenantuser.TenantRoleView{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Code: r.Code, Remark: r.Remark,
		PermCodes: r.PermCodes, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// ---- 租户用户 ----

func (g *TenantUserGateway) ListUsers(ctx context.Context, p biztenantuser.UserListParams, page, pageSize int) ([]*biztenantuser.TenantUserView, pagination.Page, error) {
	f := platformsdk.TenantUserFilter{Keyword: p.Keyword, Phone: p.Phone, Status: p.Status}
	if p.TenantID != nil {
		f.TenantID = *p.TenantID
	}
	list, pg, err := g.c.ListTenantUsers(ctx, page, pageSize, f)
	if err != nil {
		return nil, pagination.Page{}, mapTenantUserErr(err)
	}
	if pg == nil {
		pg = &platformsdk.Page{}
	}
	out := make([]*biztenantuser.TenantUserView, 0, len(list))
	for _, u := range list {
		out = append(out, toTenantUserView(u))
	}
	return out, pagination.Page{Page: pg.Page, PageSize: pg.PageSize, Total: pg.Total}, nil
}

func (g *TenantUserGateway) CreateUser(ctx context.Context, p biztenantuser.UserCreateParams) (*biztenantuser.TenantUserView, error) {
	u, err := g.c.CreateTenantUser(ctx, platformsdk.TenantUserCreateRequest{
		TenantID: p.TenantID, Username: p.Username, Password: p.Password,
		Nickname: p.Nickname, Phone: p.Phone, Email: p.Email, RoleIDs: p.RoleIDs,
	})
	if err != nil {
		return nil, mapTenantUserErr(err)
	}
	return toTenantUserView(u), nil
}

func (g *TenantUserGateway) UpdateUser(ctx context.Context, id uint, p biztenantuser.UserUpdateParams) error {
	return mapTenantUserErr(g.c.UpdateTenantUser(ctx, id, platformsdk.TenantUserUpdateRequest{
		Nickname: p.Nickname, Phone: p.Phone, Email: p.Email, Status: p.Status,
	}))
}

func (g *TenantUserGateway) SetUserStatus(ctx context.Context, id uint, status int) error {
	return mapTenantUserErr(g.c.SetTenantUserStatus(ctx, id, status == 1))
}

func (g *TenantUserGateway) ResetUserPassword(ctx context.Context, id uint, password string) error {
	return mapTenantUserErr(g.c.ResetTenantUserPassword(ctx, id, password))
}

func (g *TenantUserGateway) SetUserRoles(ctx context.Context, id uint, roleIDs []uint) error {
	return mapTenantUserErr(g.c.SetTenantUserRoles(ctx, id, roleIDs))
}

func (g *TenantUserGateway) DeleteUser(ctx context.Context, id uint) error {
	return mapTenantUserErr(g.c.DeleteTenantUser(ctx, id))
}

// ---- 租户角色 ----

func (g *TenantUserGateway) ListRoles(ctx context.Context, p biztenantuser.RoleListParams, page, pageSize int) ([]*biztenantuser.TenantRoleView, pagination.Page, error) {
	f := platformsdk.TenantRoleFilter{Keyword: p.Keyword}
	if p.TenantID != nil {
		f.TenantID = *p.TenantID
	}
	list, pg, err := g.c.ListTenantRoles(ctx, page, pageSize, f)
	if err != nil {
		return nil, pagination.Page{}, mapTenantRoleErr(err)
	}
	if pg == nil {
		pg = &platformsdk.Page{}
	}
	out := make([]*biztenantuser.TenantRoleView, 0, len(list))
	for _, r := range list {
		out = append(out, toTenantRoleView(r))
	}
	return out, pagination.Page{Page: pg.Page, PageSize: pg.PageSize, Total: pg.Total}, nil
}

func (g *TenantUserGateway) CreateRole(ctx context.Context, p biztenantuser.RoleCreateParams) (*biztenantuser.TenantRoleView, error) {
	r, err := g.c.CreateTenantRole(ctx, platformsdk.TenantRoleCreateRequest{
		TenantID: p.TenantID, Name: p.Name, Code: p.Code, Remark: p.Remark, PermCodes: p.PermCodes,
	})
	if err != nil {
		return nil, mapTenantPermErr(mapTenantRoleErr(err))
	}
	return toTenantRoleView(r), nil
}

func (g *TenantUserGateway) UpdateRole(ctx context.Context, id uint, p biztenantuser.RoleUpdateParams) error {
	return mapTenantPermErr(mapTenantRoleErr(g.c.UpdateTenantRole(ctx, id, platformsdk.TenantRoleUpdateRequest{
		Name: p.Name, Remark: p.Remark, PermCodes: p.PermCodes,
	})))
}

func (g *TenantUserGateway) SetRolePerms(ctx context.Context, id uint, permCodes []string) error {
	return mapTenantPermErr(mapTenantRoleErr(g.c.SetTenantRolePerms(ctx, id, permCodes)))
}

func (g *TenantUserGateway) DeleteRole(ctx context.Context, id uint) error {
	return mapTenantRoleErr(g.c.DeleteTenantRole(ctx, id))
}

func (g *TenantUserGateway) ListPermCatalog(ctx context.Context) ([]*biztenantuser.PermDefView, error) {
	catalog, err := g.c.ListTenantUserPerms(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*biztenantuser.PermDefView, 0, len(catalog))
	for _, d := range catalog {
		out = append(out, &biztenantuser.PermDefView{Code: d.Code, Group: d.Group})
	}
	return out, nil
}
