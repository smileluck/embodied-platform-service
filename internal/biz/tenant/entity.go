// Package tenant 租户限界上下文 —— 领域层
package tenant

import "time"

// Status 租户状态值对象
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// Tenant 租户聚合根（纯 Go，不依赖任何框架）
type Tenant struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	ContactName  string    `json:"contact_name"`
	ContactPhone string    `json:"contact_phone"`
	Remark       string    `json:"remark"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Enabled 是否启用
func (t *Tenant) Enabled() bool { return t.Status == StatusEnabled }
