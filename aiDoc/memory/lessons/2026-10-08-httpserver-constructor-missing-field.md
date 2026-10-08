<!-- last-updated: 2026-10-08 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 大构造体加字段漏赋值零告警：HTTPServer 字面量遗漏 → 整域 handler nil panic

## 情境

用户报告「登录后 dashboard 提示服务异常」（2026-10-08）。定位：`internal/server/router.go`
的 `NewHTTPServer` 构造体字面量从未给 `job/dashboard/notify/mcp/skill` 五个字段赋值
（参数收了、struct 声明了，字面量漏写）——git 历史里搜不到任何时刻存在过这些赋值，
即自这些域引入起 dashboard/jobs/notify/mcp/skills 页面就一直 nil 指针 panic→500（空响应体），
属存量缺陷；`s.job.Stop()` 停机也会 panic。

## 坑 / 模式

- Go 构造体字面量**漏字段不报错、不警告**，`go build`/`go vet`/wire 全部拦不住；
  症状只在运行期出现：该域任意 handler 一调用就 500（gin Recovery 吞 panic，zap 日志无痕，
  栈只在 stderr——air/dev 控制台或旁路实例才能看到）。
- 排查捷径：`HTTP 500 且响应体为空` 基本就是 panic 而非业务错误（`FailI18n` 一定有 JSON 体）；
  旁路实例 `APP_SERVER_PORT=xxx ./bin/server 2>err.log` 一发请求即得完整栈。
- 防呆：给 `HTTPServer` 增删字段或 `NewHTTPServer` 增删参数时，收尾 `grep -n "<field>:" internal/server/router.go`
  核对字面量已赋值；改完后对受影响域做一次真实请求冒烟。

## 出现次数

1（首次建档；波及面为存量五域）

## 状态

pending

## 晋升去向

暂未晋升（次数 1）；若再犯，规则应写入 `aiDoc/modules/module-development.md`（handler/路由注册步骤的完成前检查）。

## 晋升后复发

无（0）。

## 记录日期

2026-10-08
