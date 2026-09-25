// Package notify 告警通知限界上下文 —— 领域层。
// 通知渠道（邮件 SMTP / Webhook）+ 告警规则（任务失败 / 监控阈值 / IP 自动封禁）+ 发送记录；
// 分发器订阅 eventbus 事件并周期评估监控指标，命中规则后按渠道发送并落记录（Redis 冷却去重）。
package notify

import (
	"context"
	"errors"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/monitor"
)

// 哨兵错误
var (
	ErrChannelNotFound   = errors.New("通知渠道不存在")
	ErrRuleNotFound      = errors.New("告警规则不存在")
	ErrChannelNameExists = errors.New("渠道名称已存在，请更换")
	ErrRuleNameExists    = errors.New("规则名称已存在，请更换")
	ErrInvalidChannel    = errors.New("渠道配置不完整或格式有误")
	ErrInvalidRule       = errors.New("规则配置有误（阈值/冷却/渠道）")
)

// ChannelType 渠道类型
type ChannelType string

const (
	ChannelEmail ChannelType = "email"
	// ChannelWebhook 通用 Webhook（HMAC-SHA256 签名头）
	ChannelWebhook ChannelType = "webhook"
	// 以下三类为 IM 群机器人（URL 形态复用 WebhookURL 字段，密钥复用 WebhookEnc/mask）：
	ChannelWecom    ChannelType = "wecom"    // 企业微信群机器人（key 在 URL，无需密钥）
	ChannelDingtalk ChannelType = "dingtalk" // 钉钉群机器人（可选加签密钥）
	ChannelFeishu   ChannelType = "feishu"   // 飞书群机器人（可选加签密钥）
)

func ValidChannelType(t string) bool {
	switch ChannelType(t) {
	case ChannelEmail, ChannelWebhook, ChannelWecom, ChannelDingtalk, ChannelFeishu:
		return true
	}
	return false
}

// IsRobotType 是否 IM 群机器人渠道（配置面相同：机器人 Webhook 地址 + 可选密钥）
func IsRobotType(t ChannelType) bool {
	return t == ChannelWecom || t == ChannelDingtalk || t == ChannelFeishu
}

// Source 告警规则来源
type Source string

const (
	// SourceJobFailed 定时任务执行失败（事件驱动：job.failed）
	SourceJobFailed Source = "job_failed"
	// SourceMonitor 监控指标越限（周期评估：CPU/内存/交换分区使用率）
	SourceMonitor Source = "monitor"
	// SourceIPAutoban 登录失败 IP 自动封禁（事件驱动：blacklist.autoban）
	SourceIPAutoban Source = "ip_autoban"
	// SourceTest 渠道测试（仅出现在发送记录，不可配置为规则来源）
	SourceTest Source = "test"
)

func ValidSource(s string) bool {
	return s == string(SourceJobFailed) || s == string(SourceMonitor) || s == string(SourceIPAutoban)
}

// Metric 监控指标（仅 SourceMonitor 规则使用）
type Metric string

const (
	MetricCPU  Metric = "cpu"
	MetricMem  Metric = "mem"
	MetricSwap Metric = "swap"
)

func ValidMetric(m string) bool {
	return m == string(MetricCPU) || m == string(MetricMem) || m == string(MetricSwap)
}

// MetricName 指标中文名（告警文案）
func MetricName(m Metric) string {
	switch m {
	case MetricCPU:
		return "CPU 使用率"
	case MetricMem:
		return "内存使用率"
	case MetricSwap:
		return "交换分区使用率"
	}
	return string(m)
}

// 渠道/规则启停状态
const (
	StatusEnabled  int = 1
	StatusDisabled int = 0
)

// 发送记录状态
const (
	RecordSent   = "sent"
	RecordFailed = "failed"
)

// Channel 通知渠道（密文字段存储加密值，Mask 字段仅回显）
type Channel struct {
	ID           uint        `json:"id"`
	Name         string      `json:"name"`
	Type         ChannelType `json:"type"`
	Status       int         `json:"status"`
	SMTPHost     string      `json:"smtp_host"`
	SMTPPort     int         `json:"smtp_port"`
	SMTPUser     string      `json:"smtp_user"`
	SMTPPassEnc  string      `json:"-"` // 密文，不外发
	SMTPPassMask string      `json:"smtp_password_mask"`
	SMTPFrom     string      `json:"smtp_from"`
	Recipients   []string    `json:"recipients"` // email 收件人
	WebhookURL   string      `json:"webhook_url"`
	WebhookEnc   string      `json:"-"` // 密文，不外发
	WebhookMask  string      `json:"webhook_secret_mask"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// Rule 告警规则
type Rule struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	Source          Source     `json:"source"`
	Metric          Metric     `json:"metric"`           // 仅 source=monitor
	Threshold       float64    `json:"threshold"`        // 越限阈值（百分比，仅 monitor）
	ChannelIDs      []uint     `json:"channel_ids"`      // 命中后发送的渠道
	CooldownSeconds int        `json:"cooldown_seconds"` // 同指纹冷却窗口（防轰炸）
	Status          int        `json:"status"`
	Remark          string     `json:"remark"`
	LastTriggeredAt *time.Time `json:"last_triggered_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Record 发送记录（每次投递一条，含失败原因）
type Record struct {
	ID          uint        `json:"id"`
	ChannelID   uint        `json:"channel_id"`
	ChannelName string      `json:"channel_name"`
	ChannelType ChannelType `json:"channel_type"`
	RuleName    string      `json:"rule_name"`
	Source      Source      `json:"source"`
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Status      string      `json:"status"` // sent | failed
	Error       string      `json:"error,omitempty"`
	DurationMs  int64       `json:"duration_ms"`
	CreatedAt   time.Time   `json:"created_at"`
}

// RecordQuery 发送记录筛选
type RecordQuery struct {
	ChannelID uint
	Source    string
	Status    string
}

// 事件主题（发布方定义具体事件类型，notify 按 topic 订阅）
const (
	TopicJobFailed    = "job.failed"
	TopicBlacklistBan = "blacklist.autoban"
)

// SnapshotReader 监控快照读取（由 monitor.Usecase 实现，wire 绑定；单向依赖 monitor）
type SnapshotReader interface {
	LatestSnapshot() *monitor.Snapshot
}

// MetricValue 指标当前值（越限评估）
func MetricValue(s *monitor.Snapshot, m Metric) float64 {
	switch m {
	case MetricCPU:
		return s.CPUPercent
	case MetricMem:
		return s.MemPercent
	case MetricSwap:
		return s.SwapPercent
	}
	return 0
}

// ChannelQuery 渠道列表筛选（零值 = 全量；Name 全模糊，Type/Status 精确）
type ChannelQuery struct {
	Name   string
	Type   string // email | webhook
	Status *int   // 1 启用 2 禁用
}

// RuleQuery 规则列表筛选（零值 = 全量；Name 全模糊，Source/Status 精确）
type RuleQuery struct {
	Name   string
	Source string // job_failed | monitor | ip_autoban
	Status *int   // 1 启用 2 禁用
}

// Repo 仓储接口
type Repo interface {
	// 渠道
	CreateChannel(ctx context.Context, ch *Channel) error
	UpdateChannel(ctx context.Context, ch *Channel) error
	DeleteChannel(ctx context.Context, id uint) error
	FindChannel(ctx context.Context, id uint) (*Channel, error)
	ListChannels(ctx context.Context, q ChannelQuery) ([]*Channel, error)
	// 规则
	CreateRule(ctx context.Context, r *Rule) error
	UpdateRule(ctx context.Context, r *Rule) error
	DeleteRule(ctx context.Context, id uint) error
	FindRule(ctx context.Context, id uint) (*Rule, error)
	ListRules(ctx context.Context, q RuleQuery) ([]*Rule, error)
	// 分发器
	ListEnabledRules(ctx context.Context) ([]*Rule, error)
	FindEnabledChannels(ctx context.Context, ids []uint) ([]*Channel, error)
	TouchRule(ctx context.Context, id uint, at time.Time) error
	// 发送记录
	AppendRecord(ctx context.Context, rec *Record) error
	ListRecords(ctx context.Context, q RecordQuery, page, pageSize int) ([]*Record, int64, error)
	ClearRecords(ctx context.Context) error
	// 定时清理（job 清理任务调用）
	DeleteRecordsBefore(ctx context.Context, before time.Time) error
}
