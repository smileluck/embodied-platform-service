package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bizadmission "github.com/smilex/smilex-admin-gin/internal/biz/admission"
	admissionsvc "github.com/smilex/smilex-admin-gin/internal/service/admission"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 用户（准入管理） ----

type listResult struct {
	List interface{} `json:"list"`
	Page interface{} `json:"page"`
}

func (s *HTTPServer) listUsers(c *gin.Context) {
	page, size := pageParams(c)
	var req admissionsvc.ListRequest
	_ = c.ShouldBindQuery(&req)
	users, pg, err := s.admission.List(c.Request.Context(), req, page, size)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, listResult{List: users, Page: pg})
}

func (s *HTTPServer) getUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.admission.Get(c.Request.Context(), id)
	if err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, vo)
}

// addUser 新增成员：推送平台（无此账号则创建并绑定本商户，有则仅绑定）并建本地准入投影
func (s *HTTPServer) addUser(c *gin.Context) {
	var req admissionsvc.AddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	p, existed, err := s.admission.Add(c.Request.Context(), req)
	if err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, gin.H{"projection": p, "existed": existed})
}

func (s *HTTPServer) setUserAdmission(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req admissionsvc.SetEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.admission.SetEnabled(c.Request.Context(), id, *req.Enabled); err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setUserRoles(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req admissionsvc.SetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.admission.SetRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, nil)
}

// deleteUser 移除成员：解除平台侧绑定（账号本体保留）并删除本地投影
func (s *HTTPServer) deleteUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.admission.Delete(c.Request.Context(), id); err != nil {
		s.admissionErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) syncUsersFromPlatform(c *gin.Context) {
	created, refreshed, err := s.admission.SyncFromPlatform(c.Request.Context())
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, gin.H{"created": created, "refreshed": refreshed})
}

// admissionErr 准入/成员操作错误映射：不存在 404；平台信封错误透传；其余 400
func (s *HTTPServer) admissionErr(c *gin.Context, err error) {
	if isErr(err, bizadmission.ErrNotFound) {
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
		return
	}
	var perr *platformError
	if errorsAs(err, &perr) {
		s.platformErr(c, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}
