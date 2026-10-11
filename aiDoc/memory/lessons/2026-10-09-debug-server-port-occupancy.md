<!-- last-updated: 2026-10-11 -->
<!-- lesson-meta: status=promoted count=2 post=0 target=aiDoc/relations/development-workflow.md -->
# 调试二进制占用 dev 端口导致 air 热重载静默失效

## 情境

验证数据映射/物模型功能时，手动 `go build -o tmp/server-verify` 起调试实例占用 28180，未在验证结束后杀掉；用户随后反馈页面「未授权」，实际流量一直由旧二进制服务。

## 坑 / 模式

在 air 热重载链路（`air -c .air.toml` → `bin/server`）之外，另起调试二进制占住同一端口（28180）会产生三重静默故障，且各方都不报错到显眼处：

1. air 每次重建的 `bin/server` 启动即 bind 失败（错误只在 logs/app 日志里），用户流量继续由旧调试实例服务——代码改了、air 显示在跑，实际服务的是旧代码；
2. 调试实例不退出地挂在后台（bind 失败后进程不死），越积越多；
3. air 的 `go build -o bin/server` 在其子进程运行中会 "text file busy" 失败，`stop_on_error=true` 下 air 逐渐 wedge（不再触发重建），bin/server 停留在旧版本。

容易再犯：验证脚本/手动起实例是调试常态，且 bind 失败不致命、进程残留无感知。

规避：手动验证用实例须用独立端口（或验证完立即 kill 并删除二进制）；发现「改了代码不生效」先 `lsof -iTCP:<port> -sTCP:LISTEN` 确认监听者是 air 的 `bin/server` 再看日志。

## 出现次数

2（2026-10-09，数据映射「未授权」排查中发现：3 个 server-verify 残留 + bin/server 悬挂 + air wedge；2026-10-11，设备中心「改了不生效」排查中发现：28170 被另一目录 `wujie-embodied-ego-platform/web` 的 vite 占用，用户一直看旧前端——同族模式从前端 dev server 方向复发，监听者身份不核对就会误诊为代码缺陷）

## 状态

promoted

## 晋升去向

`aiDoc/relations/development-workflow.md` 联调验证段（2026-10-11 第 2 次出现后晋升）

## 晋升后复发

0

## 记录日期

2026-10-09
