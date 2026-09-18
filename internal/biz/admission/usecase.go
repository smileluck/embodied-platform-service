package admission

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// DecisionCache 准入/授权决策缓存（RBAC 判定缓存；变更时整体失效，
// 保证「禁用/删除投影、改角色」对已登录请求即时生效——即本地授权的同步吊销）
type DecisionCache interface {
	Flush(ctx context.Context)
}

// PlatformAccount 平台侧商户成员信息（data 层经开放面商户 HMAC 拉取；PII 不出开放面）
type PlatformAccount struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Status   int    `json:"status"`   // 平台侧状态（1 启用 0 禁用；平台禁用=全局）
	IsAdmin  bool   `json:"is_admin"` // 商户管理员标记（平台侧唯一事实源）
	Admitted bool   `json:"admitted"` // 业务端准入状态（平台侧事实源，本端开关写回）
}

// MerchantIdentity 本商户身份解析（开放面 ping 惰性发现；由 data 层实现）
type MerchantIdentity interface {
	// MerchantID 返回本商户的平台 ID；known=false 表示尚未识别成功
	MerchantID(ctx context.Context) (uint, bool)
}

// MemberRow 成员列表合并行：平台绑定成员（来源=平台，唯一身份源）+ 本地准入投影（可空）
type MemberRow struct {
	PlatformUserID uint   `json:"platform_user_id"`
	Username       string `json:"username"`
	Nickname       string `json:"nickname"`
	PlatformStatus int    `json:"platform_status"` // 平台侧账号状态（禁用=平台全局封禁）
	IsAdmin        bool   `json:"is_admin"`        // 平台侧商户管理员标记（实时；本地角色2在其登录/同步后对齐）
	Admitted       bool   `json:"admitted"`        // 业务端准入状态（平台侧实时事实源）
	BoundAt        string `json:"bound_at"`        // 平台侧绑定时间（平台格式化文本）
	// Projection 本地准入投影；nil=平台已绑定但本系统尚未准入（先「同步成员」或重新添加）
	Projection *Projection `json:"projection"`
}

// ErrPlatformNotBound 平台侧该账号未绑定本商户（解绑幂等容忍：本地清理照常进行）
var ErrPlatformNotBound = errors.New("平台侧该账号未关联本商户")

// MemberGateway 平台商户成员网关（开放面 user:* 域，data 层经平台 SDK 实现）。
// 账号本体全局在平台：新增=平台无此账号则创建（初始密码调用方设置）、有则绑定本商户；
// 删除=仅解绑；列表=本商户绑定成员。
type MemberGateway interface {
	ListMembers(ctx context.Context, keyword string, page, pageSize int) ([]*PlatformAccount, int64, error)
	// CreateOrBind 返回（平台用户 ID, 是否为已存在账号仅绑定）
	CreateOrBind(ctx context.Context, username, nickname, password string) (platformUserID uint, existed bool, err error)
	// Unbind 解除平台侧绑定；未绑定/不存在返回 ErrPlatformNotBound（解绑幂等）
	Unbind(ctx context.Context, platformUserID uint) error
	// SetAdmission 开/关成员准入（写回平台事实源；未绑定返回 ErrPlatformNotBound）
	SetAdmission(ctx context.Context, platformUserID uint, admitted bool) error
}

// Usecase 准入领域用例
type Usecase struct {
	repo     Repo
	platform MemberGateway
	cache    DecisionCache
	identity MerchantIdentity // 本商户身份（成员/管理员判定用；测试可空）
}

func NewUsecase(repo Repo, platform MemberGateway, cache DecisionCache, identity MerchantIdentity) *Usecase {
	return &Usecase{repo: repo, platform: platform, cache: cache, identity: identity}
}

// flushDecisions 主动失效决策缓存（禁用/删除/改角色后立即生效；失败仅告警不阻断主流程）
func (uc *Usecase) flushDecisions(ctx context.Context) {
	if uc.cache != nil {
		uc.cache.Flush(ctx)
	}
}

// Admission 取平台用户的准入投影；未建投影返回 nil（认证链路据此判定 403）
func (uc *Usecase) Admission(ctx context.Context, platformUserID uint) (*Projection, error) {
	return uc.repo.FindByPlatformUserID(ctx, platformUserID)
}

// ---- 商户成员/管理员判定与投影（平台标记 → 本地角色 2） ----

// MerchantStatus 平台身份与本商户的关系判定
type MerchantStatus struct {
	Known    bool // 本商户身份是否已知（ping 未成功=false：调用方须跳过成员闸门与管理员自愈，绝不因未知而误拒/误绑）
	Member   bool // 是否本商户绑定成员（平台 merchant_users 唯一事实源；false=平台侧已解绑，本地授权同步吊销）
	Admitted bool // 业务端准入状态（平台侧事实源，本端开关写回；false=已暂停）
	IsAdmin  bool // 是否本商户管理员（Member 隐含 true 时才有意义）
}

// ResolveMerchantStatus 判定平台身份与本商户的关系（平台 /auth/profile 下发的
// 已准入商户引用 × 本商户 ID）。
func (uc *Usecase) ResolveMerchantStatus(ctx context.Context, merchants []MerchantRef) MerchantStatus {
	if uc.identity == nil {
		return MerchantStatus{}
	}
	mid, ok := uc.identity.MerchantID(ctx)
	if !ok {
		return MerchantStatus{}
	}
	for _, m := range merchants {
		if m.ID == mid {
			return MerchantStatus{Known: true, Member: true, Admitted: m.Admitted, IsAdmin: m.IsAdmin}
		}
	}
	// 身份已知但未绑定本商户（或旧版平台未下发 merchants）：明确非成员
	return MerchantStatus{Known: true}
}

// EnsureMerchantAdmin 为平台标记的商户管理员懒建准入投影（Enabled + 角色2）：
// 平台侧把成员设为管理员后，对方首次请求本系统即自动准入，零人工介入。
// 已有投影时交由 ReconcileMerchantAdmin 对账，不做整体替换。
func (uc *Usecase) EnsureMerchantAdmin(ctx context.Context, platformUserID uint, username, nickname string) (*Projection, error) {
	p, err := uc.repo.FindByPlatformUserID(ctx, platformUserID)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	if nickname == "" {
		nickname = username
	}
	np := &Projection{
		PlatformUserID: platformUserID,
		Username:       username,
		Nickname:       nickname,
		Enabled:        true,
		RoleIDs:        []uint{MerchantAdminRoleID},
	}
	if err := uc.repo.Create(ctx, np); err != nil {
		return nil, err
	}
	uc.flushDecisions(ctx)
	return np, nil
}

// ReconcileMerchantAdmin 商户管理员绑定自愈：平台标记与本地持有不一致时绑/解角色2
// 并失效决策缓存（即时生效）。变更延迟受 pid: 自省缓存 TTL（30-60s）兜底，
// 与「平台禁用/吊销在 TTL 内感知」同口径。幂等：一致时零写入。
func (uc *Usecase) ReconcileMerchantAdmin(ctx context.Context, p *Projection, isAdmin bool) error {
	if p == nil {
		return nil
	}
	has := containsRole(p.RoleIDs, MerchantAdminRoleID)
	if has == isAdmin {
		return nil
	}
	if isAdmin {
		if err := uc.repo.GrantRole(ctx, p.PlatformUserID, MerchantAdminRoleID); err != nil {
			return err
		}
	} else {
		if err := uc.repo.RevokeRole(ctx, p.PlatformUserID, MerchantAdminRoleID); err != nil {
			return err
		}
	}
	uc.flushDecisions(ctx)
	return nil
}

// List 准入列表
func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Projection, pagination.Page, error) {
	ps, total, err := uc.repo.List(ctx, q, page, pageSize)
	return ps, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// Get 单条
func (uc *Usecase) Get(ctx context.Context, id uint) (*Projection, error) {
	return uc.repo.FindByID(ctx, id)
}

// SetEnabled 开/关准入。关闭即「同步吊销本地授权」：决策缓存整体失效，
// 该用户的存量平台 token 对本系统立即失效（403）；平台侧账号不受影响。
func (uc *Usecase) SetEnabled(ctx context.Context, id uint, enabled bool) error {
	p, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	p.Enabled = enabled
	if err := uc.repo.Update(ctx, p); err != nil {
		return err
	}
	uc.flushDecisions(ctx)
	return nil
}

// SetRoles 调整本地角色（变更后决策缓存失效，权限即时生效）
func (uc *Usecase) SetRoles(ctx context.Context, id uint, roleIDs []uint) error {
	if err := uc.repo.SetRoles(ctx, id, roleIDs); err != nil {
		return err
	}
	uc.flushDecisions(ctx)
	return nil
}

// Delete 删除投影（移出本系统）：本地授权同步吊销，平台账号不受影响。
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.flushDecisions(ctx)
	return nil
}

// ListFromPlatform 成员列表（来源=平台本商户绑定成员，kw/分页在平台侧）：
// 批量合并本地准入投影（无投影=平台已绑定但本系统未准入）。
func (uc *Usecase) ListFromPlatform(ctx context.Context, keyword string, page, pageSize int) ([]*MemberRow, pagination.Page, error) {
	accounts, total, err := uc.platform.ListMembers(ctx, keyword, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	ids := make([]uint, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	projections, err := uc.repo.FindByPlatformUserIDs(ctx, ids)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	rows := make([]*MemberRow, 0, len(accounts))
	for _, a := range accounts {
		rows = append(rows, &MemberRow{
			PlatformUserID: a.ID, Username: a.Username, Nickname: a.Nickname,
			PlatformStatus: a.Status, IsAdmin: a.IsAdmin, Admitted: a.Admitted, Projection: projections[a.ID],
		})
	}
	return rows, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// AddMember 新增成员：先推平台（无此账号则创建并绑定本商户，有则仅绑定），
// 再建本地准入投影（默认开启准入）。账号已在本系统时拒绝（用启用/角色管理）。
func (uc *Usecase) AddMember(ctx context.Context, username, nickname, password string, enabled bool, roleIDs []uint) (*Projection, bool, error) {
	pid, existed, err := uc.platform.CreateOrBind(ctx, username, nickname, password)
	if err != nil {
		return nil, false, err
	}
	if p, _ := uc.repo.FindByPlatformUserID(ctx, pid); p != nil {
		return nil, existed, ErrDuplicatePlatformUser
	}
	np := &Projection{
		PlatformUserID: pid,
		Username:       username,
		Nickname:       nickname,
		Email:          "",
		Enabled:        enabled,
		RoleIDs:        roleIDs,
	}
	if err := uc.repo.Create(ctx, np); err != nil {
		return nil, existed, err
	}
	uc.flushDecisions(ctx)
	return np, existed, nil
}

// DeleteMember 移除成员：先解除平台侧绑定（账号本体保留；未绑定容忍为幂等），
// 再删除本地投影（同步吊销本地授权）。
func (uc *Usecase) DeleteMember(ctx context.Context, platformUserID uint) error {
	if err := uc.platform.Unbind(ctx, platformUserID); err != nil && !errors.Is(err, ErrPlatformNotBound) {
		return err
	}
	p, err := uc.repo.FindByPlatformUserID(ctx, platformUserID)
	if err != nil {
		return err
	}
	if p == nil {
		return nil // 平台已解绑且本地无投影：目标状态已达成
	}
	if err := uc.repo.Delete(ctx, p.ID); err != nil {
		return err
	}
	uc.flushDecisions(ctx)
	return nil
}

// resolveByPlatformUser 按平台用户 ID 取投影（成员行操作入口；未准入返回 ErrNotFound）
func (uc *Usecase) resolveByPlatformUser(ctx context.Context, platformUserID uint) (*Projection, error) {
	p, err := uc.repo.FindByPlatformUserID(ctx, platformUserID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// SetEnabledByPlatformUser 开/关准入（按平台用户 ID）。平台先行：准入状态先写回平台
// （merchant_users 事实源，经开放面），失败整体失败；成功后本地投影跟随并即时吊销/恢复本地授权。
func (uc *Usecase) SetEnabledByPlatformUser(ctx context.Context, platformUserID uint, enabled bool) error {
	if err := uc.platform.SetAdmission(ctx, platformUserID, enabled); err != nil {
		return err
	}
	p, err := uc.resolveByPlatformUser(ctx, platformUserID)
	if err != nil {
		return err
	}
	return uc.SetEnabled(ctx, p.ID, enabled)
}

// SetRolesByPlatformUser 调整本地角色（按平台用户 ID）
func (uc *Usecase) SetRolesByPlatformUser(ctx context.Context, platformUserID uint, roleIDs []uint) error {
	p, err := uc.resolveByPlatformUser(ctx, platformUserID)
	if err != nil {
		return err
	}
	return uc.SetRoles(ctx, p.ID, roleIDs)
}

// GetByPlatformUser 单条（按平台用户 ID；未准入返回 ErrNotFound）
func (uc *Usecase) GetByPlatformUser(ctx context.Context, platformUserID uint) (*Projection, error) {
	return uc.resolveByPlatformUser(ctx, platformUserID)
}

// SyncFromPlatform 从平台拉取本商户绑定成员：为尚无投影的成员建投影（成员语义=准入开启），
// 刷新存量投影的用户名/昵称快照，并按平台的商户管理员标记对账本地角色2绑定
// （标记缺失/被移出成员列表的持有者一并回收）。返回（新建数，刷新数）。
func (uc *Usecase) SyncFromPlatform(ctx context.Context) (created, refreshed int, err error) {
	page := 1
	const size = 100
	adminMarked := map[uint]bool{}
	for {
		accounts, _, err := uc.platform.ListMembers(ctx, "", page, size)
		if err != nil {
			return created, refreshed, err
		}
		for _, a := range accounts {
			adminMarked[a.ID] = a.IsAdmin
			p, _ := uc.repo.FindByPlatformUserID(ctx, a.ID)
			if p == nil {
				np := &Projection{
					PlatformUserID: a.ID,
					Username:       a.Username,
					Nickname:       a.Nickname,
					Email:          "",
					Enabled:        true, // 绑定即成员：成员默认准入（角色另行分配）
				}
				if a.IsAdmin {
					np.RoleIDs = []uint{MerchantAdminRoleID}
				}
				if cerr := uc.repo.Create(ctx, np); cerr == nil {
					created++
				}
				continue
			}
			if p.Username != a.Username || p.Nickname != a.Nickname || p.Enabled != a.Admitted {
				p.Username, p.Nickname = a.Username, a.Nickname
				p.Enabled = a.Admitted // 准入状态以平台为事实源（漂移自愈）
				if uerr := uc.repo.Update(ctx, p); uerr == nil {
					refreshed++
				}
			}
			// 管理员标记对账（不一致时绑/解角色2，Reconcile 内部已失效缓存）
			_ = uc.ReconcileMerchantAdmin(ctx, p, a.IsAdmin)
		}
		if len(accounts) < size {
			break
		}
		page++
	}
	// 持有角色2但已不在平台成员列表（或未标记）的：回收管理员绑定
	holders, err := uc.repo.RoleHolderUserIDs(ctx, MerchantAdminRoleID)
	if err != nil {
		return created, refreshed, err
	}
	revoked := false
	for _, uid := range holders {
		if adminMarked[uid] {
			continue
		}
		if err := uc.repo.RevokeRole(ctx, uid, MerchantAdminRoleID); err != nil {
			return created, refreshed, err
		}
		revoked = true
	}
	if revoked {
		uc.flushDecisions(ctx)
	}
	return created, refreshed, nil
}

func containsRole(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
