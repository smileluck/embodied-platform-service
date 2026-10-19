package admission

import (
	"context"
	"errors"
	"testing"
)

// memRepo 内存准入投影仓储（SyncFromPlatform 对账用例）
type memRepo struct {
	next   uint
	byID   map[uint]*Projection   // 本地 ID → 投影
	byPID  map[uint]*Projection   // 平台用户 ID → 投影
	roles  map[uint]map[uint]bool // 平台用户 ID → 角色集
	lastID uint                   // 最近删除的本地 ID（断言用）
}

func newMemRepo(seed ...*Projection) *memRepo {
	r := &memRepo{next: 100, byID: map[uint]*Projection{}, byPID: map[uint]*Projection{}, roles: map[uint]map[uint]bool{}}
	for _, p := range seed {
		cp := *p
		r.next++
		cp.ID = r.next
		r.byID[cp.ID] = &cp
		r.byPID[cp.PlatformUserID] = &cp
		if len(cp.RoleIDs) > 0 {
			m := map[uint]bool{}
			for _, id := range cp.RoleIDs {
				m[id] = true
			}
			r.roles[cp.PlatformUserID] = m
		}
	}
	return r
}

func (r *memRepo) Create(_ context.Context, p *Projection) error {
	r.next++
	p.ID = r.next
	cp := *p
	r.byID[p.ID] = &cp
	r.byPID[p.PlatformUserID] = &cp
	if len(p.RoleIDs) > 0 {
		m := map[uint]bool{}
		for _, id := range p.RoleIDs {
			m[id] = true
		}
		r.roles[p.PlatformUserID] = m
	}
	return nil
}

func (r *memRepo) Update(_ context.Context, p *Projection) error {
	if cur, ok := r.byPID[p.PlatformUserID]; ok {
		cp := *p
		cp.ID = cur.ID
		r.byID[cp.ID] = &cp
		r.byPID[p.PlatformUserID] = &cp
		return nil
	}
	return ErrNotFound
}

func (r *memRepo) Delete(_ context.Context, id uint) error {
	p, ok := r.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(r.byID, id)
	delete(r.byPID, p.PlatformUserID)
	delete(r.roles, p.PlatformUserID)
	r.lastID = id
	return nil
}

func (r *memRepo) FindByID(_ context.Context, id uint) (*Projection, error) {
	if p, ok := r.byID[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, ErrNotFound
}

func (r *memRepo) FindByPlatformUserID(_ context.Context, pid uint) (*Projection, error) {
	if p, ok := r.byPID[pid]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, nil
}

func (r *memRepo) FindByPlatformUserIDs(_ context.Context, pids []uint) (map[uint]*Projection, error) {
	out := map[uint]*Projection{}
	for _, pid := range pids {
		if p, ok := r.byPID[pid]; ok {
			cp := *p
			out[pid] = &cp
		}
	}
	return out, nil
}

func (r *memRepo) List(_ context.Context, q Query, page, pageSize int) ([]*Projection, int64, error) {
	return nil, 0, nil
}

func (r *memRepo) ListPlatformUserIDs(_ context.Context) ([]uint, error) {
	ids := []uint{}
	for pid := range r.byPID {
		ids = append(ids, pid)
	}
	return ids, nil
}

func (r *memRepo) SetRoles(_ context.Context, id uint, roleIDs []uint) error { return nil }

func (r *memRepo) GrantRole(_ context.Context, pid, roleID uint) error {
	if r.roles[pid] == nil {
		r.roles[pid] = map[uint]bool{}
	}
	r.roles[pid][roleID] = true
	return nil
}

func (r *memRepo) RevokeRole(_ context.Context, pid, roleID uint) error {
	delete(r.roles[pid], roleID)
	return nil
}

func (r *memRepo) RoleHolderUserIDs(_ context.Context, roleID uint) ([]uint, error) {
	ids := []uint{}
	for pid, set := range r.roles {
		if set[roleID] {
			ids = append(ids, pid)
		}
	}
	return ids, nil
}

// fakeMemberGateway 假平台成员网关：单页全量返回；记录写调用供守卫断言
type fakeMemberGateway struct {
	members []*PlatformAccount

	unbinds        int
	setAdmissions  int
	lastAdmittedTo bool
}

func (f *fakeMemberGateway) ListMembers(_ context.Context, kw string, page, size int) ([]*PlatformAccount, int64, error) {
	if page > 1 {
		return nil, 0, nil
	}
	return f.members, int64(len(f.members)), nil
}
func (f *fakeMemberGateway) CreateOrBind(_ context.Context, u, n, p string) (uint, bool, error) {
	return 0, false, nil
}
func (f *fakeMemberGateway) Unbind(_ context.Context, pid uint) error {
	f.unbinds++
	return nil
}
func (f *fakeMemberGateway) SetAdmission(_ context.Context, pid uint, b bool) error {
	f.setAdmissions++
	f.lastAdmittedTo = b
	return nil
}

// TestSyncFromPlatform_RecyclesOrphanProjections 平台删号/解绑后未感知的投影
// 在同步时被回收（含其角色绑定），不再成为阻塞角色删除的幽灵
func TestSyncFromPlatform_RecyclesOrphanProjections(t *testing.T) {
	repo := newMemRepo(
		&Projection{PlatformUserID: 11, Username: "alive", Enabled: true},
		&Projection{PlatformUserID: 22, Username: "ghost", Enabled: true, RoleIDs: []uint{MerchantAdminRoleID}},
	)
	uc := NewUsecase(repo, &fakeMemberGateway{members: []*PlatformAccount{
		{ID: 11, Username: "alive", Status: 1, Admitted: true},
	}}, nil, nil)

	if _, _, err := uc.SyncFromPlatform(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.FindByPlatformUserID(context.Background(), 22); p != nil {
		t.Fatal("平台已无此成员，孤儿投影应被回收")
	}
	holders, _ := repo.RoleHolderUserIDs(context.Background(), MerchantAdminRoleID)
	if len(holders) != 0 {
		t.Fatalf("孤儿持有的角色绑定应一并回收, got %v", holders)
	}
	if p, _ := repo.FindByPlatformUserID(context.Background(), 11); p == nil {
		t.Fatal("在册成员投影应保留")
	}
}

// TestSyncFromPlatform_NewMemberCreated 平台新成员自动建投影（既有语义回归）
func TestSyncFromPlatform_NewMemberCreated(t *testing.T) {
	repo := newMemRepo()
	uc := NewUsecase(repo, &fakeMemberGateway{members: []*PlatformAccount{
		{ID: 31, Username: "newbie", Status: 1, Admitted: true, IsAdmin: true},
	}}, nil, nil)

	created, _, err := uc.SyncFromPlatform(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("应新建 1 个投影, got %d", created)
	}
	p, _ := repo.FindByPlatformUserID(context.Background(), 31)
	if p == nil || !p.Enabled {
		t.Fatalf("新成员投影应启用: %+v", p)
	}
	holders, _ := repo.RoleHolderUserIDs(context.Background(), MerchantAdminRoleID)
	if len(holders) != 1 || holders[0] != 31 {
		t.Fatalf("管理员标记应投影为角色2: %v", holders)
	}
}

// TestSyncFromPlatform_EmptyPlatformListSkipsRecycle 空结果护栏：平台成员列表为空时
// 跳过回收阶段（孤儿投影与管理员角色原样保留）——空结果无法区分「平台真无成员」与
// 「列表接口异常返回空」，定时对账无人值守，批量删投影不可逆（与 tenant_reconcile 同口径）
func TestSyncFromPlatform_EmptyPlatformListSkipsRecycle(t *testing.T) {
	repo := newMemRepo(
		&Projection{PlatformUserID: 11, Username: "alive", Enabled: true, RoleIDs: []uint{MerchantAdminRoleID}},
		&Projection{PlatformUserID: 22, Username: "orphan", Enabled: true},
	)
	uc := NewUsecase(repo, &fakeMemberGateway{members: nil}, nil, nil)

	if _, _, err := uc.SyncFromPlatform(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p, _ := repo.FindByPlatformUserID(context.Background(), 22); p == nil {
		t.Fatal("空结果护栏应跳过孤儿回收（防误删）")
	}
	holders, _ := repo.RoleHolderUserIDs(context.Background(), MerchantAdminRoleID)
	if len(holders) != 1 || holders[0] != 11 {
		t.Fatalf("空结果护栏应跳过管理员角色回收: %v", holders)
	}
}

// ---- 平台超管成员行锁定（页面隐藏 + 本地哨兵 + 平台侧守卫三层） ----

// TestSuperAdmin_MemberOpsRejected 超管行的开关准入/分配角色/移除全部本地拒绝，
// 且不触发任何平台写回（平台写回由平台侧开放面守卫另行兜底）
func TestSuperAdmin_MemberOpsRejected(t *testing.T) {
	repo := newMemRepo(
		&Projection{PlatformUserID: 1, Username: "admin", IsSuper: true, Enabled: true},
	)
	gw := &fakeMemberGateway{}
	uc := NewUsecase(repo, gw, nil, nil)
	ctx := context.Background()

	if err := uc.SetEnabledByPlatformUser(ctx, 1, false); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("关准入应拒绝: %v", err)
	}
	if err := uc.SetEnabledByPlatformUser(ctx, 1, true); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("开准入应拒绝: %v", err)
	}
	if err := uc.SetRolesByPlatformUser(ctx, 1, []uint{3}); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("分配角色应拒绝: %v", err)
	}
	if err := uc.DeleteMember(ctx, 1); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("移除成员应拒绝: %v", err)
	}
	if gw.setAdmissions != 0 || gw.unbinds != 0 {
		t.Fatalf("守卫应先于平台写回: setAdmissions=%d unbinds=%d", gw.setAdmissions, gw.unbinds)
	}
	if p, _ := repo.FindByPlatformUserID(ctx, 1); p == nil || !p.Enabled {
		t.Fatalf("超管投影应原样保留: %+v", p)
	}
}

// TestSyncFromPlatform_SuperFlagSnapshotted 同步把平台 is_super 标记快照进投影
// （存量投影升级后首次同步即补齐标记，守卫随之生效）
func TestSyncFromPlatform_SuperFlagSnapshotted(t *testing.T) {
	repo := newMemRepo(
		&Projection{PlatformUserID: 1, Username: "admin", Enabled: true}, // 升级前建的投影：无标记
	)
	uc := NewUsecase(repo, &fakeMemberGateway{members: []*PlatformAccount{
		{ID: 1, Username: "admin", Status: 1, IsSuper: true, Admitted: true},
	}}, nil, nil)

	if _, _, err := uc.SyncFromPlatform(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, _ := repo.FindByPlatformUserID(context.Background(), 1)
	if p == nil || !p.IsSuper {
		t.Fatalf("同步应快照 is_super 标记: %+v", p)
	}
	if err := uc.SetEnabledByPlatformUser(context.Background(), 1, false); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("标记补齐后守卫应生效: %v", err)
	}
}

// TestSuperAdmin_ID1HardLockedWithoutSnapshot ID=1 硬锁定：投影 is_super 快照缺失
// （升级前存量/旧平台未下发标记）时三类操作同样本地拒绝，不依赖标记链路
func TestSuperAdmin_ID1HardLockedWithoutSnapshot(t *testing.T) {
	repo := newMemRepo(
		&Projection{PlatformUserID: PlatformSuperAdminID, Username: "admin", Enabled: true}, // 无 is_super 快照
	)
	gw := &fakeMemberGateway{}
	uc := NewUsecase(repo, gw, nil, nil)
	ctx := context.Background()

	if err := uc.SetEnabledByPlatformUser(ctx, PlatformSuperAdminID, false); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("关准入应拒绝: %v", err)
	}
	if err := uc.SetRolesByPlatformUser(ctx, PlatformSuperAdminID, []uint{3}); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("分配角色应拒绝: %v", err)
	}
	if err := uc.DeleteMember(ctx, PlatformSuperAdminID); !errors.Is(err, ErrPlatformSuperAdmin) {
		t.Fatalf("移除成员应拒绝: %v", err)
	}
	if gw.setAdmissions != 0 || gw.unbinds != 0 {
		t.Fatalf("ID 硬锁定应先于平台写回: setAdmissions=%d unbinds=%d", gw.setAdmissions, gw.unbinds)
	}
}

// TestListFromPlatform_ID1MarkedSuperWithoutPlatformFlag 列表行归一化：平台未下发
// is_super 标记（旧平台）时 ID=1 行仍标记为超管，页面据此隐藏操作按钮
func TestListFromPlatform_ID1MarkedSuperWithoutPlatformFlag(t *testing.T) {
	repo := newMemRepo()
	uc := NewUsecase(repo, &fakeMemberGateway{members: []*PlatformAccount{
		{ID: PlatformSuperAdminID, Username: "admin", Status: 1, Admitted: true}, // IsSuper 未下发
		{ID: 7, Username: "member", Status: 1, Admitted: true},
	}}, nil, nil)

	rows, _, err := uc.ListFromPlatform(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].IsSuper || rows[1].IsSuper {
		t.Fatalf("ID=1 行应归一化为超管: %+v", rows)
	}
}
