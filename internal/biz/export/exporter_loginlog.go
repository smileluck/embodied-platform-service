package export

import (
	"context"
	"net/url"
	"strconv"
	"time"

	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// parseUnix 解析 unix 秒级时间戳查询参数（与日志列表页 start/end 入参一致；空/非法返回零值表示不限）
func parseUnix(s string) (t time.Time) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		t = time.Unix(n, 0)
	}
	return
}

// LoginLogExporter 登录日志导出（复用日志仓储；查询条件与列表页一致：username / ip / status / start / end）
type LoginLogExporter struct {
	logs bizlog.Repo
	mask map[string]string
}

func NewLoginLogExporter(logs bizlog.Repo, c *conf.Bootstrap) *LoginLogExporter {
	return &LoginLogExporter{logs: logs, mask: c.Export.Mask}
}

func (e *LoginLogExporter) Biz() string  { return "login_log" }
func (e *LoginLogExporter) Name() string { return "登录日志" }

func (e *LoginLogExporter) Columns() []Column {
	return []Column{
		{Key: "id", Title: "ID"},
		{Key: "username", Title: "用户名"},
		{Key: "ip", Title: "IP"},
		{Key: "device", Title: "设备"},
		{Key: "status", Title: "状态"},
		{Key: "msg", Title: "信息"},
		{Key: "created_at", Title: "登录时间"},
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
		status := "失败"
		if l.Status == bizlog.LoginStatusSuccess {
			status = "成功"
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
