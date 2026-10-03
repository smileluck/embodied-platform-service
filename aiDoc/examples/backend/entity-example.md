<!-- last-updated: 2026-10-03 -->
# 示例：领域层 entity（实体 + 哨兵错误 + Repo 接口）

> 目的：展示 `internal/biz/<ctx>/entity.go` 的组织标准——领域实体、哨兵错误、仓储接口三者同文件，是限界上下文的契约核心。

## 核心原则

1. 实体只有 json tag（snake_case），不带 gorm tag——持久化细节属于 PO（data 层）
2. 可预期业务失败一律定义哨兵 `ErrXxx`（中文文案），data 层负责把库错误映射进来
3. Repo 接口在这里定义、在 data 实现（依赖倒置）；方法第一个参数必为 `ctx`

## 示例（提取自 internal/biz/dict/entity.go，节选）

```go
// Package dict 数据字典限界上下文 —— 领域层。
package dict

// 哨兵错误
var (
	ErrTypeNotFound   = errors.New("字典类型不存在")
	ErrTypeCodeExists = errors.New("字典类型编码已存在，请更换")
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
	FindTypeByID(ctx context.Context, id uint) (*DictType, error)
	FindTypeByCode(ctx context.Context, code string) (*DictType, error)
	ListTypes(ctx context.Context, q Query, page, pageSize int) ([]*DictType, int64, error)
	CountItemsByType(ctx context.Context, typeID uint) (int64, error)
	// ...
}
```

## 要点

- 包注释写清上下文边界与两级行政结构（类型/项），业务口径写在字段旁（长度、唯一性）
- 哨兵错误文案即用户可读文案；需要 i18n 时在 server 层注册 key（见 `internal/server/i18n_errors.go:errKeys`）
- 查询条件独立 `Query` 结构，不与实体混用

## 真实参考文件

- `internal/biz/dict/entity.go`（最小完整范本）
- `internal/biz/tenant/entity.go`（含平台同步状态的上下文）
