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

// NoticeInput 写入参数（更新时空字符串/零值=保持原值；ExpireAt nil=保持，非 nil 零值=长期；
// Scope 空=保持原范围与目标，非空则整体替换范围+目标）
type NoticeInput struct {
	Title     string
	Content   string
	Level     string
	Scope     string
	RoleIDs   []uint
	UserIDs   []uint
	PublishAt *time.Time
	ExpireAt  *time.Time
}

// normalizeScope 归一化范围：非法值回落全体；定向范围必须至少一个目标
func normalizeScope(scope string, roleIDs, userIDs []uint) (Scope, []uint, []uint, error) {
	switch scope {
	case string(ScopeRoles):
		if len(roleIDs) == 0 {
			return "", nil, nil, ErrInvalidTargets
		}
		return ScopeRoles, dedupeIDs(roleIDs), nil, nil
	case string(ScopeUsers):
		if len(userIDs) == 0 {
			return "", nil, nil, ErrInvalidTargets
		}
		return ScopeUsers, nil, dedupeIDs(userIDs), nil
	default: // 空/未知值一律全体广播
		return ScopeAll, nil, nil, nil
	}
}

func dedupeIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (uc *Usecase) Create(ctx context.Context, in NoticeInput, creatorID uint, creatorName string) (*Notice, error) {
	if !ValidLevel(in.Level) {
		in.Level = string(LevelInfo)
	}
	scope, roleIDs, userIDs, err := normalizeScope(in.Scope, in.RoleIDs, in.UserIDs)
	if err != nil {
		return nil, err
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
		Scope: scope, RoleIDs: roleIDs, UserIDs: userIDs,
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
	// 范围非空才整体替换（含目标）；空=保持原范围与目标
	if in.Scope != "" {
		scope, roleIDs, userIDs, err := normalizeScope(in.Scope, in.RoleIDs, in.UserIDs)
		if err != nil {
			return err
		}
		n.Scope, n.RoleIDs, n.UserIDs = scope, roleIDs, userIDs
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

// ListActive 消费端：生效中且送达本人的公告（带本人已读标记）
func (uc *Usecase) ListActive(ctx context.Context, userID uint) ([]*Notice, error) {
	return uc.repo.ListActive(ctx, userID)
}

// CountUnread 未读数
func (uc *Usecase) CountUnread(ctx context.Context, userID uint) (int64, error) {
	return uc.repo.CountUnread(ctx, userID)
}

// MarkRead 已读上报（公告须存在、生效且送达本人；定向公告由仓储层校验送达，防越权探测/写入）
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

// ListRoleOptions / ListUserOptions 发布表单送达范围选项
func (uc *Usecase) ListRoleOptions(ctx context.Context) ([]RoleOption, error) {
	return uc.repo.ListRoleOptions(ctx)
}

func (uc *Usecase) ListUserOptions(ctx context.Context, kw string, limit int) ([]UserOption, error) {
	kw = strings.TrimSpace(kw)
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	return uc.repo.ListUserOptions(ctx, kw, limit)
}
