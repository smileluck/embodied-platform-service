// 租户端业务端点（/app-api/v1）：成员自助管理（TenantAdmin 鉴权）与
// 设备只读（归属即准入，v1 不含指令下发）。租户上下文一律取自 AppAuth
// 闸门（X-Tenant-ID 解析的平台租户 ID）与认证主体，不信任客户端参数。
package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	biztenantmember "github.com/smilex/smilex-admin-gin/internal/biz/tenantmember"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	tenantmembersvc "github.com/smilex/smilex-admin-gin/internal/service/tenantmember"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// errDeviceNotInTenant 设备不属于当前租户（404 语义，不泄露存在性——与平台开放面越权口径一致）
var errDeviceNotInTenant = errors.New("设备不存在或不属于当前租户")

// tenantCtx 取当前租户平台 ID 与操作者 app_user ID（AppAuth 之后可用）
func tenantCtx(c *gin.Context) (tenantPlatformID uint, actorID uint, ok bool) {
	tn := middleware.AppAuthTenant(c)
	sub := middleware.AppAuthSubject(c)
	if tn == nil || sub == nil {
		response.Unauthorized(c, "unauthenticated")
		return 0, 0, false
	}
	return tn.PlatformID, sub.UserID, true
}

// ---- 成员自助管理（仅 tenant_admin，路由组已挂 TenantAdmin） ----

func (s *HTTPServer) appApiListMembers(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	page, size := s.pageParams(c)
	list, pg, err := s.tenantmember.List(c.Request.Context(), tid, strings.TrimSpace(c.Query("kw")), page, size)
	if err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) appApiCreateMember(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	var req tenantmembersvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	vo, err := s.tenantmember.Create(c.Request.Context(), tid, req)
	if err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) appApiUpdateMember(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantmembersvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantmember.Update(c.Request.Context(), tid, id, req); err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) appApiSetMemberStatus(c *gin.Context) {
	tid, actorID, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantmembersvc.SetStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantmember.SetStatus(c.Request.Context(), tid, id, actorID, req.Status); err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) appApiResetMemberPassword(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantmembersvc.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantmember.ResetPassword(c.Request.Context(), tid, id, req.Password); err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) appApiRemoveMember(c *gin.Context) {
	tid, actorID, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.tenantmember.Remove(c.Request.Context(), tid, id, actorID); err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) appApiSetMemberRole(c *gin.Context) {
	tid, actorID, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantmembersvc.SetRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantmember.SetRole(c.Request.Context(), tid, id, actorID, req.Role); err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 个人中心（本人数据：持本人 token 代理平台，与 B 端 /auth/password 同模式） ----

// appApiChangePasswordRequest 本人修改密码入参（平台校验旧密码并吊销其他端会话）
type appApiChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6,max=64"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
}

// appApiChangePassword PUT /app-api/v1/profile/password：代理平台 PUT /app-auth/password
func (s *HTTPServer) appApiChangePassword(c *gin.Context) {
	if middleware.AppAuthSubject(c) == nil {
		response.Unauthorized(c, "unauthenticated")
		return
	}
	var req appApiChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	err := s.appIds.AppChangePassword(c.Request.Context(), middleware.AppAuthToken(c), req.OldPassword, req.NewPassword)
	switch {
	case err == nil:
		response.OK(c, nil)
	case isErr(err, bizauth.ErrInvalidToken):
		response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, err)
	case isErr(err, bizauth.ErrPlatformUnavailable):
		response.FailI18n(c, http.StatusServiceUnavailable, response.CodeErr, err)
	default:
		s.platformErr(c, err) // 旧密码错误等平台 4xx 按状态码透传
	}
}

// tenantMemberErr 成员操作错误映射：本地守卫 404/409/400，其余沿应用用户开放面映射
func (s *HTTPServer) tenantMemberErr(c *gin.Context, err error) {
	switch {
	case isErr(err, biztenantmember.ErrMemberNotInTenant):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, biztenantmember.ErrLastTenantAdmin):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, biztenantmember.ErrCannotModifySelf):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
	default:
		s.appuserErr(c, err)
	}
}

// ---- 设备只读（归属即准入：列表强制按当前租户过滤，单查先验归属） ----

func (s *HTTPServer) appApiListDevices(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	page, size := s.pageParams(c)
	var req devicesvc.ListRequest
	_ = c.ShouldBindQuery(&req)
	req.TenantID = &tid // 强制当前租户过滤（客户端传值无效）
	list, pg, err := s.device.List(c.Request.Context(), req, page, size)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

// appApiDeviceScoped 取设备并校验归属当前租户（越权 404 语义不泄露存在性）
func (s *HTTPServer) appApiDeviceScoped(c *gin.Context, tid, id uint) (*platformsdk.Device, error) {
	dev, err := s.device.Get(c.Request.Context(), id)
	if err != nil {
		return nil, err
	}
	if dev.TenantID != tid {
		return nil, errDeviceNotInTenant
	}
	return dev, nil
}

func (s *HTTPServer) appApiGetDevice(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	dev, err := s.appApiDeviceScoped(c, tid, id)
	if err != nil {
		s.tenantDeviceErr(c, err)
		return
	}
	response.OK(c, dev)
}

func (s *HTTPServer) appApiGetDeviceShadow(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	if _, err := s.appApiDeviceScoped(c, tid, id); err != nil {
		s.tenantDeviceErr(c, err)
		return
	}
	shadow, err := s.device.Shadow(c.Request.Context(), id)
	if err != nil {
		s.deviceErr(c, err)
		return
	}
	response.OK(c, shadow)
}

func (s *HTTPServer) appApiGetDeviceTelemetry(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	if _, err := s.appApiDeviceScoped(c, tid, id); err != nil {
		s.tenantDeviceErr(c, err)
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

// tenantDeviceErr 设备租户面错误映射：越权 404，其余沿设备代理透传
func (s *HTTPServer) tenantDeviceErr(c *gin.Context, err error) {
	if isErr(err, errDeviceNotInTenant) {
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
		return
	}
	s.deviceErr(c, err)
}
