package export

import (
	"context"
	"net/url"
	"strconv"

	bizuser "github.com/smilex/smilex-admin-gin/internal/biz/user"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// UserExporter 用户列表导出（复用用户仓储分页查询；查询条件与列表页一致：username / status）
type UserExporter struct {
	users bizuser.Repo
	mask  map[string]string
}

func NewUserExporter(users bizuser.Repo, c *conf.Bootstrap) *UserExporter {
	return &UserExporter{users: users, mask: c.Export.Mask}
}

func (e *UserExporter) Biz() string  { return "user" }
func (e *UserExporter) Name() string { return "用户列表" }

func (e *UserExporter) Columns() []Column {
	return []Column{
		{Key: "id", Title: "ID"},
		{Key: "username", Title: "用户名"},
		{Key: "nickname", Title: "昵称"},
		{Key: "phone", Title: "手机号"},
		{Key: "email", Title: "邮箱"},
		{Key: "status", Title: "状态"},
		{Key: "created_at", Title: "创建时间"},
	}
}

func (e *UserExporter) Fetch(ctx context.Context, params url.Values, offset, limit int) ([][]string, int64, error) {
	q := bizuser.Query{Username: params.Get("username")}
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
		status := "禁用"
		if u.Enabled() {
			status = "启用"
		}
		row := []string{
			strconv.FormatUint(uint64(u.ID), 10),
			u.Username,
			u.Nickname,
			u.Phone,
			u.Email,
			status,
			u.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		maskRow(cols, e.mask, row)
		rows = append(rows, row)
	}
	return rows, total, nil
}
