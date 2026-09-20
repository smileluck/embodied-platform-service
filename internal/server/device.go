// 设备管理 handlers（纯代理平台开放面 /open-api/v1；错误经 platformErr 透传）
package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bizdevice "github.com/smilex/smilex-admin-gin/internal/biz/device"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	sdk "github.com/smilex/smilex-admin-gin/sdk"
)

func (s *HTTPServer) listDevices(c *gin.Context) {
	page, size := s.pageParams(c)
	var req devicesvc.ListRequest
	_ = c.ShouldBindQuery(&req)
	devices, pg, err := s.device.List(c.Request.Context(), req, page, size)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, listResult{List: devices, Page: pg})
}

func (s *HTTPServer) registerDevice(c *gin.Context) {
	var req devicesvc.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	dev, err := s.device.Register(c.Request.Context(), req)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, dev)
}

func (s *HTTPServer) getDevice(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	dev, err := s.device.Get(c.Request.Context(), id)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, dev)
}

func (s *HTTPServer) getDeviceShadow(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	shadow, err := s.device.Shadow(c.Request.Context(), id)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, shadow)
}

func (s *HTTPServer) listDeviceCommands(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	page, size := s.pageParams(c)
	cmds, pg, err := s.device.ListCommands(c.Request.Context(), id, c.Query("status"), page, size)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, listResult{List: cmds, Page: pg})
}

func (s *HTTPServer) issueDeviceCommand(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req devicesvc.IssueCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	// Idempotency-Key 头优先于 body 字段（与平台契约一致）
	if k := c.GetHeader("Idempotency-Key"); k != "" {
		req.IdempotencyKey = k
	}
	cmd, err := s.device.IssueCommand(c.Request.Context(), id, req)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, cmd)
}

func (s *HTTPServer) getDeviceCommand(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	cmd, err := s.device.GetCommand(c.Request.Context(), id)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, cmd)
}

func (s *HTTPServer) getDeviceTelemetry(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req devicesvc.TelemetryRequest
	_ = c.ShouldBindQuery(&req)
	hist, err := s.device.Telemetry(c.Request.Context(), id, req)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, hist)
}

func (s *HTTPServer) listDeviceDataEvents(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sinceID, _ := strconv.ParseUint(c.Query("since_id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	events, err := s.device.DataEvents(c.Request.Context(), id, uint(sinceID), limit)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, gin.H{"list": events})
}

// deviceErr 设备代理错误映射：本地租户校验 400；平台错误（含 SDK *sdk.Error）透传
func (s *HTTPServer) deviceErr(c *gin.Context, err error) {
	if isErr(err, bizdevice.ErrTenantNotSynced) {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	s.platformErr(c, normalizeSDKError(err))
}

// normalizeSDKError 把 SDK 的 *sdk.Error 归一为平台信封错误（复用 platformErr 的透传逻辑）
func normalizeSDKError(err error) error {
	var se *sdk.Error
	if errors.As(err, &se) {
		return &platformError{HTTPStatus: se.HTTPStatus, Code: se.Code, Msg: se.Msg}
	}
	return err
}
