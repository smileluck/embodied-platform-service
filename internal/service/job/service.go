// Package job 定时任务应用服务
package job

import (
	"context"

	bizjob "github.com/smilex/smilex-admin-gin/internal/biz/job"
)

type Service struct {
	uc *bizjob.Usecase
}

func NewService(uc *bizjob.Usecase) *Service { return &Service{uc: uc} }

// CreateRequest 任务写入入参
type CreateRequest struct {
	Name       string `json:"name" binding:"required,max=20"`
	Cron       string `json:"cron" binding:"required,max=32"`
	HandlerKey string `json:"handler_key" binding:"required,max=64"`
	Params     string `json:"params" binding:"max=512"`
	Remark     string `json:"remark" binding:"max=200"`
	Status     *int   `json:"status" binding:"omitempty,gte=0,lte=1"`
}

type UpdateRequest = CreateRequest

// StatusRequest 启停入参
type StatusRequest struct {
	Status int `json:"status" binding:"oneof=0 1"`
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*bizjob.Job, error) {
	return s.uc.Create(ctx, bizjob.JobInput{
		Name: req.Name, Cron: req.Cron, HandlerKey: req.HandlerKey,
		Params: req.Params, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	return s.uc.Update(ctx, id, bizjob.JobInput{
		Name: req.Name, Cron: req.Cron, HandlerKey: req.HandlerKey,
		Params: req.Params, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) SetStatus(ctx context.Context, id uint, st int) error {
	return s.uc.SetStatus(ctx, id, bizjob.Status(st))
}

func (s *Service) Delete(ctx context.Context, id uint) error             { return s.uc.Delete(ctx, id) }
func (s *Service) Get(ctx context.Context, id uint) (*bizjob.Job, error) { return s.uc.Get(ctx, id) }
func (s *Service) List(ctx context.Context, q bizjob.Query, page, pageSize int) ([]*bizjob.Job, interface{}, error) {
	return s.uc.List(ctx, q, page, pageSize)
}
func (s *Service) ListLogs(ctx context.Context, jobID uint, page, pageSize int) ([]*bizjob.JobLog, interface{}, error) {
	return s.uc.ListLogs(ctx, jobID, page, pageSize)
}
func (s *Service) RunOnce(ctx context.Context, id uint) error { return s.uc.RunOnce(ctx, id) }

// Handlers 可选处理器清单（表单下拉）
func (s *Service) Handlers() []bizjob.HandlerInfo { return s.uc.Handlers() }

// EnsureSeededAndStart 启动播种并拉起调度器（由 server 构造时调用）
func (s *Service) EnsureSeededAndStart() error {
	if err := s.uc.EnsureSeeded(context.Background()); err != nil {
		return err
	}
	return s.uc.Start()
}

// Stop 停止调度器
func (s *Service) Stop() { s.uc.Stop() }

func statusOf(s *int) bizjob.Status {
	if s != nil && *s == 0 {
		return bizjob.StatusDisabled
	}
	return bizjob.StatusEnabled
}
