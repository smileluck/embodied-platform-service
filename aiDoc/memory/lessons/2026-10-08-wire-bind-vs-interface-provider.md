<!-- last-updated: 2026-10-08 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# wire：Provider 直接返回 biz 接口时不要再 wire.Bind

## 情境

新增 tenantmember 上下文注册依赖（2026-10-08，`cmd/server/wire.go`）：仓储 Provider 写成
`func NewRepo(d *data.Data) biztenantmember.Repo`（直接返回接口），同时又写了
`wire.Bind(new(biztenantmember.Repo), new(*datatenantmember.Repo))`——wire 报
`undefined: datatenantmember.Repo`（实现类型未导出，Bind 引用不到），去掉 Bind 后通过。

## 坑 / 模式

本仓仓储 Provider 有两种风格，Bind 的用法随之不同，混用即报错：
- Provider 返回**未导出实现类型**（如 `datatenant.NewSyncer` 返回 `*Syncer`）→ 消费方要接口时**必须** `wire.Bind(接口, *实现)`；
- Provider **直接返回 biz 接口**（如 `datatenant.NewRepo` 返回 `biztenant.Repo`）→ **不需要也不能**再 Bind（wire 已按返回类型供图）。

新增上下文抄 wire.go 里相邻条目时容易把两种风格的 Bind 一起抄。判断口诀：看 Provider 的返回类型签名即可，不必看实现。

## 出现次数

1（首次建档）

## 状态

pending

## 晋升去向

暂未晋升（次数 1）；若再犯，规则应写入 `aiDoc/modules/module-development.md` 第 6 步（依赖注入）。

## 晋升后复发

无（0）。

## 记录日期

2026-10-08
