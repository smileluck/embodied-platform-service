package export

import (
	"context"
	"net/url"

	"github.com/smilex/smilex-admin-gin/pkg/security"
)

// Column 导出列定义：Key 为脱敏配置（export.mask）的匹配键，Title 为 CSV 表头
type Column struct {
	Key   string
	Title string
}

// Exporter 业务导出器：worker 循环调用 Fetch 分批拉数（offset/limit 语义），
// 返回行须与 Columns 一一对应，且已按 export.mask 对列 Key 应用 security.Mask。
type Exporter interface {
	// Biz 业务类型标识（提交路由与记录落库用，如 user / login_log / op_log）
	Biz() string
	// Name 展示名（如「用户列表」，用于生成记录 Name）
	Name() string
	// Columns 导出列（顺序即 CSV 列序）
	Columns() []Column
	// Fetch 拉取 [offset, offset+limit) 一批数据行，total 为满足条件的总行数
	Fetch(ctx context.Context, params url.Values, offset, limit int) (rows [][]string, total int64, err error)
}

// Registry 导出器注册表（biz -> Exporter）
type Registry struct {
	exporters map[string]Exporter
}

// NewRegistry 注册全部导出器（wire 按具体类型注入，避免同接口多实现歧义）
func NewRegistry(user *UserExporter, loginLog *LoginLogExporter, opLog *OpLogExporter) *Registry {
	r := &Registry{exporters: make(map[string]Exporter, 3)}
	for _, e := range []Exporter{user, loginLog, opLog} {
		r.exporters[e.Biz()] = e
	}
	return r
}

// Get 按业务类型取导出器
func (r *Registry) Get(biz string) (Exporter, bool) {
	e, ok := r.exporters[biz]
	return e, ok
}

// maskRow 按 export.mask 配置对数据行就地脱敏（列 Key 命中规则才处理；行须与 cols 等长）
func maskRow(cols []Column, mask map[string]string, row []string) {
	for i, col := range cols {
		if rule, ok := mask[col.Key]; ok {
			row[i] = security.Mask(rule, row[i])
		}
	}
}
