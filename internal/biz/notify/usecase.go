package notify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	"github.com/smilex/smilex-admin-gin/internal/biz/job"
	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/eventbus"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"go.uber.org/zap"
)

const (
	cooldownKeyPrefix = "alert:cd:" // 冷却键：alert:cd:{ruleID}:{指纹}，TTL=冷却窗口
	queueSize         = 256
	monitorEvalTick   = time.Minute
	minCooldownSec    = 60
	maxCooldownSec    = 86400
	defaultCooldown   = 300
	outputMaxRunes    = 500 // 任务错误输出进告警正文的截断长度
)

// Usecase 告警通知用例：管理端 CRUD + 渠道测试 + 后台分发器
type Usecase struct {
	repo   Repo
	rdb    *redis.Client
	reader SnapshotReader
	crypto *security.AESGCM

	queue chan pendingAlert
	stop  chan struct{}
	done  chan struct{}
}

// pendingAlert 待分发告警（事件类来源在 dispatch 时按规则来源匹配，monitor 评估时已锁定规则）
type pendingAlert struct {
	source  Source
	key     string // 冷却指纹：job:{id} / ip:{ip} / metric:{cpu}
	title   string
	content string
	rule    *Rule // 仅 monitor
}

// NewUsecase 构造并启动分发器（订阅事件 + 监控周期评估），返回清理函数。
// material 未配置 notify.cryptoKey 时从 jwt.secret 派生（更换后已存密钥需重新保存一次）。
func NewUsecase(repo Repo, rdb *redis.Client, reader SnapshotReader, cfg *conf.Bootstrap) (*Usecase, func()) {
	material := ""
	if cfg != nil {
		material = cfg.Notify.CryptoKey
		if material == "" {
			material = cfg.JWT.Secret
		}
	}
	uc := &Usecase{
		repo: repo, rdb: rdb, reader: reader,
		crypto: security.NewAESGCM("smilex-notify:", material),
		queue:  make(chan pendingAlert, queueSize),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go uc.worker()
	go uc.monitorLoop()

	// 事件订阅：Publish 是同步调用，回调只入队，不做 I/O
	eventbus.Subscribe(TopicJobFailed, func(e eventbus.Event) {
		if ev, ok := e.(job.FailedEvent); ok {
			started := ev.StartedAt.Format("2006-01-02 15:04:05")
			uc.enqueue(pendingAlert{
				source: SourceJobFailed,
				key:    fmt.Sprintf("job:%d", ev.JobID),
				title:  fmt.Sprintf("定时任务执行失败：%s", ev.JobName),
				content: fmt.Sprintf("- 任务：%s（#%d）\n- 处理器：`%s`\n- 开始时间：%s\n- 耗时：%dms\n- 错误输出：\n\n```\n%s\n```",
					ev.JobName, ev.JobID, ev.HandlerKey, started, ev.DurationMs, truncate(ev.Output, outputMaxRunes)),
			})
		}
	})
	eventbus.Subscribe(TopicBlacklistBan, func(e eventbus.Event) {
		if ev, ok := e.(blacklist.AutoBanEvent); ok {
			uc.enqueue(pendingAlert{
				source: SourceIPAutoban,
				key:    "ip:" + ev.IP,
				title:  fmt.Sprintf("登录失败 IP 已自动封禁：%s", ev.IP),
				content: fmt.Sprintf("- IP：`%s`\n- 连续登录失败：%d 次\n- 封禁时长：%s\n- 时间：%s",
					ev.IP, ev.FailCount, ev.Duration, time.Now().Format("2006-01-02 15:04:05")),
			})
		}
	})

	return uc, func() {
		close(uc.stop)
		<-uc.done
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// ---- 分发器 ----

func (uc *Usecase) enqueue(a pendingAlert) {
	select {
	case uc.queue <- a:
	case <-uc.stop:
	default:
		logger.Warn("notify queue full, alert dropped", zap.String("title", a.title))
	}
}

func (uc *Usecase) worker() {
	defer close(uc.done)
	for {
		select {
		case <-uc.stop:
			return
		case a := <-uc.queue:
			uc.dispatch(a)
		}
	}
}

// monitorLoop 周期评估监控规则（CPU/内存/交换分区使用率越限即入队）
func (uc *Usecase) monitorLoop() {
	uc.evalMonitor()
	t := time.NewTicker(monitorEvalTick)
	defer t.Stop()
	for {
		select {
		case <-uc.stop:
			return
		case <-t.C:
			uc.evalMonitor()
		}
	}
}

func (uc *Usecase) evalMonitor() {
	if uc.reader == nil {
		return
	}
	snap := uc.reader.LatestSnapshot()
	if snap == nil || snap.Ts.IsZero() {
		return
	}
	rules, err := uc.repo.ListEnabledRules(context.Background())
	if err != nil {
		logger.Warn("notify list rules failed", zap.Error(err))
		return
	}
	for _, r := range rules {
		if r.Source != SourceMonitor {
			continue
		}
		if v := MetricValue(snap, r.Metric); v > 0 && v >= r.Threshold {
			uc.enqueue(pendingAlert{
				source: SourceMonitor,
				key:    "metric:" + string(r.Metric),
				title:  fmt.Sprintf("监控指标越限：%s %.1f%% ≥ %.1f%%", MetricName(r.Metric), v, r.Threshold),
				content: fmt.Sprintf("- 指标：%s\n- 当前值：**%.1f%%**\n- 阈值：%.1f%%\n- 采样时间：%s",
					MetricName(r.Metric), v, r.Threshold, snap.Ts.Format("2006-01-02 15:04:05")),
				rule: r,
			})
		}
	}
}

// dispatch 命中规则 → 冷却去重 → 逐渠道投递并落记录
func (uc *Usecase) dispatch(a pendingAlert) {
	ctx := context.Background()
	rules := []*Rule{}
	if a.rule != nil {
		rules = append(rules, a.rule)
	} else {
		all, err := uc.repo.ListEnabledRules(ctx)
		if err != nil {
			logger.Warn("notify list rules failed", zap.Error(err))
			return
		}
		for _, r := range all {
			if r.Source == a.source {
				rules = append(rules, r)
			}
		}
	}
	for _, r := range rules {
		if !uc.acquireCooldown(ctx, r, a.key) {
			continue
		}
		channels, err := uc.repo.FindEnabledChannels(ctx, r.ChannelIDs)
		if err != nil {
			logger.Warn("notify list channels failed", zap.Error(err))
			continue
		}
		for _, ch := range channels {
			uc.deliver(ctx, ch, r, a)
		}
		if err := uc.repo.TouchRule(ctx, r.ID, time.Now()); err != nil {
			logger.Warn("notify touch rule failed", zap.Error(err))
		}
	}
}

// acquireCooldown 同规则同指纹在冷却窗口内只告警一次；Redis 故障 fail-open（宁可重复不可漏报）
func (uc *Usecase) acquireCooldown(ctx context.Context, r *Rule, key string) bool {
	if uc.rdb == nil {
		return true
	}
	ok, err := uc.rdb.SetNX(ctx, fmt.Sprintf("%s%d:%s", cooldownKeyPrefix, r.ID, key), 1,
		time.Duration(r.CooldownSeconds)*time.Second).Result()
	if err != nil {
		logger.Warn("alert cooldown check failed, fail-open", zap.Error(err))
		return true
	}
	return ok
}

func (uc *Usecase) deliver(ctx context.Context, ch *Channel, r *Rule, a pendingAlert) {
	rec := &Record{
		ChannelID: ch.ID, ChannelName: ch.Name, ChannelType: ch.Type,
		RuleName: r.Name, Source: a.source, Title: a.title, Content: a.content,
	}
	start := time.Now()
	var err error
	switch ch.Type {
	case ChannelEmail:
		err = uc.sendEmail(ctx, ch, a.title, a.content)
	case ChannelWebhook:
		err = uc.sendWebhook(ctx, ch, a.source, a.title, a.content, r.Name)
	case ChannelWecom:
		err = uc.sendWecom(ctx, ch, a.title, a.content)
	case ChannelDingtalk:
		err = uc.sendDingtalk(ctx, ch, a.title, a.content)
	case ChannelFeishu:
		err = uc.sendFeishu(ctx, ch, a.title, a.content)
	}
	rec.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		rec.Status, rec.Error = RecordFailed, truncate(err.Error(), 480)
		logger.Warn("notify send failed", zap.String("channel", ch.Name), zap.Error(err))
	} else {
		rec.Status = RecordSent
	}
	if err := uc.repo.AppendRecord(ctx, rec); err != nil {
		logger.Warn("notify append record failed", zap.Error(err))
	}
}

// ---- 渠道管理 ----

// ChannelInput 渠道写入参数（密码/密钥明文仅在写入时出现，空=保持原值；类型创建后不可改）
type ChannelInput struct {
	Name          string
	Type          string
	Status        int
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPassword  string
	SMTPFrom      string
	Recipients    []string
	WebhookURL    string
	WebhookSecret string
}

func (uc *Usecase) buildChannel(in ChannelInput) (*Channel, error) {
	if !ValidChannelType(in.Type) {
		return nil, ErrInvalidChannel
	}
	ch := &Channel{Name: strings.TrimSpace(in.Name), Type: ChannelType(in.Type), Status: in.Status}
	if ch.Name == "" {
		return nil, ErrInvalidChannel
	}
	switch ch.Type {
	case ChannelEmail:
		ch.SMTPHost = strings.TrimSpace(in.SMTPHost)
		ch.SMTPPort = in.SMTPPort
		ch.SMTPUser = strings.TrimSpace(in.SMTPUser)
		ch.SMTPFrom = strings.TrimSpace(in.SMTPFrom)
		ch.Recipients = in.Recipients
		if ch.SMTPHost == "" || ch.SMTPPort <= 0 || ch.SMTPPort > 65535 || ch.SMTPFrom == "" {
			return nil, ErrInvalidChannel
		}
		if len(ch.Recipients) == 0 {
			return nil, ErrInvalidChannel
		}
		for _, r := range ch.Recipients {
			if !strings.Contains(r, "@") {
				return nil, ErrInvalidChannel
			}
		}
		enc, err := uc.crypto.Encrypt(in.SMTPPassword)
		if err != nil {
			return nil, err
		}
		ch.SMTPPassEnc, ch.SMTPPassMask = enc, security.MaskSecret(in.SMTPPassword)
	case ChannelWebhook, ChannelWecom, ChannelDingtalk, ChannelFeishu:
		// 通用 Webhook 与 IM 群机器人共用配置面：Webhook 地址必填 http(s)，
		// 密钥可选（通用 Webhook 不填则不带签名头；机器人中仅钉钉/飞书加签用，企业微信不需要）
		ch.WebhookURL = strings.TrimSpace(in.WebhookURL)
		if !strings.HasPrefix(ch.WebhookURL, "http://") && !strings.HasPrefix(ch.WebhookURL, "https://") {
			return nil, ErrInvalidChannel
		}
		enc, err := uc.crypto.Encrypt(in.WebhookSecret)
		if err != nil {
			return nil, err
		}
		ch.WebhookEnc, ch.WebhookMask = enc, security.MaskSecret(in.WebhookSecret)
	}
	return ch, nil
}

func (uc *Usecase) nameExists(name string, excludeID uint) (bool, error) {
	list, err := uc.repo.ListChannels(context.Background(), ChannelQuery{})
	if err != nil {
		return false, err
	}
	for _, ch := range list {
		if ch.Name == name && ch.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (uc *Usecase) CreateChannel(ctx context.Context, in ChannelInput) (*Channel, error) {
	ch, err := uc.buildChannel(in)
	if err != nil {
		return nil, err
	}
	if exists, err := uc.nameExists(ch.Name, 0); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrChannelNameExists
	}
	if err := uc.repo.CreateChannel(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// UpdateChannel 全量更新（类型不可改；SMTP 密码/Webhook 密钥留空=保持原密文）
func (uc *Usecase) UpdateChannel(ctx context.Context, id uint, in ChannelInput) (*Channel, error) {
	old, err := uc.repo.FindChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Type != "" && in.Type != string(old.Type) {
		return nil, ErrInvalidChannel
	}
	in.Type = string(old.Type)
	ch, err := uc.buildChannel(in)
	if err != nil {
		return nil, err
	}
	// 留空保持原密文与掩码
	if in.SMTPPassword == "" {
		ch.SMTPPassEnc, ch.SMTPPassMask = old.SMTPPassEnc, old.SMTPPassMask
	}
	if in.WebhookSecret == "" {
		ch.WebhookEnc, ch.WebhookMask = old.WebhookEnc, old.WebhookMask
	}
	if exists, err := uc.nameExists(ch.Name, id); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrChannelNameExists
	}
	ch.ID = id
	if err := uc.repo.UpdateChannel(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (uc *Usecase) DeleteChannel(ctx context.Context, id uint) error {
	return uc.repo.DeleteChannel(ctx, id)
}

func (uc *Usecase) GetChannel(ctx context.Context, id uint) (*Channel, error) {
	return uc.repo.FindChannel(ctx, id)
}

func (uc *Usecase) ListChannels(ctx context.Context, q ChannelQuery) ([]*Channel, error) {
	return uc.repo.ListChannels(ctx, q)
}

// TestChannel 渠道连通性测试：发送测试消息并落一条记录，返回记录（含失败原因）
func (uc *Usecase) TestChannel(ctx context.Context, id uint) (*Record, error) {
	ch, err := uc.repo.FindChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	rec := &Record{ChannelID: ch.ID, ChannelName: ch.Name, ChannelType: ch.Type,
		RuleName: "渠道测试", Source: SourceTest, Title: "告警渠道测试", Content: "这是一条来自 SmileX-Admin 告警通知的测试消息，收到即代表渠道配置有效。"}
	start := time.Now()
	switch ch.Type {
	case ChannelEmail:
		err = uc.sendEmail(ctx, ch, rec.Title, rec.Content)
	case ChannelWebhook:
		err = uc.sendWebhook(ctx, ch, SourceTest, rec.Title, rec.Content, rec.RuleName)
	case ChannelWecom:
		err = uc.sendWecom(ctx, ch, rec.Title, rec.Content)
	case ChannelDingtalk:
		err = uc.sendDingtalk(ctx, ch, rec.Title, rec.Content)
	case ChannelFeishu:
		err = uc.sendFeishu(ctx, ch, rec.Title, rec.Content)
	}
	rec.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		rec.Status, rec.Error = RecordFailed, truncate(err.Error(), 480)
	} else {
		rec.Status = RecordSent
	}
	if appendErr := uc.repo.AppendRecord(ctx, rec); appendErr != nil {
		logger.Warn("notify append record failed", zap.Error(appendErr))
	}
	return rec, nil // 记录内含成败与原因，调用方按 rec.Status 展示
}

// ---- 规则管理 ----

// RuleInput 规则写入参数
type RuleInput struct {
	Name            string
	Source          string
	Metric          string
	Threshold       float64
	ChannelIDs      []uint
	CooldownSeconds int
	Status          int
	Remark          string
}

func (uc *Usecase) buildRule(in RuleInput) (*Rule, error) {
	if !ValidSource(in.Source) {
		return nil, ErrInvalidRule
	}
	r := &Rule{
		Name: strings.TrimSpace(in.Name), Source: Source(in.Source),
		Metric: Metric(in.Metric), Threshold: in.Threshold,
		ChannelIDs: dedupeIDs(in.ChannelIDs), Status: in.Status,
		CooldownSeconds: in.CooldownSeconds, Remark: in.Remark,
	}
	if r.Name == "" || len(r.ChannelIDs) == 0 {
		return nil, ErrInvalidRule
	}
	if r.Status != StatusEnabled {
		r.Status = StatusDisabled
	}
	if r.CooldownSeconds <= 0 {
		r.CooldownSeconds = defaultCooldown
	}
	if r.CooldownSeconds < minCooldownSec {
		r.CooldownSeconds = minCooldownSec
	}
	if r.CooldownSeconds > maxCooldownSec {
		r.CooldownSeconds = maxCooldownSec
	}
	if r.Source == SourceMonitor {
		if !ValidMetric(string(r.Metric)) || r.Threshold <= 0 || r.Threshold > 100 {
			return nil, ErrInvalidRule
		}
	}
	// 渠道须存在且启用
	channels, err := uc.repo.FindEnabledChannels(context.Background(), r.ChannelIDs)
	if err != nil {
		return nil, err
	}
	if len(channels) != len(r.ChannelIDs) {
		return nil, ErrInvalidRule
	}
	return r, nil
}

func dedupeIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (uc *Usecase) ruleNameExists(name string, excludeID uint) (bool, error) {
	list, err := uc.repo.ListRules(context.Background(), RuleQuery{})
	if err != nil {
		return false, err
	}
	for _, r := range list {
		if r.Name == name && r.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (uc *Usecase) CreateRule(ctx context.Context, in RuleInput) (*Rule, error) {
	r, err := uc.buildRule(in)
	if err != nil {
		return nil, err
	}
	if exists, err := uc.ruleNameExists(r.Name, 0); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrRuleNameExists
	}
	if err := uc.repo.CreateRule(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (uc *Usecase) UpdateRule(ctx context.Context, id uint, in RuleInput) (*Rule, error) {
	if _, err := uc.repo.FindRule(ctx, id); err != nil {
		return nil, err
	}
	r, err := uc.buildRule(in)
	if err != nil {
		return nil, err
	}
	if exists, err := uc.ruleNameExists(r.Name, id); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrRuleNameExists
	}
	r.ID = id
	if err := uc.repo.UpdateRule(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (uc *Usecase) DeleteRule(ctx context.Context, id uint) error {
	return uc.repo.DeleteRule(ctx, id)
}

func (uc *Usecase) GetRule(ctx context.Context, id uint) (*Rule, error) {
	return uc.repo.FindRule(ctx, id)
}

func (uc *Usecase) ListRules(ctx context.Context, q RuleQuery) ([]*Rule, error) {
	return uc.repo.ListRules(ctx, q)
}

// ---- 发送记录 ----

func (uc *Usecase) ListRecords(ctx context.Context, q RecordQuery, page, pageSize int) ([]*Record, pagination.Page, error) {
	list, total, err := uc.repo.ListRecords(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

func (uc *Usecase) ClearRecords(ctx context.Context) error {
	return uc.repo.ClearRecords(ctx)
}

// DeleteRecordsBefore 定时清理入口（job 内置清理任务调用）
func (uc *Usecase) DeleteRecordsBefore(ctx context.Context, before time.Time) error {
	return uc.repo.DeleteRecordsBefore(ctx, before)
}
