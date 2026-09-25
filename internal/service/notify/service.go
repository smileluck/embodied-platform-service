// Package notify 告警通知应用服务
package notify

import (
	"context"

	biznotify "github.com/smilex/smilex-admin-gin/internal/biz/notify"
)

type Service struct {
	uc *biznotify.Usecase
}

func NewService(uc *biznotify.Usecase) *Service { return &Service{uc: uc} }

// ChannelRequest 渠道写入参数（SMTP 密码/Webhook 密钥留空=保持原值；类型创建后不可改）
type ChannelRequest struct {
	Name          string   `json:"name" binding:"required,max=20"`
	Type          string   `json:"type" binding:"required,oneof=email webhook wecom dingtalk feishu"`
	Status        int      `json:"status"`
	SMTPHost      string   `json:"smtp_host" binding:"max=128"`
	SMTPPort      int      `json:"smtp_port"`
	SMTPUser      string   `json:"smtp_user" binding:"max=128"`
	SMTPPassword  string   `json:"smtp_password" binding:"max=128"`
	SMTPFrom      string   `json:"smtp_from" binding:"max=128"`
	Recipients    []string `json:"recipients"`
	WebhookURL    string   `json:"webhook_url" binding:"max=512"`
	WebhookSecret string   `json:"webhook_secret" binding:"max=128"`
}

func toChannelInput(req ChannelRequest) biznotify.ChannelInput {
	return biznotify.ChannelInput{
		Name: req.Name, Type: req.Type, Status: req.Status,
		SMTPHost: req.SMTPHost, SMTPPort: req.SMTPPort, SMTPUser: req.SMTPUser,
		SMTPPassword: req.SMTPPassword, SMTPFrom: req.SMTPFrom, Recipients: req.Recipients,
		WebhookURL: req.WebhookURL, WebhookSecret: req.WebhookSecret,
	}
}

// RuleRequest 规则写入参数
type RuleRequest struct {
	Name            string  `json:"name" binding:"required,max=20"`
	Source          string  `json:"source" binding:"required,oneof=job_failed monitor ip_autoban"`
	Metric          string  `json:"metric" binding:"omitempty,oneof=cpu mem swap"`
	Threshold       float64 `json:"threshold"`
	ChannelIDs      []uint  `json:"channel_ids"`
	CooldownSeconds int     `json:"cooldown_seconds"`
	Status          int     `json:"status"`
	Remark          string  `json:"remark" binding:"max=200"`
}

// ---- 渠道 ----

func (s *Service) CreateChannel(ctx context.Context, req ChannelRequest) (*biznotify.Channel, error) {
	return s.uc.CreateChannel(ctx, toChannelInput(req))
}

func (s *Service) UpdateChannel(ctx context.Context, id uint, req ChannelRequest) (*biznotify.Channel, error) {
	return s.uc.UpdateChannel(ctx, id, toChannelInput(req))
}

func (s *Service) DeleteChannel(ctx context.Context, id uint) error {
	return s.uc.DeleteChannel(ctx, id)
}
func (s *Service) GetChannel(ctx context.Context, id uint) (*biznotify.Channel, error) {
	return s.uc.GetChannel(ctx, id)
}
func (s *Service) ListChannels(ctx context.Context, q biznotify.ChannelQuery) ([]*biznotify.Channel, error) {
	return s.uc.ListChannels(ctx, q)
}

// TestChannel 渠道连通性测试（返回发送记录：status=failed 时 error 字段含原因）
func (s *Service) TestChannel(ctx context.Context, id uint) (*biznotify.Record, error) {
	return s.uc.TestChannel(ctx, id)
}

// ---- 规则 ----

func (s *Service) CreateRule(ctx context.Context, req RuleRequest) (*biznotify.Rule, error) {
	return s.uc.CreateRule(ctx, biznotify.RuleInput{
		Name: req.Name, Source: req.Source, Metric: req.Metric, Threshold: req.Threshold,
		ChannelIDs: req.ChannelIDs, CooldownSeconds: req.CooldownSeconds,
		Status: req.Status, Remark: req.Remark,
	})
}

func (s *Service) UpdateRule(ctx context.Context, id uint, req RuleRequest) (*biznotify.Rule, error) {
	return s.uc.UpdateRule(ctx, id, biznotify.RuleInput{
		Name: req.Name, Source: req.Source, Metric: req.Metric, Threshold: req.Threshold,
		ChannelIDs: req.ChannelIDs, CooldownSeconds: req.CooldownSeconds,
		Status: req.Status, Remark: req.Remark,
	})
}

func (s *Service) DeleteRule(ctx context.Context, id uint) error { return s.uc.DeleteRule(ctx, id) }
func (s *Service) GetRule(ctx context.Context, id uint) (*biznotify.Rule, error) {
	return s.uc.GetRule(ctx, id)
}
func (s *Service) ListRules(ctx context.Context, q biznotify.RuleQuery) ([]*biznotify.Rule, error) {
	return s.uc.ListRules(ctx, q)
}

// ---- 发送记录 ----

func (s *Service) ListRecords(ctx context.Context, q biznotify.RecordQuery, page, pageSize int) ([]*biznotify.Record, interface{}, error) {
	return s.uc.ListRecords(ctx, q, page, pageSize)
}

func (s *Service) ClearRecords(ctx context.Context) error { return s.uc.ClearRecords(ctx) }
