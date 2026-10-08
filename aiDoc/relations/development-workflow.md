<!-- last-updated: 2026-10-03 -->
# 开发流程（development workflow）

> 开发顺序、协作方式、分支与提交规范、环境命令。命令必须来自真实配置文件，禁止编造。

## 推荐开发顺序

后端新增限界上下文（完整步骤见 [../modules/module-development.md](../modules/module-development.md)）：

1. 领域层：`internal/biz/<ctx>/entity.go`（实体 + 哨兵错误 + Repo 接口）→ `usecase.go`
2. 数据层：`internal/data/model/model.go` 增 PO 与转换 → `internal/data/<ctx>/repo.go` 实现 Repo
3. 应用层：`internal/service/<ctx>/service.go`（DTO + binding）
4. HTTP 层：`internal/server/handler_<ctx>.go` → `router.go` 注册分组 → 需要时 `i18n_errors.go` 注册错误 key
5. 注入：`cmd/server/wire.go` 对应 set 增 provider → `make wire` 重新生成 `wire_gen.go`
6. 迁移：PO 有表结构变化时同步 `migrations/{mysql,postgres,sqlite}.sql`
7. 前端：`web/src/api/types.ts` 类型 → `api/index.ts` 函数 → 菜单/权限点 → `views/` 页面 → `dynamic.ts` 登记

## 契约两侧协作

本仓库有三条契约边界（详见 [../contracts/boundary.md](../contracts/boundary.md)）：

- **前端 ↔ 本服务**：接口字段先定契约（snake_case、统一信封），前端 `api/types.ts` 与后端 DTO 同步改
- **本服务 ↔ 平台开放面**（embodied-platform）：接口/签名/信封以平台为准，改动两侧仓库分别留痕（跨项目变更见项目集根 `../../AGENTS.md`）
- **本服务 → 平台认证公开 API**（管理端登录/刷新/登出/验证码，后端代理转发）：平台是唯一身份源，token 双用；平台接口变化需同步 `internal/data/platform/identity.go` 与 `web/src/api/index.ts` 的 auth 函数（租户端 app-auth 仍浏览器直调平台，见 `web/src/api/platform.ts`）

联调验证：`make dev` 同起后端(28180)与前端(28170)；平台相关功能需平台侧服务可用（默认 `http://localhost:27080`）。

## 分支策略

| 分支 | 用途 |
|---|---|
| `main` | 唯一长期分支，直接提交（当前无 feature 分支流程） |

## 提交规范

- 格式：`type(scope): 中文描述`（conventional commits）
- 常见 type：`feat` / `fix` / `refactor` / `chore` / `build` / `docs`
- 常见 scope：`web`（前端）、`sdk`、`ci`、模块名；无明确归属可省略 scope
- 依据：`git log`（如 `feat: 新增 MCP 服务管理…`、`fix(web): 标签栏滚轮横向滑动兼容…`）

## 环境与依赖

| 操作 | 命令 |
|---|---|
| 安装前端依赖 | `make web-install`（web/ 下 `npm install`） |
| 一键开发（后端热载 28180 + 前端 28170） | `make dev`（需 `go install github.com/air-verse/air@latest`） |
| 仅前端开发 | `make web-dev` |
| 运行后端（不经 air） | `make run` |
| 构建 | `make build`（后端二进制）；`make web-build`（前端产物 web/dist） |
| 测试 | `make test`（`go test ./...`） |
| 静态检查 | `make lint`（go vet；装有 golangci-lint 时追加） |
| 重新生成 DI | `make wire`（需 `go install github.com/google/wire/cmd/wire@latest`） |
| 依赖整理 | `make tidy` |
| Docker 部署（app+MySQL+Redis） | `make docker-up` / `docker-down` / `docker-logs` / `docker-rebuild` |
| 清理产物 | `make clean` |

前端目录内直用 npm：`npm run dev` / `npm run build`（vue-tsc -b && vite build）/ `npm run lint`；`predev`/`prebuild` 会先跑 `web/scripts/changelog.mjs` 生成 `web/src/generated/changelog.json`。

## API 文档

无独立 Swagger 入口；对外接口以 `api/admin/v1/admin.proto`、`api/auth/v1/auth.proto` 为描述基准，实际路由以 `internal/server/router.go` 为准。
