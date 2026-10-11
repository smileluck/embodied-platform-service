<!-- last-updated: 2026-10-11 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 管理端 GUI 冒烟的准入凭据路径

## 情境

设备详情命令下发修复后按「用户可见输出走真实入口冒烟」打开 http://127.0.0.1:28170 做登录冒烟（2026-10-11）。

## 坑 / 模式

管理端登录代理平台：admin 密码在平台仓库 `embodied-platform/configs/config.yaml` 的 `adminPassword`（当前 `123456`；留空则首次播种随机生成、只在后台日志打印一次）。密码正确仍可能被拒——报「该账号无权进入本系统」是本系统准入 403（setupDynamicRoutes 阶段），不是密码错；需要平台侧本商户成员账号。做 GUI 冒烟时：先从平台 config/日志取凭据，若撞准入 403 即阻塞（凭据只有用户能提供），不要反复试密码，如实按 not-run 报告。

## 出现次数

1（首次建档）

## 状态

pending

## 晋升去向

未达晋升条件（出现 1 次）；若再次出现考虑晋升到 relations/development-workflow.md 冒烟/调试一节。

## 晋升后复发

0

## 记录日期

2026-10-11
