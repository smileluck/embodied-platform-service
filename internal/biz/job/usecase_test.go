package job

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRepo 内存仓储（只实装本测试用到的语义）
type fakeRepo struct {
	mu   sync.Mutex
	next uint
	jobs map[uint]*Job
	logs []*JobLog
}

func newFakeRepo(seed ...*Job) *fakeRepo {
	r := &fakeRepo{next: 10, jobs: map[uint]*Job{}}
	for _, j := range seed {
		r.jobs[j.ID] = j
	}
	return r
}

func (r *fakeRepo) Create(ctx context.Context, j *Job) error        { return nil }
func (r *fakeRepo) Update(ctx context.Context, j *Job) error        { return nil }
func (r *fakeRepo) Delete(ctx context.Context, id uint) error       { return nil }
func (r *fakeRepo) Find(ctx context.Context, id uint) (*Job, error) { return r.jobs[id], nil }
func (r *fakeRepo) List(ctx context.Context, q Query, page, pageSize int) ([]*Job, int64, error) {
	return nil, 0, nil
}
func (r *fakeRepo) ListEnabled(ctx context.Context) ([]*Job, error) { return nil, nil }
func (r *fakeRepo) AppendLog(ctx context.Context, l *JobLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, l)
	return nil
}
func (r *fakeRepo) ListLogs(ctx context.Context, jobID uint, page, pageSize int) ([]*JobLog, int64, error) {
	return nil, 0, nil
}
func (r *fakeRepo) CleanupLogsBefore(ctx context.Context, before time.Time) error { return nil }

// noopCleaners 清理器替身（测试不触发对应 handler）
type noopCleaner struct{}

func (noopCleaner) CleanupExpired(ctx context.Context) error { return nil }

type noopNotifyCleaner struct{}

func (noopNotifyCleaner) DeleteRecordsBefore(ctx context.Context, before time.Time) error { return nil }

// blockingReconciler 可阻塞的对账替身（占用运行标记）
type blockingReconciler struct {
	block chan struct{}
	once  sync.Once
}

func (b *blockingReconciler) ReconcileFromPlatform(ctx context.Context) (string, error) {
	<-b.block
	return "done", nil
}

func newTestUsecase(repo *fakeRepo, tr TenantReconciler) *Usecase {
	return NewUsecase(repo, noopCleaner{}, noopCleaner{}, noopCleaner{}, noopNotifyCleaner{}, tr)
}

// TestExecute_HandlerPanicIsolated handler panic → 失败日志而非进程崩溃
func TestExecute_HandlerPanicIsolated(t *testing.T) {
	repo := newFakeRepo(&Job{ID: 1, Name: "对账", HandlerKey: HandlerTenantReconcile, Status: StatusEnabled})
	uc := newTestUsecase(repo, panicReconciler{})

	done := make(chan struct{})
	go func() { uc.execute(repo.jobs[1]); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("execute 未返回（疑似 panic 未隔离）")
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.logs) != 1 || repo.logs[0].Status != RunFailed || !strings.Contains(repo.logs[0].Output, "panic") {
		t.Fatalf("panic 应转为失败日志: %+v", repo.logs)
	}
}

type panicReconciler struct{}

func (panicReconciler) ReconcileFromPlatform(ctx context.Context) (string, error) {
	panic("boom")
}

// TestExecute_MutualExclusion 同任务互斥：在跑时第二次触发直接跳过（不产生第二条日志）
func TestExecute_MutualExclusion(t *testing.T) {
	repo := newFakeRepo(&Job{ID: 2, Name: "对账", HandlerKey: HandlerTenantReconcile, Status: StatusEnabled})
	br := &blockingReconciler{block: make(chan struct{})}
	uc := newTestUsecase(repo, br)

	go uc.execute(repo.jobs[2])
	// 等第一次进入运行态
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if uc.isRunning(2) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !uc.isRunning(2) {
		t.Fatal("第一次执行应处于运行态")
	}
	uc.execute(repo.jobs[2]) // 应跳过
	if err := uc.RunOnce(context.Background(), 2); err != ErrJobRunning {
		t.Fatalf("在跑时 RunOnce 应返回 ErrJobRunning, got %v", err)
	}
	close(br.block)
	time.Sleep(200 * time.Millisecond)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.logs) != 1 {
		t.Fatalf("互斥失败：应只有 1 条执行日志, got %d", len(repo.logs))
	}
	if repo.logs[0].Status != RunSuccess {
		t.Fatalf("第一次执行应成功: %+v", repo.logs[0])
	}
	if uc.isRunning(2) {
		t.Fatal("执行完成后应释放运行标记")
	}
}
