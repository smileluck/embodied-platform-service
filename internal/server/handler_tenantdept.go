package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	biztenantdept "github.com/smilex/smilex-admin-gin/internal/biz/tenantdept"
	tenantdeptsvc "github.com/smilex/smilex-admin-gin/internal/service/tenantdept"
	tenantusersvc "github.com/smilex/smilex-admin-gin/internal/service/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 租户部门（管理端：本地域，租户内部数据隔离基础能力） ----

// listTenantDepts 部门平表全量（含直属成员数；树形组装在前端按 parent_id 完成）
func (s *HTTPServer) listTenantDepts(c *gin.Context) {
	tid, err := strconv.ParseUint(c.Query("tenant_id"), 10, 64)
	if err != nil || tid == 0 {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	list, err := s.tenantdept.ListDepts(c.Request.Context(), uint(tid))
	if err != nil {
		s.tenantDeptErr(c, err)
		return
	}
	response.OK(c, list)
}

func (s *HTTPServer) createTenantDept(c *gin.Context) {
	var req tenantdeptsvc.DeptCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.tenantdept.CreateDept(c.Request.Context(), req)
	if err != nil {
		s.tenantDeptErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateTenantDept(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantdeptsvc.DeptUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantdept.UpdateDept(c.Request.Context(), id, req); err != nil {
		s.tenantDeptErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteTenantDept(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.tenantdept.DeleteDept(c.Request.Context(), id); err != nil {
		s.tenantDeptErr(c, err)
		return
	}
	response.OK(c, nil)
}

// setTenantUserDepts 全量替换用户部门绑定（多部门；先经平台定位用户防 id 漂移/越权）
func (s *HTTPServer) setTenantUserDepts(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantusersvc.SetDeptsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.SetUserDepts(c.Request.Context(), id, req); err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, nil)
}

// tenantDeptErr 租户部门操作错误映射：不存在 404、重名/占用 409、租户越界/父级无效 403
func (s *HTTPServer) tenantDeptErr(c *gin.Context, err error) {
	switch {
	case isErr(err, biztenantdept.ErrDeptNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, biztenantdept.ErrDuplicateDeptCode):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, biztenantdept.ErrDeptHasChildren):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, biztenantdept.ErrParentInvalid):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	default:
		s.tenantErr(c, err)
	}
}
