// Package model GORM 持久化对象（PO）与领域实体的转换。
// PO 只在 data 层出现，biz 层不可见。
package model

import (
	"encoding/json"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/admission"
	"github.com/smilex/smilex-admin-gin/internal/biz/agent"
	"github.com/smilex/smilex-admin-gin/internal/biz/appuser"
	"github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	"github.com/smilex/smilex-admin-gin/internal/biz/dict"
	"github.com/smilex/smilex-admin-gin/internal/biz/export"
	"github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/biz/job"
	"github.com/smilex/smilex-admin-gin/internal/biz/log"
	"github.com/smilex/smilex-admin-gin/internal/biz/notice"
	"github.com/smilex/smilex-admin-gin/internal/biz/permission"
	"github.com/smilex/smilex-admin-gin/internal/biz/role"
	"github.com/smilex/smilex-admin-gin/internal/biz/sysconfig"
	"github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	"gorm.io/gorm"
)

// PlatformUserPO 平台用户准入投影表（平台是唯一身份源：账号/密码/会话都在
// embodied-platform，本地仅存准入开关与角色绑定；删除投影=移出本系统，物理删除）
type PlatformUserPO struct {
	ID             uint   `gorm:"primaryKey"`
	PlatformUserID uint   `gorm:"uniqueIndex"`         // 平台用户 ID
	Username       string `gorm:"size:64;uniqueIndex"` // 平台用户名快照
	Nickname       string `gorm:"size:64"`
	Email          string `gorm:"size:128"`
	Enabled        bool   `gorm:"not null;default:false"` // 准入开关
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (PlatformUserPO) TableName() string { return "platform_users" }

// PlatformUserRolePO 准入投影-角色关联（主键为平台用户 ID，与投影一对一联动）
type PlatformUserRolePO struct {
	PlatformUserID uint `gorm:"primaryKey"`
	RoleID         uint `gorm:"primaryKey"`
}

func (PlatformUserRolePO) TableName() string { return "platform_user_roles" }

// RolePO 角色表
type RolePO struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"size:64;uniqueIndex"`
	Remark    string `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (RolePO) TableName() string { return "roles" }

// PermissionPO 权限表
type PermissionPO struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"size:64"`
	Code      string `gorm:"size:64;uniqueIndex"`
	Type      string `gorm:"size:16"` // dir | menu | button（api 已废弃，启动迁移自动转为 button；有菜单子级的 menu 迁移为 dir）
	Method    string `gorm:"size:16"`
	Path      string `gorm:"size:255"`
	ParentID  uint
	Icon      string `gorm:"size:512"`
	Sort      int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (PermissionPO) TableName() string { return "permissions" }

// RolePermissionPO 角色-权限关联
type RolePermissionPO struct {
	RoleID       uint `gorm:"primaryKey"`
	PermissionID uint `gorm:"primaryKey"`
}

func (RolePermissionPO) TableName() string { return "role_permissions" }

// OperationLogPO 操作日志表（写请求审计流水：无软删，清空/保留期清理均为物理删除）
type OperationLogPO struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"index"`         // 操作人（平台用户 ID；认证被拒时为 0）
	Username   string    `gorm:"size:64;index"` // 操作人用户名快照
	Method     string    `gorm:"size:8;index"`
	Path       string    `gorm:"size:255"`  // 实际请求路径（含资源 ID 与 query）
	Route      string    `gorm:"size:128"`  // 路由模板（如 /api/v1/users/:id）
	Action     string    `gorm:"size:64"`   // 中文动作名（如「新增用户」）
	Params     string    `gorm:"type:text"` // 请求参数摘要（敏感字段脱敏、超长截断）
	IP         string    `gorm:"size:64"`
	UserAgent  string    `gorm:"size:255"`
	StatusCode int       // 响应状态码
	LatencyMs  int       // 耗时（毫秒）
	CreatedAt  time.Time `gorm:"index"`
}

func (OperationLogPO) TableName() string { return "operation_logs" }

// FilePO 文件元数据表（对象本体在平台 storage-gateway；本地仅登记 bucket/key，
// 下载经网关预签名 URL 302）
type FilePO struct {
	ID           uint   `gorm:"primaryKey"`
	Driver       string `gorm:"size:16;index"`                 // 恒为 platform（历史记录可能为 local 等旧驱动）
	Bucket       string `gorm:"size:64;index:idx_bucket_key"`  // 平台存储桶
	ObjectKey    string `gorm:"size:512;index:idx_bucket_key"` // 服务端生成的对象 key
	Name         string `gorm:"size:255"`                      // 原始文件名
	Ext          string `gorm:"size:16;index"`                 // 扩展名（小写，不含点）
	Size         int64
	ContentType  string `gorm:"size:128"`
	UploaderID   uint   `gorm:"index"` // 上传者（平台用户 ID）
	UploaderName string `gorm:"size:64"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (FilePO) TableName() string { return "files" }

// IPBlacklistPO IP 黑名单表（手工持久化封禁；软删即解封留痕）
type IPBlacklistPO struct {
	ID          uint       `gorm:"primaryKey"`
	IP          string     `gorm:"size:64;uniqueIndex"`    // 仅单个 IP（不支持 CIDR），归一化后存储
	Reason      string     `gorm:"size:255"`               // 封禁原因（选填）
	Source      string     `gorm:"size:16;default:manual"` // manual
	ExpireAt    *time.Time // 过期时间（NULL 为永久封禁，到期惰性放行）
	CreatorID   uint       // 操作人
	CreatorName string     `gorm:"size:64"` // 操作人用户名快照
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (IPBlacklistPO) TableName() string { return "ip_blacklist" }

// ExportRecordPO 异步导出任务记录表（产物落平台存储；追加型流水：无软删，
// 保留期清理与手动删除均为物理删除）
type ExportRecordPO struct {
	ID         uint       `gorm:"primaryKey"`
	UserID     uint       `gorm:"index"`     // 任务归属用户（平台用户 ID）
	Biz        string     `gorm:"size:32"`   // 业务类型（user / op_log）
	Name       string     `gorm:"size:255"`  // 展示名（兼作下载文件名）
	Params     string     `gorm:"type:text"` // 查询条件快照（JSON）
	Driver     string     `gorm:"size:16"`   // 产物落库时的存储后端
	ObjectKey  string     `gorm:"size:512"`  // 产物对象 key
	Size       int64      // 产物字节数（含 BOM）
	Rows       int        // 已导出数据行数（不含表头）
	Status     string     `gorm:"size:16;index"` // pending | running | done | failed
	Truncated  bool       // 触及大小/行数上限被截断
	Error      string     `gorm:"size:512"` // 失败原因（成功为空）
	CreatedAt  time.Time  `gorm:"index"`
	FinishedAt *time.Time // 完成/失败时间（未结束为 NULL）
}

func (ExportRecordPO) TableName() string { return "export_records" }

// TenantPO 租户表（name/code 均唯一，软删留痕；platform_id 为 embodied-platform
// 侧租户 ID——创建/更新/删除与平台强一致同步，绑入商户租户集后开放面设备注册才可用）
type TenantPO struct {
	ID           uint   `gorm:"primaryKey"`
	PlatformID   uint   `gorm:"index"` // 平台租户 ID（0=尚未同步）
	Name         string `gorm:"size:64;uniqueIndex"`
	Code         string `gorm:"size:64;uniqueIndex"`
	ContactName  string `gorm:"size:64"`
	ContactPhone string `gorm:"size:32"`
	Remark       string `gorm:"size:255"`
	Status       int    // 1 启用 0 禁用
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (TenantPO) TableName() string { return "tenants" }

// AppUserPO 应用用户表（多租户终端用户；username 唯一，软删留痕；密码只存 bcrypt 哈希）
type AppUserPO struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"size:64;uniqueIndex"`
	PasswordHash string `gorm:"size:128"` // bcrypt 哈希，列表/详情查询不输出
	Nickname     string `gorm:"size:64"`
	Phone        string `gorm:"size:32"`
	Email        string `gorm:"size:128"`
	Status       int    // 1 启用 0 禁用
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (AppUserPO) TableName() string { return "app_users" }

// AppUserTenantPO 应用用户-租户关联表（复合唯一索引；替换关联时物理删除避免软删残留阻碍重绑）
type AppUserTenantPO struct {
	ID        uint `gorm:"primaryKey"`
	AppUserID uint `gorm:"uniqueIndex:uk_app_user_tenant"`
	TenantID  uint `gorm:"uniqueIndex:uk_app_user_tenant"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (AppUserTenantPO) TableName() string { return "app_user_tenants" }

// AgentProviderPO LLM 供应商配置表（智能体底座；api_key 只存 AES-GCM 密文，掩码仅展示用）
type AgentProviderPO struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"size:20"`
	Code       string `gorm:"size:64;uniqueIndex"`
	BaseURL    string `gorm:"size:255"`
	APIKeyEnc  string `gorm:"size:512"`               // AES-GCM 密文（base64），永不输出
	APIKeyMask string `gorm:"size:32"`                // 展示掩码（如 sk-****abcd）
	Protocol   string `gorm:"size:32;default:openai"` // 调用协议（预留多协议扩展）
	Remark     string `gorm:"size:200"`
	Status     int    // 1 启用 0 禁用
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (AgentProviderPO) TableName() string { return "agent_providers" }

// AgentModelPO 供应商下的模型配置表（同一供应商下模型名唯一）
type AgentModelPO struct {
	ID            uint    `gorm:"primaryKey"`
	ProviderID    uint    `gorm:"index;uniqueIndex:uk_agent_provider_model"`
	Name          string  `gorm:"size:128;uniqueIndex:uk_agent_provider_model"`
	DisplayName   string  `gorm:"size:20"`
	ContextWindow int     // 上下文窗口（token，0=未知）
	MaxOutput     int     // 单次最大输出（token，0=上游默认）
	SupportsTools bool    // 支持工具调用（function call）
	InputPrice    float64 // 每千 token 输入单价（0=未设置不估算）
	OutputPrice   float64 // 每千 token 输出单价
	Remark        string  `gorm:"size:200"`
	Status        int     // 1 启用 0 禁用
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

func (AgentModelPO) TableName() string { return "agent_models" }

// AgentPO 智能体配置表（业务按 code 稳定引用）
type AgentPO struct {
	ID           uint   `gorm:"primaryKey"`
	Name         string `gorm:"size:20"`
	Code         string `gorm:"size:64;uniqueIndex"`
	ModelID      uint   `gorm:"index"` // 绑定 agent_models.id（经模型定位供应商）
	SystemPrompt string `gorm:"type:text"`
	Temperature  float64
	TopP         float64
	MaxTokens    int    // 0=上游默认
	Tools        string `gorm:"size:512"` // 绑定的本地工具名 JSON 数组（空=无工具）
	Remark       string `gorm:"size:200"`
	Status       int    // 1 启用 0 禁用
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (AgentPO) TableName() string { return "agents" }

// AgentConversationPO 对话会话表（本人数据；AgentName 冗余，Agent 改名/删除后历史仍可读）
type AgentConversationPO struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index"` // 归属用户（列表/详情强制过滤）
	AgentID   uint   `gorm:"index"` // 关联 agents.id（逻辑引用，Agent 删除不级联）
	AgentName string `gorm:"size:20"`
	Title     string `gorm:"size:64"`
	LastMsgAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (AgentConversationPO) TableName() string { return "agent_conversations" }

// AgentConversationMsgPO 会话消息表（追加流水；会话删除时物理级联清理）
type AgentConversationMsgPO struct {
	ID             uint   `gorm:"primaryKey"`
	ConversationID uint   `gorm:"index"`
	Role           string `gorm:"size:16"` // user | assistant
	Content        string `gorm:"type:text"`
	TotalTokens    int    // assistant 消息的 usage.total_tokens
	CreatedAt      time.Time
}

func (AgentConversationMsgPO) TableName() string { return "agent_conversation_msgs" }

// AgentUsageLogPO 用量计量流水表（追加；保留期每日清理）
type AgentUsageLogPO struct {
	ID               uint `gorm:"primaryKey"`
	AgentID          uint `gorm:"index"`
	ModelID          uint
	UserID           uint `gorm:"index"`
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LatencyMs        int64
	CreatedAt        time.Time `gorm:"index"`
}

func (AgentUsageLogPO) TableName() string { return "agent_usage_logs" }

// ---- 转换器 ----

func PlatformUserToPO(p *admission.Projection) *PlatformUserPO {
	return &PlatformUserPO{
		ID: p.ID, PlatformUserID: p.PlatformUserID, Username: p.Username,
		Nickname: p.Nickname, Email: p.Email, Enabled: p.Enabled,
	}
}

func PlatformUserFromPO(po *PlatformUserPO) *admission.Projection {
	return &admission.Projection{
		ID: po.ID, PlatformUserID: po.PlatformUserID, Username: po.Username,
		Nickname: po.Nickname, Email: po.Email, Enabled: po.Enabled,
		CreatedAt: po.CreatedAt, UpdatedAt: po.UpdatedAt,
	}
}

func RoleToPO(r *role.Role) *RolePO {
	return &RolePO{ID: r.ID, Name: r.Name, Remark: r.Remark}
}

func RoleFromPO(p *RolePO) *role.Role {
	return &role.Role{ID: p.ID, Name: p.Name, Remark: p.Remark, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func PermissionToPO(m *permission.Permission) *PermissionPO {
	return &PermissionPO{
		ID: m.ID, Name: m.Name, Code: m.Code, Type: string(m.Type),
		Method: m.Method, Path: m.Path, ParentID: m.ParentID,
		Icon: m.Icon, Sort: m.Sort,
	}
}

func PermissionFromPO(p *PermissionPO) *permission.Permission {
	return &permission.Permission{
		ID: p.ID, Name: p.Name, Code: p.Code, Type: permission.Type(p.Type),
		Method: p.Method, Path: p.Path, ParentID: p.ParentID,
		Icon: p.Icon, Sort: p.Sort,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func OperationLogToPO(o *log.OperationLog) *OperationLogPO {
	return &OperationLogPO{
		ID: o.ID, UserID: o.UserID, Username: o.Username, Method: o.Method,
		Path: o.Path, Route: o.Route, Action: o.Action, Params: o.Params,
		IP: o.IP, UserAgent: o.UserAgent, StatusCode: o.StatusCode,
		LatencyMs: o.LatencyMs, CreatedAt: o.CreatedAt,
	}
}

func OperationLogFromPO(p *OperationLogPO) *log.OperationLog {
	return &log.OperationLog{
		ID: p.ID, UserID: p.UserID, Username: p.Username, Method: p.Method,
		Path: p.Path, Route: p.Route, Action: p.Action, Params: p.Params,
		IP: p.IP, UserAgent: p.UserAgent, StatusCode: p.StatusCode,
		LatencyMs: p.LatencyMs, CreatedAt: p.CreatedAt,
	}
}

func FileToPO(f *file.File) *FilePO {
	return &FilePO{
		ID: f.ID, Driver: f.Driver, Bucket: f.Bucket, ObjectKey: f.ObjectKey, Name: f.Name,
		Ext: f.Ext, Size: f.Size, ContentType: f.ContentType,
		UploaderID: f.UploaderID, UploaderName: f.UploaderName,
	}
}

func FileFromPO(p *FilePO) *file.File {
	return &file.File{
		ID: p.ID, Driver: p.Driver, Bucket: p.Bucket, ObjectKey: p.ObjectKey, Name: p.Name,
		Ext: p.Ext, Size: p.Size, ContentType: p.ContentType,
		UploaderID: p.UploaderID, UploaderName: p.UploaderName,
		CreatedAt: p.CreatedAt,
	}
}

func ExportRecordToPO(r *export.ExportRecord) *ExportRecordPO {
	return &ExportRecordPO{
		ID: r.ID, UserID: r.UserID, Biz: r.Biz, Name: r.Name, Params: r.Params,
		Driver: r.Driver, ObjectKey: r.ObjectKey, Size: r.Size, Rows: r.Rows,
		Status: r.Status, Truncated: r.Truncated, Error: r.Error,
		CreatedAt: r.CreatedAt, FinishedAt: r.FinishedAt,
	}
}

func ExportRecordFromPO(p *ExportRecordPO) *export.ExportRecord {
	return &export.ExportRecord{
		ID: p.ID, UserID: p.UserID, Biz: p.Biz, Name: p.Name, Params: p.Params,
		Driver: p.Driver, ObjectKey: p.ObjectKey, Size: p.Size, Rows: p.Rows,
		Status: p.Status, Truncated: p.Truncated, Error: p.Error,
		CreatedAt: p.CreatedAt, FinishedAt: p.FinishedAt,
	}
}

func IPBlacklistToPO(b *blacklist.IPBlacklist) *IPBlacklistPO {
	return &IPBlacklistPO{
		ID: b.ID, IP: b.IP, Reason: b.Reason, Source: b.Source, ExpireAt: b.ExpireAt,
		CreatorID: b.CreatorID, CreatorName: b.CreatorName,
	}
}

func IPBlacklistFromPO(p *IPBlacklistPO) *blacklist.IPBlacklist {
	return &blacklist.IPBlacklist{
		ID: p.ID, IP: p.IP, Reason: p.Reason, Source: p.Source, ExpireAt: p.ExpireAt,
		CreatorID: p.CreatorID, CreatorName: p.CreatorName,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func TenantToPO(t *tenant.Tenant) *TenantPO {
	return &TenantPO{
		ID: t.ID, PlatformID: t.PlatformID, Name: t.Name, Code: t.Code,
		ContactName: t.ContactName, ContactPhone: t.ContactPhone,
		Remark: t.Remark, Status: int(t.Status),
	}
}

func TenantFromPO(p *TenantPO) *tenant.Tenant {
	return &tenant.Tenant{
		ID: p.ID, PlatformID: p.PlatformID, Name: p.Name, Code: p.Code,
		ContactName: p.ContactName, ContactPhone: p.ContactPhone,
		Remark: p.Remark, Status: tenant.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func AppUserToPO(u *appuser.AppUser) *AppUserPO {
	return &AppUserPO{
		ID: u.ID, Username: u.Username, PasswordHash: u.PasswordHash,
		Nickname: u.Nickname, Phone: u.Phone, Email: u.Email, Status: int(u.Status),
	}
}

func AppUserFromPO(p *AppUserPO) *appuser.AppUser {
	return &appuser.AppUser{
		ID: p.ID, Username: p.Username, PasswordHash: p.PasswordHash,
		Nickname: p.Nickname, Phone: p.Phone, Email: p.Email, Status: appuser.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func AgentProviderToPO(a *agent.Provider) *AgentProviderPO {
	return &AgentProviderPO{
		ID: a.ID, Name: a.Name, Code: a.Code, BaseURL: a.BaseURL,
		APIKeyEnc: a.APIKeyEnc, APIKeyMask: a.APIKeyMask,
		Protocol: a.Protocol, Remark: a.Remark, Status: int(a.Status),
	}
}

func AgentProviderFromPO(p *AgentProviderPO) *agent.Provider {
	return &agent.Provider{
		ID: p.ID, Name: p.Name, Code: p.Code, BaseURL: p.BaseURL,
		APIKeyEnc: p.APIKeyEnc, APIKeyMask: p.APIKeyMask,
		Protocol: p.Protocol, Remark: p.Remark, Status: agent.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func AgentModelToPO(m *agent.Model) *AgentModelPO {
	return &AgentModelPO{
		ID: m.ID, ProviderID: m.ProviderID, Name: m.Name, DisplayName: m.DisplayName,
		ContextWindow: m.ContextWindow, MaxOutput: m.MaxOutput, SupportsTools: m.SupportsTools,
		InputPrice: m.InputPrice, OutputPrice: m.OutputPrice,
		Remark: m.Remark, Status: int(m.Status),
	}
}

func AgentModelFromPO(p *AgentModelPO) *agent.Model {
	return &agent.Model{
		ID: p.ID, ProviderID: p.ProviderID, Name: p.Name, DisplayName: p.DisplayName,
		ContextWindow: p.ContextWindow, MaxOutput: p.MaxOutput, SupportsTools: p.SupportsTools,
		InputPrice: p.InputPrice, OutputPrice: p.OutputPrice,
		Remark: p.Remark, Status: agent.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func AgentToPO(a *agent.Agent) *AgentPO {
	return &AgentPO{
		ID: a.ID, Name: a.Name, Code: a.Code, ModelID: a.ModelID,
		SystemPrompt: a.SystemPrompt, Temperature: a.Temperature, TopP: a.TopP,
		MaxTokens: a.MaxTokens, Tools: MarshalAgentTools(a.Tools), Remark: a.Remark, Status: int(a.Status),
	}
}

func AgentFromPO(p *AgentPO) *agent.Agent {
	return &agent.Agent{
		ID: p.ID, Name: p.Name, Code: p.Code, ModelID: p.ModelID,
		SystemPrompt: p.SystemPrompt, Temperature: p.Temperature, TopP: p.TopP,
		MaxTokens: p.MaxTokens, Tools: UnmarshalAgentTools(p.Tools), Remark: p.Remark, Status: agent.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// marshalTools 工具名列表 <-> JSON 文本（空列表与空串互转）
func MarshalAgentTools(names []string) string {
	if len(names) == 0 {
		return ""
	}
	b, err := json.Marshal(names)
	if err != nil {
		return ""
	}
	return string(b)
}

func UnmarshalAgentTools(s string) []string {
	if s == "" {
		return nil
	}
	var names []string
	if json.Unmarshal([]byte(s), &names) != nil {
		return nil
	}
	return names
}

func AgentConversationToPO(cv *agent.Conversation) *AgentConversationPO {
	return &AgentConversationPO{
		ID: cv.ID, UserID: cv.UserID, AgentID: cv.AgentID, AgentName: cv.AgentName,
		Title: cv.Title, LastMsgAt: cv.LastMsgAt,
	}
}

func AgentConversationFromPO(p *AgentConversationPO) *agent.Conversation {
	return &agent.Conversation{
		ID: p.ID, UserID: p.UserID, AgentID: p.AgentID, AgentName: p.AgentName,
		Title: p.Title, LastMsgAt: p.LastMsgAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func AgentMsgToPO(m *agent.ConversationMessage) *AgentConversationMsgPO {
	return &AgentConversationMsgPO{
		ID: m.ID, ConversationID: m.ConversationID, Role: m.Role,
		Content: m.Content, TotalTokens: m.TotalTokens, CreatedAt: m.CreatedAt,
	}
}

func AgentMsgFromPO(p *AgentConversationMsgPO) *agent.ConversationMessage {
	return &agent.ConversationMessage{
		ID: p.ID, ConversationID: p.ConversationID, Role: p.Role,
		Content: p.Content, TotalTokens: p.TotalTokens, CreatedAt: p.CreatedAt,
	}
}

func AgentUsageToPO(u *agent.UsageLog) *AgentUsageLogPO {
	return &AgentUsageLogPO{
		ID: u.ID, AgentID: u.AgentID, ModelID: u.ModelID, UserID: u.UserID,
		PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens,
		TotalTokens: u.TotalTokens, LatencyMs: u.LatencyMs, CreatedAt: u.CreatedAt,
	}
}

func AgentUsageFromPO(p *AgentUsageLogPO) *agent.UsageLog {
	return &agent.UsageLog{
		ID: p.ID, AgentID: p.AgentID, ModelID: p.ModelID, UserID: p.UserID,
		PromptTokens: p.PromptTokens, CompletionTokens: p.CompletionTokens,
		TotalTokens: p.TotalTokens, LatencyMs: p.LatencyMs, CreatedAt: p.CreatedAt,
	}
}

// DictTypePO 字典类型表
type DictTypePO struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"size:20"`
	Code      string `gorm:"size:64;uniqueIndex"`
	Remark    string `gorm:"size:200"`
	Status    int    // 1 启用 0 禁用
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (DictTypePO) TableName() string { return "dict_types" }

// DictItemPO 字典项表（同类型下 label/value 唯一）
type DictItemPO struct {
	ID        uint   `gorm:"primaryKey"`
	TypeID    uint   `gorm:"index;uniqueIndex:uk_dict_item"`
	Label     string `gorm:"size:20;uniqueIndex:uk_dict_item"`
	Value     string `gorm:"size:64;uniqueIndex:uk_dict_item"`
	Sort      int
	Remark    string `gorm:"size:200"`
	Status    int    // 1 启用 0 禁用
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (DictItemPO) TableName() string { return "dict_items" }

func DictTypeToPO(t *dict.DictType) *DictTypePO {
	return &DictTypePO{ID: t.ID, Name: t.Name, Code: t.Code, Remark: t.Remark, Status: int(t.Status)}
}

func DictTypeFromPO(p *DictTypePO) *dict.DictType {
	return &dict.DictType{
		ID: p.ID, Name: p.Name, Code: p.Code, Remark: p.Remark, Status: dict.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func DictItemToPO(i *dict.DictItem) *DictItemPO {
	return &DictItemPO{
		ID: i.ID, TypeID: i.TypeID, Label: i.Label, Value: i.Value,
		Sort: i.Sort, Remark: i.Remark, Status: int(i.Status),
	}
}

func DictItemFromPO(p *DictItemPO) *dict.DictItem {
	return &dict.DictItem{
		ID: p.ID, TypeID: p.TypeID, Label: p.Label, Value: p.Value,
		Sort: p.Sort, Remark: p.Remark, Status: dict.Status(p.Status),
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// SysConfigPO 系统参数表（key 唯一；运行时可调，读走缓存）
type SysConfigPO struct {
	Key         string `gorm:"column:key;primaryKey;size:64"`
	Value       string `gorm:"size:512"`
	Type        string `gorm:"size:16"` // string | number | bool
	Description string `gorm:"size:200"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SysConfigPO) TableName() string { return "sys_configs" }

func SysConfigToPO(c *sysconfig.Config) *SysConfigPO {
	return &SysConfigPO{Key: c.Key, Value: c.Value, Type: string(c.Type), Description: c.Description}
}

func SysConfigFromPO(p *SysConfigPO) *sysconfig.Config {
	return &sysconfig.Config{Key: p.Key, Value: p.Value, Type: sysconfig.ValueType(p.Type), Description: p.Description, UpdatedAt: p.UpdatedAt}
}

// NoticePO 通知公告表
type NoticePO struct {
	ID          uint      `gorm:"primaryKey"`
	Title       string    `gorm:"size:100"`
	Content     string    `gorm:"type:text"`                 // markdown
	Level       string    `gorm:"size:16"`                   // info | warning | important
	Scope       string    `gorm:"size:16;default:all;index"` // 送达范围：all | roles | users（存量广播公告缺省 all）
	PublishAt   time.Time `gorm:"index"`
	ExpireAt    *time.Time
	CreatorID   uint
	CreatorName string `gorm:"size:20"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (NoticePO) TableName() string { return "notices" }

// NoticeReadPO 公告已读记录（用户+公告 唯一）
type NoticeReadPO struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint `gorm:"uniqueIndex:uk_notice_read"`
	NoticeID  uint `gorm:"uniqueIndex:uk_notice_read"`
	CreatedAt time.Time
}

func (NoticeReadPO) TableName() string { return "notice_reads" }

// NoticeTargetPO 定向送达目标（scope=roles/users 时生效；公告+目标类型+目标ID 唯一）
type NoticeTargetPO struct {
	ID         uint   `gorm:"primaryKey"`
	NoticeID   uint   `gorm:"uniqueIndex:uk_notice_target;index"`
	TargetType string `gorm:"size:8;uniqueIndex:uk_notice_target"` // role | user
	TargetID   uint   `gorm:"uniqueIndex:uk_notice_target"`
	CreatedAt  time.Time
}

func (NoticeTargetPO) TableName() string { return "notice_targets" }

func NoticeToPO(n *notice.Notice) *NoticePO {
	return &NoticePO{
		ID: n.ID, Title: n.Title, Content: n.Content, Level: string(n.Level),
		Scope:     string(n.Scope),
		PublishAt: n.PublishAt, ExpireAt: n.ExpireAt,
		CreatorID: n.CreatorID, CreatorName: n.CreatorName,
	}
}

func NoticeFromPO(p *NoticePO) *notice.Notice {
	return &notice.Notice{
		ID: p.ID, Title: p.Title, Content: p.Content, Level: notice.Level(p.Level),
		Scope:     notice.Scope(p.Scope),
		PublishAt: p.PublishAt, ExpireAt: p.ExpireAt,
		CreatorID: p.CreatorID, CreatorName: p.CreatorName,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// JobPO 定时任务表
type JobPO struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"size:20"`
	Cron       string `gorm:"size:32"`
	HandlerKey string `gorm:"size:64"`
	Params     string `gorm:"size:512"`
	Remark     string `gorm:"size:200"`
	Status     int    // 1 启用 0 停用
	LastRunAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (JobPO) TableName() string { return "jobs" }

// JobLogPO 任务执行记录表（追加流水）
type JobLogPO struct {
	ID         uint   `gorm:"primaryKey"`
	JobID      uint   `gorm:"index"`
	JobName    string `gorm:"size:20"`
	HandlerKey string `gorm:"size:64"`
	Status     string `gorm:"size:16"` // success | failed
	Output     string `gorm:"size:2000"`
	DurationMs int64
	StartedAt  time.Time `gorm:"index"`
}

func (JobLogPO) TableName() string { return "job_logs" }

func JobToPO(j *job.Job) *JobPO {
	return &JobPO{
		ID: j.ID, Name: j.Name, Cron: j.Cron, HandlerKey: j.HandlerKey,
		Params: j.Params, Remark: j.Remark, Status: int(j.Status), LastRunAt: j.LastRunAt,
	}
}

func JobFromPO(p *JobPO) *job.Job {
	return &job.Job{
		ID: p.ID, Name: p.Name, Cron: p.Cron, HandlerKey: p.HandlerKey,
		Params: p.Params, Remark: p.Remark, Status: job.Status(p.Status),
		LastRunAt: p.LastRunAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func JobLogToPO(l *job.JobLog) *JobLogPO {
	return &JobLogPO{
		ID: l.ID, JobID: l.JobID, JobName: l.JobName, HandlerKey: l.HandlerKey,
		Status: l.Status, Output: l.Output, DurationMs: l.DurationMs, StartedAt: l.StartedAt,
	}
}

func JobLogFromPO(p *JobLogPO) *job.JobLog {
	return &job.JobLog{
		ID: p.ID, JobID: p.JobID, JobName: p.JobName, HandlerKey: p.HandlerKey,
		Status: p.Status, Output: p.Output, DurationMs: p.DurationMs, StartedAt: p.StartedAt,
	}
}

// MonitorSnapshotPO 监控历史快照表（60s 一点，保留 7 天）
type MonitorSnapshotPO struct {
	ID          uint      `gorm:"primaryKey"`
	Ts          time.Time `gorm:"index"`
	CPUPercent  float64
	MemPercent  float64
	SwapPercent float64
	NetSendRate float64 // B/s
	NetRecvRate float64
}

func (MonitorSnapshotPO) TableName() string { return "monitor_snapshots" }
