// Package role 角色限界上下文 —— 领域层
package role

import "time"

// Role 角色聚合根（json tag 与前端 Role 类型字段对齐，缺失会导致列表/详情序列化键大写、前端取不到值）
type Role struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Remark        string    `json:"remark"`
	PermissionIDs []uint    `json:"permission_ids"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
