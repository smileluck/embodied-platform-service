package admission

import (
	"context"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
)

// PlatformUserReaderAdapter 平台用户列表读取适配器（管理面服务账号 → 准入同步数据源）
type PlatformUserReaderAdapter struct {
	admin *platform.AdminClient
}

// NewPlatformUserReaderAdapter 构造（wire provider，绑定 bizadmission.PlatformUserReader）
func NewPlatformUserReaderAdapter(admin *platform.AdminClient) *PlatformUserReaderAdapter {
	return &PlatformUserReaderAdapter{admin: admin}
}

func (a *PlatformUserReaderAdapter) ListPlatformUsers(ctx context.Context, username string, page, pageSize int) ([]*bizadmission.PlatformAccount, int64, error) {
	users, pg, err := a.admin.ListPlatformUsers(ctx, username, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*bizadmission.PlatformAccount, 0, len(users))
	for _, u := range users {
		out = append(out, &bizadmission.PlatformAccount{
			ID: u.ID, Username: u.Username, Nickname: u.Nickname,
			Phone: u.Phone, Email: u.Email, Status: u.Status,
		})
	}
	var total int64
	if pg != nil {
		total = pg.Total
	}
	return out, total, nil
}
