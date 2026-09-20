package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

// getMonitorHistory 历史快照（?hours=24，上限 72）
func (s *HTTPServer) getMonitorHistory(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	list, err := s.monitor.History(c.Request.Context(), hours)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}
