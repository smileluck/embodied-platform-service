package blacklist

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// Usecase IP 黑名单领域用例（含登录防护：临时封禁 / 失败计数 / 限流，状态经 LoginProtector 存 Redis）
type Usecase struct {
	repo      Repo
	protector LoginProtector
}

func NewUsecase(repo Repo, protector LoginProtector) *Usecase {
	return &Usecase{repo: repo, protector: protector}
}

// Create 新增 IP 黑名单（手工）
func (uc *Usecase) Create(ctx context.Context, b *IPBlacklist) error {
	b.IP = strings.TrimSpace(b.IP)
	parsed := net.ParseIP(b.IP)
	if parsed == nil {
		return ErrInvalidIP
	}
	b.IP = parsed.String()
	if b.ExpireAt != nil && !b.ExpireAt.After(time.Now()) {
		return ErrInvalidExpire
	}
	b.Source = SourceManual
	if err := uc.repo.Create(ctx, b); err != nil {
		return err
	}
	// 写穿缓存：DB 已成功，缓存失败仅记日志（下次加载回源 DB 自愈）
	if err := uc.repo.CacheAdd(ctx, b.IP); err != nil {
		logger.Warn("blacklist cache add failed", zap.String("ip", b.IP), zap.Error(err))
	}
	return nil
}

// Delete 解封指定 ID：删 DB 记录，并联动清理 Redis（读缓存 + 临时封禁 + 失败计数）
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	b, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := uc.repo.CacheRemove(ctx, b.IP); err != nil {
		logger.Warn("blacklist cache remove failed", zap.String("ip", b.IP), zap.Error(err))
	}
	// 自动封禁记录被提前解封时，同时清掉 Redis 临时封禁状态、失败计数与限流计数
	if err := uc.protector.ClearTempBan(ctx, b.IP); err != nil {
		logger.Warn("clear temp ban failed", zap.String("ip", b.IP), zap.Error(err))
	}
	if err := uc.protector.ResetFail(ctx, b.IP); err != nil {
		logger.Warn("reset login fail counter failed", zap.String("ip", b.IP), zap.Error(err))
	}
	if err := uc.protector.ResetRate(ctx, b.IP); err != nil {
		logger.Warn("reset login rate counter failed", zap.String("ip", b.IP), zap.Error(err))
	}
	return nil
}

// List 分页查询
func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*IPBlacklist, pagination.Page, error) {
	list, total, err := uc.repo.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// IsBlocked 判定 IP 是否在生效的手工黑名单中（Redis 缓存 + DB 回源；故障 fail-open 放行）
func (uc *Usecase) IsBlocked(ip string) bool {
	blocked, err := uc.repo.IsBlocked(context.Background(), ip)
	if err != nil {
		logger.Warn("blacklist check failed, fail-open", zap.String("ip", ip), zap.Error(err))
		return false
	}
	return blocked
}

// ---- 登录防护（供登录中间件调用；Redis 故障一律 fail-open 并记日志） ----

// TempBanRemaining 返回临时封禁剩余时间；ok=false 表示未被封禁
func (uc *Usecase) TempBanRemaining(ip string) (time.Duration, bool) {
	d, err := uc.protector.TempBanRemaining(context.Background(), ip)
	if err != nil {
		logger.Warn("temp ban check failed, fail-open", zap.String("ip", ip), zap.Error(err))
		return 0, false
	}
	return d, d > 0
}

// RecordLoginFail 登录失败计数 +1；窗口内达阈值则临时封禁并落库留痕（管理页可见、可提前解封）
func (uc *Usecase) RecordLoginFail(ctx context.Context, ip string) {
	n, err := uc.protector.IncrFail(ctx, ip, LoginFailWindow)
	if err != nil {
		logger.Warn("login fail counter failed", zap.String("ip", ip), zap.Error(err))
		return
	}
	if n < LoginFailThreshold {
		return
	}
	if err := uc.protector.TempBan(ctx, ip, TempBanDuration); err != nil {
		logger.Warn("temp ban failed", zap.String("ip", ip), zap.Error(err))
		return
	}
	_ = uc.protector.ResetFail(ctx, ip)
	expire := time.Now().Add(TempBanDuration)
	b := &IPBlacklist{IP: ip, Reason: AutoBanReason, Source: SourceAuto, ExpireAt: &expire, CreatorName: "system"}
	if err := uc.repo.UpsertAutoBan(ctx, b); err != nil {
		logger.Warn("auto ban persist failed", zap.String("ip", ip), zap.Error(err))
	}
	logger.Info("ip temp banned", zap.String("ip", ip), zap.Int64("fails", n))
}

// ResetLoginFail 登录成功清空失败计数
func (uc *Usecase) ResetLoginFail(ctx context.Context, ip string) {
	if err := uc.protector.ResetFail(ctx, ip); err != nil {
		logger.Warn("reset login fail counter failed", zap.String("ip", ip), zap.Error(err))
	}
}

// HitLoginRate 登录限流计数；返回 true 表示超出窗口上限应拒绝
func (uc *Usecase) HitLoginRate(ctx context.Context, ip string) bool {
	n, err := uc.protector.IncrRate(ctx, ip, LoginRateWindow)
	if err != nil {
		logger.Warn("login rate limit failed, fail-open", zap.String("ip", ip), zap.Error(err))
		return false
	}
	return n > LoginRateMax
}
