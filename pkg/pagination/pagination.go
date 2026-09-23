// Package pagination 通用分页结果
package pagination

// Page 分页元信息（列表接口响应体；参数解析统一在 server 层 pageParams，
// 支持 page_size=0 全量与运行时上限夹取，故此处不再提供 query 解析）
type Page struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}
