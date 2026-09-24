// Package notice 通知公告限界上下文 —— 领域层。
// 管理端发布（markdown 正文/级别/生效窗口/送达范围），顶栏铃铛消费（未读计数 + 已读上报）。
// 送达范围：全体广播（all）或定向（按角色 roles / 按用户 users），消费端查询只看发给我的。
package notice

import (
	"context"
	"errors"
	"time"
)

// 哨兵错误
var (
	ErrNotFound       = errors.New("公告不存在")
	ErrInvalidTitle   = errors.New("公告不存在或未生效")
	ErrTimeRange      = errors.New("过期时间必须晚于发布时间")
	ErrInvalidTargets = errors.New("定向送达必须选择至少一个目标")
	ErrNotDelivered   = errors.New("公告未送达当前用户")
)

// Level 公告级别（前端着色）
type Level string

const (
	LevelInfo      Level = "info"
	LevelWarning   Level = "warning"
	LevelImportant Level = "important"
)

func ValidLevel(l string) bool {
	return l == string(LevelInfo) || l == string(LevelWarning) || l == string(LevelImportant)
}

// Scope 送达范围
type Scope string

const (
	ScopeAll   Scope = "all"   // 全体广播
	ScopeRoles Scope = "roles" // 按角色定向
	ScopeUsers Scope = "users" // 按用户定向
)

func ValidScope(s string) bool {
	return s == string(ScopeAll) || s == string(ScopeRoles) || s == string(ScopeUsers)
}

// TargetType 定向目标类型
const (
	TargetRole = "role"
	TargetUser = "user"
)

// Notice 公告
type Notice struct {
	ID          uint       `json:"id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"` // markdown
	Level       Level      `json:"level"`
	Scope       Scope      `json:"scope"` // all | roles | users
	PublishAt   time.Time  `json:"publish_at"`
	ExpireAt    *time.Time `json:"expire_at"` // nil=长期
	CreatorID   uint       `json:"creator_id"`
	CreatorName string     `json:"creator_name"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// 定向目标（scope=roles/users 时管理端回显；消费端不填充；名称与 ID 同序）
	RoleIDs   []uint   `json:"role_ids"`
	RoleNames []string `json:"role_names"`
	UserIDs   []uint   `json:"user_ids"`
	UserNames []string `json:"user_names"`
	// 已读标记（消费端查询时联表填充；管理端列表恒 false）
	HasRead bool `json:"has_read"`
}

// 生效中：已发布且未过期
func (n *Notice) Active() bool {
	now := time.Now()
	if n.PublishAt.After(now) {
		return false
	}
	return n.ExpireAt == nil || n.ExpireAt.After(now)
}

// Query 管理端列表条件
type Query struct {
	Title  string
	Level  string
	Status string // active | expired | pending；空=全部
}

// RoleOption 送达范围角色选项（发布表单用，仅 id+name）
type RoleOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// UserOption 送达范围用户选项（发布表单远程搜索用）
type UserOption struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// Repo 仓储接口
type Repo interface {
	Create(ctx context.Context, n *Notice) error
	Update(ctx context.Context, n *Notice) error
	Delete(ctx context.Context, id uint) error
	Find(ctx context.Context, id uint) (*Notice, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Notice, int64, error)
	// ListActive 生效中且送达本人的公告（消费端，publish_at 倒序，带 has_read 标记）
	ListActive(ctx context.Context, userID uint) ([]*Notice, error)
	// CountUnread 未读数（消费端角标，仅送达本人的）
	CountUnread(ctx context.Context, userID uint) (int64, error)
	// MarkRead 已读上报（幂等；须送达本人）
	MarkRead(ctx context.Context, userID, noticeID uint) error
	// ListRoleOptions / ListUserOptions 发布表单送达范围选项
	ListRoleOptions(ctx context.Context) ([]RoleOption, error)
	ListUserOptions(ctx context.Context, kw string, limit int) ([]UserOption, error)
}
