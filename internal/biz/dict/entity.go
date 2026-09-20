// Package dict 数据字典限界上下文 —— 领域层。
// 类型（dict_types）+ 字典项（dict_items）两级：类型按 code 稳定引用，
// 项按 value 消费；供表单下拉/枚举展示等场景统一维护。
package dict

import (
	"context"
	"errors"
	"time"
)

// 哨兵错误
var (
	ErrTypeNotFound   = errors.New("字典类型不存在")
	ErrTypeCodeExists = errors.New("字典类型编码已存在，请更换")
	ErrTypeHasItems   = errors.New("该类型下存在字典项，请先删除")
	ErrItemNotFound   = errors.New("字典项不存在")
	ErrItemExists     = errors.New("该类型下已存在相同标签或取值，请更换")
)

// Status 通用状态（1 启用 0 禁用；禁用的项不对外输出）
type Status int

const (
	StatusDisabled Status = 0
	StatusEnabled  Status = 1
)

// DictType 字典类型
type DictType struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"` // ≤20
	Code      string    `json:"code"` // ≤64，唯一，业务按此引用
	Remark    string    `json:"remark"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DictItem 字典项（同一类型下 label/value 均唯一）
type DictItem struct {
	ID        uint      `json:"id"`
	TypeID    uint      `json:"type_id"`
	Label     string    `json:"label"` // 展示文本 ≤20
	Value     string    `json:"value"` // 存储取值 ≤64
	Sort      int       `json:"sort"`  // 升序展示
	Remark    string    `json:"remark"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Query 类型列表查询条件
type Query struct {
	Name string
	Code string
}

// Repo 仓储接口
type Repo interface {
	CreateType(ctx context.Context, t *DictType) error
	UpdateType(ctx context.Context, t *DictType) error
	DeleteType(ctx context.Context, id uint) error
	FindTypeByID(ctx context.Context, id uint) (*DictType, error)
	FindTypeByCode(ctx context.Context, code string) (*DictType, error)
	ListTypes(ctx context.Context, q Query, page, pageSize int) ([]*DictType, int64, error)
	CountItemsByType(ctx context.Context, typeID uint) (int64, error)

	CreateItem(ctx context.Context, i *DictItem) error
	UpdateItem(ctx context.Context, i *DictItem) error
	DeleteItem(ctx context.Context, id uint) error
	FindItemByID(ctx context.Context, id uint) (*DictItem, error)
	// FindItemByLabelOrValue 同类型下按 label 或 value 查重
	FindItemByLabelOrValue(ctx context.Context, typeID uint, label, value string, excludeID uint) (*DictItem, error)
	ListItems(ctx context.Context, typeID uint, page, pageSize int) ([]*DictItem, int64, error)
	// ListEnabledItemsByCode 消费接口：按类型编码取启用项（sort 升序）
	ListEnabledItemsByCode(ctx context.Context, code string) ([]*DictItem, error)
}
