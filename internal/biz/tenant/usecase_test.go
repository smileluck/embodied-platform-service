package tenant

import (
	"context"
	"errors"
	"testing"
)

// fakeSyncer 平台同步接口假实现：可脚本化各方法返回值，记录调用
type fakeSyncer struct {
	createErr      error
	updateErr      error
	deleteErr      error
	statusGoneOnce bool // 第一次 SetStatus 返回 gone（模拟平台已删），重试成功
	statusCalls    int  // SetStatus 调用计数
	linkPID        uint // LinkOrCreate 返回的平台 ID
	linkErr        error
	linkCalled     int
	statusTo       map[uint]bool // 记录 SetStatusOnPlatform(pid, enabled)
}

func (f *fakeSyncer) CreateOnPlatform(ctx context.Context, t *Tenant) (uint, error) {
	return 0, f.createErr
}
func (f *fakeSyncer) UpdateOnPlatform(ctx context.Context, t *Tenant) error { return f.updateErr }
func (f *fakeSyncer) DeleteFromPlatform(ctx context.Context, platformID uint) error {
	return f.deleteErr
}
func (f *fakeSyncer) SetStatusOnPlatform(ctx context.Context, platformID uint, enabled bool) error {
	f.statusCalls++
	if f.statusGoneOnce && f.statusCalls == 1 {
		return ErrPlatformTenantGone
	}
	if f.statusTo == nil {
		f.statusTo = map[uint]bool{}
	}
	f.statusTo[platformID] = enabled
	return nil
}
func (f *fakeSyncer) LinkOrCreateOnPlatform(ctx context.Context, t *Tenant) (uint, error) {
	f.linkCalled++
	if f.linkErr != nil {
		return 0, f.linkErr
	}
	return f.linkPID, nil
}

// memRepo 内存仓储（仅覆盖用例用到的方法语义）
type memRepo struct {
	tenants map[uint]*Tenant
	next    uint
}

func newMemRepo(seed ...*Tenant) *memRepo {
	r := &memRepo{tenants: map[uint]*Tenant{}, next: 100}
	for _, t := range seed {
		r.tenants[t.ID] = t
	}
	return r
}

func (r *memRepo) Create(ctx context.Context, t *Tenant) error {
	r.next++
	t.ID = r.next
	cp := *t
	r.tenants[t.ID] = &cp
	return nil
}
func (r *memRepo) Update(ctx context.Context, t *Tenant) error {
	if _, ok := r.tenants[t.ID]; !ok {
		return ErrTenantNotFound
	}
	cp := *t
	r.tenants[t.ID] = &cp
	return nil
}
func (r *memRepo) Delete(ctx context.Context, id uint) error {
	if _, ok := r.tenants[id]; !ok {
		return ErrTenantNotFound
	}
	delete(r.tenants, id)
	return nil
}
func (r *memRepo) Get(ctx context.Context, id uint) (*Tenant, error) {
	if t, ok := r.tenants[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, ErrTenantNotFound
}
func (r *memRepo) GetByPlatformID(ctx context.Context, platformID uint) (*Tenant, error) {
	for _, t := range r.tenants {
		if t.PlatformID == platformID {
			cp := *t
			return &cp, nil
		}
	}
	return nil, ErrTenantNotFound
}
func (r *memRepo) List(ctx context.Context, q Query, page, pageSize int) ([]*Tenant, int64, error) {
	return nil, 0, nil
}
func (r *memRepo) ListUnsynced(ctx context.Context) ([]*Tenant, error) { return nil, nil }

// TestDelete_PlatformGoneTolerated 平台侧已删（ErrPlatformTenantGone）→ 视为已删，本地删除放行
func TestDelete_PlatformGoneTolerated(t *testing.T) {
	repo := newMemRepo(&Tenant{ID: 1, Name: "孤儿", Code: "orphan", PlatformID: 55, Status: StatusEnabled})
	uc := NewUsecase(repo, &fakeSyncer{deleteErr: ErrPlatformTenantGone})

	if err := uc.Delete(context.Background(), 1); err != nil {
		t.Fatalf("平台已删应幂等放行本地删除, got %v", err)
	}
	if _, err := repo.Get(context.Background(), 1); !errors.Is(err, ErrTenantNotFound) {
		t.Fatal("本地租户应已删除")
	}
}

// TestDelete_PlatformErrorPropagates 平台侧其他错误（如关联冲突）→ 本地不删
func TestDelete_PlatformErrorPropagates(t *testing.T) {
	repo := newMemRepo(&Tenant{ID: 1, Name: "占用", Code: "busy", PlatformID: 55, Status: StatusEnabled})
	uc := NewUsecase(repo, &fakeSyncer{deleteErr: errors.New("409: 该租户下存在设备")})

	if err := uc.Delete(context.Background(), 1); err == nil {
		t.Fatal("平台侧冲突应透传")
	}
	if _, err := repo.Get(context.Background(), 1); err != nil {
		t.Fatalf("本地租户不应被删除: %v", err)
	}
}

// TestUpdate_PlatformGoneHeals 更新撞平台已删 → 补链重建回填新 platform_id，本地更新成功
func TestUpdate_PlatformGoneHeals(t *testing.T) {
	repo := newMemRepo(&Tenant{ID: 1, Name: "旧名", Code: "heal", PlatformID: 55, Status: StatusEnabled})
	syncer := &fakeSyncer{updateErr: ErrPlatformTenantGone, linkPID: 77}
	uc := NewUsecase(repo, syncer)

	if err := uc.Update(context.Background(), 1, "新名", "", "", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(context.Background(), 1)
	if got.PlatformID != 77 || got.Name != "新名" {
		t.Fatalf("应补链到新平台 ID 并更新资料: %+v", got)
	}
	if syncer.linkCalled != 1 {
		t.Fatalf("应触发一次补链, got %d", syncer.linkCalled)
	}
}

// TestSetStatus_PlatformGoneHeals 禁用撞平台已删 → 补链重建后对新平台 ID 重试禁用
func TestSetStatus_PlatformGoneHeals(t *testing.T) {
	repo := newMemRepo(&Tenant{ID: 1, Name: "禁用", Code: "heal2", PlatformID: 55, Status: StatusEnabled})
	syncer := &fakeSyncer{statusGoneOnce: true, linkPID: 88}
	uc := NewUsecase(repo, syncer)

	if err := uc.SetStatus(context.Background(), 1, StatusDisabled); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(context.Background(), 1)
	if got.PlatformID != 88 || got.Status != StatusDisabled {
		t.Fatalf("应补链并落禁用: %+v", got)
	}
	if syncer.statusCalls != 2 {
		t.Fatalf("应对新平台 ID 重试 SetStatus（共 2 次调用）, got %d", syncer.statusCalls)
	}
	if enabled, ok := syncer.statusTo[88]; !ok || enabled {
		t.Fatalf("应对新平台 ID 重试 SetStatus(false): %+v", syncer.statusTo)
	}
}

// TestRelinkByPlatformID 设备注册自愈入口：按平台租户 ID 定位本地租户补链并回填新 platform_id
func TestRelinkByPlatformID(t *testing.T) {
	repo := newMemRepo(&Tenant{ID: 1, Name: "悬挂", Code: "dangle", PlatformID: 55, Status: StatusEnabled})
	syncer := &fakeSyncer{linkPID: 77}
	uc := NewUsecase(repo, syncer)

	pid, err := uc.RelinkByPlatformID(context.Background(), 55)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 77 || syncer.linkCalled != 1 {
		t.Fatalf("应补链到新平台 ID 77, got %d (link called %d)", pid, syncer.linkCalled)
	}
	got, _ := repo.Get(context.Background(), 1)
	if got.PlatformID != 77 {
		t.Fatalf("本地投影应回填新平台 ID: %+v", got)
	}
	if _, err := uc.RelinkByPlatformID(context.Background(), 404); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("本地无此平台租户应报 ErrTenantNotFound, got %v", err)
	}
}
