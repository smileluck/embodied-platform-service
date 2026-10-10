<!-- last-updated: 2026-10-10 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# 远程搜索 n-select 未预载首屏选项——「打开即空」被感知为无数据

## 情境

消息通知「按用户」发布的用户多选框（`web/src/views/system/Notices.vue`）用 `filterable remote :remote-method`，选项仅在输入关键词后请求；用户不输入直接点开下拉恒为空，反馈为「用户列表为空」。

## 坑 / 模式

`remote` 语义下 options 完全由调用方维护：初始 `[]` 就一直空到首次搜索。用户默认心智是「下拉=可选项列表」，不会先读 placeholder 提示「输入关键词搜索」。凡是「打开下拉应有默认候选」的场景，必须在字段可见（v-if 切换/弹窗打开）时以空关键词预载一次首屏；后端空关键词分支要能返回默认页（本例 limit 缺省 20）。

## 出现次数

1（2026-10-10 消息通知用户选项）。

## 状态

pending。

## 晋升去向

若复发，候选正文：`aiDoc/frontend/frontend-rules.md` 组件规范小节。

## 晋升后复发

无复发保持 0。

## 记录日期

2026-10-10
