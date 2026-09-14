# 侧边栏图标改造：本地图标 + 网络图片，菜单管理可配置

## 现状
- 菜单即 `type='menu'` 的 Permission，`icon` 字段已端到端存在（DB → MenuNode → 前端 n-menu），但前端 `AdminLayout.vue:49-52` 只是 emoji 兜底渲染，无图标库。
- 菜单管理 `web/src/views/system/Menus.vue` 图标是普通文本输入框。
- 后端 `model.go:51` `Icon gorm:"size:64"`，URL 会超长；`usecase.go:39` icon 为空时跳过更新，导致无法清空图标。

## 方案（采用推荐项）

### 1. 前端：引入 @vicons/ionicons5
- `web/package.json` 安装 `@vicons/ionicons5`（Naive UI 官方推荐）。

### 2. 新建统一图标渲染工具 `web/src/utils/menuIcon.ts`
- 导出 `renderMenuIcon(icon?: string)`：
  - 以 `http://` / `https://` / `/` 开头（或含图片扩展名）→ 视为网络图片，`h('img', { src, style: 'width:18px;height:18px;...'})`，加载失败降级为 📄。
  - 否则按名称在 ionicons5 中查组件（`import * as icons from '@vicons/ionicons5'` 动态取 `icons[name]`），用 `h(NIcon, null, { default: () => h(Component) })` 渲染。
  - 未知名称 → 📄 兜底（保持现状体验）。

### 3. 改造侧边栏 `web/src/layout/AdminLayout.vue`
- 删除 `iconMap` emoji 映射，`renderIcon` 改为调用 `renderMenuIcon`。

### 4. 菜单管理 `web/src/views/system/Menus.vue`
- 图标字段改为：`n-input`（可手动填名称或 URL）+ 内嵌前缀/后缀实时预览（用 `renderMenuIcon` 渲染当前值）。
- 新增"选择图标"按钮，弹出 `n-modal` 内置图标选择器：可搜索过滤 ionicons5 常用图标网格，点击回填名称；同时提供"切换为 URL 模式"输入网络图片地址。
- 表格图标列由纯文本改为用 `renderMenuIcon` 渲染预览。

### 5. 后端调整
- `internal/data/model/model.go:51`：`size:64` → `size:512`。
- 迁移 SQL（`migrations/{mysql,postgres,sqlite}.sql` 中 permissions.icon 列，如定义了长度）同步改为 512。
- `internal/biz/permission/usecase.go` Update：icon 改为指针或显式区分"未传/传空"，允许清空图标（与现有其他可清空字段保持一致的做法）。
- `internal/service/permission/service.go` DTO 如需同步调整（Icon 加 omitempty 指针语义）。

### 6. 种子数据
- `internal/data/data.go` 种子菜单图标名核对为 ionicons5 中存在的名称（如 `HomeOutline`、`SettingsOutline`、`PersonOutline` 等），避免落地后全是 📄。

### 7. 验证
- 前端 `npm run build`（air :8080 托管 dist，rebuild 后强刷验证）。
- 手动验证：菜单管理配置本地图标名 / 配置 http 图片 URL，侧边栏分别渲染为矢量图标和图片；清空 icon 可保存。

## 不做
- 不引入 unplugin-icons 自动按需构建（运行时动态查表已够用、菜单名存 DB 需运行时解析）。
- 不做图标上传，网络图片直接以 URL 形式配置。