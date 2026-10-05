// App 面（/app-api/v1）试点端点：AppAuth 之后返回应用用户身份与可访问租户的本地视图。
// 作为 App 直调本系统的授权验收探针；业务端点后续按需求逐个挂入本组。
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// appAPIUserVO App 面用户视图（不含 phone/email——App 端自身资料走平台 /app-auth/profile）
type appAPIUserVO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// appAPITenantVO App 面租户视图：platform_id 为全链路口径（X-Tenant-ID、平台归属集），
// local_id 为本系统内部主键
type appAPITenantVO struct {
	PlatformID uint   `json:"platform_id"`
	LocalID    uint   `json:"local_id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Status     int    `json:"status"`
}

func newAppAPITenantVO(platformID uint, localID uint, name, code string, status int) appAPITenantVO {
	return appAPITenantVO{PlatformID: platformID, LocalID: localID, Name: name, Code: code, Status: status}
}

// appAPIProfileVO App 面身份探针响应
type appAPIProfileVO struct {
	User       appAPIUserVO     `json:"user"`
	Tenant     appAPITenantVO   `json:"tenant"`             // 本次请求上下文租户（X-Tenant-ID 解析命中）
	Accessible []appAPITenantVO `json:"accessible_tenants"` // 归属 ∩ 本地已同步
}

// appApiProfile GET /app-api/v1/profile：App 用户 + 可访问租户（本地视图）
func (s *HTTPServer) appApiProfile(c *gin.Context) {
	sub := middleware.AppAuthSubject(c)
	if sub == nil {
		response.FailI18n(c, http.StatusUnauthorized, response.CodeUnauthorized, nil)
		return
	}
	vo := appAPIProfileVO{
		User:       appAPIUserVO{ID: sub.UserID, Username: sub.Username, Nickname: sub.Nickname},
		Accessible: []appAPITenantVO{},
	}
	if tn := middleware.AppAuthTenant(c); tn != nil {
		vo.Tenant = newAppAPITenantVO(tn.PlatformID, tn.ID, tn.Name, tn.Code, int(tn.Status))
	}
	for _, pid := range sub.TenantIDs {
		tn, err := s.tenantUC.GetByPlatformID(c.Request.Context(), pid)
		if err != nil || tn == nil {
			continue // 本地未同步的归属租户不出现在本地视图
		}
		vo.Accessible = append(vo.Accessible, newAppAPITenantVO(tn.PlatformID, tn.ID, tn.Name, tn.Code, int(tn.Status)))
	}
	response.OK(c, vo)
}
