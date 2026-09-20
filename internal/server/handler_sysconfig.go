// 系统参数 handler
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	syssvc "github.com/smilex/smilex-admin-gin/internal/service/sysconfig"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listSysConfigs(c *gin.Context) {
	list, err := s.syscfg.List(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}

func (s *HTTPServer) createSysConfig(c *gin.Context) {
	var req syssvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	cfg, err := s.syscfg.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, cfg)
}

func (s *HTTPServer) updateSysConfig(c *gin.Context) {
	var req syssvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	cfg, err := s.syscfg.Update(c.Request.Context(), c.Param("key"), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, cfg)
}

func (s *HTTPServer) deleteSysConfig(c *gin.Context) {
	if err := s.syscfg.Delete(c.Request.Context(), c.Param("key")); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
