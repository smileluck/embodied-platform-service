// Package pagination 通用分页参数与结果
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type Page struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// Parse 从 query 中解析 page / page_size
func Parse(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return
}

func Offset(page, pageSize int) int { return (page - 1) * pageSize }
