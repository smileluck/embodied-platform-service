// 租户部门本地域（2026-10-10 租户内部数据隔离基础能力）：部门聚合、仓储端口与视图类型。
// tenant_id 一律为平台租户 ID、user_id 一律为平台 tenant_user ID（账号身份在平台，
// 本地只持组织架构与归属绑定；部门 (tenant_id, code) 唯一，删除走软删+墓碑释放槽位）。
package tenantdept

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrDeptNotFound 租户部门不存在（含不属于该租户的越权引用）
	ErrDeptNotFound = errors.New("租户部门不存在")
	// ErrDuplicateDeptCode 租户部门编码重复（(tenant_id, code) 唯一）
	ErrDuplicateDeptCode = errors.New("部门编码已存在，请更换")
	// ErrDeptHasChildren 部门存在子部门，禁止删除（须先删除/移走子部门）
	ErrDeptHasChildren = errors.New("部门下存在子部门，须先处理子部门")
	// ErrParentInvalid 父部门无效（不存在、跨租户、指向自身或后代成环）
	ErrParentInvalid = errors.New("父部门无效（不存在、跨租户或形成环路）")
)

// TenantDept 租户部门聚合根（树形自引用，ParentID=0 为根）
type TenantDept struct {
	ID        uint      `json:"id"`
	TenantID  uint      `json:"tenant_id"` // 平台租户 ID
	ParentID  uint      `json:"parent_id"` // 父部门 ID，0=根
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Sort      int       `json:"sort"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeptView 部门视图（MemberCount 为直属成员数，树形组装由前端完成——后端返回平表）
type DeptView struct {
	ID          uint   `json:"id"`
	TenantID    uint   `json:"tenant_id"`
	ParentID    uint   `json:"parent_id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Sort        int    `json:"sort"`
	Remark      string `json:"remark"`
	MemberCount int64  `json:"member_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// DeptRepo 本地租户部门仓储端口（由 data/tenantdept 实现，依赖倒置）。
// Delete 事务内软删部门（墓碑改写 code）并物理清理成员绑定（usecase 层已拒绝有子部门）
type DeptRepo interface {
	Create(ctx context.Context, d *TenantDept) error
	Update(ctx context.Context, d *TenantDept) error
	Delete(ctx context.Context, id uint) error
	Get(ctx context.Context, id uint) (*TenantDept, error)
	// List 租户下全量部门平表（单租户部门量级小，不分页；树形组装在前端）
	List(ctx context.Context, tenantID uint) ([]*TenantDept, error)
	// CountChildren 直属子部门数（删除守卫）
	CountChildren(ctx context.Context, parentID uint) (int64, error)
	// ReplaceUserDepts 事务内全量替换用户部门绑定（先删后插；deptIDs 已由用例层校验同租户）
	ReplaceUserDepts(ctx context.Context, userID uint, deptIDs []uint) error
	// DeleteUserDepts 清理用户全部部门绑定（用户删号时级联——成员计数依赖 binds，
	// 残留会虚增 member_count；幂等，无绑定删 0 行）
	DeleteUserDepts(ctx context.Context, userID uint) error
	// UserDeptIDs 批量取用户部门绑定（user_id -> dept_ids；列表补齐用）
	UserDeptIDs(ctx context.Context, userIDs []uint) (map[uint][]uint, error)
	// UserIDsByDepts 取部门直属成员用户 ID 集合（不含子部门——后代展开由用例层完成）
	UserIDsByDepts(ctx context.Context, deptIDs []uint) ([]uint, error)
	// MemberCounts 租户下各部门直属成员数（dept_id -> count）
	MemberCounts(ctx context.Context, tenantID uint) (map[uint]int64, error)
	// DeptNamesByIDs 批量取部门名（dept_id -> name；用户列表回填部门名用）
	DeptNamesByIDs(ctx context.Context, deptIDs []uint) (map[uint]string, error)
}
