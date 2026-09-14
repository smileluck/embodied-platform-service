// Package appuser 应用用户（多租户下的终端用户）限界上下文 —— 领域层
package appuser

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Status 应用用户状态值对象
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// passwordCost bcrypt 成本因子（与系统用户一致的强度约定）
const passwordCost = 12

// AppUser 应用用户聚合根（纯 Go，不依赖任何框架）。
// PasswordHash 只存 bcrypt 哈希，永不输出；TenantIDs/TenantNames 为聚合的租户关联。
type AppUser struct {
	ID           uint      `json:"id"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	Phone        string    `json:"phone"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Status       Status    `json:"status"`
	TenantIDs    []uint    `json:"tenant_ids"`
	TenantNames  []string  `json:"tenant_names"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Enabled 是否启用
func (u *AppUser) Enabled() bool { return u.Status == StatusEnabled }

// SetPassword 设置密码（只落 bcrypt 哈希）
func (u *AppUser) SetPassword(plain string) error {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), passwordCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(b)
	return nil
}

// CheckPassword 校验明文密码
func (u *AppUser) CheckPassword(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plain)) == nil
}

// TokenPair 应用用户令牌对（与后台账号体系相互隔离）
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenIssuer 应用用户令牌签发接口（typ 为 app-access/app-refresh，由 data 层 jwtIssuer 实现，依赖倒置）。
// 不携带会话 ID（sid 留空），应用用户无服务端会话状态。
type TokenIssuer interface {
	IssueAppAccessToken(uid uint, username string) (token string, expiresAt time.Time, err error)
	IssueAppRefreshToken(uid uint, username string) (token string, err error)
	ParseAppAccessToken(token string) (uid uint, username string, err error)
	ParseAppRefreshToken(token string) (uid uint, username string, err error)
}
