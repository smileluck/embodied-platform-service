package admission

import (
	"context"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// DecisionCache 准入/授权决策缓存（RBAC 判定缓存；变更时整体失效，
// 保证「禁用/删除投影、改角色」对已登录请求即时生效——即本地授权的同步吊销）
type DecisionCache interface {
	Flush(ctx context.Context)
}

// PlatformAccount 平台账号信息（data 层经管理面服务账号拉取）
type PlatformAccount struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Status   int    `json:"status"` // 平台侧状态（1 启用 0 禁用；平台禁用=全局）
}

// PlatformUserReader 平台用户读取接口（data 层实现，跨上下文走最小接口）
type PlatformUserReader interface {
	ListPlatformUsers(ctx context.Context, username string, page, pageSize int) ([]*PlatformAccount, int64, error)
}

// Usecase 准入领域用例
type Usecase struct {
	repo      Repo
	platform  PlatformUserReader
	cache     DecisionCache
	bootstrap []string // 免准入引导账号（平台用户名）
}

func NewUsecase(repo Repo, platform PlatformUserReader, cache DecisionCache, c *conf.Bootstrap) *Usecase {
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
func (uc *Usecase) EnsureBootstrap(ctx context.Context, platformUserID uint, username string) (*Projection, error) {
	if p, err := uc.repo.FindByPlatformUserID(ctx, platformUserID); err == nil {
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
	p := &Projection{
		PlatformUserID: platformUserID,
		Username:       username,
		Nickname:       username,
		Enabled:        true,
		RoleIDs:        []uint{SuperRoleID},
	}
	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	uc.flushDecisions(ctx)
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

// SyncFromPlatform 从平台拉取用户列表：为尚无投影的平台用户建投影（默认停用），
// 并刷新存量投影的用户名/昵称快照。返回（新建数，刷新数）。
func (uc *Usecase) SyncFromPlatform(ctx context.Context) (created, refreshed int, err error) {
	page := 1
	const size = 100
	for {
		accounts, _, err := uc.platform.ListPlatformUsers(ctx, "", page, size)
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
					Email:          a.Email,
					Enabled:        false,
				}
				if cerr := uc.repo.Create(ctx, np); cerr == nil {
					created++
				}
				continue
			}
			if p.Username != a.Username || p.Nickname != a.Nickname || p.Email != a.Email {
				p.Username, p.Nickname, p.Email = a.Username, a.Nickname, a.Email
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
