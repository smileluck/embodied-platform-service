// Package session 会话应用服务（在线用户管理）。
package session

import (
	"context"
	"time"

	bizsession "github.com/smilex/smilex-admin-gin/internal/biz/session"
	bizuser "github.com/smilex/smilex-admin-gin/internal/biz/user"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

type Service struct {
	uc *bizsession.Usecase
}

func NewService(uc *bizsession.Usecase) *Service {
	return &Service{uc: uc}
}

// OnlineVO 在线会话视图（一行 = 一个「用户 × 设备端」会话）
type OnlineVO struct {
	SID          string `json:"sid"`
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Device       string `json:"device"` // web / app
	IP           string `json:"ip"`
	UserAgent    string `json:"user_agent"`
	LoginAt      string `json:"login_at"`
	LastActiveAt string `json:"last_active_at"`
	IsCurrent    bool   `json:"is_current"` // 是否当前操作者自己的会话
}

func toVO(s *bizsession.Session, currentSid string) *OnlineVO {
	return &OnlineVO{
		SID: s.ID, UserID: s.UserID, Username: s.Username, Nickname: s.Nickname,
		Device: s.Device, IP: s.IP, UserAgent: s.UserAgent,
		LoginAt:      formatTime(s.LoginAt),
		LastActiveAt: formatTime(s.LastActiveAt),
		IsCurrent:    s.ID == currentSid,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// List 在线会话列表
func (s *Service) List(ctx context.Context, q bizsession.Query, page, pageSize int, currentSid string) ([]*OnlineVO, pagination.Page, error) {
	sessions, pg, err := s.uc.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pg, err
	}
	vos := make([]*OnlineVO, 0, len(sessions))
	for _, sess := range sessions {
		vos = append(vos, toVO(sess, currentSid))
	}
	return vos, pg, nil
}

// Kick 下线单个会话（踢某端）；超管会话仅超管本人可操作
func (s *Service) Kick(ctx context.Context, sid string, operatorID uint) error {
	sess, err := s.uc.Get(ctx, sid)
	if err != nil {
		return err
	}
	if err := guardSuperAdmin(sess.UserID, operatorID); err != nil {
		return err
	}
	return s.uc.Revoke(ctx, sid)
}

// KickUser 下线用户全部会话（踢个人下线）；超管仅本人可操作
func (s *Service) KickUser(ctx context.Context, userID, operatorID uint) (int, error) {
	if err := guardSuperAdmin(userID, operatorID); err != nil {
		return 0, err
	}
	return s.uc.RevokeByUser(ctx, userID)
}

// guardSuperAdmin 目标为超管且操作者不是超管本人时拒绝（与用户上下文保护规则一致）
func guardSuperAdmin(targetID, operatorID uint) error {
	if targetID == bizuser.SuperAdminID && operatorID != bizuser.SuperAdminID {
		return bizuser.ErrSuperAdminProtected
	}
	return nil
}
