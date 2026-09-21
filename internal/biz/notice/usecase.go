package notice

import (
	"context"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 通知公告用例
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase { return &Usecase{repo: repo} }

// NoticeInput 写入参数（更新时空字符串/零值=保持原值；ExpireAt nil=保持，非 nil 零值=长期）
type NoticeInput struct {
	Title     string
	Content   string
	Level     string
	PublishAt *time.Time
	ExpireAt  *time.Time
}

func (uc *Usecase) Create(ctx context.Context, in NoticeInput, creatorID uint, creatorName string) (*Notice, error) {
	if !ValidLevel(in.Level) {
		in.Level = string(LevelInfo)
	}
	if in.PublishAt == nil {
		t := time.Now()
		in.PublishAt = &t
	}
	// 过期时间必须晚于发布时间，否则同一公告会同时处于「待发布」与「已过期」
	if in.ExpireAt != nil && !in.ExpireAt.After(*in.PublishAt) {
		return nil, ErrTimeRange
	}
	n := &Notice{
		Title: in.Title, Content: in.Content, Level: Level(in.Level),
		PublishAt: *in.PublishAt, ExpireAt: in.ExpireAt,
		CreatorID: creatorID, CreatorName: creatorName,
	}
	if err := uc.repo.Create(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (uc *Usecase) Update(ctx context.Context, id uint, in NoticeInput) error {
	n, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if in.Title != "" {
		n.Title = in.Title
	}
	if in.Content != "" {
		n.Content = in.Content
	}
	if ValidLevel(in.Level) {
		n.Level = Level(in.Level)
	}
	if in.PublishAt != nil {
		n.PublishAt = *in.PublishAt
	}
	if in.ExpireAt != nil {
		n.ExpireAt = in.ExpireAt
	}
	// 校验合并后的完整时间窗（更新可能只改其一）
	if n.ExpireAt != nil && !n.ExpireAt.After(n.PublishAt) {
		return ErrTimeRange
	}
	return uc.repo.Update(ctx, n)
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Notice, error) {
	return uc.repo.Find(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Notice, pagination.Page, error) {
	q.Title = strings.TrimSpace(q.Title)
	list, total, err := uc.repo.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// ListActive 消费端：生效中公告（带本人已读标记）
func (uc *Usecase) ListActive(ctx context.Context, userID uint) ([]*Notice, error) {
	return uc.repo.ListActive(ctx, userID)
}

// CountUnread 未读数
func (uc *Usecase) CountUnread(ctx context.Context, userID uint) (int64, error) {
	return uc.repo.CountUnread(ctx, userID)
}

// MarkRead 已读上报（公告须存在且生效）
func (uc *Usecase) MarkRead(ctx context.Context, userID, noticeID uint) error {
	n, err := uc.repo.Find(ctx, noticeID)
	if err != nil {
		return err
	}
	if !n.Active() {
		return ErrInvalidTitle
	}
	return uc.repo.MarkRead(ctx, userID, noticeID)
}
