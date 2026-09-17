package admission

import (
	"context"
	"errors"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

// PlatformGateway 平台商户成员网关（开放面 user:* 域，经平台 SDK 商户 HMAC）。
// 2026-09-17 起用户列表/新增/删除全部走开放面（账号本体全局在平台，绑定按本商户收敛），
// 管理面服务账号（platform.admin）通道随之废弃移除。
type PlatformGateway struct {
	client *sdk.Client
}

// NewPlatformGateway 构造（wire provider，绑定 bizadmission.MemberGateway）
func NewPlatformGateway(client *sdk.Client) *PlatformGateway {
	return &PlatformGateway{client: client}
}

func (g *PlatformGateway) ListMembers(ctx context.Context, keyword string, page, pageSize int) ([]*bizadmission.PlatformAccount, int64, error) {
	users, pg, err := g.client.ListUsers(ctx, page, pageSize, sdk.UserFilter{Keyword: keyword})
	if err != nil {
		return nil, 0, err
	}
	out := make([]*bizadmission.PlatformAccount, 0, len(users))
	for _, u := range users {
		out = append(out, &bizadmission.PlatformAccount{
			ID: u.ID, Username: u.Username, Nickname: u.Nickname, Status: u.Status,
		})
	}
	var total int64
	if pg != nil {
		total = pg.Total
	}
	return out, total, nil
}

func (g *PlatformGateway) CreateOrBind(ctx context.Context, username, nickname, password string) (uint, bool, error) {
	res, err := g.client.CreateOrBindUser(ctx, sdk.UserCreateOrBindRequest{
		Username: username, Nickname: nickname, Password: password,
	})
	if err != nil {
		return 0, false, err
	}
	if res.User == nil {
		return 0, res.Existed, errors.New("platform: create user returned no user")
	}
	return res.User.ID, res.Existed, nil
}

// Unbind 解除平台侧绑定；404（未绑定/账号不存在）映射为 ErrPlatformNotBound（解绑幂等语义）
func (g *PlatformGateway) Unbind(ctx context.Context, platformUserID uint) error {
	err := g.client.UnbindUser(ctx, platformUserID)
	var perr *sdk.Error
	if errors.As(err, &perr) && perr.HTTPStatus == 404 {
		return bizadmission.ErrPlatformNotBound
	}
	return err
}
