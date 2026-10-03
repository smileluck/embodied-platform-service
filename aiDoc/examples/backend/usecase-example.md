<!-- last-updated: 2026-10-03 -->
# 示例：业务用例 usecase

> 目的：展示 `internal/biz/<ctx>/usecase.go` 的组织标准——业务校验与编排只依赖 Repo 接口，不碰 GORM。

## 核心原则

1. 构造函数注入 Repo：`NewUsecase(repo Repo) *Usecase`
2. 业务规则（查重、引用存在性、状态流转）在这里完成，经 repo 查询而非直连 DB
3. 分页列表统一返回 `(list, pagination.Page, error)`；写入参数用 `XxxInput` 结构

## 示例（提取自 internal/biz/dict/usecase.go，节选）

```go
// Usecase 数据字典领域用例
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase { return &Usecase{repo: repo} }

// TypeInput 类型写入参数（更新时空字符串=保持原值）
type TypeInput struct {
	Name   string
	Code   string
	Remark string
	Status Status
}

func (uc *Usecase) CreateType(ctx context.Context, in TypeInput) (*DictType, error) {
	in.Code = strings.TrimSpace(in.Code)
	if _, err := uc.repo.FindTypeByCode(ctx, in.Code); err == nil {
		return nil, ErrTypeCodeExists
	} else if !errors.Is(err, ErrTypeNotFound) {
		return nil, err
	}
	t := &DictType{Name: in.Name, Code: in.Code, Remark: in.Remark, Status: in.Status}
	if err := uc.repo.CreateType(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (uc *Usecase) ListTypes(ctx context.Context, q Query, page, pageSize int) ([]*DictType, pagination.Page, error) {
	list, total, err := uc.repo.ListTypes(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}
```

## 要点

- 查重模式：先查存在性——`err == nil` 视为冲突返回哨兵错误；`!errors.Is(err, ErrXxxNotFound)` 才是意外错误上抛
- 更新语义「空值=保持原值」在 usecase 内解释（读旧值回填），service 层不做判断
- 删除有引用检查的（如 `DeleteType` 先 `CountItemsByType`），跨表约束也在此层把关

## 真实参考文件

- `internal/biz/dict/usecase.go`
- `internal/biz/tenant/usecase.go`（含平台开放面同步编排的复杂用例）
