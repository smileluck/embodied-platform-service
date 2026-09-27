# embodied-platform 开放 API Go SDK

独立 go module（`github.com/smilex/smilex-admin-gin/sdk`），仅标准库依赖，**禁止 import 平台 `internal/`**。
封装：商户 HMAC 签名（`signer.go`，与平台 `VerifySign` 权威定义逐段一致）+ 客户端（信封解包/分页/错误透出）+ 契约类型镜像（`types.go`）+ scope 常量。

## 引入方式

私有仓库经 GOPRIVATE 拉取：

```bash
git config --global url."git@github.com:smilex/embodied-platform.git".insteadOf "https://github.com/smilex/embodied-platform"
export GOPRIVATE=github.com/smilex/*
go get github.com/smilex/smilex-admin-gin/sdk
```

联调期可直接 replace：

```go
// go.mod
require github.com/smilex/smilex-admin-gin/sdk v0.0.0
replace github.com/smilex/smilex-admin-gin/sdk => ../embodied-platform/sdk
```

## 快速开始

```go
// 直连主服务
c := sdk.NewClient("http://platform:27080", appKey, appSecret)
// 经 /gw 网关（签名 path 自动切 /gw/open-api/v1 口径）
c = sdk.NewClient("http://gw:27081/gw/open-api", appKey, appSecret)

who, err := c.Ping(ctx)                          // 凭证自检（不受 scope 限制）
devs, page, err := c.ListDevices(ctx, 1, 20, sdk.DeviceFilter{Keyword: "sn-001"})

dev, err := c.RegisterDevice(ctx, sdk.RegisterDeviceRequest{SN: "sn-9", Name: "采集器", ModelID: 3, TenantID: 10})
shadow, err := c.GetShadow(ctx, dev.ID)

cmd, err := c.IssueCommand(ctx, dev.ID, sdk.IssueCommandRequest{   // 受理即返回
    CommandType: "service:reboot", Priority: 3, IdempotencyKey: "job-42"})
cmd, err = c.GetCommand(ctx, cmd.ID)             // 轮询至终态 succeeded/failed

hist, err := c.QueryTelemetry(ctx, dev.ID, sdk.TelemetryQuery{Metric: "temp", IntervalSeconds: 60})
events, err := c.ListDataEvents(ctx, dev.ID, sinceID, 100)  // 上轮最大 ID 作下轮 sinceID
```

## 关键契约

- 统一信封 `{code,msg,data}`：`code!=0` 返回 `*sdk.Error{HTTPStatus,Code,Msg}`；429 按 `Retry-After` 退避。
- 签名 path = **实际发送的完整 path**（不含 query）；body hash 对原始字节（先拼 JSON 再签名）。
- 时间字段 RFC3339；nonce 8–64 字符、600s 内不可重复；时钟偏差 ≤300s。
- scope（`device:*` / `telemetry:read` / `data-event:read`…）由平台商户管理界面配置，default deny，未授权接口 403。
- 契约漂移防护：平台仓库 `internal/service/openapi/contract_test.go` 对本包类型做字段比对，平台侧改契约必须同改 SDK。

## 版本

跟随平台 `/open-api/v1` 契约演进；破坏性变更时平台侧先改 boundary.md 与 integration-guide.md，本 SDK 同变更发布。
