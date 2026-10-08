<!-- last-updated: 2026-10-08 -->
# 前端开发规范（frontend rules）

> 前端开发必须遵守的规范（`web/`，Vue 3 + TS + Vite + Naive UI + Pinia）。所有约定来自真实代码。

## 基础规则

- HTTP 请求：本服务 API 一律走 `web/src/api/request.ts` 的 axios 实例（`baseURL: '/api/v1'`，自动带 token/Accept-Language/deepTrim，401 单飞刷新重放）；**禁止**页面里裸用 axios/fetch 调本服务
- 登录/刷新/登出直调平台：`web/src/api/platform.ts`（token 双用，平台是唯一身份源）；**不得**用本服务 axios 实例调平台
- 状态管理：全局仅 `web/src/stores/{user,settings,theme}.ts`；页面局部状态留在组件内，禁止为单页面建 store
- 路由：登录后按菜单动态生成（`web/src/router/dynamic.ts`）；新页面必须登记 `viewModules` 的 `menu:<code>` 映射；路由守卫依赖 `useUserStore().routesLoaded`

## 命名规范

| 对象 | 约定 | 示例 |
|---|---|---|
| 页面文件名 | PascalCase.vue，按域分目录 | `views/system/Dicts.vue`、`views/device/Devices.vue` |
| 组件文件名 | PascalCase.vue（共享组件进 `components/`） | `components/AppPagination.vue` |
| 组合函数 | camelCase，use 前缀 | `composables/useTable.ts` |
| API 函数 | 动词开头 camelCase，与后端路由语义一致 | `listDictTypes`、`createRole` |
| 类型 | PascalCase，与后端 DTO 一一对应 | `type DictType = {...}`（`api/types.ts`） |

## 类型要求

- 服务端数据类型唯一声明在 `web/src/api/types.ts`，字段 snake_case 与后端 json tag 逐字一致；禁止页面内重复声明接口类型
- API 封装函数必须声明返回类型（泛型传给 request）；`npm run build`（vue-tsc -b）必须零错误

## 组件规范

- 公共组件位置：`web/src/components/`（跨页面复用才提升）；页面私有子组件就近放页面同目录
- 页面组件位置：`web/src/views/<域>/`
- Props 定义方式：`defineProps<{...}>()` 类型式 + `defineEmits`（`<script setup>` TS）
- 搜索区 `n-select` 绑定的 query 字段一律初始化 `null`（如 `status: null as number | null`），`null = 不筛`、清空归 null；禁止用 `''`——Naive UI 把空串当作已选值，placeholder 不显示（教训见 memory/lessons/2026-10-08-nselect-query-null-placeholder.md）

## 页面规范

新增页面必须完成：

1. `web/src/api/types.ts` 类型 + `api/index.ts` 函数
2. 管理端建菜单（`menu:<code>`）与按钮权限点（后端路由须在 protected 组）
3. `web/src/views/<域>/<Page>.vue` 页面（表格/表单模式参考 `views/system/Dicts.vue`）
4. `web/src/router/dynamic.ts:viewModules` 登记
5. `locales/zh-CN` 与 `en-US` 同步增文案 key

## 样式规范

- 优先级：Naive UI 组件与主题变量 > 全局样式类（`web/src/styles/`）> 页面 scoped 样式
- 主题改动三处同步：`stores/theme.ts`（Naive UI 主题）、`web/src/styles/`（暗壳亮芯 + 工业琥珀）、图表配色；签名元素复用 sx-led/sx-plate 既有类
- 恒定深/浅底的元素（侧栏深壳、品牌色块）必须用亮暗不变 token（`--sx-shell*`、`--sx-panel` 系）；禁止用随主题反转的 `--sx-ink`/`--sx-bg`/`--sx-surface` 作此类底色或其上文字色——暗色下会翻色导致不可读（见 notes/implemented/bug-fix/2026-10-08-dark-mode-sider-shell-tokens.md）
- 使用 `var(--sx-*)` 前确认变量已在 `web/src/styles/variables.css` 定义；未定义变量静默落入 CSS 兜底（inherit/initial），行为偶然不可靠
- 禁止内联硬编码颜色绕过主题变量

## 国际化规范

- 全部用户可见文案走 vue-i18n；key 按域分组（`user.*`、`role.*`、`common.*`…），zh-CN 与 en-US **同一次变更**内同步新增
- 语言由请求头同步到后端（`Accept-Language`），后端错误 msg/菜单名依赖它——勿在页面层另造语言状态

## 环境变量

- 前缀 `VITE_`；现役：`VITE_PLATFORM_API`（平台地址，见 `web/.env.development`、`web/.env.production`）
- 读取只在 `web/src/api/platform.ts` 等入口层，禁止散落组件；敏感值不入库

## 常用脚本命令

（web/ 目录内；根目录等价目标见 [../relations/development-workflow.md](../relations/development-workflow.md)）

| 操作 | 命令 |
|---|---|
| 开发（28170，/api 代理 28180） | `npm run dev` |
| 构建（vue-tsc -b && vite build） | `npm run build` |
| 静态检查 | `npm run lint` |
| 生成变更日志 | `npm run changelog` |

## 代码注释要求

- 默认不写解释"做了什么"的注释，代码与命名自解释
- 只为非显然的"为什么"写注释（范本：`web/src/api/request.ts` 的 401 刷新死锁注释）
