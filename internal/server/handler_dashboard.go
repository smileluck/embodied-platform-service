// 仪表盘 handler（basic 组：登录即可查看首页数据）
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) dashboardStats(c *gin.Context) {
	stats, err := s.dashboard.Stats(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, stats)
}
