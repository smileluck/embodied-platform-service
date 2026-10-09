// 租户门户（第四套身份 tenant_users）平台身份源：TenantAuth 中间件的领域主体与接口。
// 门户登录/刷新经本服务后端代理平台 /tenant-api/v1/auth（token 双用于平台与本系统），
// 本系统对门户侧不碰账密、不签发、不换签、无本地投影（授权=平台租户作用域 RBAC
// 的 perm_codes 随自省下发，RequireTenantPerm 精确匹配）。
package auth

import "context"

// TenantSubject 租户门户认证主体（来自平台 GET /tenant-api/v1/profile 自省）。
// UserID 为平台 tenant_user ID；TenantID 为平台租户 ID（单租户绑定，全链路唯一口径）；
// PermCodes 为平台租户 RBAC 汇总后的权限码集合。
// 经 tnt: 缓存 JSON 往返，新增字段必须有 json tag 才能穿透缓存。
type TenantSubject struct {
	UserID    uint     `json:"user_id"`
	Username  string   `json:"username"`
	Nickname  string   `json:"nickname"`
	TenantID  uint     `json:"tenant_id"`
	PermCodes []string `json:"perm_codes"`
}

// TenantIdentitySource 平台租户门户身份源（data 层实现，依赖倒置）：
// TenantLogin/TenantRefresh 为门户公开 API 代理（本服务不落任何凭证，仅转发，
// 平台业务错误以 *platform.Error 透传 msg）；TenantProfile 为自省入口；
// TenantChangePassword 为「用户本人 token」的平台自身数据代理。
type TenantIdentitySource interface {
	TenantLogin(ctx context.Context, username, password string) (*TokenPair, error)
	TenantRefresh(ctx context.Context, refreshToken string) (*TokenPair, error)
	TenantProfile(ctx context.Context, token string) (*TenantSubject, error)
	TenantChangePassword(ctx context.Context, token, oldPassword, newPassword string) error
}
