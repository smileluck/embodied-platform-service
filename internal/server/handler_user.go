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
	page, size := s.pageParams(c)
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

// admissionErr 准入/成员操作错误映射：
//   - 本地哨兵：不存在/平台未绑定 404，重复成员 400（i18n）
//   - 平台信封错误（含 SDK *platformsdk.Error 归一后）透传 HTTP 状态与 msg——
//     避免裸 err.Error()（"openapi: http 403 code 403: ..."）把 SDK 内部格式泄露给前端
//   - 其余本地错误 400
func (s *HTTPServer) admissionErr(c *gin.Context, err error) {
	switch {
	case isErr(err, bizadmission.ErrNotFound), isErr(err, bizadmission.ErrPlatformNotBound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, bizadmission.ErrDuplicatePlatformUser):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	default:
		if e := normalizeSDKError(err); e != err {
			s.platformErr(c, e)
			return
		}
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	}
}
