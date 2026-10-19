<!-- last-updated: 2026-10-17 -->
<!-- lesson-meta: status=promoted count=2 post=0 target=aiDoc/relations/development-workflow.md -->
# 管理端 GUI 冒烟的准入凭据路径

## 情境

设备详情命令下发修复后按「用户可见输出走真实入口冒烟」打开 http://127.0.0.1:28170 做登录冒烟（2026-10-11）；2026-10-17 侧边栏滚动条优化冒烟再次遇到同一准入问题。

## 坑 / 模式

管理端登录代理平台：admin 密码在平台仓库 `embodied-platform/configs/config.yaml` 的 `adminPassword`（当前 `123456`；留空则首次播种随机生成、只在后台日志打印一次）。密码正确仍可能被拒——报「该账号无权进入本系统」是本系统准入 403（setupDynamicRoutes 阶段），不是密码错；需要平台侧本商户成员账号。做 GUI 冒烟时：先从平台 config/日志取凭据，若撞准入 403 即阻塞（凭据只有用户能提供），不要反复试密码，如实按 not-run 报告。

## 出现次数

2

1. 2026-10-11 设备详情命令下发修复冒烟：`admin` 密码正确但报「该账号无权进入本系统」，按 not-run 报告
2. 2026-10-17 侧边栏滚动条优化冒烟：`admin` 仍 403（平台 `merchant_users` 中 `admin` 的 `admitted=0`）；改用已准入的商户成员 `ego`（密码同平台开发默认）登录成功。结论：GUI 冒烟应直接用 `merchant_users.admitted=1` 的成员账号，而非平台超管

## 状态

promoted

## 晋升去向

`aiDoc/relations/development-workflow.md`「联调验证」段新增「界面冒烟凭据」规则（2026-10-17 第 2 次出现后晋升）。

## 晋升后复发

0

## 记录日期

2026-10-17
