<!-- last-updated: 2026-10-20 -->
# 存储配置合并：platform.storage 并入 storage.platform

> 路径：`aiDoc/notes/implemented/simplification/2026-10-20-storage-config-merge.md`
> class = simplification

## 问题

`configs/config.yaml` 中文件存储的配置拆在两处：`platform.storage`（storage-gateway 连接：baseUrl/apiKeyId/apiSecret/bucket）与顶层 `storage`（driver 选择、上传限制、local 降级目录）。两段是同一主题，唯一的交汇点只有 `internal/data/file/manager.go`，且 storage-gateway 客户端只服务文件存储（不像 `platform.baseUrl`/`appKey` 被身份、租户、设备等域共享），拆开纯属历史摆放，读者需要在两段之间来回对照。

## 提案 / 决策

把 `platform.storage` 整体迁入 `storage.platform`，与 `storage.local` 对称；`platform:` 段回归纯粹的平台主服务接入（baseUrl/appKey/appSecret/timeoutSeconds）。同步改动：

- `internal/conf/config.go`：`PlatformStorage` 从 `Platform.Storage` 挪到 `Storage.Platform`；默认值键 `platform.storage.bucket` → `storage.platform.bucket`
- 消费方：`internal/data/platform/storage.go`、`internal/data/file/manager.go`、`cmd/server/main.go` 改读 `cfg.Storage.Platform`
- 文档：`configs/config.yaml` 注释、`README.md`、`aiDoc/contracts/boundary.md`、`aiDoc/relations/system-map.md`

## 真实考虑过的备选

| 备选 | 放弃理由 |
|---|---|
| 反向合并（策略并入 `platform` 段） | `local` 后端与 maxSizeMB/denyExts 等限制与平台无关，塞进 platform 段语义更差 |
| 维持现状 | 两段配置无功能冲突，但同一主题两地维护，driver 自动选择逻辑需跨段读值，长期可读性差 |

## 验收标准

- [x] `go build ./... && go vet ./...` 通过（Platform 段已无 Storage 字段，残留引用编译期即暴露）
- [x] `conf.Load("configs/config.yaml")` 冒烟：`storage.platform` 的 baseUrl/apiKeyId/bucket 正确解析，`storage.local.dir` 与默认值不受影响
- [x] 全仓 grep 无 `platform.storage` / `Platform.Storage` / `PLATFORM_STORAGE` 残留（`platform.StorageClient` 为 Go 包限定名，非配置路径）
- [x] 启动时 storage-gateway 连通性自检（`cmd/server/main.go:checkPlatform`）改读新路径，行为不变

## 风险与后果

- **破坏性配置变更**：存量部署的 `config.yaml` 需把 `platform.storage` 块移到 `storage.platform` 下；旧结构下 `storage.platform` 解析为空 → driver 自动降级 local（不报错但文件不进平台存储），升级后需核对启动自检日志
- **环境变量改名**：viper `APP_` 前缀覆盖键随配置路径变化，`APP_PLATFORM_STORAGE_*` → `APP_STORAGE_PLATFORM_*`（当前 docker-compose 未注入该类变量，无 compose 变更）
- 「平台侧要 provision 的凭证都在 `platform:` 段」的部署清单约定被打破：storage API Key 改在 `storage.platform` 段登记（README 部署前置清单已同步）

## 交叉链接

- 配置结构：`internal/conf/config.go:Storage` / `PlatformStorage`
- 后端注册与 driver 自动选择：`internal/data/file/manager.go:NewStorageManager`
- 契约侧描述：`aiDoc/contracts/boundary.md`「组件间契约：本服务 ↔ 平台开放面」
