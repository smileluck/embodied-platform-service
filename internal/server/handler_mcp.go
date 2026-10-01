// MCP 服务 handler：配置 CRUD + 连通测试（protected）
package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bizmcp "github.com/smilex/smilex-admin-gin/internal/biz/mcp"
	mcpsvc "github.com/smilex/smilex-admin-gin/internal/service/mcp"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listMcpServers(c *gin.Context) {
	page, size := s.pageParams(c)
	q := bizmcp.Query{Kw: c.Query("kw"), Transport: c.Query("transport")}
	if st := c.Query("status"); st != "" {
		v, err := strconv.Atoi(st)
		if err != nil || (v != 0 && v != 1) {
			response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
			return
		}
		q.Status = &v
	}
	list, pg, err := s.mcp.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createMcpServer(c *gin.Context) {
	var req mcpsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	srv, err := s.mcp.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, srv)
}

func (s *HTTPServer) getMcpServer(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	srv, err := s.mcp.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, srv)
}

func (s *HTTPServer) updateMcpServer(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req mcpsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.mcp.Update(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteMcpServer(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.mcp.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// testMcpServer 连通测试：真实握手 initialize + tools/list（失败也是有效结果：ok=false 携带原因）
func (s *HTTPServer) testMcpServer(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	result, err := s.mcp.Test(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, result)
}
