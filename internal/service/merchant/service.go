// Package merchant 商户（开放 API 授权）应用服务
package merchant

import (
	"context"
	"time"

	bizmerchant "github.com/smilex/smilex-admin-gin/internal/biz/merchant"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type Service struct {
	uc *bizmerchant.Usecase
}

func NewService(uc *bizmerchant.Usecase) *Service { return &Service{uc: uc} }

// CreateRequest 创建商户入参
type CreateRequest struct {
	Name         string `json:"name" binding:"required,max=64"`
	Code         string `json:"code" binding:"required,min=2,max=64"`
	ContactName  string `json:"contact_name" binding:"max=64"`
	ContactPhone string `json:"contact_phone" binding:"max=32"`
	ContactEmail string `json:"contact_email" binding:"omitempty,max=128,email"`
	Remark       string `json:"remark" binding:"max=255"`
}

// UpdateRequest 更新商户入参（code 创建后不可改，故不含 code）
type UpdateRequest struct {
	Name         string `json:"name" binding:"required,max=64"`
	ContactName  string `json:"contact_name" binding:"max=64"`
	ContactPhone string `json:"contact_phone" binding:"max=32"`
	ContactEmail string `json:"contact_email" binding:"omitempty,max=128,email"`
	Remark       string `json:"remark" binding:"max=255"`
}

// SetStatusRequest 修改商户状态入参（1 启用 2 禁用）
type SetStatusRequest struct {
	Status int `json:"status" binding:"required,oneof=1 2"`
}

// VO 商户视图（联系方式一律脱敏；永不输出 app_secret_hash）
type VO struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	AppKey       string `json:"app_key"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	ContactEmail string `json:"contact_email"`
	Status       int    `json:"status"`
	Remark       string `json:"remark"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// SecretVO 创建/重置密钥响应：商户视图 + 明文 app_secret（仅此一次返回）
type SecretVO struct {
	*VO
	AppSecret string `json:"app_secret"`
}

// ToVO 商户实体转视图（联系方式脱敏；开放 API ping 与管理端共用）
func ToVO(m *bizmerchant.Merchant) *VO {
	return &VO{
		ID: m.ID, Name: m.Name, Code: m.Code, AppKey: m.AppKey,
		ContactName:  security.MaskName(m.ContactName),
		ContactPhone: security.MaskPhone(m.ContactPhone),
		ContactEmail: security.MaskEmail(m.ContactEmail),
		Status:       int(m.Status), Remark: m.Remark,
		CreatedAt: formatTime(m.CreatedAt), UpdatedAt: formatTime(m.UpdatedAt),
	}
}

// APILogVO API 调用日志视图（IP 脱敏）
type APILogVO struct {
	ID         uint   `json:"id"`
	MerchantID uint   `json:"merchant_id"`
	AppKey     string `json:"app_key"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	IP         string `json:"ip"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int    `json:"latency_ms"`
	Msg        string `json:"msg"`
	CreatedAt  string `json:"created_at"`
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*SecretVO, error) {
	m, secret, err := s.uc.Create(ctx, req.Name, req.Code, req.ContactName, req.ContactPhone, req.ContactEmail, req.Remark)
	if err != nil {
		return nil, err
	}
	return &SecretVO{VO: ToVO(m), AppSecret: secret}, nil
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	return s.uc.Update(ctx, id, req.Name, req.ContactName, req.ContactPhone, req.ContactEmail, req.Remark)
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }

func (s *Service) Get(ctx context.Context, id uint) (*VO, error) {
	m, err := s.uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToVO(m), nil
}

func (s *Service) List(ctx context.Context, q bizmerchant.Query, page, pageSize int) ([]*VO, interface{}, error) {
	merchants, pg, err := s.uc.List(ctx, q, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	out := make([]*VO, 0, len(merchants))
	for _, m := range merchants {
		out = append(out, ToVO(m))
	}
	return out, pg, nil
}

// ResetSecret 重置密钥：返回商户视图 + 新明文 app_secret（旧 secret 立即失效）
func (s *Service) ResetSecret(ctx context.Context, id uint) (*SecretVO, error) {
	secret, err := s.uc.ResetSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	m, err := s.uc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SecretVO{VO: ToVO(m), AppSecret: secret}, nil
}

func (s *Service) SetStatus(ctx context.Context, id uint, req SetStatusRequest) error {
	return s.uc.SetStatus(ctx, id, bizmerchant.Status(req.Status))
}

// ListAPILogs API 调用日志分页查询（IP 脱敏输出）
func (s *Service) ListAPILogs(ctx context.Context, q bizmerchant.APILogQuery, page, pageSize int) ([]*APILogVO, pagination.Page, error) {
	logs, pg, err := s.uc.ListAPILogs(ctx, q, page, pageSize)
	if err != nil {
		return nil, pg, err
	}
	out := make([]*APILogVO, 0, len(logs))
	for _, l := range logs {
		out = append(out, &APILogVO{
			ID: l.ID, MerchantID: l.MerchantID, AppKey: l.AppKey, Method: l.Method,
			Path: l.Path, IP: security.MaskIP(l.IP), StatusCode: l.StatusCode,
			LatencyMs: l.LatencyMs, Msg: l.Msg, CreatedAt: formatTime(l.CreatedAt),
		})
	}
	return out, pg, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
