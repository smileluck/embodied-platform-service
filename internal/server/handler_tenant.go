package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	tenantsvc "github.com/smilex/smilex-admin-gin/internal/service/tenant"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"strconv"
	"strings"
)

// ---- 租户 ----

func (s *HTTPServer) listTenants(c *gin.Context) {
	page, size := pageParams(c)
	q := biztenant.Query{
		Name: strings.TrimSpace(c.Query("name")),
		Code: strings.TrimSpace(c.Query("code")),
	}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	list, pg, err := s.tenant.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createTenant(c *gin.Context) {
	var req tenantsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.tenant.Create(c.Request.Context(), req)
	if err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) getTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.tenant.Get(c.Request.Context(), id)
	if err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenant.Update(c.Request.Context(), id, req); err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.tenant.Delete(c.Request.Context(), id); err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setTenantStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantsvc.SetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenant.SetStatus(c.Request.Context(), id, req); err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, nil)
}

// syncTenant 存量补链：未同步租户在平台创建/绑定并回填 platform_id
func (s *HTTPServer) syncTenant(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.tenant.SyncExisting(c.Request.Context(), id)
	if err != nil {
		s.tenantErr(c, err)
		return
	}
	response.OK(c, vo)
}

// tenantErr 租户操作错误映射：不存在 404，关联应用用户/平台冲突 409，平台调用失败按平台状态透传
func (s *HTTPServer) tenantErr(c *gin.Context, err error) {
	switch {
	case isErr(err, biztenant.ErrTenantNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, biztenant.ErrTenantInUse):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	default:
		var perr *platformError
		if errorsAs(err, &perr) {
			s.platformErr(c, err)
			return
		}
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	}
}
