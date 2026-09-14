// Package auth 认证限界上下文 —— 领域层
package auth

import (
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/user"
)

// Subject 认证主体（JWT 载荷）
type Subject struct {
	UserID    uint
	Username  string
	SessionID string // 会话 ID：中间件据此校验会话存活，吊销即失效
	Roles     []string
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenIssuer 令牌签发接口（由 infrastructure/auth 实现，依赖倒置）
type TokenIssuer interface {
	IssueAccessToken(s Subject) (token string, expiresAt time.Time, err error)
	IssueRefreshToken(s Subject) (token string, err error)
	ParseAccessToken(token string) (*Subject, error)
	ParseRefreshToken(token string) (*Subject, error)
}

// Profile 个人中心聚合视图：用户 + 角色名 + 权限点
type Profile struct {
	User        *user.User
	RoleNames   []string
	Permissions []*permission.Permission
}
