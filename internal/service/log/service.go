// Package log 日志应用服务（操作日志的记录入口、查询与清空）。
package log

import (
	"context"
	"time"

	bizlog "github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

type Service struct {
	uc *bizlog.Usecase
}

func NewService(uc *bizlog.Usecase) *Service { return &Service{uc: uc} }

// OperationLogVO 操作日志视图
type OperationLogVO struct {
	ID         uint   `json:"id"`
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Route      string `json:"route"`
	Action     string `json:"action"`
	Params     string `json:"params"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int    `json:"latency_ms"`
	CreatedAt  string `json:"created_at"`
}

// RecordOperation 写请求审计落操作日志（异步；供 OpLog 中间件调用）
func (s *Service) RecordOperation(ctx context.Context, o *bizlog.OperationLog) {
	s.uc.RecordOperation(o)
}

// ListOperationLogs 操作日志分页查询
func (s *Service) ListOperationLogs(ctx context.Context, q bizlog.OperationLogQuery, page, pageSize int) ([]*OperationLogVO, pagination.Page, error) {
	logs, pg, err := s.uc.ListOperationLogs(ctx, q, page, pageSize)
	if err != nil {
		return nil, pg, err
	}
	vos := make([]*OperationLogVO, 0, len(logs))
	for _, o := range logs {
		vos = append(vos, &OperationLogVO{
			ID: o.ID, UserID: o.UserID, Username: o.Username, Method: o.Method,
			Path: o.Path, Route: o.Route, Action: o.Action, Params: o.Params,
			IP: o.IP, UserAgent: o.UserAgent, StatusCode: o.StatusCode,
			LatencyMs: o.LatencyMs, CreatedAt: formatTime(o.CreatedAt),
		})
	}
	return vos, pg, nil
}

// ClearOperationLogs 清空操作日志，返回删除行数
func (s *Service) ClearOperationLogs(ctx context.Context) (int64, error) {
	return s.uc.ClearOperationLogs(ctx)
}

// RetentionDays 日志保留天数（0 = 永久保留），前端展示保留说明用
func (s *Service) RetentionDays() int {
	return s.uc.RetentionDays()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
