package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bizappuser "github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	appusersvc "github.com/smilex/smilex-admin-gin/internal/service/appuser"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 应用用户独立认证（app-auth） ----

// appLogin 应用用户登录：成功返回令牌对 + 用户信息；失败统一 401（防爆破计数由 LoginIPGuard 负责）
func (s *HTTPServer) appLogin(c *gin.Context) {
	var req appusersvc.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.appuser.Login(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) appRefresh(c *gin.Context) {
	var req appusersvc.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	tp, err := s.appuser.Refresh(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
		return
	}
	response.OK(c, tp)
}

func (s *HTTPServer) appProfile(c *gin.Context) {
	sub := middleware.AppSubject(c)
	vo, err := s.appuser.Profile(c.Request.Context(), sub.UserID)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

// appChangePassword 本人修改密码（校验旧密码）
func (s *HTTPServer) appChangePassword(c *gin.Context) {
	sub := middleware.AppSubject(c)
	var req appusersvc.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.appuser.ChangePassword(c.Request.Context(), sub.Username, req); err != nil {
		if isErr(err, bizappuser.ErrBadCredentials) {
			response.BadRequest(c, i18n.T(c.Request.Context(), "profile.wrong_old_password"))
			return
		}
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
