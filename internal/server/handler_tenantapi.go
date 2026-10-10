// 租户门户业务端点（/tenant-api/v1）：认证代理（登录/刷新经本服务代理平台公开 API，
// 服务端挂 LoginIPGuard/限流）+ 自身数据（profile/改密）+ 成员自治（RequireTenantPerm
// member:* 精确匹配）+ 设备只读（device:list）。租户上下文一律取自 TenantAuth 自省
// （tid 单租户绑定），不信任客户端参数。
package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	bizauth "github.com/smilex/smilex-admin-gin/internal/biz/auth"
	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/internal/platformsdk"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	devicesvc "github.com/smilex/smilex-admin-gin/internal/service/device"
	tenantmembersvc "github.com/smilex/smilex-admin-gin/internal/service/tenantmember"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// errDeviceNotInTenant 设备不属于当前租户（404 语义，不泄露存在性——与平台开放面越权口径一致）
var errDeviceNotInTenant = errors.New("设备不存在或不属于当前租户")

// tenantCtx 取当前租户平台 ID 与操作者 tenant_user ID（TenantAuth 之后可用）
func tenantCtx(c *gin.Context) (tenantPlatformID uint, actorID uint, ok bool) {
	tn := middleware.TenantAuthTenant(c)
	sub := middleware.TenantAuthSubject(c)
	if tn == nil || sub == nil {
		response.Unauthorized(c, "unauthenticated")
		return 0, 0, false
	}
	return tn.PlatformID, sub.UserID, true
}

// ---- 认证代理（公开：平台 /tenant-api/v1/auth，token 双用于平台与本系统） ----

type tenantPortalLoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=20"`
}

func (s *HTTPServer) tenantPortalLogin(c *gin.Context) {
	var req tenantPortalLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	pair, err := s.tenantIds.TenantLogin(c.Request.Context(), req.Username, req.Password)

	// 成功/失败均落租户登录日志（异步，不阻塞登录响应）；成功时用新 access token
	// 自省平台 profile 补 tid/username（自省失败零值落库，不影响响应）
	l := &bizlog.TenantLoginLog{
		Username:  req.Username,
		IP:        c.ClientIP(),
		UserAgent: truncate(c.GetHeader("User-Agent"), 255),
		Status:    bizlog.LoginStatusSuccess,
	}
	if err != nil {
		l.Status = bizlog.LoginStatusFail
		l.Msg = truncate(loginFailMsg(err), 255)
	} else if sub, perr := s.tenantIds.TenantProfile(c.Request.Context(), pair.AccessToken); perr == nil && sub != nil {
		l.TenantID, l.Username = sub.TenantID, sub.Username
	}
	s.log.RecordTenantLogin(context.Background(), l)

	if err != nil {
		s.platformErr(c, err) // 401 密码错等平台业务错误按状态码透传
		return
	}
	response.OK(c, pair)
}

type tenantPortalRefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (s *HTTPServer) tenantPortalRefresh(c *gin.Context) {
	var req tenantPortalRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	pair, err := s.tenantIds.TenantRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		// 平台 401（token 无效/过期）统一回明确文案，不透传平台内部兜底文案（同管理端 refresh）
		s.refreshProxyErr(c, err)
		return
	}
	response.OK(c, pair)
}

// ---- 自身数据（TenantAuth 之后：profile/改密） ----

// tenantPortalProfile 门户 profile：平台自省主体 + 本地租户投影 + 权限码集合
func (s *HTTPServer) tenantPortalProfile(c *gin.Context) {
	sub := middleware.TenantAuthSubject(c)
	tn := middleware.TenantAuthTenant(c)
	if sub == nil || tn == nil {
		response.Unauthorized(c, "unauthenticated")
		return
	}
	response.OK(c, gin.H{
		"user": gin.H{
			"id": sub.UserID, "username": sub.Username, "nickname": sub.Nickname,
			"tenant_id": sub.TenantID,
		},
		"tenant": gin.H{
			"platform_id": tn.PlatformID, "local_id": tn.ID,
			"name": tn.Name, "code": tn.Code, "status": tn.Status,
		},
		"perm_codes": sub.PermCodes,
	})
}

// tenantPortalChangePassword 本人修改密码（持本人 token 代理平台，平台校验旧密码）
func (s *HTTPServer) tenantPortalChangePassword(c *gin.Context) {
	if middleware.TenantAuthSubject(c) == nil {
		response.Unauthorized(c, "unauthenticated")
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required,min=6,max=64"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=20"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	err := s.tenantIds.TenantChangePassword(c.Request.Context(), middleware.TenantAuthToken(c), req.OldPassword, req.NewPassword)
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

// ---- 成员自治（member:* 权限码；tenant_id 锁定认证主体 tid） ----

func (s *HTTPServer) tenantApiListMembers(c *gin.Context) {
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

// tenantApiListMemberRoles 本租户角色选项（成员表单数据源；member:role:list）
func (s *HTTPServer) tenantApiListMemberRoles(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	roles, err := s.tenantmember.ListRoles(c.Request.Context(), tid)
	if err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, roles)
}

func (s *HTTPServer) tenantApiCreateMember(c *gin.Context) {
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

func (s *HTTPServer) tenantApiUpdateMember(c *gin.Context) {
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

func (s *HTTPServer) tenantApiSetMemberStatus(c *gin.Context) {
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

func (s *HTTPServer) tenantApiResetMemberPassword(c *gin.Context) {
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

func (s *HTTPServer) tenantApiSetMemberRoles(c *gin.Context) {
	tid, actorID, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req tenantmembersvc.SetRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.tenantmember.SetRoles(c.Request.Context(), tid, id, actorID, req.RoleIDs); err != nil {
		s.tenantMemberErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) tenantApiRemoveMember(c *gin.Context) {
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

// tenantMemberErr 成员操作错误映射：本地守卫 404/409/400，其余沿开放面 tenant-user 域映射
func (s *HTTPServer) tenantMemberErr(c *gin.Context, err error) {
	switch {
	case isErr(err, tenantmembersvc.ErrMemberNotInTenant):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, tenantmembersvc.ErrLastTenantAdmin):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, tenantmembersvc.ErrCannotModifySelf):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
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

// ---- 设备只读（device:list；列表强制按当前租户过滤，单查先验归属） ----

func (s *HTTPServer) tenantApiListDevices(c *gin.Context) {
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

// tenantApiDeviceScoped 取设备并校验归属当前租户（越权 404 语义不泄露存在性）
func (s *HTTPServer) tenantApiDeviceScoped(c *gin.Context, tid, id uint) (*platformsdk.Device, error) {
	dev, err := s.device.Get(c.Request.Context(), id)
	if err != nil {
		return nil, err
	}
	if dev.TenantID != tid {
		return nil, errDeviceNotInTenant
	}
	return dev, nil
}

func (s *HTTPServer) tenantApiGetDevice(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	dev, err := s.tenantApiDeviceScoped(c, tid, id)
	if err != nil {
		s.tenantDeviceErr(c, err)
		return
	}
	response.OK(c, dev)
}

func (s *HTTPServer) tenantApiGetDeviceShadow(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	if _, err := s.tenantApiDeviceScoped(c, tid, id); err != nil {
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

func (s *HTTPServer) tenantApiGetDeviceTelemetry(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	if _, err := s.tenantApiDeviceScoped(c, tid, id); err != nil {
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

// ---- 门户日志查询（log:list；tid 强制锁定本租户，不信任客户端参数） ----

func (s *HTTPServer) tenantApiListLoginLogs(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	page, size := s.pageParams(c)
	q := bizlog.TenantLoginLogQuery{TenantID: &tid, Username: strings.TrimSpace(c.Query("username")), IP: strings.TrimSpace(c.Query("ip"))}
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			q.Status = &st
		}
	}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.log.ListTenantLoginLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, logListResult{List: logs, Page: pg, RetentionDays: s.log.RetentionDays()})
}

func (s *HTTPServer) tenantApiListOperationLogs(c *gin.Context) {
	tid, _, ok := tenantCtx(c)
	if !ok {
		return
	}
	page, size := s.pageParams(c)
	q := bizlog.TenantOperationLogQuery{TenantID: &tid, Username: strings.TrimSpace(c.Query("username")), Method: strings.TrimSpace(c.Query("method")), Keyword: strings.TrimSpace(c.Query("kw"))}
	if t, ok := parseUnixParam(c.Query("start")); ok {
		q.Start = t
	}
	if t, ok := parseUnixParam(c.Query("end")); ok {
		q.End = t
	}
	logs, pg, err := s.log.ListTenantOperationLogs(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, logListResult{List: logs, Page: pg, RetentionDays: s.log.RetentionDays()})
}
