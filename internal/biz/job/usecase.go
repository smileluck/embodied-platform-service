package job

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/smilex/smilex-admin-gin/pkg/eventbus"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
	"runtime/debug"
)

// 内置处理器键（种子任务按此引用）
const (
	HandlerLogCleanup      = "log_cleanup"
	HandlerExportCleanup   = "export_cleanup"
	HandlerUsageCleanup    = "agent_usage_cleanup"
	HandlerNotifyCleanup   = "notify_cleanup"
	HandlerTenantReconcile = "tenant_reconcile"
)

// 清理器接口（各仓储实现；保留期取各自配置）
type LogCleaner interface {
	CleanupExpired(ctx context.Context) error
}
type ExportCleaner interface {
	CleanupExpired(ctx context.Context) error
}
type UsageCleaner interface {
	CleanupExpired(ctx context.Context) error
}

// TenantReconciler 租户投影对账回流（tenant.Usecase 实现）：
// 平台为租户事实端，本地投影经此跟随平台侧直接变更（漂移上界=任务周期）
type TenantReconciler interface {
	ReconcileFromPlatform(ctx context.Context) (string, error)
}

// NotifyCleaner 告警发送记录清理（notify.Usecase 实现）
type NotifyCleaner interface {
	DeleteRecordsBefore(ctx context.Context, before time.Time) error
}

// Usecase 定时任务用例：CRUD + cron 调度器 + 手动执行
type Usecase struct {
	repo     Repo
	handlers map[string]Handler

	mu       sync.Mutex
	cron     *cron.Cron
	entryIDs map[uint]cron.EntryID
	running  map[uint]struct{} // 同任务在跑标记（cron tick 与手动 RunOnce 共用互斥）
}

func NewUsecase(repo Repo, lc LogCleaner, ec ExportCleaner, uc UsageCleaner, nc NotifyCleaner, tr TenantReconciler) *Usecase {
	handlers := map[string]Handler{
		HandlerLogCleanup: {
			Key: HandlerLogCleanup, Description: "清理保留期外的操作日志",
			Run: func(ctx context.Context, _ string) (string, error) {
				if err := lc.CleanupExpired(ctx); err != nil {
					return "", err
				}
				return "操作日志清理完成", nil
			},
		},
		HandlerExportCleanup: {
			Key: HandlerExportCleanup, Description: "清理保留期外的导出记录与产物",
			Run: func(ctx context.Context, _ string) (string, error) {
				if err := ec.CleanupExpired(ctx); err != nil {
					return "", err
				}
				return "导出记录清理完成", nil
			},
		},
		HandlerUsageCleanup: {
			Key: HandlerUsageCleanup, Description: "清理保留期外的 Agent 用量流水",
			Run: func(ctx context.Context, _ string) (string, error) {
				if err := uc.CleanupExpired(ctx); err != nil {
					return "", err
				}
				return "用量流水清理完成", nil
			},
		},
		HandlerNotifyCleanup: {
			Key: HandlerNotifyCleanup, Description: "清理保留期外的告警发送记录",
			Run: func(ctx context.Context, _ string) (string, error) {
				if err := nc.DeleteRecordsBefore(ctx, time.Now().AddDate(0, 0, -notifyRecordKeepDays)); err != nil {
					return "", err
				}
				return "告警发送记录清理完成", nil
			},
		},
		HandlerTenantReconcile: {
			Key: HandlerTenantReconcile, Description: "租户投影对账：本地租户跟随平台真相（补建/更新/下线）",
			Run: func(ctx context.Context, _ string) (string, error) {
				return tr.ReconcileFromPlatform(ctx)
			},
		},
	}
	return &Usecase{repo: repo, handlers: handlers, entryIDs: map[uint]cron.EntryID{}, running: map[uint]struct{}{}}
}

// HandlerInfo 处理器元信息（表单展示；Run 闭包不出域）
type HandlerInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Handlers 可选处理器清单（表单下拉）
func (uc *Usecase) Handlers() []HandlerInfo {
	out := make([]HandlerInfo, 0, len(uc.handlers))
	for _, h := range uc.handlers {
		out = append(out, HandlerInfo{Key: h.Key, Description: h.Description})
	}
	return out
}

// Start 启动调度器并装载全部启用任务（幂等）
func (uc *Usecase) Start() error {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.cron != nil {
		return nil
	}
	uc.cron = cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	jobs, err := uc.repo.ListEnabled(context.Background())
	if err != nil {
		return err
	}
	for _, j := range jobs {
		uc.addLocked(j)
	}
	uc.cron.Start()
	return nil
}

// Stop 停止调度器（等待在跑任务完成）
func (uc *Usecase) Stop() {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.cron != nil {
		ctx := uc.cron.Stop() // 返回等待 ctx
		// 有界等待：任务幂等（下次启动重跑），不因单个慢任务阻塞停机
		select {
		case <-ctx.Done():
		case <-time.After(15 * time.Second):
			logger.Warn("job stop timeout, abandon in-flight jobs（幂等语义，下次启动重跑）")
		}
		uc.cron = nil
		uc.entryIDs = map[uint]cron.EntryID{}
	}
}

// addLocked 装载单个任务（须持锁；表达式已校验）
func (uc *Usecase) addLocked(j *Job) {
	id, err := uc.cron.AddJob(j.Cron, &scheduledJob{uc: uc, job: j})
	if err != nil {
		logger.Warn("job schedule failed", zap.String("name", j.Name), zap.Error(err))
		return
	}
	uc.entryIDs[j.ID] = id
}

// reschedule 变更后重载该任务（增删改/启停统一走这里）
func (uc *Usecase) reschedule(jobID uint) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.cron == nil {
		return // 调度器未启动（测试环境）
	}
	if id, ok := uc.entryIDs[jobID]; ok {
		uc.cron.Remove(id)
		delete(uc.entryIDs, jobID)
	}
	j, err := uc.repo.Find(context.Background(), jobID)
	if err != nil || j.Status != StatusEnabled {
		return
	}
	uc.addLocked(j)
}

// ---- CRUD ----

// JobInput 写入参数
type JobInput struct {
	Name       string
	Cron       string
	HandlerKey string
	Params     string
	Remark     string
	Status     Status
}

func (uc *Usecase) validate(in *JobInput) error {
	in.Cron = strings.TrimSpace(in.Cron)
	if _, err := cron.ParseStandard(in.Cron); err != nil {
		return ErrBadCron
	}
	if _, ok := uc.handlers[in.HandlerKey]; !ok {
		return ErrUnknownHandler
	}
	return nil
}

func (uc *Usecase) Create(ctx context.Context, in JobInput) (*Job, error) {
	if err := uc.validate(&in); err != nil {
		return nil, err
	}
	j := &Job{
		Name: in.Name, Cron: in.Cron, HandlerKey: in.HandlerKey,
		Params: in.Params, Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.Create(ctx, j); err != nil {
		return nil, err
	}
	if j.Status == StatusEnabled {
		uc.reschedule(j.ID)
	}
	return j, nil
}

func (uc *Usecase) Update(ctx context.Context, id uint, in JobInput) error {
	j, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.validate(&in); err != nil {
		return err
	}
	j.Name = in.Name
	j.Cron = in.Cron
	j.HandlerKey = in.HandlerKey
	j.Params = in.Params
	if in.Remark != "" {
		j.Remark = in.Remark
	}
	j.Status = in.Status
	if err := uc.repo.Update(ctx, j); err != nil {
		return err
	}
	uc.reschedule(id)
	return nil
}

// SetStatus 启停
func (uc *Usecase) SetStatus(ctx context.Context, id uint, st Status) error {
	j, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	j.Status = st
	if err := uc.repo.Update(ctx, j); err != nil {
		return err
	}
	uc.reschedule(id)
	return nil
}

func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.mu.Lock()
	if uc.cron != nil {
		if eid, ok := uc.entryIDs[id]; ok {
			uc.cron.Remove(eid)
			delete(uc.entryIDs, id)
		}
	}
	uc.mu.Unlock()
	return nil
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Job, error) { return uc.repo.Find(ctx, id) }

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Job, pagination.Page, error) {
	list, total, err := uc.repo.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

func (uc *Usecase) ListLogs(ctx context.Context, jobID uint, page, pageSize int) ([]*JobLog, pagination.Page, error) {
	list, total, err := uc.repo.ListLogs(ctx, jobID, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// RunOnce 手动立即执行（异步，不等结果；记录执行日志）
func (uc *Usecase) RunOnce(ctx context.Context, id uint) error {
	j, err := uc.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if uc.isRunning(j.ID) {
		return ErrJobRunning
	}
	go uc.execute(j)
	return nil
}

// EnsureSeeded 启动播种内置清理任务（幂等：已存在则跳过；默认每日 04:00）
func (uc *Usecase) EnsureSeeded(ctx context.Context) error {
	existing, _, err := uc.repo.List(ctx, Query{}, 1, 0)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for _, j := range existing {
		names[j.Name] = true
	}
	seeds := []Job{
		{Name: "日志保留期清理", Cron: "0 4 * * *", HandlerKey: HandlerLogCleanup, Status: StatusEnabled,
			Remark: "清理保留期外的操作日志（保留期见 log.retentionDays）"},
		{Name: "导出保留期清理", Cron: "10 4 * * *", HandlerKey: HandlerExportCleanup, Status: StatusEnabled,
			Remark: "清理保留期外的导出记录与产物文件"},
		{Name: "用量流水保留期清理", Cron: "20 4 * * *", HandlerKey: HandlerUsageCleanup, Status: StatusEnabled,
			Remark: "清理保留期外的 Agent 用量流水"},
		{Name: "告警记录保留期清理", Cron: "40 4 * * *", HandlerKey: HandlerNotifyCleanup, Status: StatusEnabled,
			Remark: "清理保留期外的告警发送记录（保留 30 天）"},
		{Name: "租户投影对账", Cron: "*/5 * * * *", HandlerKey: HandlerTenantReconcile, Status: StatusEnabled,
			Remark: "本地租户投影跟随平台真相：平台侧直接变更（改名/启停/新建/删除）≤5 分钟回流"},
	}
	for _, s := range seeds {
		if names[s.Name] {
			continue
		}
		if err := uc.repo.Create(ctx, &s); err != nil {
			return err
		}
	}
	return nil
}

// ---- 执行 ----

// scheduledJob cron 适配器：每次触发从库里取最新配置执行（改表达式即时生效）
type scheduledJob struct {
	uc  *Usecase
	job *Job
}

func (s *scheduledJob) Run() {
	j, err := s.uc.repo.Find(context.Background(), s.job.ID)
	if err != nil || j.Status != StatusEnabled {
		return
	}
	s.uc.execute(j)
}

// 单次执行超时与输出截断
const (
	execTimeout          = 10 * time.Minute
	outputMax            = 2000
	logKeepDays          = 30
	notifyRecordKeepDays = 30 // 告警发送记录保留期
)

// execute 执行任务并落执行日志（顺带清理过期日志）。
// 同任务互斥：cron 的 SkipIfStillRunning 只覆盖调度触发，手动 RunOnce 会绕过——
// 在跑标记统一兜底（对账类任务重叠会双拉双写）；handler panic 在此隔离
// （goroutine panic 会打死整个进程）
func (uc *Usecase) execute(j *Job) {
	h, ok := uc.handlers[j.HandlerKey]
	if !ok {
		logger.Warn("job handler missing", zap.String("job", j.Name), zap.String("handler", j.HandlerKey))
		return
	}
	if !uc.tryMarkRunning(j.ID) {
		logger.Warn("job still running, skip trigger", zap.Uint("job_id", j.ID), zap.String("job", j.Name))
		return
	}
	defer uc.unmarkRunning(j.ID)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	out, err := runHandler(ctx, h, j)

	status := RunSuccess
	if err != nil {
		status = RunFailed
		out = err.Error()
	}
	if len(out) > outputMax {
		out = out[:outputMax] + "…（截断）"
	}
	now := time.Now()
	if err := uc.repo.AppendLog(context.Background(), &JobLog{
		JobID: j.ID, JobName: j.Name, HandlerKey: j.HandlerKey,
		Status: status, Output: out,
		DurationMs: time.Since(start).Milliseconds(), StartedAt: start,
	}); err != nil {
		logger.Warn("job log append failed", zap.Error(err))
	}
	// 执行失败跨上下文广播（告警通知订阅 job.failed；同步发布，订阅方须自行异步）
	if status == RunFailed {
		eventbus.Publish(FailedEvent{
			JobID: j.ID, JobName: j.Name, HandlerKey: j.HandlerKey,
			Output: out, DurationMs: time.Since(start).Milliseconds(), StartedAt: start,
		})
	}
	_ = uc.repo.CleanupLogsBefore(context.Background(), now.AddDate(0, 0, -logKeepDays))
	j.LastRunAt = &now
	_ = uc.repo.Update(context.Background(), j)
}

// runHandler 隔离 handler panic：转为失败日志而非进程崩溃
func runHandler(ctx context.Context, h Handler, j *Job) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()
	return h.Run(ctx, j.Params)
}

func (uc *Usecase) tryMarkRunning(jobID uint) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if _, ok := uc.running[jobID]; ok {
		return false
	}
	uc.running[jobID] = struct{}{}
	return true
}

func (uc *Usecase) unmarkRunning(jobID uint) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	delete(uc.running, jobID)
}

// isRunning 任务是否在跑（RunOnce 预检提示用；原子占位仍由 execute 兜底）
func (uc *Usecase) isRunning(jobID uint) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	_, ok := uc.running[jobID]
	return ok
}
