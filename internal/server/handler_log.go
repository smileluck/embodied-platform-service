package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 日志 ----

// logListResult 日志列表响应（附保留天数，前端展示保留说明用）
type logListResult struct {
	List          interface{} `json:"list"`
	Page          interface{} `json:"page"`
	RetentionDays int         `json:"retention_days"`
}

func (s *HTTPServer) listOperationLogs(c *gin.Context) {
	page, size := pageParams(c)
	q := bizlog.OperationLogQuery{Username: c.Query("username"), Method: c.Query("method"), Keyword: c.Query("kw")}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.log.ListOperationLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, logListResult{List: logs, Page: pg, RetentionDays: s.log.RetentionDays()})
}

func (s *HTTPServer) clearOperationLogs(c *gin.Context) {
	n, err := s.log.ClearOperationLogs(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, gin.H{"deleted": n})
}
