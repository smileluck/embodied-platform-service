<!-- last-updated: 2026-10-09 -->
# 物模型草稿的前端会话跟踪（开放面无草稿枚举端点）

## 问题

物模型页升级为商户可管理后需要草稿编辑流程，但平台开放面物模型域**没有草稿列表/详情端点**：`GET /thing-models/:id/versions` 仅返回 published 选择器视图（不含 schema），草稿 vid 与 schema 只在创建（`POST /:id/versions`）与回滚（`POST /versions/:vid/rollback`）响应中返回。刷新页面后草稿无法重新枚举，但平台侧单草稿制仍占用（再建 409）。

## 提案 / 决策

前端以 `localStorage` 键 `tm_draft:<nodeId>` 跟踪本会话创建的草稿（`{vid, version, schema}`）：

- 创建/回滚（draft 模式）成功时写入；发布/删除成功时清除；编辑保存时同步 schema
- 版本页签打开时：本地草稿 vid 若已出现在 published 列表（他端已发布）则惰性清理；草稿操作遇 404（他端已删）同样清理并刷新
- 平台侧有草稿而本地无记录时，用 `inheritance-status` 的 `baseline==='draft'` 佐证，页面提示「存在非本会话草稿，本端无法定位」（孤儿草稿不可编辑，只能在平台管理端处理）
- 「新建草稿」在确认时才调 `POST /:id/versions`（避免用户取消留下空草稿）；编辑器非空则随后 `PUT /versions/:vid` 保存内容
- 新建草稿可选「预填最新发布版本」：`resolve(node)` 基线合并结果 − `resolve(parent)` 合并结果（同名且定义一致的要素视为继承剔除，稳定序列化比较），得到本层自有要素作初始编辑内容

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 只存内存（刷新即丢） | 刷新后草稿 409 死锁（建不了新草稿也找不到旧草稿），比 localStorage 方案更差 |
| 预填直接用 resolve 合并结果（不做减法） | 合并结果含父层继承要素，存进本层 schema 会冻结继承（父层后续更新不再传播到同名要素），语义错误 |
| 预填 pin 到最新 published 版本（`resolve?version_id=`） | 无草稿时基线即最新 published，等价；省一次请求用基线即可 |
| 等开放面补草稿枚举端点再做草稿编辑 | 阻塞页面交付；本地跟踪方案在端点补齐后可平滑替换 |

## 验收标准

- [x] `cd web && npm run build`（vue-tsc -b && vite build）绿
- [x] eslint 涉及文件 0 error
- [ ] 端到端（创建草稿→刷新页面→继续编辑→发布）依赖平台开放面写路由落地后验证（见 thingmodel-write-proxy 记录的已知缺口）

## 风险与后果

- 草稿与浏览器 localStorage 绑定：换浏览器/清缓存后草稿变孤儿（页面有明确提示文案），平台补齐枚举端点后应移除本地跟踪
- 预填减法是启发式（同名同义判继承）：父层在两次 resolve 间发新版会产生轻微误差，仅影响初始编辑内容，用户保存前可审阅
- 回滚（publish=false）响应含完整 schema，是除创建外唯一能获得草稿内容的途径，已进入本地跟踪链路

## 交叉链接

- 计划：`aiDoc/plans/completed/2026-10-09-thingmodel-manageable.md`（阶段 3）
- 代理链路决策：`aiDoc/notes/implemented/architecture/2026-10-09-thingmodel-write-proxy.md`
- 页面实现：`web/src/views/device/ThingModels.vue`（`loadDraftRecord`/`saveDraftRecord`/`subtractSchema`）
