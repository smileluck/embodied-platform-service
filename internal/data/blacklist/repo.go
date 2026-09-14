// Package blacklist 黑名单仓储 GORM + Redis 实现
//
// Key 设计：
//
//	bl:ips        SET     生效手工封禁 IP 镜像（write-through 维护，未加载时回源 DB 重建）
//	bl:loaded     STRING  镜像已加载标记（TTL 自愈，防止 Redis 数据丢失后长期漏判）
//	bl:ban:{ip}   STRING  登录临时封禁标记（TTL = 封禁时长，到期自动解封）
//	bl:fail:{ip}  STRING  登录失败计数（固定窗口 TTL）
//	bl:rl:{ip}    STRING  登录限流计数（固定窗口 TTL）
package blacklist

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	keyIPSet  = "bl:ips"
	keyLoaded = "bl:loaded"
	loadedTTL = time.Minute // 镜像标记 TTL：到期后下次判定回源重建
)

func banKey(ip string) string  { return "bl:ban:" + ip }
func failKey(ip string) string { return "bl:fail:" + ip }
func rateKey(ip string) string { return "bl:rl:" + ip }

// Repo 黑名单仓储
type Repo struct {
	data *data.Data
	rdb  *redis.Client
}

func NewRepo(d *data.Data, rdb *redis.Client) *Repo {
	return &Repo{data: d, rdb: rdb}
}

func (r *Repo) Create(ctx context.Context, b *bizblacklist.IPBlacklist) error {
	po := model.IPBlacklistToPO(b)
	po.CreatedAt = time.Now()
	po.UpdatedAt = po.CreatedAt
	return r.data.DB.WithContext(ctx).Create(po).Error
}

func (r *Repo) Delete(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Delete(&model.IPBlacklistPO{}, id).Error
}

func (r *Repo) Get(ctx context.Context, id uint) (*bizblacklist.IPBlacklist, error) {
	var po model.IPBlacklistPO
	err := r.data.DB.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, bizblacklist.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.IPBlacklistFromPO(&po), nil
}

// purgeExpired 惰性清理：物理删除已过期的封禁记录（Redis 封禁键已随 TTL 消失，DB 记录同步清除）；
// 随 List 调用触发（管理页低频），失败仅告警不影响查询
func (r *Repo) purgeExpired(ctx context.Context) {
	res := r.data.DB.WithContext(ctx).Unscoped().
		Where("expire_at IS NOT NULL AND expire_at < ?", time.Now()).
		Delete(&model.IPBlacklistPO{})
	if res.Error != nil {
		logger.Warn("blacklist purge expired failed", zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		logger.Info("blacklist expired records purged", zap.Int64("deleted", res.RowsAffected))
	}
}

func (r *Repo) List(ctx context.Context, q bizblacklist.Query, page, pageSize int) ([]*bizblacklist.IPBlacklist, int64, error) {
	r.purgeExpired(ctx)
	tx := r.data.DB.WithContext(ctx).Model(&model.IPBlacklistPO{})
	if q.IP != "" {
		tx = tx.Where("ip LIKE ? ESCAPE '/'", security.EscapeLike(q.IP)+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.IPBlacklistPO
	tx = tx.Order("id DESC")
	if pageSize > 0 {
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := tx.Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*bizblacklist.IPBlacklist, 0, len(pos))
	for i := range pos {
		out = append(out, model.IPBlacklistFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *Repo) ListActive(ctx context.Context) ([]string, error) {
	var ips []string
	now := time.Now()
	err := r.data.DB.WithContext(ctx).Model(&model.IPBlacklistPO{}).
		// 仅手工封禁参与全局拦截；自动临时封禁只拦登录接口（由 bl:ban:* 承担）
		Where("(expire_at IS NULL OR expire_at > ?) AND deleted_at IS NULL AND (source IS NULL OR source = '' OR source = ?)", now, bizblacklist.SourceManual).
		Pluck("ip", &ips).Error
	return ips, err
}

// UpsertAutoBan 自动封禁落库：生效中的手工记录优先不覆盖；自动/软删记录复活刷新；否则新建
func (r *Repo) UpsertAutoBan(ctx context.Context, b *bizblacklist.IPBlacklist) error {
	var po model.IPBlacklistPO
	err := r.data.DB.WithContext(ctx).Unscoped().Where("ip = ?", b.IP).First(&po).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return r.Create(ctx, b)
	case err != nil:
		return err
	}
	if !po.DeletedAt.Valid && po.Source == bizblacklist.SourceManual {
		return nil // 手工封禁记录优先（其封禁强度不低于临时封禁）
	}
	return r.data.DB.WithContext(ctx).Unscoped().Model(&model.IPBlacklistPO{}).Where("id = ?", po.ID).
		Updates(map[string]interface{}{
			"reason":     b.Reason,
			"source":     bizblacklist.SourceAuto,
			"expire_at":  b.ExpireAt,
			"deleted_at": nil, // 复活软删记录
			"updated_at": time.Now(),
		}).Error
}

// ---- Redis 读缓存（write-through；loaded 标记缺失时回源 DB 重建） ----

func (r *Repo) IsBlocked(ctx context.Context, ip string) (bool, error) {
	loaded, err := r.rdb.Exists(ctx, keyLoaded).Result()
	if err != nil {
		return false, err
	}
	if loaded > 0 {
		return r.rdb.SIsMember(ctx, keyIPSet, ip).Result()
	}
	// 缓存未加载：回源 DB 重建镜像
	ips, err := r.ListActive(ctx)
	if err != nil {
		return false, err
	}
	pipe := r.rdb.TxPipeline()
	pipe.Del(ctx, keyIPSet)
	if len(ips) > 0 {
		pipe.SAdd(ctx, keyIPSet, ips)
	}
	pipe.Set(ctx, keyLoaded, "1", loadedTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	for _, v := range ips {
		if v == ip {
			return true, nil
		}
	}
	return false, nil
}

// CacheAdd 新增记录后写穿缓存（仅在镜像已加载时更新；未加载则下次判定回源重建，无需处理）
func (r *Repo) CacheAdd(ctx context.Context, ip string) error {
	loaded, err := r.rdb.Exists(ctx, keyLoaded).Result()
	if err != nil || loaded == 0 {
		return err
	}
	return r.rdb.SAdd(ctx, keyIPSet, ip).Err()
}

// CacheRemove 删除记录后写穿缓存
func (r *Repo) CacheRemove(ctx context.Context, ip string) error {
	loaded, err := r.rdb.Exists(ctx, keyLoaded).Result()
	if err != nil || loaded == 0 {
		return err
	}
	return r.rdb.SRem(ctx, keyIPSet, ip).Err()
}

// ---- 登录防护（Redis 实现 bizblacklist.LoginProtector） ----

func (r *Repo) TempBanRemaining(ctx context.Context, ip string) (time.Duration, error) {
	d, err := r.rdb.TTL(ctx, banKey(ip)).Result()
	if err != nil {
		return 0, err
	}
	if d < 0 { // key 不存在（-2）或无 TTL（-1，不应对外视为封禁）
		return 0, nil
	}
	return d, nil
}

func (r *Repo) TempBan(ctx context.Context, ip string, dur time.Duration) error {
	return r.rdb.Set(ctx, banKey(ip), 1, dur).Err()
}

func (r *Repo) ClearTempBan(ctx context.Context, ip string) error {
	return r.rdb.Del(ctx, banKey(ip)).Err()
}

func (r *Repo) IncrFail(ctx context.Context, ip string, window time.Duration) (int64, error) {
	n, err := r.rdb.Incr(ctx, failKey(ip)).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = r.rdb.Expire(ctx, failKey(ip), window).Err()
	}
	return n, nil
}

func (r *Repo) ResetFail(ctx context.Context, ip string) error {
	return r.rdb.Del(ctx, failKey(ip)).Err()
}

func (r *Repo) IncrRate(ctx context.Context, ip string, window time.Duration) (int64, error) {
	n, err := r.rdb.Incr(ctx, rateKey(ip)).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = r.rdb.Expire(ctx, rateKey(ip), window).Err()
	}
	return n, nil
}

func (r *Repo) ResetRate(ctx context.Context, ip string) error {
	return r.rdb.Del(ctx, rateKey(ip)).Err()
}
