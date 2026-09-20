package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"strings"
)

// ---- 应用用户（管理端） ----

func (s *HTTPServer) listAppUsers(c *gin.Context) {
	page, size := pageParams(c)
	q := bizappuser.Query{
		Keyword: strings.TrimSpace(c.Query("kw")),
		Phone:   strings.TrimSpace(c.Query("phone")),
	}
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	if v := c.Query("tenant_id"); v != "" {
		if tid, err := strconv.ParseUint(v, 10, 64); err == nil && tid > 0 {
			t := uint(tid)
			q.TenantID = &t
		}
	}
	list, pg, err := s.appuser.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createAppUser(c *gin.Context) {
	var req appusersvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.appuser.Create(c.Request.Context(), req)
	if err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) getAppUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vo, err := s.appuser.Get(c.Request.Context(), id)
	if err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) updateAppUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req appusersvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.appuser.Update(c.Request.Context(), id, req); err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteAppUser(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.appuser.Delete(c.Request.Context(), id); err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) resetAppUserPassword(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req appusersvc.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.appuser.ResetPassword(c.Request.Context(), id, req); err != nil {
		s.appuserErr(c, err)
		return
	}
	response.OK(c, nil)
}

// appuserErr 应用用户操作错误映射：不存在 404，其余 400
func (s *HTTPServer) appuserErr(c *gin.Context, err error) {
	if isErr(err, bizappuser.ErrAppUserNotFound) {
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// getServerStatus 服务器状态监控快照（主机/CPU/内存/磁盘/网络 + Go 进程运行时）
