package server

import (
	"net/http"
<<<<<<< HEAD
=======
	"strconv"
	"strings"
>>>>>>> 12ccad6d (fix: 批量修复 10 项页面问题（日志/租户/监控/智能体）)

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

<<<<<<< HEAD
=======
func (s *HTTPServer) listLoginLogs(c *gin.Context) {
	page, size := s.pageParams(c)
	q := bizlog.LoginLogQuery{Username: strings.TrimSpace(c.Query("username")), IP: strings.TrimSpace(c.Query("ip"))}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.log.ListLoginLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, logListResult{List: logs, Page: pg, RetentionDays: s.log.RetentionDays()})
}

func (s *HTTPServer) clearLoginLogs(c *gin.Context) {
	n, err := s.log.ClearLoginLogs(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, gin.H{"deleted": n})
}

>>>>>>> 12ccad6d (fix: 批量修复 10 项页面问题（日志/租户/监控/智能体）)
func (s *HTTPServer) listOperationLogs(c *gin.Context) {
	page, size := s.pageParams(c)
	// 关键词/用户名 TrimSpace：过滤输入首尾空格，避免 IME 残留空格导致搜不到
	q := bizlog.OperationLogQuery{Username: strings.TrimSpace(c.Query("username")), Method: strings.TrimSpace(c.Query("method")), Keyword: strings.TrimSpace(c.Query("kw"))}
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
