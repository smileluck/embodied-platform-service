package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	authsvc "github.com/smilex/smilex-admin-gin/internal/service/auth"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 认证代理（登录/刷新/验证码公开，登出需平台认证） ----
// 管理端登录经本服务后端代理平台公开 API：服务端得以落登录日志（含失败），
// 密码不落库、不签发本地令牌（平台仍是唯一身份源）。

// login 登录：透传平台令牌对；成功/失败均异步记登录日志
// （失败计数由 LoginIPGuard 按响应状态码自动完成，此处不重复计数）
func (s *HTTPServer) login(c *gin.Context) {
	ctx := c.Request.Context()
	var req authsvc.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(ctx, "common.invalid_params"))
		return
	}
	pair, err := s.auth.Login(ctx, req)

	l := &bizlog.LoginLog{
		Username:  req.Username,
		IP:        c.ClientIP(),
		UserAgent: truncate(c.GetHeader("User-Agent"), 255),
		Device:    "web", // 管理端固定 web（平台同端互斥口径）
		Status:    bizlog.LoginStatusSuccess,
	}
	if err != nil {
		l.Status = bizlog.LoginStatusFail
		l.Msg = truncate(loginFailMsg(err), 255)
	}
	s.log.RecordLogin(context.Background(), l)

	if err != nil {
		s.authProxyErr(c, err)
		return
	}
	response.OK(c, pair)
}

// refreshToken 刷新令牌（refresh_token 放 body——跨域 cookie 不可用）
func (s *HTTPServer) refreshToken(c *gin.Context) {
	ctx := c.Request.Context()
	var req authsvc.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(ctx, "common.invalid_params"))
		return
	}
	pair, err := s.auth.Refresh(ctx, req)
	if err != nil {
		s.refreshProxyErr(c, err)
		return
	}
	response.OK(c, pair)
}

// refreshProxyErr 刷新令牌错误映射：平台 401（refresh_token 无效/过期/已吊销）统一回
// 明确的「登录已过期」文案——平台侧该分支的 msg 是内部兜底文案（“服务器内部错误”），
// 透传会给用户 500 语义的误导；其余错误沿用认证代理映射（平台业务错透传/不可达 503）
func (s *HTTPServer) refreshProxyErr(c *gin.Context, err error) {
	var perr *platform.Error
	if errors.As(err, &perr) && perr.HTTPStatus == http.StatusUnauthorized {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized,
			i18n.T(c.Request.Context(), "auth.refresh_failed"))
		return
	}
	s.authProxyErr(c, err)
}

// captcha 取平台登录验证码（开关在平台侧；enabled=false 时无需传 captcha 字段）
func (s *HTTPServer) captcha(c *gin.Context) {
	vo, err := s.auth.Captcha(c.Request.Context())
	if err != nil {
		s.authProxyErr(c, err)
		return
	}
	response.OK(c, vo)
}

// logout 登出：带用户本人 token 代理平台吊销会话
func (s *HTTPServer) logout(c *gin.Context) {
	if err := s.auth.Logout(c.Request.Context(), middleware.Token(c)); err != nil {
		s.authProxyErr(c, err)
		return
	}
	response.OK(c, nil)
}

// authProxyErr 认证代理错误映射：平台业务错误（401 密码错 / 400 参数错等）
// 原样透传 HTTP 状态与 msg（前端回显；401 同时被 LoginIPGuard 计入失败）；
// 平台不可达 503；其余 500
func (s *HTTPServer) authProxyErr(c *gin.Context, err error) {
	var perr *platform.Error
	if errors.As(err, &perr) {
		response.Fail(c, perr.HTTPStatus, response.CodeErr, perr.Msg)
		return
	}
	if errors.Is(err, bizauth.ErrPlatformUnavailable) {
		response.FailI18n(c, http.StatusServiceUnavailable, response.CodeErr, err)
		return
	}
	response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
}

// loginFailMsg 登录失败原因（落登录日志 msg 列）：平台信封错误取平台 msg，其余取错误文本
func loginFailMsg(err error) string {
	var perr *platform.Error
	if errors.As(err, &perr) {
		return perr.Msg
	}
	return err.Error()
}
