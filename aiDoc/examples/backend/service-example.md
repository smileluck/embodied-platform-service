<!-- last-updated: 2026-10-03 -->
# 示例：应用服务 service（DTO + 参数绑定）

> 目的：展示 `internal/service/<ctx>/service.go` 的组织标准——请求 DTO、binding 校验、到 biz Input 的转换；不含业务判断。

## 核心原则

1. 每上下文一个 `Service`，构造注入 `*biz<ctx>.Usecase`；方法与 handler 一一对应
2. 校验全部走 gin binding tag（`required/max/omitempty/gte/lte`）；可缺省字段用指针类型
3. 只做「DTO → Input」的搬运与默认值，业务规则一律留给 usecase

## 示例（提取自 internal/service/dict/service.go，节选）

```go
// Package dict 数据字典应用服务
package dict

type Service struct {
	uc *bizdict.Usecase
}

func NewService(uc *bizdict.Usecase) *Service { return &Service{uc: uc} }

type TypeCreateRequest struct {
	Name   string `json:"name" binding:"required,max=20"`
	Code   string `json:"code" binding:"required,max=64"`
	Remark string `json:"remark" binding:"max=200"`
	Status *int   `json:"status" binding:"omitempty,gte=0,lte=1"` // 指针：区分「未传」与 0
}

func (s *Service) CreateType(ctx context.Context, req TypeCreateRequest) (*bizdict.DictType, error) {
	return s.uc.CreateType(ctx, bizdict.TypeInput{
		Name: req.Name, Code: req.Code, Remark: req.Remark, Status: statusOf(req.Status),
	})
}

func statusOf(s *int) bizdict.Status {
	if s != nil && *s == 0 {
		return bizdict.StatusDisabled
	}
	return bizdict.StatusEnabled
}
```

## 要点

- json tag 与前端 `web/src/api/types.ts` 字段逐字一致（snake_case）
- binding 失败由 handler 统一转 `common.invalid_params`，service 不处理 validator 错误文本
- 列表方法返回 `(..., interface{}, error)` 透传 `pagination.Page`，由 handler 包 `listResult`

## 真实参考文件

- `internal/service/dict/service.go`
- `internal/service/tenant/service.go`（含平台同步参数的上下文）
