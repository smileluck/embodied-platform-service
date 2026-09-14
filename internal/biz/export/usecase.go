package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// Enqueuer 导出任务队列入口（由 data 层 worker 实现注入，biz 不感知 channel 细节）
type Enqueuer interface {
	// Enqueue 任务入队；队列满返回 ErrQueueFull
	Enqueue(id uint) error
}

// Usecase 异步导出领域用例（提交入队、本人记录查询、下载解析与删除）
type Usecase struct {
	repo     Repo
	registry *Registry
	enq      Enqueuer
	storage  *bizfile.StorageManager
	cfg      *conf.Bootstrap
}

func NewUsecase(repo Repo, registry *Registry, enq Enqueuer, storage *bizfile.StorageManager, c *conf.Bootstrap) *Usecase {
	return &Usecase{repo: repo, registry: registry, enq: enq, storage: storage, cfg: c}
}

// Submit 提交导出任务：校验业务类型 → 落 pending 记录 → 入队；
// 队列满时回滚记录并返回 ErrQueueFull（不残留永不执行的 pending 数据）
func (uc *Usecase) Submit(ctx context.Context, biz string, params url.Values, userID uint, username string) (*ExportRecord, error) {
	exp, ok := uc.registry.Get(biz)
	if !ok {
		return nil, ErrUnsupportedBiz
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	rec := &ExportRecord{
		UserID:    userID,
		Biz:       biz,
		Name:      fmt.Sprintf("%s-%s.csv", exp.Name(), time.Now().Format("20060102150405")),
		Params:    string(paramsJSON),
		Status:    StatusPending,
		CreatedAt: time.Now(),
	}
	if err := uc.repo.Create(ctx, rec); err != nil {
		return nil, err
	}
	if err := uc.enq.Enqueue(rec.ID); err != nil {
		if derr := uc.repo.Delete(ctx, rec.ID); derr != nil {
			logger.Warn("export enqueue failed, record rollback failed",
				zap.Uint("id", rec.ID), zap.String("username", username), zap.Error(derr))
		}
		return nil, err
	}
	return rec, nil
}

// ListMine 本人导出记录分页查询
func (uc *Usecase) ListMine(ctx context.Context, userID uint, page, pageSize int) ([]*ExportRecord, pagination.Page, error) {
	records, total, err := uc.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return records, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// RecentMine 本人最近 limit 条导出记录（任务浮层轮询用）
func (uc *Usecase) RecentMine(ctx context.Context, userID uint, limit int) ([]*ExportRecord, error) {
	if limit <= 0 {
		limit = 5
	}
	return uc.repo.RecentByUser(ctx, userID, limit)
}

// Download 导出产物下载解析结果：云存储返回预签名 URL（URL 非空），本地存储返回内容流（Body 非空）
type Download struct {
	Record *ExportRecord
	URL    string        // 预签名下载地址（云存储；鉴权通过后签发，短时效）
	Body   io.ReadCloser // 本地存储代理内容流
}

// ResolveDownload 校验记录存在、归属本人且已完成，再按落库 driver 解析存储后端：
// 云存储签发预签名 URL，本地存储打开内容流（与文件管理下载同一套路径）
func (uc *Usecase) ResolveDownload(ctx context.Context, id, userID uint) (*Download, error) {
	rec, err := uc.mine(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if rec.Status != StatusDone {
		return nil, ErrNotReady
	}
	backend, err := uc.storage.For(rec.Driver)
	if err != nil {
		return nil, err
	}
	d := &Download{Record: rec}
	expire := time.Duration(uc.cfg.Storage.SignExpireMinutes) * time.Minute
	if expire <= 0 {
		expire = 15 * time.Minute
	}
	presigned, err := backend.PresignGet(ctx, rec.ObjectKey, expire)
	switch {
	case err == nil:
		d.URL = presigned
		return d, nil
	case errors.Is(err, bizfile.ErrPresignUnsupported):
		body, err := backend.Get(ctx, rec.ObjectKey)
		if err != nil {
			return nil, err
		}
		d.Body = body
		return d, nil
	default:
		return nil, err
	}
}

// Delete 删除本人记录并尽力删除存储产物（对象删除失败仅告警，不阻塞）
func (uc *Usecase) Delete(ctx context.Context, id, userID uint) error {
	rec, err := uc.mine(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	if rec.ObjectKey == "" {
		return nil
	}
	if backend, err := uc.storage.For(rec.Driver); err == nil {
		if derr := backend.Delete(ctx, rec.ObjectKey); derr != nil {
			logger.Warn("export object delete failed", zap.String("driver", rec.Driver),
				zap.String("key", rec.ObjectKey), zap.Error(derr))
		}
	} else {
		logger.Warn("export object backend unavailable, object left behind",
			zap.String("driver", rec.Driver), zap.String("key", rec.ObjectKey))
	}
	return nil
}

// mine 取记录并校验归属（下载/删除共用的越权防线）
func (uc *Usecase) mine(ctx context.Context, id, userID uint) (*ExportRecord, error) {
	rec, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.UserID != userID {
		return nil, ErrNotOwner
	}
	return rec, nil
}
