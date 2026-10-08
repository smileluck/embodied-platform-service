package auth

import (
	"context"
)

// Usecase 认证领域用例（平台身份自省 + 本地准入/权限组合）
type Usecase struct {
	identity  IdentitySource
	admission AdmissionReader
	perms     PermissionReader
	roles     RoleNameReader
}

func NewUsecase(identity IdentitySource, admission AdmissionReader, perms PermissionReader, roles RoleNameReader) *Usecase {
	return &Usecase{identity: identity, admission: admission, perms: perms, roles: roles}
}

// Introspect 平台 token 自省（中间件用；缓存策略由传输层/基础设施决定）
func (uc *Usecase) Introspect(ctx context.Context, token string) (*Subject, error) {
	return uc.identity.Profile(ctx, token)
}

// Login 代理平台登录（透传平台令牌对与业务错误；登录日志记录由 handler 层完成）
func (uc *Usecase) Login(ctx context.Context, username, password, captchaID, captchaCode string) (*TokenPair, error) {
	return uc.identity.Login(ctx, username, password, captchaID, captchaCode)
}

// Refresh 代理平台刷新令牌
func (uc *Usecase) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	return uc.identity.Refresh(ctx, refreshToken)
}

// Logout 代理平台登出（吊销平台侧会话）
func (uc *Usecase) Logout(ctx context.Context, token string) error {
	return uc.identity.Logout(ctx, token)
}

// Captcha 取平台登录验证码（开关关时返回 enabled=false）
func (uc *Usecase) Captcha(ctx context.Context) (*CaptchaInfo, error) {
	return uc.identity.Captcha(ctx)
}

// Authorize RBAC 接口鉴权：未准入直接拒绝；已准入按本地权限点匹配
func (uc *Usecase) Authorize(ctx context.Context, platformUserID uint, method, path string) bool {
	p, err := uc.admission.Admission(ctx, platformUserID)
	if err != nil || p == nil || !p.Enabled {
		return false
	}
	ps, err := uc.perms.FindByUserID(ctx, platformUserID)
	if err != nil {
		return false
	}
	for _, p := range ps {
		if p.Match(method, path) {
			return true
		}
	}
	return false
}

// Profile 个人中心：平台身份 + 准入状态 + 角色名 + 本地权限点
func (uc *Usecase) Profile(ctx context.Context, sub *Subject) (*Profile, error) {
	p := &Profile{Subject: sub}
	if proj, err := uc.admission.Admission(ctx, sub.UserID); err == nil && proj != nil {
		p.Admitted = proj.Enabled
		if len(proj.RoleIDs) > 0 {
			// 角色名仅用于展示，读取失败时降级为空列表，不阻断个人中心
			if names, err := uc.roles.FindNamesByIDs(ctx, proj.RoleIDs); err == nil {
				p.RoleNames = names
			}
		}
	}
	if ps, err := uc.perms.FindByUserID(ctx, sub.UserID); err == nil {
		p.Permissions = ps
	}
	return p, nil
}

// UpdateProfile 代理平台「本人更新昵称/邮箱」（用户本人 token）
func (uc *Usecase) UpdateProfile(ctx context.Context, token, nickname, email string) error {
	return uc.identity.UpdateProfile(ctx, token, nickname, email)
}

// ChangePassword 代理平台「本人修改密码」（平台侧校验旧密码并吊销其他端会话）
func (uc *Usecase) ChangePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	return uc.identity.ChangePassword(ctx, token, oldPassword, newPassword)
}
