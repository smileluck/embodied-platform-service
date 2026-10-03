<!-- last-updated: 2026-10-03 -->
# 示例：HTTP handler

> 目的：展示 `internal/server/handler_<ctx>.go` 的组织标准——参数提取、调 service、统一响应；无业务逻辑。

## 核心原则

1. 方法挂在 `HTTPServer` 上、小写不导出；一个上下文一个 handler 文件
2. 响应只经 `pkg/response`：成功 `response.OK`；业务错误 `response.FailI18n`（哨兵错误已注册 i18n key）；绑定失败固定 `common.invalid_params`
3. 路径参数用 `idParam(c)`，分页用 `pageParams(c)`，列表包 `listResult{List, Page}`

## 示例（提取自 internal/server/handler_dict.go，节选）

```go
// 数据字典 handler：类型 CRUD + 项 CRUD；按编码取项挂在 basic 组（登录即可消费）。
package server

// dictErr 字典错误映射（未注册 i18n 的走原错误文本）
func (s *HTTPServer) dictErr(c *gin.Context, err error) {
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

func (s *HTTPServer) listDictTypes(c *gin.Context) {
	page, size := s.pageParams(c)
	q := bizdict.Query{Name: c.Query("name"), Code: c.Query("code")}
	list, pg, err := s.dict.ListTypes(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createDictType(c *gin.Context) {
	var req dictsvc.TypeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	t, err := s.dict.CreateType(c.Request.Context(), req)
	if err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, t)
}
```

## 要点

- 每个上下文可定义 `xxxErr` 辅助函数收敛错误 HTTP 状态码选择（业务失败多为 400，系统错 500）
- 子资源归属以路径为准（如 `req.TypeID = typeID` 覆盖 body，防越权改归属）
- 新增哨兵错误 → 在 `internal/server/i18n_errors.go:errKeys` 注册 key，否则降级输出中文原文

## 真实参考文件

- `internal/server/handler_dict.go`
- `internal/server/handler_tenant.go`（含平台错误转换 `s.platformErr`）
