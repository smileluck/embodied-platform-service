// Package dashboard 仪表盘聚合仓储 GORM 实现。
// 日期聚合用 DATE()：MySQL / PostgreSQL / SQLite 三方言均原生支持。
package dashboard

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/dashboard"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
)

type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) dashboard.Repo { return &repo{data: d} }

// Counts 用户数=准入投影数（平台是唯一身份源，本地无独立用户表）
func (r *repo) Counts() (users, roles int64, err error) {
	db := r.data.DB
	if err = db.Model(&model.PlatformUserPO{}).Count(&users).Error; err != nil {
		return
	}
	err = db.Model(&model.RolePO{}).Count(&roles).Error
	return
}

// TodayLogins 本地无登录日志（登录在平台侧）；以今日发起过操作的用户数近似活跃度
func (r *repo) TodayLogins() (int64, error) {
	var n int64
	start := time.Now().Truncate(24 * time.Hour)
	err := r.data.DB.Model(&model.OperationLogPO{}).
		Where("created_at >= ? AND user_id > 0", start).
		Distinct("user_id").Count(&n).Error
	return n, err
}

// LoginTrend 活跃趋势：按日聚合操作量与活跃用户数（复用登录趋势的总量/成功双线语义）
func (r *repo) LoginTrend(days int) ([]dashboard.DailyPoint, error) {
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	var rows []struct {
		Date    string `gorm:"column:d"`
		Total   int64  `gorm:"column:total"`
		Success int64  `gorm:"column:success"`
	}
	err := r.data.DB.Model(&model.OperationLogPO{}).
		Select("DATE(created_at) AS d, COUNT(*) AS total, COUNT(DISTINCT user_id) AS success").
		Where("created_at >= ?", since).
		Group("d").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dashboard.DailyPoint, 0, len(rows))
	for _, w := range rows {
		out = append(out, dashboard.DailyPoint{Date: w.Date, Total: w.Total, Success: w.Success})
	}
	return out, nil
}

// OpTrend 按日聚合操作日志量
func (r *repo) OpTrend(days int) ([]dashboard.DailyPoint, error) {
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	var rows []struct {
		Date  string `gorm:"column:d"`
		Total int64  `gorm:"column:total"`
	}
	err := r.data.DB.Model(&model.OperationLogPO{}).
		Select("DATE(created_at) AS d, COUNT(*) AS total").
		Where("created_at >= ?", since).
		Group("d").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dashboard.DailyPoint, 0, len(rows))
	for _, w := range rows {
		out = append(out, dashboard.DailyPoint{Date: w.Date, Total: w.Total})
	}
	return out, nil
}

// RecentLogins 最近操作（本地无登录日志；状态按响应码 2xx/3xx 视为成功）
func (r *repo) RecentLogins(n int) ([]dashboard.LoginItem, error) {
	var pos []model.OperationLogPO
	if err := r.data.DB.WithContext(context.Background()).
		Where("user_id > 0").
		Order("id DESC").Limit(n).Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]dashboard.LoginItem, 0, len(pos))
	for _, p := range pos {
		status := dashboard.LoginFail
		if p.StatusCode < 400 {
			status = dashboard.LoginSuccess
		}
		out = append(out, dashboard.LoginItem{
			Username: p.Username, IP: p.IP,
			Status: status, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

var _ dashboard.Repo = (*repo)(nil)
var _ = gorm.ErrRecordNotFound // 保留 gorm 引用（扫描目标为匿名结构体）
