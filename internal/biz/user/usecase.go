package user

import (
	"context"
	"errors"
	"strings"

	"github.com/smilex/smilex-admin-gin/pkg/eventbus"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// 用户被创建事件（跨上下文通知，如发欢迎邮件、初始化数据等）
type CreatedEvent struct{ UserID uint }

func (CreatedEvent) Topic() string { return "user.created" }

// SessionRevoker 会话吊销接口（由 session 上下文实现，跨上下文走接口）：
// 禁用/删除用户、管理员重置密码时联动下线其全部会话
type SessionRevoker interface {
	RevokeByUser(ctx context.Context, userID uint) (int, error)
}

// 超级管理员保护：admin（id=1）账号仅允许其本人操作，其余用户一律拒绝
var ErrSuperAdminProtected = errors.New("无权操作超级管理员账号")

// ErrDeleteSuperAdmin 超级管理员账号一律禁止删除（含其本人）
var ErrDeleteSuperAdmin = errors.New("超级管理员账号禁止删除")

// SuperAdminID 超级管理员固定用户 ID（跨上下文保护规则共用：仅本人可操作其账号/会话）
const SuperAdminID uint = 1

type operatorKey struct{}

// WithOperator 将当前操作者 ID 注入 context（由传输层从认证信息中取出）
func WithOperator(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, operatorKey{}, userID)
}

func operatorFrom(ctx context.Context) uint {
	if v, ok := ctx.Value(operatorKey{}).(uint); ok {
		return v
	}
	return 0
}

// guardSuperAdmin 目标为超管且操作者不是超管本人时拒绝
func guardSuperAdmin(ctx context.Context, targetID uint) error {
	if targetID == SuperAdminID && operatorFrom(ctx) != SuperAdminID {
		return ErrSuperAdminProtected
	}
	return nil
}

// Usecase 用户领域用例
type Usecase struct {
	repo     Repo
	sessions SessionRevoker
}

func NewUsecase(repo Repo, sessions SessionRevoker) *Usecase {
	return &Usecase{repo: repo, sessions: sessions}
}

// revokeSessions 联动下线（尽力而为：吊销失败仅告警，不影响主操作结果）
func (uc *Usecase) revokeSessions(ctx context.Context, userID uint, reason string) {
	if uc.sessions == nil {
		return
	}
	if _, err := uc.sessions.RevokeByUser(ctx, userID); err != nil {
		logger.Warn("revoke user sessions failed", zap.String("reason", reason), zap.Uint("user_id", userID), zap.Error(err))
	}
}

func (uc *Usecase) Create(ctx context.Context, username, password, nickname, phone, email string, roleIDs []uint) (*User, error) {
	if _, err := uc.repo.FindByUsername(ctx, username); err == nil {
		return nil, ErrDuplicateUsername
	}
	if strings.TrimSpace(nickname) == "" {
		nickname = username
	}
	u := &User{Username: username, Nickname: nickname, Phone: phone, Email: email, Status: StatusEnabled, RoleIDs: roleIDs}
	if err := u.SetPassword(password); err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	eventbus.Publish(CreatedEvent{UserID: u.ID})
	return u, nil
}

func (uc *Usecase) Update(ctx context.Context, id uint, nickname, phone, email string, status *Status) error {
	if err := guardSuperAdmin(ctx, id); err != nil {
		return err
	}
	u, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if nickname != "" {
		u.Nickname = nickname
	}
	if phone != "" {
		u.Phone = phone
	}
	if email != "" {
		u.Email = email
	}
	if status != nil {
		u.Status = *status
	}
	if err := uc.repo.Update(ctx, u); err != nil {
		return err
	}
	// 禁用联动下线：会话立即失效
	if status != nil && *status == StatusDisabled {
		uc.revokeSessions(ctx, id, "user disabled")
	}
	return nil
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	if id == SuperAdminID {
		return ErrDeleteSuperAdmin
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.revokeSessions(ctx, id, "user deleted")
	return nil
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*User, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*User, pagination.Page, error) {
	users, total, err := uc.repo.List(ctx, q, page, pageSize)
	return users, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// SetRoles 分配角色
func (uc *Usecase) SetRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	if err := guardSuperAdmin(ctx, userID); err != nil {
		return err
	}
	return uc.repo.SetRoles(ctx, userID, roleIDs)
}

// ResetPassword 管理员重置密码（重置后原会话全部下线，需重新登录）
func (uc *Usecase) ResetPassword(ctx context.Context, userID uint, newPlain string) error {
	if err := guardSuperAdmin(ctx, userID); err != nil {
		return err
	}
	u, err := uc.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := u.SetPassword(newPlain); err != nil {
		return err
	}
	if err := uc.repo.UpdatePassword(ctx, userID, string(u.Password)); err != nil {
		return err
	}
	uc.revokeSessions(ctx, userID, "password reset")
	return nil
}
