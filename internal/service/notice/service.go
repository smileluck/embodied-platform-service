// Package notice 通知公告应用服务
package notice

import (
	"context"
	"time"

	biznotice "github.com/smilex/smilex-admin-gin/internal/biz/notice"
)

type Service struct {
	uc *biznotice.Usecase
}

func NewService(uc *biznotice.Usecase) *Service { return &Service{uc: uc} }

// CreateRequest 发布入参
type CreateRequest struct {
	Title     string `json:"title" binding:"required,max=100"`
	Content   string `json:"content" binding:"required,max=8000"`
	Level     string `json:"level" binding:"omitempty,oneof=info warning important"`
	PublishAt string `json:"publish_at"` // RFC3339，空=立即
	ExpireAt  string `json:"expire_at"`  // RFC3339，空=长期
}

type UpdateRequest = CreateRequest

func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) toInput(req CreateRequest) (biznotice.NoticeInput, error) {
	pub, err := parseTime(req.PublishAt)
	if err != nil {
		return biznotice.NoticeInput{}, err
	}
	exp, err := parseTime(req.ExpireAt)
	if err != nil {
		return biznotice.NoticeInput{}, err
	}
	return biznotice.NoticeInput{
		Title: req.Title, Content: req.Content, Level: req.Level,
		PublishAt: pub, ExpireAt: exp,
	}, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest, creatorID uint, creatorName string) (*biznotice.Notice, error) {
	in, err := s.toInput(req)
	if err != nil {
		return nil, err
	}
	return s.uc.Create(ctx, in, creatorID, creatorName)
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	in, err := s.toInput(req)
	if err != nil {
		return err
	}
	return s.uc.Update(ctx, id, in)
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }
func (s *Service) Get(ctx context.Context, id uint) (*biznotice.Notice, error) {
	return s.uc.Get(ctx, id)
}
func (s *Service) List(ctx context.Context, q biznotice.Query, page, pageSize int) ([]*biznotice.Notice, interface{}, error) {
	return s.uc.List(ctx, q, page, pageSize)
}

// ---- 消费端（basic 组，登录即可） ----

func (s *Service) ListActive(ctx context.Context, userID uint) ([]*biznotice.Notice, error) {
	return s.uc.ListActive(ctx, userID)
}

func (s *Service) CountUnread(ctx context.Context, userID uint) (int64, error) {
	return s.uc.CountUnread(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, noticeID uint) error {
	return s.uc.MarkRead(ctx, userID, noticeID)
}
