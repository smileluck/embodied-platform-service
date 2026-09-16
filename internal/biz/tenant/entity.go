// Package tenant 租户限界上下文 —— 领域层
package tenant

import "time"

// Status 租户状态值对象
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// Tenant 租户聚合根（纯 Go，不依赖任何框架）。
// PlatformID 为 embodied-platform 侧租户 ID：本系统租户与平台强一致同步
// （创建即在平台建租户并绑入本服务商户租户集），未同步的租户无法注册设备。
type Tenant struct {
	ID           uint      `json:"id"`
	PlatformID   uint      `json:"platform_id"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	ContactName  string    `json:"contact_name"`
	ContactPhone string    `json:"contact_phone"`
	Remark       string    `json:"remark"`
	Status       Status    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Synced 是否已与平台同步（platform_id 非零）
func (t *Tenant) Synced() bool { return t.PlatformID > 0 }

// Enabled 是否启用
func (t *Tenant) Enabled() bool { return t.Status == StatusEnabled }
