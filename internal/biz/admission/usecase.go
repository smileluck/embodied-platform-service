package admission

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/conf"
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
	Status   int    `json:"status"` // 平台侧状态（1 启用 0 禁用；平台禁用=全局）
}

// MemberRow 成员列表合并行：平台绑定成员（来源=平台，唯一身份源）+ 本地准入投影（可空）
type MemberRow struct {
	PlatformUserID uint   `json:"platform_user_id"`
	Username       string `json:"username"`
	Nickname       string `json:"nickname"`
	PlatformStatus int    `json:"platform_status"` // 平台侧账号状态（禁用=平台全局封禁）
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
}

// Usecase 准入领域用例
type Usecase struct {
	repo      Repo
	platform  MemberGateway
	cache     DecisionCache
	bootstrap []string // 免准入引导账号（平台用户名）
}

func NewUsecase(repo Repo, platform MemberGateway, cache DecisionCache, c *conf.Bootstrap) *Usecase {
	return &Usecase{repo: repo, platform: platform, cache: cache, bootstrap: c.Platform.BootstrapAdmins}
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

// IsBootstrap 是否免准入引导账号
func (uc *Usecase) IsBootstrap(username string) bool {
	for _, u := range uc.bootstrap {
		if u == username {
			return true
		}
	}
	return false
}

// EnsureBootstrap 为引导账号建投影：已存在则直接返回（并确保已开启+绑超管角色）；
// 不存在则创建并绑定超管角色。冷启动时让配置内的平台管理员能进入本系统完成初始化。
// 注意 repo 契约：未建投影返回 (nil, nil)（非错误），须先判 nil 再解引用。
func (uc *Usecase) EnsureBootstrap(ctx context.Context, platformUserID uint, username string) (*Projection, error) {
	p, err := uc.repo.FindByPlatformUserID(ctx, platformUserID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		np := &Projection{
			PlatformUserID: platformUserID,
			Username:       username,
			Nickname:       username,
			Enabled:        true,
			RoleIDs:        []uint{SuperRoleID},
		}
		if err := uc.repo.Create(ctx, np); err != nil {
			return nil, err
		}
		uc.flushDecisions(ctx)
		return np, nil
	}
	if !p.Enabled || !containsRole(p.RoleIDs, SuperRoleID) {
		p.Enabled = true
		roles := append(p.RoleIDs, SuperRoleID)
		if err := uc.repo.Update(ctx, p); err != nil {
			return nil, err
		}
		if err := uc.repo.SetRoles(ctx, p.ID, unique(roles)); err != nil {
			return nil, err
		}
		uc.flushDecisions(ctx)
	}
	return p, nil
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
			PlatformStatus: a.Status, Projection: projections[a.ID],
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

// SetEnabledByPlatformUser 开/关准入（按平台用户 ID；关闭即同步吊销本地授权，即时生效）
func (uc *Usecase) SetEnabledByPlatformUser(ctx context.Context, platformUserID uint, enabled bool) error {
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
// 并刷新存量投影的用户名/昵称快照。返回（新建数，刷新数）。
func (uc *Usecase) SyncFromPlatform(ctx context.Context) (created, refreshed int, err error) {
	page := 1
	const size = 100
	for {
		accounts, _, err := uc.platform.ListMembers(ctx, "", page, size)
		if err != nil {
			return created, refreshed, err
		}
		for _, a := range accounts {
			p, _ := uc.repo.FindByPlatformUserID(ctx, a.ID)
			if p == nil {
				np := &Projection{
					PlatformUserID: a.ID,
					Username:       a.Username,
					Nickname:       a.Nickname,
					Email:          "",
					Enabled:        true, // 绑定即成员：成员默认准入（角色另行分配）
				}
				if cerr := uc.repo.Create(ctx, np); cerr == nil {
					created++
				}
				continue
			}
			if p.Username != a.Username || p.Nickname != a.Nickname {
				p.Username, p.Nickname = a.Username, a.Nickname
				if uerr := uc.repo.Update(ctx, p); uerr == nil {
					refreshed++
				}
			}
		}
		if len(accounts) < size {
			break
		}
		page++
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

func unique(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, v := range ids {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
