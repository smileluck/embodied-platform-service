package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) getServerStatus(c *gin.Context) {
	vo, err := s.monitor.ServerStatus(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

// getMonitorHistory 历史快照：hours 缺省 24；显式传入须为正整数（类型/取值前置校验），
// 超过最大回看窗口 72h 由 biz 层夹取
func (s *HTTPServer) getMonitorHistory(c *gin.Context) {
	hours := 24
	if v := c.Query("hours"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
			return
		}
		hours = n
	}
	list, err := s.monitor.History(c.Request.Context(), hours)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}
