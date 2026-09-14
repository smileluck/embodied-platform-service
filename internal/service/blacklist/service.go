// Package blacklist 黑名单应用服务（CRUD + 拦截判定入口）
package blacklist

import (
	"context"
	"time"

	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

type Service struct {
	uc *bizblacklist.Usecase
}

func NewService(uc *bizblacklist.Usecase) *Service {
	return &Service{uc: uc}
}

// BlacklistVO 黑名单视图
type BlacklistVO struct {
	ID          uint    `json:"id"`
	IP          string  `json:"ip"`
	Reason      string  `json:"reason"`
	Source      string  `json:"source"`    // manual | auto（登录连续失败自动封禁）
	ExpireAt    *string `json:"expire_at"` // 空字符串表示永久
	CreatorName string  `json:"creator_name"`
	CreatedAt   string  `json:"created_at"`
}

// CreateRequest 新增黑名单请求
type CreateRequest struct {
	IP       string `json:"ip" binding:"required"`
	Reason   string `json:"reason"`
	ExpireAt *int64 `json:"expire_at"` // unix 秒，nil 为永久
}

// Checker 暴露给中间件的黑名单判定器
type Checker interface {
	IsBlocked(ip string) bool
}

// Checker 返回判定器接口
func (s *Service) Checker() Checker {
	return s.uc
}

// LoginGuard 返回登录防护用例（实现 middleware.LoginGuard：临时封禁 / 失败计数 / 限流）
func (s *Service) LoginGuard() *bizblacklist.Usecase {
	return s.uc
}

// Create 新增 IP 黑名单
func (s *Service) Create(ctx context.Context, req CreateRequest, creatorID uint, creatorName string) (*BlacklistVO, error) {
	b := &bizblacklist.IPBlacklist{
		IP: req.IP, Reason: req.Reason,
		CreatorID: creatorID, CreatorName: creatorName,
	}
	if req.ExpireAt != nil {
		t := time.Unix(*req.ExpireAt, 0)
		b.ExpireAt = &t
	}
	if err := s.uc.Create(ctx, b); err != nil {
		return nil, err
	}
	return s.toVO(b), nil
}

// Delete 按 ID 解封
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.uc.Delete(ctx, id)
}

// List 分页查询
func (s *Service) List(ctx context.Context, q bizblacklist.Query, page, pageSize int) ([]*BlacklistVO, pagination.Page, error) {
	list, pg, err := s.uc.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pg, err
	}
	vos := make([]*BlacklistVO, 0, len(list))
	for _, b := range list {
		vos = append(vos, s.toVO(b))
	}
	return vos, pg, nil
}

func (s *Service) toVO(b *bizblacklist.IPBlacklist) *BlacklistVO {
	vo := &BlacklistVO{
		ID: b.ID, IP: b.IP, Reason: b.Reason, Source: b.Source,
		CreatorName: b.CreatorName,
		CreatedAt:   formatTime(b.CreatedAt),
	}
	if b.ExpireAt != nil {
		t := formatTime(*b.ExpireAt)
		vo.ExpireAt = &t
	}
	return vo
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
