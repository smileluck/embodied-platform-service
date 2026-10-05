// 平台开放面应用用户网关：包装 platformsdk 调用并映射业务哨兵
// （404→ErrAppUserNotFound 不泄露存在性；409 按 msg 区分重名/跨商户删除；
// 403→ErrTenantNotInScope——开放面错误信封无机器可读子码，按平台哨兵文案匹配，
// 平台侧文案变更需同步此处）。
package platform

import (
	"context"
	"errors"
	"net/http"
	"strings"

	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// AppUserGateway 平台开放面应用用户网关（wire 绑定 bizappuser.Gateway）
type AppUserGateway struct {
	c *platformsdk.Client
}

// NewAppUserGateway 构造（wire provider；client 来自 NewOpenAPIClient）
func NewAppUserGateway(c *platformsdk.Client) *AppUserGateway {
	return &AppUserGateway{c: c}
}

// mapErr 开放面错误 → 业务哨兵
func mapAppUserErr(err error) error {
	if err == nil {
		return nil
	}
	var perr *platformsdk.Error
	if !errors.As(err, &perr) {
		return err
	}
	switch perr.HTTPStatus {
	case http.StatusNotFound:
		return bizappuser.ErrAppUserNotFound
	case http.StatusForbidden:
		return bizappuser.ErrTenantNotInScope
	case http.StatusConflict:
		if strings.Contains(perr.Msg, "已存在") {
			return bizappuser.ErrDuplicateUsername
		}
		return bizappuser.ErrCrossMerchantDelete
	}
	return err
}

func toAppUserView(u *platformsdk.AppUser) *bizappuser.AppUserView {
	if u == nil {
		return nil
	}
	return &bizappuser.AppUserView{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname,
		Phone: u.Phone, Email: u.Email, Status: u.Status,
		TenantIDs: u.TenantIDs, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func (g *AppUserGateway) List(ctx context.Context, p bizappuser.ListParams, page, pageSize int) ([]*bizappuser.AppUserView, pagination.Page, error) {
	f := platformsdk.AppUserFilter{Keyword: p.Keyword, Phone: p.Phone, Status: p.Status}
	if p.TenantID != nil {
		f.TenantID = *p.TenantID
	}
	list, pg, err := g.c.ListAppUsers(ctx, page, pageSize, f)
	if err != nil {
		return nil, pagination.Page{}, mapAppUserErr(err)
	}
	if pg == nil {
		pg = &platformsdk.Page{}
	}
	out := make([]*bizappuser.AppUserView, 0, len(list))
	for _, u := range list {
		out = append(out, toAppUserView(u))
	}
	return out, pagination.Page{Page: pg.Page, PageSize: pg.PageSize, Total: pg.Total}, nil
}

func (g *AppUserGateway) Create(ctx context.Context, p bizappuser.CreateParams) (*bizappuser.AppUserView, error) {
	u, err := g.c.CreateAppUser(ctx, platformsdk.AppUserCreateRequest{
		Username: p.Username, Password: p.Password, Nickname: p.Nickname,
		Phone: p.Phone, Email: p.Email, TenantIDs: p.TenantIDs,
	})
	if err != nil {
		return nil, mapAppUserErr(err)
	}
	return toAppUserView(u), nil
}

func (g *AppUserGateway) Update(ctx context.Context, id uint, p bizappuser.UpdateParams) error {
	return mapAppUserErr(g.c.UpdateAppUser(ctx, id, platformsdk.AppUserUpdateRequest{
		Nickname: p.Nickname, Phone: p.Phone, Email: p.Email,
		Status: p.Status, TenantIDs: p.TenantIDs,
	}))
}

func (g *AppUserGateway) SetStatus(ctx context.Context, id uint, status int) error {
	return mapAppUserErr(g.c.SetAppUserStatus(ctx, id, status == 1))
}

func (g *AppUserGateway) ResetPassword(ctx context.Context, id uint, password string) error {
	return mapAppUserErr(g.c.ResetAppUserPassword(ctx, id, password))
}

func (g *AppUserGateway) Delete(ctx context.Context, id uint) error {
	return mapAppUserErr(g.c.DeleteAppUser(ctx, id))
}
