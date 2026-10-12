<!-- last-updated: 2026-10-12 -->
<!-- lesson-meta: status=promoted count=2 post=0 target=frontend/frontend-utils.md -->
# 格式化规则无机械闸门必漂移：formatDateTime 两轮漏网

## 情境

2026-10-12 全站时间格式统一：`frontend-utils.md` 早已规定「时间展示必须 formatDateTime」，且 2026-10-10 批次已修过一轮（设备页），本次仍扫出 12 个文件用 `slice(0,16/19).replace('T',' ')` 自造格式或裸显 RFC3339。

## 坑 / 模式

纯文档约束 + 无机械检测手段 = 必然漂移：新页面复制旧页面的坏样板时没人被拦住，批量清理只清「当时看到的」。防漂移要把规则转成可执行的 rg 模式自查（或 lint），否则每次收尾都靠人肉记忆枚举漏网。

## 出现次数

2

1. 2026-10-10 四项界面缺陷修复批次：设备页时间统一 formatDateTime（只修了当批页面）
2. 2026-10-12 全站清零：skills/notify/jobs/notices/mcp/dashboard/monitor/chat/DeviceModels/AdminLayout 共 12 文件漏网

## 状态

promoted

## 晋升去向

`frontend/frontend-utils.md`「强制使用场景清单·时间展示」行扩充禁止清单（slice 截断 / toLocaleTimeString / 裸显），并新增「时间格式的机械自查」rg 命令（收尾零命中为过）；契约侧例外清单写入 `contracts/boundary.md`「时间字段输出与展示」。

## 晋升后复发

0

## 记录日期

2026-10-12
