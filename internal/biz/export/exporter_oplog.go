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

// OpLogExporter 操作日志导出（复用日志仓储；查询条件与列表页一致：username / method / kw / start / end）
type OpLogExporter struct {
	logs bizlog.Repo
	mask map[string]string
}

func NewOpLogExporter(logs bizlog.Repo, c *conf.Bootstrap) *OpLogExporter {
	return &OpLogExporter{logs: logs, mask: c.Export.Mask}
}

func (e *OpLogExporter) Biz() string     { return "op_log" }
func (e *OpLogExporter) NameKey() string { return "export.name.op_log" }

func (e *OpLogExporter) Columns() []Column {
	return []Column{
		{Key: "id", Title: "export.col.id"},
		{Key: "username", Title: "export.col.operator"},
		{Key: "method", Title: "export.col.method"},
		{Key: "path", Title: "export.col.path"},
		{Key: "route", Title: "export.col.route"},
		{Key: "action", Title: "export.col.action"},
		{Key: "params", Title: "export.col.params"},
		{Key: "ip", Title: "export.col.ip"},
		{Key: "status_code", Title: "export.col.status_code"},
		{Key: "latency_ms", Title: "export.col.latency"},
		{Key: "created_at", Title: "export.col.op_time"},
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
