// Package monitor 监控历史快照仓储 GORM 实现
package monitor

import (
	"context"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/monitor"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
)

type SnapshotRepo struct {
	data *data.Data
}

func NewSnapshotRepo(d *data.Data) *SnapshotRepo {
	return &SnapshotRepo{data: d}
}

func (r *SnapshotRepo) SaveSnapshot(ctx context.Context, s *monitor.Snapshot) error {
	return r.data.DB.WithContext(ctx).Create(&model.MonitorSnapshotPO{
		Ts: s.Ts, CPUPercent: s.CPUPercent, MemPercent: s.MemPercent, SwapPercent: s.SwapPercent,
		NetSendRate: s.NetSendRate, NetRecvRate: s.NetRecvRate,
	}).Error
}

func (r *SnapshotRepo) ListSnapshots(ctx context.Context, since time.Time, limit int) ([]*monitor.Snapshot, error) {
	var pos []model.MonitorSnapshotPO
	if err := r.data.DB.WithContext(ctx).Where("ts >= ?", since).
		Order("ts ASC").Limit(limit).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]*monitor.Snapshot, 0, len(pos))
	for i := range pos {
		out = append(out, &monitor.Snapshot{
			Ts: pos[i].Ts, CPUPercent: pos[i].CPUPercent, MemPercent: pos[i].MemPercent,
			SwapPercent: pos[i].SwapPercent, NetSendRate: pos[i].NetSendRate, NetRecvRate: pos[i].NetRecvRate,
		})
	}
	return out, nil
}

func (r *SnapshotRepo) CleanupSnapshotsBefore(ctx context.Context, before time.Time) error {
	return r.data.DB.WithContext(ctx).Where("ts < ?", before).
		Delete(&model.MonitorSnapshotPO{}).Error
}
