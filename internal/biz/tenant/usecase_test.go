package tenant

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
)

// fakeSyncer 平台同步接口假实现：可脚本化各方法返回值，记录调用
type fakeSyncer struct {
	remote         []*Tenant // ListOnPlatform 返回的平台侧真相
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
func (f *fakeSyncer) ListOnPlatform(ctx context.Context, page, pageSize int) ([]*Tenant, int64, error) {
	if page > 1 {
		return nil, int64(len(f.remote)), nil
	}
	return f.remote, int64(len(f.remote)), nil
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

func (r *memRepo) GetByPlatformIDs(ctx context.Context, platformIDs []uint) ([]*Tenant, error) {
	want := make(map[uint]struct{}, len(platformIDs))
	for _, id := range platformIDs {
		want[id] = struct{}{}
	}
	out := []*Tenant{}
	for _, t := range r.tenants {
		if _, ok := want[t.PlatformID]; ok {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *memRepo) List(ctx context.Context, q Query, page, pageSize int) ([]*Tenant, int64, error) {
	out := []*Tenant{}
	for _, t := range r.tenants {
		if q.Status != nil && t.Status != Status(*q.Status) {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, int64(len(out)), nil
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

// TestReconcileFromPlatform 对账回流：本地投影跟随平台真相
// （平台改名→更新、平台新建→补建、平台已删→下线、未同步存量不动）
func TestReconcileFromPlatform(t *testing.T) {
	repo := newMemRepo(
		&Tenant{ID: 1, PlatformID: 11, Name: "旧名", Code: "a", Status: StatusEnabled},    // 平台已改名
		&Tenant{ID: 2, PlatformID: 22, Name: "b", Code: "b", Status: StatusEnabled},     // 一致
		&Tenant{ID: 3, PlatformID: 33, Name: "c", Code: "c", Status: StatusEnabled},     // 平台已删
		&Tenant{ID: 4, PlatformID: 0, Name: "legacy", Code: "l", Status: StatusEnabled}, // 未同步存量
	)
	remote := []*Tenant{
		{PlatformID: 11, Name: "新名", Code: "a", Status: StatusEnabled},
		{PlatformID: 22, Name: "b", Code: "b", Status: StatusDisabled}, // 平台侧停用
		{PlatformID: 44, Name: "d", Code: "d", Status: StatusEnabled},  // 平台侧直接新建
	}
	uc := NewUsecase(repo, &fakeSyncer{remote: remote})

	summary, err := uc.ReconcileFromPlatform(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"新增 1", "更新 2", "下线 1", "未变 0"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("对账摘要 %q 应含 %q", summary, want)
		}
	}
	// 平台改名/停用 → 本地跟随
	a, _ := repo.GetByPlatformID(context.Background(), 11)
	if a.Name != "新名" {
		t.Fatalf("平台改名应回流: %+v", a)
	}
	b, _ := repo.GetByPlatformID(context.Background(), 22)
	if b.Status != StatusDisabled {
		t.Fatalf("平台停用应回流: %+v", b)
	}
	// 平台直接新建 → 本地补建（platform_id 直填）
	d, err := repo.GetByPlatformID(context.Background(), 44)
	if err != nil || d.Name != "d" {
		t.Fatalf("平台新建应补建: %+v err=%v", d, err)
	}
	// 平台已删 → 本地下线
	if _, err := repo.GetByPlatformID(context.Background(), 33); err == nil {
		t.Fatal("平台已删的租户应下线")
	}
	// 未同步存量不参与对账
	if _, err := repo.Get(context.Background(), 4); err != nil {
		t.Fatal("未同步存量租户应保持原状")
	}
}

// TestReconcileFromPlatform_EmptyPlatformGuardian 护栏：平台拉取成功但为空时，
// 已同步本地租户不做批量下线（防列表接口异常返回空导致误删；未同步存量本就不参与）
func TestReconcileFromPlatform_EmptyPlatformGuardian(t *testing.T) {
	repo := newMemRepo(
		&Tenant{ID: 1, PlatformID: 11, Name: "a", Code: "a", Status: StatusEnabled},
	)
	uc := NewUsecase(repo, &fakeSyncer{remote: nil}) // 平台返回空

	summary, err := uc.ReconcileFromPlatform(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "下线 0") {
		t.Fatalf("空平台不应触发下线: %q", summary)
	}
	if _, err := repo.GetByPlatformID(context.Background(), 11); err != nil {
		t.Fatal("护栏应保住本地已同步租户")
	}
}
