package export

import (
	"context"
	"net/url"
	"strconv"

	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
)

// LoginLogExporter 登录日志导出（复用日志仓储；查询条件与列表页一致：username / ip / status / start / end）
type LoginLogExporter struct {
	logs bizlog.Repo
	mask map[string]string
}

func NewLoginLogExporter(logs bizlog.Repo, c *conf.Bootstrap) *LoginLogExporter {
	return &LoginLogExporter{logs: logs, mask: c.Export.Mask}
}

func (e *LoginLogExporter) Biz() string     { return "login_log" }
func (e *LoginLogExporter) NameKey() string { return "export.name.login_log" }

func (e *LoginLogExporter) Columns() []Column {
	return []Column{
		{Key: "id", Title: "export.col.id"},
		{Key: "username", Title: "export.col.username"},
		{Key: "ip", Title: "export.col.ip"},
		{Key: "device", Title: "export.col.device"},
		{Key: "status", Title: "export.col.result"},
		{Key: "msg", Title: "export.col.message"},
		{Key: "created_at", Title: "export.col.login_time"},
	}
}

func (e *LoginLogExporter) Fetch(ctx context.Context, params url.Values, offset, limit int) ([][]string, int64, error) {
	q := bizlog.LoginLogQuery{
		Username: params.Get("username"),
		IP:       params.Get("ip"),
		Start:    parseUnix(params.Get("start")),
		End:      parseUnix(params.Get("end")),
	}
	if v := params.Get("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	logs, total, err := e.logs.ListLoginLogs(ctx, q, offset/limit+1, limit)
	if err != nil {
		return nil, 0, err
	}
	cols := e.Columns()
	rows := make([][]string, 0, len(logs))
	for _, l := range logs {
		status := i18n.T(ctx, "export.value.failure")
		if l.Status == bizlog.LoginStatusSuccess {
			status = i18n.T(ctx, "export.value.success")
		}
		row := []string{
			strconv.FormatUint(uint64(l.ID), 10),
			l.Username,
			l.IP,
			l.Device,
			status,
			l.Msg,
			l.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		maskRow(cols, e.mask, row)
		rows = append(rows, row)
	}
	return rows, total, nil
}
