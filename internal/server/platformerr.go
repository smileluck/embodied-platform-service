// 平台调用错误 → 本系统响应的映射（设备/型号/租户同步/准入同步等代理接口共用）
package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/internal/data/platform"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// isErr errors.Is 别名（缩短调用点）
func isErr(err, target error) bool { return errors.Is(err, target) }

// platformError 平台信封错误别名（*platform.Error）
type platformError = platform.Error

// errorsAs errors.As 别名（**platformError 用）
func errorsAs(err error, target **platformError) bool { return errors.As(err, target) }

// platformErr 平台调用失败统一映射：
//   - 平台信封错误：HTTP 状态与 msg 原样透传（401/403/404/409/429 语义保留）
//   - 网络/未知错误：502（上游不可达）
func (s *HTTPServer) platformErr(c *gin.Context, err error) {
	var perr *platform.Error
	if errors.As(err, &perr) {
		response.Fail(c, perr.HTTPStatus, response.CodeErr, perr.Msg)
		return
	}
	response.Fail(c, http.StatusBadGateway, response.CodeErr, "平台调用失败: "+err.Error())
}
