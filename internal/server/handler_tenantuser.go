package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	tenantusersvc "github.com/smilex/smilex-admin-gin/internal/service/tenantuser"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 租户用户（管理端：经平台开放面实时消费，本地无数据面） ----

func (s *HTTPServer) listTenantUsers(c *gin.Context) {
	page, size := s.pageParams(c)
	q := biztenantuser.UserListParams{
		Keyword: strings.TrimSpace(c.Query("kw")),
		Phone:   strings.TrimSpace(c.Query("phone")),
	}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	// tenant_id 为平台租户 ID（与租户下拉口径一致；服务端按商户绑定收敛）
	if v := c.Query("tenant_id"); v != "" {
		if tid, err := strconv.ParseUint(v, 10, 64); err == nil && tid > 0 {
			t := uint(tid)
			q.TenantID = &t
		}
	}
	list, pg, err := s.tenantuser.ListUsers(c.Request.Context(), q, page, size)
	if err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createTenantUser(c *gin.Context) {
	var req tenantusersvc.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.tenantuser.CreateUser(c.Request.Context(), req)
	if err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateTenantUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantusersvc.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.UpdateUser(c.Request.Context(), id, req); err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setTenantUserStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req struct {
		Status *int `json:"status" binding:"required,oneof=0 1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.SetUserStatus(c.Request.Context(), id, *req.Status); err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) resetTenantUserPassword(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantusersvc.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.ResetUserPassword(c.Request.Context(), id, req); err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setTenantUserRoles(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantusersvc.SetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.SetUserRoles(c.Request.Context(), id, req); err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteTenantUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.tenantuser.DeleteUser(c.Request.Context(), id); err != nil {
		s.tenantUserErr(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 租户角色（管理端） ----

func (s *HTTPServer) listTenantRoles(c *gin.Context) {
	page, size := s.pageParams(c)
	q := biztenantuser.RoleListParams{Keyword: strings.TrimSpace(c.Query("kw"))}
	if v := c.Query("tenant_id"); v != "" {
		if tid, err := strconv.ParseUint(v, 10, 64); err == nil && tid > 0 {
			t := uint(tid)
			q.TenantID = &t
		}
	}
	list, pg, err := s.tenantuser.ListRoles(c.Request.Context(), q, page, size)
	if err != nil {
		s.tenantRoleErr(c, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createTenantRole(c *gin.Context) {
	var req tenantusersvc.RoleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.tenantuser.CreateRole(c.Request.Context(), req)
	if err != nil {
		s.tenantRoleErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateTenantRole(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantusersvc.RoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.UpdateRole(c.Request.Context(), id, req); err != nil {
		s.tenantRoleErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setTenantRolePerms(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantusersvc.SetPermsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantuser.SetRolePerms(c.Request.Context(), id, req); err != nil {
		s.tenantRoleErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteTenantRole(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.tenantuser.DeleteRole(c.Request.Context(), id); err != nil {
		s.tenantRoleErr(c, err)
		return
	}
	response.OK(c, nil)
}

// listTenantUserPermCatalog 权限点目录（角色配权 UI 数据源；本地注册表唯一下发）
func (s *HTTPServer) listTenantUserPermCatalog(c *gin.Context) {
	response.OK(c, s.tenantuser.ListPermCatalog())
}

// tenantUserErr 租户用户操作错误映射（经平台开放面）：不存在 404、租户越界/重名 403/409，
// 其余 400
func (s *HTTPServer) tenantUserErr(c *gin.Context, err error) {
	switch {
	case isErr(err, biztenantuser.ErrTenantUserNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, biztenantuser.ErrTenantNotInScope):
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
	case isErr(err, biztenantuser.ErrDuplicateUsername):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	default:
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	}
}

// tenantRoleErr 租户角色操作错误映射：不存在 404、越界/重名/占用 403/409、权限点无效 400
func (s *HTTPServer) tenantRoleErr(c *gin.Context, err error) {
	switch {
	case isErr(err, biztenantuser.ErrTenantRoleNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, biztenantuser.ErrTenantNotInScope):
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
	case isErr(err, biztenantuser.ErrDuplicateRoleCode), isErr(err, biztenantuser.ErrRoleInUse):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, biztenantuser.ErrInvalidPerm):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	default:
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	}
}
