<!-- last-updated: 2026-10-03 -->
# AGENTS.md

## 目的

本文件是 embodied-platform-service 仓库内 AI 协作规则的**唯一真源**，只承载所有 AI 必须始终知道的最小高层规则与操作不变量。细节按任务路由到 `aiDoc/`，不在本文件展开。

## 加载模型

- **L0 自动加载**：根 `CLAUDE.md` 通过 `@AGENTS.md` 在每次会话自动 import 本文件。本文件不展开细节、不列完整文档清单。
- **L1 任务路由**：`aiDoc/README.md` 是文档索引 + 「任务→必读文档」路由表的唯一真源。接到任务先查路由表，决定本次读哪些子文档。
- **L2 按需深读**：路由表指向的 `aiDoc/` 具体子文档。
- **冲突优先级**（仓库内任务）：`AGENTS.md` > `aiDoc/README.md` > aiDoc 子文档 > 工具适配文件。

## 各工具加载方式

| 工具 | 加载方式 |
|---|---|
| Claude Code | 根 `CLAUDE.md @AGENTS.md` 原生 import；工具目录只放项目命令，不放规则副本 |
| Trae | 薄适配文件指向 `AGENTS.md`（只写入口指针） |
| Cursor / Copilot / 其他 | 若不支持 `@import`，参照薄适配模板新建适配文件，只写入口指针，不复制规则正文 |

任何工具的私有目录都禁止保存项目级规则副本。

## 仓库概览

Go 后端（Gin，业务管理端服务）与 Vue 3 前端（`web/`）同仓的 mixed 仓库；后端作为商户侧业务管理端接入 embodied-platform 平台开放面（项目集根 `../AGENTS.md` 有跨项目关系）。

| 目录/文件 | 职责 |
|---|---|
| `cmd/server/` | 服务入口 + wire 依赖注入装配（`main.go`、`wire.go`、`wire_gen.go`） |
| `api/` | proto 形式的对外 API 描述（`admin/v1/admin.proto`、`auth/v1/auth.proto`） |
| `internal/biz/` | 领域层：实体、哨兵错误、Repo 接口与用例编排（按限界上下文分目录） |
| `internal/data/` | 基础设施层：GORM 仓储实现、PO 模型（`model/`）、平台客户端（`platform/`） |
| `internal/service/` | 应用服务层：请求/响应 DTO、参数绑定与 biz 转换 |
| `internal/server/` | HTTP 层：路由注册（`router.go`）、各上下文 handler、`middleware/` 中间件 |
| `internal/conf/` | 配置加载（viper，`configs/config.yaml`） |
| `internal/platformsdk/` | 平台开放 API 客户端（HMAC 签名） |
| `pkg/` | 可独立复用的公共包（response、security、pagination、cache、i18n、logger 等） |
| `migrations/` | mysql/postgres/sqlite 三方言 DDL |
| `web/` | Vue 3 + TS + Vite + Naive UI 管理前端（`web/src/`） |
| `configs/` | 运行配置样例（`config.yaml`，环境变量可覆盖） |

## 工程规则

### 架构

<!-- TODO: 按实际范式填写，如分层服务（Endpoint -> Service -> Model）、包模块结构、插件体系；严格分层，禁止跨层调用。细节见 aiDoc/modules/architecture-rules.md -->

后端为 Kratos 风格四层（Router/Handler → Service → Biz/Usecase → Data/Repo），依赖注入用 wire，领域实体与 PO 分离；前端为 Vue 3 组件结构（views → stores → api）。严格单向依赖，禁止跨层与反向调用，细节路由到 `aiDoc/modules/architecture-rules.md`。

### 契约

<!-- TODO: 按项目范式（web-api / library / cli）与实际契约约定填写，细节路由到 aiDoc/contracts/boundary.md -->

统一信封 `{code, msg, data}`（`pkg/response/response.go:Body`），错误消息走 i18n；字段命名 snake_case；分页 `{list, page}`。前端 `/api/v1` 走本服务，登录/刷新/登出直调平台（token 双用）。细节路由到 `aiDoc/contracts/boundary.md`。

### 模块与目录

<!-- TODO: 按实际目录结构填写模块组织方式 -->

按限界上下文组织：同一上下文的四层文件分别落在 `internal/{biz,data,service}/<ctx>/` 与 `internal/server/handler_<ctx>.go`，仓储接口定义在 biz、实现在 data。新增上下文的完整步骤路由到 `aiDoc/modules/module-development.md`。

### 示例文档

`aiDoc/examples/` 是讲解型示例层，告诉 AI 每一层应该按什么标准组织和书写。示例不是逐字复制对象；新增某层文件前应先阅读对应示例。真实代码与示例不一致时，以真实代码为准并更新示例。

### 记忆规则

`aiDoc/memory/` 是 AI 记忆层：`long-term/` 存放跨任务稳定的用户偏好与协作方式；`business/` 存放每次用户提出的业务需求记录；`lessons/` 是踩坑与反复模式的经验 staging 区，达到晋升条件后写进约束正文。用户提出业务需求时，必须新增或更新一条 `business/` 记忆并更新索引。

### 文档维护

高层规则在 `AGENTS.md`，细节在 `aiDoc/`。本文件只保留「任务族→aiDoc 区域」的高层速查指针；「任务→必读文档」详细路由表唯一维护于 `aiDoc/README.md`，不在本文件罗列完整清单，避免双份维护导致口径漂移。新增/删除 aiDoc 子文档时，必须同步更新 `aiDoc/README.md` 的索引与路由表。

### 代码读取约束

禁止读取 `node_modules/`、`.venv/`、`__pycache__/`、`vendor/`、`dist/`、`build/` 等依赖与构建产物目录。禁止读取 `.env`、私钥等敏感文件。

## 操作不变量

所有 AI 在本仓库工作时必须始终遵守：

1. **一个事实一个维护家**：同一规则只在一个文件维护正文，他处只放相对路径链接。
2. **规则不进工具私有目录**：项目级规则正文只在 `AGENTS.md` 与 `aiDoc/`。
3. **变更要留痕**：改变行为、架构、共享契约、流程、测试策略或持久数据的变更，必须按 `aiDoc/notes/README.md` 写决策记录，涉及架构/契约/流程约束的在同一变更内回写约束正文（禁止只记决策不改规则）；跨组件、有风险或需交接的工作，先按 `aiDoc/plans/README.md` 建变更计划。
4. **证据与报告纪律**：证据按影响面选择——逻辑改动跑聚焦测试，接口改动跑 typecheck + 契约测试，用户可见输出走真实入口冒烟，文档改动做链接/格式检查；禁止反射式全量测试，禁止为变绿放宽过滤。结果按 passed / failed / skipped / unavailable / not-run 分开报告，禁止把中断的命令描述为已完成。
5. **交接要完整**：中断或移交工作时，按 `aiDoc/plans/handoff.TEMPLATE.md` 给出精确状态、已改文件、已跑命令与结果、剩余工作、阻塞、下一步安全动作。
6. **收尾双闸门**：任何改动收尾必须过漂移自检——装有 project-harness skill 时运行其 `check_sync.py`，无脚本时按 `aiDoc/README.md` 路由表核对，发现的漂移在同一变更内修复；踩坑与反复模式必须按 `aiDoc/memory/lessons/README.md` 入库（捕获与机械闸门规则的唯一正典在该文件）。二者是 Definition of Done 的一部分。

## Definition of Done

一项变更只有同时满足以下条件才算完成：

- [ ] 代码改动符合本文件与路由到的 aiDoc 子文档中的规则
- [ ] 按「证据与报告纪律」跑过对应验证命令，结果如实分类报告
- [ ] 涉及的共享契约两侧（生产方与消费方）都已追踪并同步
- [ ] 需要决策记录的变更已写入 `aiDoc/notes/`；需要计划的变更已更新 `aiDoc/plans/`
- [ ] 受影响的 aiDoc 文档已同步更新（事实就地更新，不追加 changelog）
- [ ] 文档中的路径、符号引用与真实代码一致
- [ ] 收尾双闸门已过：漂移自检（check_sync.py 或路由表核对）通过，漂移已在同一变更内处理；踩坑已按 lessons 纪律入库

## 文档索引与任务路由

### 高层速查（任务族 → aiDoc 区域）

| 任务族 | 优先查阅区域 |
|---|---|
| 模块/功能/命令开发 | `aiDoc/modules/`、`aiDoc/examples/` |
| 前端页面/功能开发 | `aiDoc/frontend/`、`aiDoc/examples/` |
| 契约/接口对接 | `aiDoc/contracts/boundary.md` |
| 仓库结构/技术栈/流程 | `aiDoc/relations/` |
| 业务需求记录 | `aiDoc/memory/business/` |
| 决策记录 | `aiDoc/notes/` |
| 变更计划与交接 | `aiDoc/plans/` |

只做高层指向；详细到具体文件的「任务→必读文档」路由表见 `aiDoc/README.md`。

### 详细路由

「任务→必读文档」详细路由表、常用入口字典统一维护在 `aiDoc/README.md`。本文件只放高层速查，避免双份维护导致口径漂移。冲突时以本文件为准。

## 何时重新生成 aiDoc

出现以下情况时，应通过 generate 工作流重新生成或增量更新文档体系：

- 架构较大变化：分层调整、新增/删除顶层目录、技术栈替换
- 新增/删除模块、功能或命令（含前端页面），导致 `aiDoc/modules/`、`aiDoc/examples/`、`aiDoc/relations/system-map.md` 与代码脱节
- `aiDoc/` 子文档新增/删除，导致 `aiDoc/README.md` 详细路由表与本文件高层速查失效
- 路由表或示例与真实代码明显漂移

小改动（单接口、单页面调整）无需整体重生成，用增量或按区域局部更新即可。重新生成后必须核验索引与路径一致性。
