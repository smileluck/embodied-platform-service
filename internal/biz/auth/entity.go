// Package auth 认证限界上下文 —— 领域层。
//
// 平台（embodied-platform）是唯一身份源：登录在本系统前端直调平台
// POST /api/v1/auth/login（token 双用），本系统后端不再签发本地令牌、
// 不再保存密码、不再维护服务端会话。本上下文只做两件事：
//   - token 自省（introspection）：拿平台 token 调 GET /auth/profile 验证并取身份
//   - 本地准入/权限组合判定：平台身份 × 准入投影 × 本地 RBAC
package auth

import (
	"context"
	"errors"

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

// IdentitySource 平台身份源接口（data 层实现，依赖倒置）：
// Profile 为自省入口；UpdateProfile/ChangePassword 为「用户本人 token」的
// 平台自身数据代理（本系统不落任何凭证，仅转发）
type IdentitySource interface {
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
