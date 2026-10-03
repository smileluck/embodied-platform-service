<!-- last-updated: 2026-10-03 -->
# 前端工具函数复用规则（frontend utils）

> 先查现有工具，禁止重复造轮子。清单来自 `web/src/utils/` 真实导出。

## 核心原则

1. 写新工具函数前，必须先查本文件清单与 `web/src/utils/`，确认无现成实现
2. 已有工具不满足需求时，优先扩展已有工具，而非新建平行实现
3. 新通用工具必须放入 `web/src/utils/<域>.ts`，并回本文件登记

## 关键工具

| 工具 | 位置 | 用途 |
|---|---|---|
| `formatDateTime` | `web/src/utils/datetime.ts` | 统一时间展示格式化 |
| `saveBlob` / `parseDispositionFilename` | `web/src/utils/download.ts` | 文件下载落盘与 Content-Disposition 文件名解析 |
| `renderMarkdown` | `web/src/utils/markdown.ts` | markdown 渲染（含复制按钮） |
| `isImageIcon` / `resolveIcon` / `renderMenuIcon` | `web/src/utils/menuIcon.ts` | 菜单图标解析与渲染 |
| `usePagination` | `web/src/utils/pagination.ts` | 列表页分页状态组合函数（配 `page_size` query） |
| `TABS_STORAGE_KEY` / 标签栏存取 | `web/src/utils/tabStorage.ts` | 多标签栏持久化 |
| `renderActions` / `TableAction` | `web/src/utils/tableActions.ts` | 表格操作列渲染 |
| `deepTrim` | `web/src/utils/trim.ts` | 对象深度去首尾空格（密码/文件字段除外）——请求层已全局应用，勿重复调用 |

另有分层位置约定：请求封装在 `web/src/api/request.ts`（axios）与 `api/platform.ts`（平台直调 fetch），不属于 utils 但同为强制复用入口。

## 强制使用场景清单

| 场景 | 必须使用的工具 |
|---|---|
| 发起本服务请求 | `web/src/api/request.ts` 实例（禁止裸 axios/fetch） |
| 调平台认证接口 | `web/src/api/platform.ts`（禁止走本服务实例） |
| 列表页分页 | `usePagination` + 后端 `{list, page}` 信封 |
| 时间展示 | `formatDateTime`（禁止各页面 toLocaleString 自造格式） |
| 文件下载 | `saveBlob`（配 `parseDispositionFilename`） |
| 表格操作列 | `renderActions` |
| 菜单图标 | `renderMenuIcon` / `resolveIcon` |
| 提交前去空格 | 已由 request 层 `deepTrim` 全局处理，页面不重复实现 |
