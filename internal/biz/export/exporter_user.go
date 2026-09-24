package export

import (
	"context"
	"net/url"
	"strconv"

	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
)

// UserExporter 用户（准入投影）列表导出（复用准入仓储分页查询；查询条件与列表页一致）
type UserExporter struct {
	users bizadmission.Repo
	mask  map[string]string
}

func NewUserExporter(users bizadmission.Repo, c *conf.Bootstrap) *UserExporter {
	return &UserExporter{users: users, mask: c.Export.Mask}
}

func (e *UserExporter) Biz() string     { return "user" }
func (e *UserExporter) NameKey() string { return "export.name.user" }

func (e *UserExporter) Columns() []Column {
	return []Column{
		{Key: "id", Title: "export.col.id"},
		{Key: "platform_user_id", Title: "export.col.platform_user_id"},
		{Key: "username", Title: "export.col.username"},
		{Key: "nickname", Title: "export.col.nickname"},
		{Key: "email", Title: "export.col.email"},
		{Key: "status", Title: "export.col.status"},
		{Key: "created_at", Title: "export.col.created_at"},
	}
}

func (e *UserExporter) Fetch(ctx context.Context, params url.Values, offset, limit int) ([][]string, int64, error) {
	q := bizadmission.Query{Username: params.Get("username")}
	if v := params.Get("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	users, total, err := e.users.List(ctx, q, offset/limit+1, limit)
	if err != nil {
		return nil, 0, err
	}
	cols := e.Columns()
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		status := i18n.T(ctx, "export.value.disabled")
		if u.Enabled {
			status = i18n.T(ctx, "export.value.enabled")
		}
		row := []string{
			strconv.FormatUint(uint64(u.ID), 10),
			strconv.FormatUint(uint64(u.PlatformUserID), 10),
			u.Username,
			u.Nickname,
			u.Email,
			status,
			u.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		maskRow(cols, e.mask, row)
		rows = append(rows, row)
	}
	return rows, total, nil
}
