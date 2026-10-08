// 应用用户（C 端）平台身份源：AppAuth 中间件的领域主体与接口。
// 与 B 端 Subject 分离：App 登录/刷新发生在 App 直连平台 /app-auth（token 双用于
// 平台与本系统），本系统对 C 端不碰账密、不签发、不换签、无本地投影与 RBAC
// （归属即准入：平台 app_user_tenants 归属即权限）。
package auth

import "context"

// AppSubject 应用用户认证主体（来自平台 GET /api/v1/app-auth/profile 的应用用户视图）。
// UserID 为平台 app_user ID；TenantIDs 为平台租户 ID 集。
// 经 pat: 缓存 JSON 往返，新增字段必须有 json tag 才能穿透缓存。
type AppSubject struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Status    int    `json:"status"`
	TenantIDs []uint `json:"tenant_ids"`
}

// AppIdentitySource 平台应用用户身份源（data 层实现，依赖倒置）：
// AppProfile 为 app-access token 自省入口；AppChangePassword 为本人改密代理
// （校验旧密码，平台侧成功后吊销其他端会话）。
type AppIdentitySource interface {
	AppProfile(ctx context.Context, token string) (*AppSubject, error)
	AppChangePassword(ctx context.Context, token, oldPassword, newPassword string) error
}
