// Package response 统一 HTTP 响应
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
)

type Body struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

const (
	CodeOK           = 0
	CodeErr          = 1
	CodeUnauthorized = 401
	CodeForbidden    = 403
)

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, Body{Code: code, Msg: msg})
}

// ErrKeyFunc 错误 -> i18n key 映射钩子（由 server 层注册，避免 response 反向依赖业务包）
var ErrKeyFunc func(err error) (key string, ok bool)

// FailI18n 按请求 locale 输出本地化错误消息：命中注册表走语言包，未命中透传 err.Error()
func FailI18n(c *gin.Context, httpStatus, code int, err error) {
	if ErrKeyFunc != nil {
		if key, ok := ErrKeyFunc(err); ok {
			Fail(c, httpStatus, code, i18n.T(c.Request.Context(), key))
			return
		}
	}
	Fail(c, httpStatus, code, err.Error())
}

func BadRequest(c *gin.Context, msg string) { Fail(c, http.StatusBadRequest, CodeErr, msg) }
func Unauthorized(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, CodeUnauthorized, msg)
}
func Forbidden(c *gin.Context, msg string)       { Fail(c, http.StatusForbidden, CodeForbidden, msg) }
func NotFound(c *gin.Context, msg string)        { Fail(c, http.StatusNotFound, CodeErr, msg) }
func ServerError(c *gin.Context, msg string)     { Fail(c, http.StatusInternalServerError, CodeErr, msg) }
func TooManyRequests(c *gin.Context, msg string) { Fail(c, http.StatusTooManyRequests, CodeErr, msg) }
