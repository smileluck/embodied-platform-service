// 平台开放面租户用户网关：包装 platformsdk 调用并映射业务哨兵
// （404 统一 ErrTenantUserNotFound 不泄露存在性；409 按平台哨兵文案区分重名——
// 开放面错误信封无机器可读子码，平台侧文案变更需同步此处，与 mapAppUserErr 同模式）。
// 租户 RBAC 自 2026-10-09 本地化，本网关只承载账号身份生命周期。
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

// TenantUserGateway 平台开放面租户用户网关（wire 绑定 biztenantuser.Gateway）
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

func toTenantUserView(u *platformsdk.TenantUser) *biztenantuser.TenantUserView {
	if u == nil {
		return nil
	}
	return &biztenantuser.TenantUserView{
		ID: u.ID, TenantID: u.TenantID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

// ---- 租户用户（账号身份生命周期） ----

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

// GetUser 按 id 取单个账号（定位成员/校验用户归属租户用；不可见统一 404）
func (g *TenantUserGateway) GetUser(ctx context.Context, id uint) (*biztenantuser.TenantUserView, error) {
	u, err := g.c.GetTenantUser(ctx, id)
	if err != nil {
		return nil, mapTenantUserErr(err)
	}
	return toTenantUserView(u), nil
}

func (g *TenantUserGateway) CreateUser(ctx context.Context, p biztenantuser.UserCreateParams) (*biztenantuser.TenantUserView, error) {
	u, err := g.c.CreateTenantUser(ctx, platformsdk.TenantUserCreateRequest{
		TenantID: p.TenantID, Username: p.Username, Password: p.Password,
		Nickname: p.Nickname, Phone: p.Phone, Email: p.Email,
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

func (g *TenantUserGateway) DeleteUser(ctx context.Context, id uint) error {
	return mapTenantUserErr(g.c.DeleteTenantUser(ctx, id))
}
