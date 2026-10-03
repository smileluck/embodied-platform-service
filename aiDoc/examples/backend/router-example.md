<!-- last-updated: 2026-10-03 -->
# 示例：路由注册与依赖注入（router + wire）

> 目的：展示新上下文接入 HTTP 与 DI 的两处注册标准——`router.go` 分组挂载与 `wire.go` provider set。

## 核心原则

1. 路由只改 `internal/server/router.go:registerRoutes`：按敏感度选组——protected（默认，RBAC）/ basic（本人数据）/ 公开
2. wire 只改 `cmd/server/wire.go` 三个 set（`bizSet`/`dataRepoSet`/`serviceSet`）+ `wire.Bind` 接口绑定，然后 `make wire`
3. `wire_gen.go` 是生成物，禁止手改；跨上下文最小依赖接口与 provider 必须同 set

## 示例一：路由注册（提取自 internal/server/router.go，节选）

```go
func (s *HTTPServer) registerRoutes() {
	// 持久化 IP 黑名单在认证之前拦截全部 /api/ 请求
	v1 := s.engine.Group("/api/v1", middleware.IPBlacklist(s.blacklist.Checker()))

	// ---- 自身数据接口：仅平台认证 + 操作日志，不做 RBAC ----
	basic := v1.Group("", middleware.PlatformAuth(s.auth, s.admissionUC, s.identityCache), middleware.OpLog(s.log))
	{
		basic.GET("/dicts/:code/items", s.listDictItemsByCode) // 消费入口：登录即可
	}

	// ---- 受保护接口：平台认证 -> 操作日志（RBAC 拒绝的尝试也记录）-> RBAC ----
	authmw := middleware.PlatformAuth(s.auth, s.admissionUC, s.identityCache)
	protected := v1.Group("", authmw, middleware.OpLog(s.log), middleware.RBAC(s.auth, s.rbacCache))

	dictTypes := protected.Group("/dict-types")
	{
		dictTypes.GET("", s.listDictTypes)
		dictTypes.POST("", s.createDictType)
		dictTypes.GET("/:id", s.getDictType)
		dictTypes.PUT("/:id", s.updateDictType)
		dictTypes.DELETE("/:id", s.deleteDictType)
	}
}
```

## 示例二：wire 装配（提取自 cmd/server/wire.go，节选）

```go
//go:build wireinject

var bizSet = wire.NewSet(
	bizdict.NewUsecase,
	// ...其余上下文
	// 跨上下文最小依赖接口绑定（provider 与 bind 需同 set）
	wire.Bind(new(bizdevice.TenantRelinker), new(*biztenant.Usecase)),
)

var dataRepoSet = wire.NewSet(
	data.NewData, datadict.NewRepo,
	// wire.Bind(new(bizdict.Repo), new(*datadict.repo)) 形态见实际文件
)

var serviceSet = wire.NewSet(
	dictsvc.NewService,
)

var providerSet = wire.NewSet(bizSet, dataRepoSet, serviceSet, ProvideConfig, server.NewHTTPServer)

func wireApp() (*server.HTTPServer, func(), error) {
	wire.Build(providerSet)
	return nil, nil, nil
}
```

## 要点

- 新 service 注入 `HTTPServer`：`HTTPServer` 结构体加字段 + `NewHTTPServer` 加参数（wire 自动装配）
- 新路由进 protected 组即自动获得权限点控制（`UserID|Method|Path`，见 `middleware.RBAC`）
- 全局中间件链（I18n/SecurityHeaders/CORS/XSSFilter/SQLInjectionGuard）在 `NewHTTPServer` 统一挂载，新上下文不单独重复挂

## 真实参考文件

- `internal/server/router.go`
- `cmd/server/wire.go` / `cmd/server/wire_gen.go`（生成物，只读对照）
- `Makefile:wire`
