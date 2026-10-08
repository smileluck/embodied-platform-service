// Package auth 认证限界上下文 —— 领域层。
//
// 平台（embodied-platform）是唯一身份源：账号密码、token 签发/刷新/吊销、验证码
// 均在平台侧。管理端登录经本服务后端代理平台公开 API（本服务不保存密码、
// 不维护服务端会话，仅转发——代理使服务端得以落登录日志）。本上下文职责：
//   - 登录/刷新/登出/验证码代理（透传平台结果与业务错误 msg）
//   - token 自省（introspection）：拿平台 token 调 GET /auth/profile 验证并取身份
//   - 本地准入/权限组合判定：平台身份 × 准入投影 × 本地 RBAC
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
)

var (
	// ErrInvalidToken 平台判定 token 无效/过期（回 401，前端走平台刷新流程）
	ErrInvalidToken = errors.New("token invalid or expired")
	// ErrPlatformUnavailable 平台不可达（fail-closed，回 503）
	ErrPlatformUnavailable = errors.New("platform unavailable")
)

// Subject 认证主体（平台身份；UserID 为平台用户 ID）。
// 经 pid: 缓存 JSON 往返，新增字段必须有 json tag 才能穿透缓存。
type Subject struct {
	UserID    uint                    `json:"user_id"`
	Username  string                  `json:"username"`
	Nickname  string                  `json:"nickname"`
	Email     string                  `json:"email"`
	Merchants []admission.MerchantRef `json:"merchants"` // 本人已准入商户（含商户管理员标记）
}

// TokenPair 平台令牌对（登录/刷新代理的返回值；expires_at 为 RFC3339 序列化）
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// CaptchaInfo 登录验证码开关与图文（captcha_image 为 PNG base64，无 data: 前缀）
type CaptchaInfo struct {
	Enabled      bool   `json:"enabled"`
	CaptchaID    string `json:"captcha_id"`
	CaptchaImage string `json:"captcha_image"`
}

// IdentitySource 平台身份源接口（data 层实现，依赖倒置）：
// Login/Refresh/Logout/Captcha 为平台公开 API 代理（本服务不落任何凭证，仅转发；
// 平台业务错误以 *platform.Error 透传 msg 供登录日志与前端回显）；
// Profile 为自省入口；UpdateProfile/ChangePassword 为「用户本人 token」的
// 平台自身数据代理
type IdentitySource interface {
	Login(ctx context.Context, username, password, captchaID, captchaCode string) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, token string) error
	Captcha(ctx context.Context) (*CaptchaInfo, error)
	Profile(ctx context.Context, token string) (*Subject, error)
	UpdateProfile(ctx context.Context, token, nickname, email string) error
	ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error
}

// AdmissionReader 本地准入读取（由 admission 上下文实现，跨上下文最小接口）
type AdmissionReader interface {
	// Admission 返回平台用户的准入投影；未建投影返回 nil
	Admission(ctx context.Context, platformUserID uint) (*admission.Projection, error)
}

// PermissionReader 权限读取接口（permission 上下文实现；按平台用户 ID 联查）
type PermissionReader interface {
	FindByUserID(ctx context.Context, platformUserID uint) ([]*permission.Permission, error)
}

// RoleNameReader 角色名读取接口（个人中心展示角色名）
type RoleNameReader interface {
	FindNamesByIDs(ctx context.Context, ids []uint) ([]string, error)
}

// Profile 个人中心聚合视图：平台身份 + 准入状态 + 角色名 + 本地权限点
type Profile struct {
	Subject     *Subject
	Admitted    bool
	RoleNames   []string
	Permissions []*permission.Permission
}
