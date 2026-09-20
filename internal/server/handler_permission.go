package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bizperm "github.com/smilex/smilex-admin-gin/internal/biz/permission"
	permsvc "github.com/smilex/smilex-admin-gin/internal/service/permission"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 权限 ----

func (s *HTTPServer) listPerms(c *gin.Context) {
	page, size := pageParams(c)
	// page_size=0：全量返回（菜单管理树/角色分配权限树需要整表构建，分页会静默截断）
	if c.Query("page_size") == "0" {
		size = 0
	}
	q := bizperm.Query{Type: c.Query("type")}
	ps, pg, err := s.perm.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: ps, Page: pg})
}

func (s *HTTPServer) createPerm(c *gin.Context) {
	var req permsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	p, err := s.perm.Create(c.Request.Context(), req)
	if err != nil {
		if isErr(err, bizperm.ErrDuplicateCode) {
			response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
			return
		}
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, p)
}

func (s *HTTPServer) getPerm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	p, err := s.perm.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, p)
}

func (s *HTTPServer) updatePerm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req permsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.perm.Update(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deletePerm(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.perm.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
