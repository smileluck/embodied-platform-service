// Package mcp MCP 服务应用服务（配置管理 + 连通测试）
package mcp

import (
	"context"
	"strings"

	bizmcp "github.com/smilex/smilex-admin-gin/internal/biz/mcp"
)

type Service struct {
	uc *bizmcp.Usecase
}

func NewService(uc *bizmcp.Usecase) *Service { return &Service{uc: uc} }

// HeaderRequest 自定义请求头（数量与长度上限；敏感头在 usecase 之前的此处过滤）
type HeaderRequest struct {
	Key   string `json:"key" binding:"max=64"`
	Value string `json:"value" binding:"max=512"`
}

const maxHeaders = 10

// CreateRequest 新增（全量字段必填除 token/headers）
type CreateRequest struct {
	Name      string          `json:"name" binding:"required,max=20"`
	Code      string          `json:"code" binding:"required,max=64"`
	Transport string          `json:"transport" binding:"required,oneof=streamable_http sse"`
	BaseURL   string          `json:"base_url" binding:"required,max=255"`
	Token     string          `json:"token" binding:"max=255"` // 明文仅写入链路；空=不配置凭证
	Headers   []HeaderRequest `json:"headers" binding:"omitempty,max=10,dive"`
	Remark    string          `json:"remark" binding:"max=200"`
	Status    *int            `json:"status" binding:"omitempty,gte=0,lte=1"` // 缺省启用
}

type UpdateRequest struct {
	Name      string          `json:"name" binding:"required,max=20"`
	Code      string          `json:"code" binding:"required,max=64"`
	Transport string          `json:"transport" binding:"required,oneof=streamable_http sse"`
	BaseURL   string          `json:"base_url" binding:"required,max=255"`
	Token     string          `json:"token" binding:"max=255"` // 空=保持不变
	Headers   []HeaderRequest `json:"headers" binding:"omitempty,max=10,dive"`
	Remark    string          `json:"remark" binding:"max=200"`
	Status    *int            `json:"status" binding:"omitempty,gte=0,lte=1"`
}

// normalizeHeaders 清洗自定义头：去空白、丢空键与协议保护头、超量截断
func normalizeHeaders(in []HeaderRequest) []bizmcp.Header {
	out := make([]bizmcp.Header, 0, len(in))
	for _, h := range in {
		key := strings.TrimSpace(h.Key)
		if key == "" || len(out) >= maxHeaders {
			continue
		}
		lower := strings.ToLower(key)
		switch lower {
		case "authorization", "content-type", "accept", "content-length", "host", "cookie", "mcp-session-id":
			continue // 敏感/协议头走 Token 通道与协议固定头，不允许自定义覆盖
		}
		out = append(out, bizmcp.Header{Key: key, Value: strings.TrimSpace(h.Value)})
	}
	return out
}

func statusOf(p *int) int {
	if p == nil {
		return bizmcp.StatusEnabled
	}
	return *p
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*bizmcp.Server, error) {
	return s.uc.Create(ctx, bizmcp.ServerInput{
		Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code),
		Transport: bizmcp.Transport(req.Transport), BaseURL: strings.TrimSpace(req.BaseURL),
		Headers: normalizeHeaders(req.Headers), Token: req.Token,
		Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) Update(ctx context.Context, id uint, req UpdateRequest) error {
	return s.uc.Update(ctx, id, bizmcp.ServerInput{
		Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code),
		Transport: bizmcp.Transport(req.Transport), BaseURL: strings.TrimSpace(req.BaseURL),
		Headers: normalizeHeaders(req.Headers), Token: req.Token,
		Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func (s *Service) Delete(ctx context.Context, id uint) error { return s.uc.Delete(ctx, id) }
func (s *Service) Get(ctx context.Context, id uint) (*bizmcp.Server, error) {
	return s.uc.Get(ctx, id)
}
func (s *Service) List(ctx context.Context, q bizmcp.Query, page, pageSize int) ([]*bizmcp.Server, interface{}, error) {
	return s.uc.List(ctx, q, page, pageSize)
}
func (s *Service) Test(ctx context.Context, id uint) (*bizmcp.TestResult, error) {
	return s.uc.Test(ctx, id)
}
