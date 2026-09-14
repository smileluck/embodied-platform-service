package export

import (
	"context"
	"net/url"
	"strconv"

	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/internal/conf"
)

// OpLogExporter 操作日志导出（复用日志仓储；查询条件与列表页一致：username / method / kw / start / end）
type OpLogExporter struct {
	logs bizlog.Repo
	mask map[string]string
}

func NewOpLogExporter(logs bizlog.Repo, c *conf.Bootstrap) *OpLogExporter {
	return &OpLogExporter{logs: logs, mask: c.Export.Mask}
}

func (e *OpLogExporter) Biz() string  { return "op_log" }
func (e *OpLogExporter) Name() string { return "操作日志" }

func (e *OpLogExporter) Columns() []Column {
	return []Column{
		{Key: "id", Title: "ID"},
		{Key: "username", Title: "操作人"},
		{Key: "method", Title: "请求方式"},
		{Key: "path", Title: "请求路径"},
		{Key: "route", Title: "路由"},
		{Key: "action", Title: "动作"},
		{Key: "params", Title: "参数"},
		{Key: "ip", Title: "IP"},
		{Key: "status_code", Title: "状态码"},
		{Key: "latency_ms", Title: "耗时(ms)"},
		{Key: "created_at", Title: "操作时间"},
	}
}

func (e *OpLogExporter) Fetch(ctx context.Context, params url.Values, offset, limit int) ([][]string, int64, error) {
	q := bizlog.OperationLogQuery{
		Username: params.Get("username"),
		Method:   params.Get("method"),
		Keyword:  params.Get("kw"),
		Start:    parseUnix(params.Get("start")),
		End:      parseUnix(params.Get("end")),
	}
	logs, total, err := e.logs.ListOperationLogs(ctx, q, offset/limit+1, limit)
	if err != nil {
		return nil, 0, err
	}
	cols := e.Columns()
	rows := make([][]string, 0, len(logs))
	for _, o := range logs {
		row := []string{
			strconv.FormatUint(uint64(o.ID), 10),
			o.Username,
			o.Method,
			o.Path,
			o.Route,
			o.Action,
			o.Params,
			o.IP,
			strconv.Itoa(o.StatusCode),
			strconv.Itoa(o.LatencyMs),
			o.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		maskRow(cols, e.mask, row)
		rows = append(rows, row)
	}
	return rows, total, nil
}
