<!-- last-updated: 2026-10-03 -->
# 示例层

`aiDoc/examples/` 是讲解型示例层，告诉 AI 每一层应该按什么标准组织和书写。

## 用途

- 示例不是要求逐字复制，而是展示项目标准的代码组织方式
- 当 AI 需要新增某一层文件时，应先阅读对应示例
- 示例代码必须从项目真实代码中提取，禁止凭空编写

## 后端示例阅读顺序（分层服务，与开发顺序一致）

1. [backend/entity-example.md](backend/entity-example.md) — 领域实体 + 哨兵错误 + Repo 接口（`internal/biz/<ctx>/entity.go`）
2. [backend/usecase-example.md](backend/usecase-example.md) — 业务用例编排（`internal/biz/<ctx>/usecase.go`）
3. [backend/repo-example.md](backend/repo-example.md) — GORM 仓储实现（`internal/data/<ctx>/repo.go`）
4. [backend/service-example.md](backend/service-example.md) — DTO 与参数绑定（`internal/service/<ctx>/service.go`）
5. [backend/handler-example.md](backend/handler-example.md) — HTTP handler（`internal/server/handler_<ctx>.go`）
6. [backend/router-example.md](backend/router-example.md) — 路由注册 + wire 依赖注入（`internal/server/router.go`、`cmd/server/wire.go`）

## 前端示例阅读顺序

1. [frontend/api-example.md](frontend/api-example.md) — 类型声明 + API 封装（`web/src/api/`）
2. [frontend/view-example.md](frontend/view-example.md) — 列表页组件（`web/src/views/`）
3. [frontend/utils-usage-example.md](frontend/utils-usage-example.md) — 工具函数复用（`web/src/utils/`）

## 原则

- 仓库真实代码与示例不一致时，以真实代码为准，并更新示例
- 每个示例文件必须包含「真实参考文件」一节，指向示例代码的出处
