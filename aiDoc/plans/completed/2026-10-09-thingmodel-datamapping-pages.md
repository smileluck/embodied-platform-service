<!-- last-updated: 2026-10-09 -->
# 物模型/数据映射页面 + 设备列表可见性 + 权限管理

## 目标

1. 设备列表页可见性收口（种子/自愈已具备，验证 + 排查结论）
2. 物模型只读浏览页（菜单 menu:thingModel + thingModel:* 权限码）
3. 数据映射完整管理：平台开放面新增 mapping 域（含写）→ 本服务全链路代理 + 管理页 + mapping:* 权限码

## 非目标

- 不改设备域既有路由/handler 链路
- 不动两仓库工作区 tenantuser 在途未提交改动
- 前端 mappings 编辑器不引入 Monaco 等重依赖（结构化表单）

## 假设

- 平台 biz `MappingUsecase` 状态机与商户一致性校验可直接复用，开放面只做收敛与透出
- 本服务 RBAC 同 path 多权限码任一匹配（已核实 `biz/auth/usecase.go:45`）

## 影响面

- 平台：开放面新域（契约变更）、scope 目录、SDK、契约测试、前端授权面板词条
- 本服务：platformsdk、biz/data/service/server devmodel 上下文扩展、权限种子、前端两页面 + locale
- 契约两侧：平台 boundary/integration-guide ↔ 本服务 boundary

## 验收标准

- [ ] 平台 `go build ./...` + openapi/devmodel/sdk 聚焦测试绿（含新增映射收敛单测）
- [ ] 本服务 `go build ./...` + 聚焦测试绿；`cd web && npm run build` 绿
- [ ] 菜单种子重启后自愈：设备中心下出现 设备管理/型号管理/物模型/数据映射；device:* 按钮 parent_id 归位
- [ ] 双仓库 check_sync.py 0 失败

## 执行步骤

1. 阶段 1 平台：scopes → biz 可见性扩展 → openapi service/handler/路由 → wire → SDK → 契约测试 → 平台侧测试与留痕
2. 阶段 2 本服务后端：platformsdk 镜像 → biz/data/service/server 映射链路 → 菜单/按钮种子 + i18n 词条
3. 阶段 3 本服务前端：api 类型/函数 → ThingModels.vue（只读）+ DataMappings.vue（完整管理）→ dynamic.ts/locale 注册
4. 阶段 4 设备列表可见性验证（重启自愈 + DB 核查）
5. 阶段 5 双仓库留痕收尾 + 漂移自检

## 交付摘要

- 阶段 1（平台 embodied-platform）：开放面数据映射域 15 端点（scope mapping:* 6 码）+ VO 商户收敛 + SDK + 契约测试 + 映射收敛单测，平台侧 build/test/check_sync 全绿；决策记录 2026-10-09-data-mapping-openapi.md（implemented）
- 阶段 2（本服务后端）：platformsdk 映射域镜像 + biz/data/service/server 全链路代理（/api/v1/data-mappings* 15 端点）+ 菜单 menu:thingModel/menu:dataMapping + 16 个按钮权限种子 + 菜单 i18n 词条；顺带修复 platformErr 的 SDK 错误归一（*platformsdk.Error 此前落 502）
- 阶段 3（前端）：ThingModels.vue（只读树浏览）+ DataMappings.vue（完整管理：def/版本/绑定）+ 15 个 API 函数 + 双语言 locale；npm run build（vue-tsc）通过
- 阶段 4（设备列表可见性）：根因=menu:device 软删不复活（种子缺软删恢复分支）；修复 ensureSystemMenus 补恢复分支，重启后菜单复活 + device:* 按钮 parent_id 自愈（本地库实测验证）；决策记录 notes/implemented/bug-fix/2026-10-09-menu-seed-softdelete-restore.md，lesson 已入库
- 验证：本服务 go build/vet/聚焦测试 passed；check_sync 7 通过 0 失败（2 提示均既存）；运行时端到端冒烟（页面真实数据渲染）依赖平台侧商户配置 mapping:* scopes，未在本地联通验证

### 阶段 2 已完成（2026-10-09，本服务后端）
- platformsdk 映射域镜像：`internal/platformsdk/types.go`（mapping scope 常量 + DataMappingDef/DataMappingVersion/Mapping/请求类型）、`internal/platformsdk/client.go`（15 方法，/data-mappings 前缀，effective 无版本 nil,nil）
- 代理链路：`internal/biz/devmodel/mapping.go`（镜像类型 + MappingGateway + MappingUsecase 纯透传）→ `internal/data/devmodel/mapping.go`（MappingGatewayAdapter）→ `internal/service/devmodel/mapping.go`（MappingService，DTO type alias）→ `internal/server/devmodel.go`（15 handler，platformErr 透传）→ `internal/server/router.go`（/data-mappings protected 组，静态段 /effective、/versions/:vid 先于参数段）
- `platformErr` 内聚 SDK 错误归一（normalizeSDKError 上提），修复 devmodel 链路 4xx 落 502 隐患
- 种子：`menu:thingModel`（Sort 3）/ `menu:dataMapping`（Sort 4）挂 menu:deviceCenter；thingModel:* 2 码 + mapping:* 14 码（全端点覆盖）；zh/en 菜单词条
- 决策记录：`aiDoc/notes/implemented/architecture/2026-10-09-datamapping-proxy-chain.md`
- 验证：`go build ./...` ✅；聚焦测试 platformsdk / biz.devmodel / data.devmodel / service.devmodel 全绿（含 mapping 适配器 3 个新测）；同 gin 版本路由形态冒烟无 panic

### 阶段 3 已完成（2026-10-09，本服务前端）
- 类型/接口：`web/src/api/types.ts` 新增 Mapping/DataMappingDef/DataMappingVersion/DataMappingCreateResult 及 3 个请求类型（镜像 biz/devmodel/mapping.go）；`web/src/api/index.ts` 新增 15 个 data-mappings API 函数（命名 createDataMapping*/rollbackDataMapping 等）
- 物模型只读浏览页 `web/src/views/device/ThingModels.vue`：全量拉取按 parent_id 组树（kw/layer 客户端过滤，命中节点保留祖先路径），右侧详情 + 已发布版本（thingModel:version 控制入口）；全页无写操作
- 数据映射管理页 `web/src/views/device/DataMappings.vue`：def 列表（通用行不渲染写操作）+ 版本管理抽屉（单草稿制：已有草稿禁用新建并提示；编辑/发布仅 draft、回退仅 published）+ 三态共用草稿编辑抽屉（结构化 mappings 子表单，无 Monaco）+ 版本明细只读抽屉 + 追加式回退弹窗（draft/publish 二选一）+ 型号绑定抽屉（listDeviceModels page_size=0 候选）
- 注册：`web/src/router/dynamic.ts` viewModules 加 menu:thingModel/menu:dataMapping；locales 新建 zh-CN/en-US 的 thingModel.ts、dataMapping.ts 并注册 index（菜单名走后端 i18n，前端无 menu 词条）
- 验证：`npm run build`（vue-tsc -b && vite build）✅；eslint 仅存量风格 any 警告（0 error）
- 教训候选：locale 文案含 `{sn}` 等字面占位符会被 vue-i18n 当作插值变量，调用处需传同名字面参数（DataMappings.vue tmPlaceholders）
