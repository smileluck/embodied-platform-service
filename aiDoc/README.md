<!-- last-updated: 2026-10-03 -->
# aiDoc

`aiDoc/` 是本仓库的结构化 AI 文档层（L1 路由层），用于把长期有效的项目上下文从工具目录中抽离出来，并按主题拆分成可维护的约束文档。

## 使用方式

1. `AGENTS.md` 已随根 `CLAUDE.md @AGENTS.md` 自动加载（L0），始终生效
2. 接到任务先查本文件「任务→必读文档」路由表，确定必读文档（L1）
3. 不清楚某个文档讲什么时，再翻「常用入口」字典（L1）
4. 按路由表打开 `aiDoc/` 下具体子文档深读（L2）

不再把项目级规则复制到工具私有目录；支持 `@import` 的工具走 `@AGENTS.md`，其余工具走薄适配指针（见 `AGENTS.md` 的「各工具加载方式」）。

## 信息归属表

一个事实只有一个维护家；他处只放链接。

| 信息类型 | 唯一维护位置 |
|---|---|
| 所有任务都需要的高层规则与不变量 | `/AGENTS.md` |
| 文档索引与「任务→必读文档」路由表 | `aiDoc/README.md`（本文件） |
| 项目定位、技术栈、目录职责 | `relations/repo-profile.md`、`relations/system-map.md` |
| 组件清单、语言构成、模块清单、入口点、命令索引 | `relations/code-index.md`（机器生成，不手改） |
| 开发流程、分支与提交规范、环境命令 | `relations/development-workflow.md` |
| 架构与模块组织约束 | `modules/architecture-rules.md` |
| 新建模块/功能/命令的步骤 | `modules/module-development.md` |
| 共享契约、字段命名、类型桥接 | `contracts/boundary.md` |
| 前端开发规范 | `frontend/frontend-rules.md` |
| 前端工具函数复用 | `frontend/frontend-utils.md` |
| 各层代码组织标准（讲解型） | `examples/` |
| 长期稳定的协作偏好 | `memory/long-term/` |
| 每次业务需求记录 | `memory/business/` |
| 踩坑与反复模式的经验（待晋升） | `memory/lessons/` |
| 为什么做某个非平凡决策 | `notes/<proposed\|implemented\|rejected>/<class>/` |
| 当前进行中的变更执行清单 | `plans/active/` |
| 可复用的情境化工作流 | `.agents/skills/<workflow>/SKILL.md` |
| 工作移交与中断续作 | `plans/` 下的 handoff 文档 |

## 常用入口

| 文档 | 用途 |
|---|---|
| `relations/repo-profile.md` | 项目定位与技术栈速查 |
| `relations/development-workflow.md` | 开发流程、提交规范、环境命令 |
| `relations/system-map.md` | 系统架构与组件关系 |
| `relations/code-index.md` | 组件清单、语言构成、入口点、命令索引（机器生成，不手改） |
| `modules/architecture-rules.md` | 架构各层必须遵守的约束 |
| `modules/module-development.md` | 新建模块/功能/命令的完整步骤 |
| `contracts/boundary.md` | 生产者与消费者之间的共享契约 |
| `frontend/frontend-rules.md` | 前端开发规范 |
| `frontend/frontend-utils.md` | 前端工具函数复用清单 |
| `examples/README.md` | 讲解型示例的阅读入口 |
| `memory/README.md` | 记忆层总览与使用规则 |
| `memory/long-term/README.md` | 长期稳定偏好的沉淀规则 |
| `memory/business/README.md` | 业务需求记录规则（索引唯一维护于 project-memory.md） |
| `memory/project-memory.md` | 记忆层索引 |
| `memory/lessons/README.md` | 经验记录的捕获与晋升纪律 |
| `notes/README.md` | 决策记录的分类与维护纪律 |
| `plans/README.md` | 变更计划的生命周期与何时需要计划 |

## 任务→必读文档 路由表

<!-- 本表已由 generate 工作流按实际存在的 aiDoc 子文档校准（2026-10-03） -->

| 任务类型 | 必读文档 |
|---|---|
| 新建后端模块（限界上下文） | `modules/module-development.md`、`modules/architecture-rules.md`、`examples/README.md` |
| 修改既有逻辑 | `modules/architecture-rules.md`、`relations/system-map.md` |
| 新建前端页面 / 功能 | `frontend/frontend-rules.md`、`frontend/frontend-utils.md`、`examples/README.md` |
| 契约 / 接口对接（前端↔本服务、本服务↔平台开放面） | `contracts/boundary.md` |
| 平台开放面对接 / HMAC 签名问题 | `contracts/boundary.md`、`relations/system-map.md` |
| 新增依赖注入 Provider / wire 报错 | `modules/module-development.md`（wire 节） |
| 了解仓库结构 / 技术栈 / 流程 | `relations/repo-profile.md`、`relations/system-map.md`、`relations/development-workflow.md` |
| 查组件清单 / 入口点 / 命令索引 | `relations/code-index.md`（机器生成，不手改） |
| 用户提出新业务需求 | `memory/business/TEMPLATE.md`、`memory/project-memory.md`（必更新索引） |
| 踩坑 / 反复模式记录 | `memory/lessons/README.md`、`memory/lessons/TEMPLATE.md`（必更新索引） |
| 非平凡决策（行为/架构/契约/流程变更） | `notes/README.md`、`notes/TEMPLATE.md` |
| 多步骤 / 跨组件 / 有风险的变更 | `plans/README.md`、`plans/change-plan.TEMPLATE.md` |
| 工作中断或移交他人 | `plans/handoff.TEMPLATE.md` |
| 代码审查 | `../.agents/skills/project-code-review/SKILL.md` |
| 推送前检查 | `../.agents/skills/project-pre-push-checks/SKILL.md` |

## 维护原则

- 稳定规则放这里，不放到工具私有目录里
- 临时会话草稿不要入库
- 项目级规则先写进 `/AGENTS.md`，细节拆到 `aiDoc/`
- 新增/删除 `aiDoc/` 子文档时，必须同步更新本文件「常用入口」与任务路由表，否则等于未入库
- 事实变化时就地更新对应文档正文，禁止在文档尾部追加 changelog
- 冲突优先级：`/AGENTS.md` > 本文件 > aiDoc 子文档 > 工具适配文件
