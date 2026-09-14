package merchant

import (
	"context"
	"errors"
	"time"
)

// ErrMerchantNotFound 商户不存在
var ErrMerchantNotFound = errors.New("商户不存在")

// ErrDuplicateCode 商户编码重复
var ErrDuplicateCode = errors.New("商户编码已存在，请更换")

// ErrMerchantDisabled 商户已被禁用（开放 API 验签时拒绝）
var ErrMerchantDisabled = errors.New("商户已被禁用")

// ErrInvalidSign 签名校验失败
var ErrInvalidSign = errors.New("签名校验失败")

// Query 商户列表查询条件（名称/编码/AppKey 按字段拆分，各自独立全模糊匹配，可叠加）
type Query struct {
	Name   string
	Code   string
	AppKey string
	Status *int
}

// APILogQuery API 调用日志查询条件
type APILogQuery struct {
	AppKey     string
	Path       string
	StatusCode *int
	Start      time.Time
	End        time.Time
}

// Repo 商户仓储接口（由 data 层实现，依赖倒置）。
// Update 仅更新基础资料与密钥哈希，code/app_key 创建后不可改。
type Repo interface {
	Create(ctx context.Context, m *Merchant) error
	Update(ctx context.Context, m *Merchant) error
	Delete(ctx context.Context, id uint) error
	Get(ctx context.Context, id uint) (*Merchant, error)
	FindByAppKey(ctx context.Context, appKey string) (*Merchant, error)
	List(ctx context.Context, q Query, page, pageSize int) ([]*Merchant, int64, error)
}

// APILogRepo API 调用日志仓储接口（记录异步、查询同步）
type APILogRepo interface {
	// Record 记录一次调用（异步落库，不阻塞请求）
	Record(l *APILog)
	List(ctx context.Context, q APILogQuery, page, pageSize int) ([]*APILog, int64, error)
}
