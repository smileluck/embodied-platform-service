package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// touch 节流：同一会话最近活跃落库间隔（避免每个请求都写 Redis）
const touchInterval = time.Minute

// Usecase 会话领域用例
type Usecase struct {
	repo Repo
	ttl  time.Duration // 会话生命周期，与 refresh token 有效期一致
}

func NewUsecase(repo Repo, c *conf.Bootstrap) *Usecase {
	return &Usecase{repo: repo, ttl: time.Duration(c.JWT.RefreshHours) * time.Hour}
}

// Create 建立会话（同端旧会话由仓储层互斥吊销），返回带 sid 的会话
func (uc *Usecase) Create(ctx context.Context, userID uint, username, nickname, device, ip, userAgent string) (*Session, error) {
	sid, err := newSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	s := &Session{
		ID: sid, UserID: userID, Username: username, Nickname: nickname,
		Device: NormalizeDevice(device), IP: ip, UserAgent: userAgent,
		LoginAt: now, LastActiveAt: now, ExpiresAt: now.Add(uc.ttl),
	}
	if err := uc.repo.Save(ctx, s, uc.ttl); err != nil {
		return nil, err
	}
	return s, nil
}

// Validate 会话是否存活；存活时顺带节流刷新最近活跃（节流状态存 Redis，多实例共享）
func (uc *Usecase) Validate(ctx context.Context, sid string) bool {
	if sid == "" {
		return false
	}
	if _, err := uc.repo.Find(ctx, sid); err != nil {
		return false
	}
	_ = uc.repo.TouchIfDue(ctx, sid, touchInterval)
	return true
}

// Get 读取单个会话（在线管理用）
func (uc *Usecase) Get(ctx context.Context, sid string) (*Session, error) {
	return uc.repo.Find(ctx, sid)
}

// Extend 会话续期（refresh token 轮转成功时调用）
func (uc *Usecase) Extend(ctx context.Context, sid string) error {
	return uc.repo.Extend(ctx, sid, uc.ttl)
}

// Revoke 吊销单个会话（登出 / 踢单端下线）
func (uc *Usecase) Revoke(ctx context.Context, sid string) error {
	return uc.repo.Revoke(ctx, sid)
}

// RevokeByUser 吊销用户全部会话（踢人下线 / 禁用 / 删除），返回吊销数
func (uc *Usecase) RevokeByUser(ctx context.Context, userID uint) (int, error) {
	return uc.revokeByUser(ctx, userID, "")
}

// RevokeByUserExcept 吊销用户除 keepSid 外的全部会话（本人改密：当前端不掉线）
func (uc *Usecase) RevokeByUserExcept(ctx context.Context, userID uint, keepSid string) (int, error) {
	return uc.revokeByUser(ctx, userID, keepSid)
}

func (uc *Usecase) revokeByUser(ctx context.Context, userID uint, keepSid string) (int, error) {
	sids, err := uc.repo.FindSidsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sid := range sids {
		if sid == keepSid {
			continue
		}
		if err := uc.Revoke(ctx, sid); err == nil {
			n++
		}
	}
	return n, nil
}

// List 在线会话列表（过滤 + 内存分页；会话量级为「活跃用户 × 端数」，无需下推数据库）
func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Session, pagination.Page, error) {
	all, err := uc.repo.ListAll(ctx)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	filtered := make([]*Session, 0, len(all))
	for _, s := range all {
		if q.Username != "" && !strings.Contains(s.Username, q.Username) {
			continue
		}
		if q.Device != "" && s.Device != NormalizeDevice(q.Device) {
			continue
		}
		filtered = append(filtered, s)
	}
	// 按登录时间倒序，新会话在前
	for i := 1; i < len(filtered); i++ {
		for j := i; j > 0 && filtered[j].LoginAt.After(filtered[j-1].LoginAt); j-- {
			filtered[j], filtered[j-1] = filtered[j-1], filtered[j]
		}
	}
	total := len(filtered)
	// page_size=0 约定为全量返回（与用户/角色列表一致）
	if pageSize <= 0 {
		return filtered, pagination.Page{Page: page, PageSize: pageSize, Total: int64(total)}, nil
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []*Session{}, pagination.Page{Page: page, PageSize: pageSize, Total: int64(total)}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], pagination.Page{Page: page, PageSize: pageSize, Total: int64(total)}, nil
}

// newSessionID 生成会话 ID：16 字节密码学随机数 hex（32 字符）
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
