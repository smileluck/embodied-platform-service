package server

import (
	"net/http"

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
