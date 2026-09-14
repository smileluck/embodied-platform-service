// Package export 异步导出应用服务（提交任务、本人记录查询、下载与删除）。
package export

import (
	"context"
	"net/url"

	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

type Service struct {
	uc *bizexport.Usecase
}

func NewService(uc *bizexport.Usecase) *Service {
	return &Service{uc: uc}
}

// RecordVO 导出记录视图（不含对象 key 等内部字段）
type RecordVO struct {
	ID         uint   `json:"id"`
	Biz        string `json:"biz"`
	Name       string `json:"name"`
	Status     string `json:"status"` // pending | running | done | failed
	Size       int64  `json:"size"`
	Rows       int    `json:"rows"`
	Truncated  bool   `json:"truncated"`
	Error      string `json:"error"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at"` // 未结束为空串
}

func toVO(r *bizexport.ExportRecord) *RecordVO {
	vo := &RecordVO{
		ID: r.ID, Biz: r.Biz, Name: r.Name, Status: r.Status,
		Size: r.Size, Rows: r.Rows, Truncated: r.Truncated, Error: r.Error,
		CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if r.FinishedAt != nil {
		vo.FinishedAt = r.FinishedAt.Format("2006-01-02 15:04:05")
	}
	return vo
}

// Submit 提交导出任务（队列满返回 bizexport.ErrQueueFull，由 handler 映射 429）
func (s *Service) Submit(ctx context.Context, biz string, params url.Values, userID uint, username string) (*RecordVO, error) {
	rec, err := s.uc.Submit(ctx, biz, params, userID, username)
	if err != nil {
		return nil, err
	}
	return toVO(rec), nil
}

// List 本人导出记录分页查询
func (s *Service) List(ctx context.Context, userID uint, page, pageSize int) ([]*RecordVO, pagination.Page, error) {
	records, pg, err := s.uc.ListMine(ctx, userID, page, pageSize)
	if err != nil {
		return nil, pg, err
	}
	vos := make([]*RecordVO, 0, len(records))
	for _, r := range records {
		vos = append(vos, toVO(r))
	}
	return vos, pg, nil
}

// Recent 本人最近 limit 条导出记录（任务浮层轮询用）
func (s *Service) Recent(ctx context.Context, userID uint, limit int) ([]*RecordVO, error) {
	records, err := s.uc.RecentMine(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	vos := make([]*RecordVO, 0, len(records))
	for _, r := range records {
		vos = append(vos, toVO(r))
	}
	return vos, nil
}

// ResolveDownload 下载解析（归属与状态校验在 biz 层完成）
func (s *Service) ResolveDownload(ctx context.Context, id, userID uint) (*bizexport.Download, error) {
	return s.uc.ResolveDownload(ctx, id, userID)
}

// Delete 删除本人记录及存储产物
func (s *Service) Delete(ctx context.Context, id, userID uint) error {
	return s.uc.Delete(ctx, id, userID)
}
