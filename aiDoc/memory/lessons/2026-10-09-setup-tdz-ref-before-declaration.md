<!-- last-updated: 2026-10-09 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# `<script setup>` 顶层 TDZ：函数先调用、`const ref` 后声明，整页白屏

## 情境

`web/src/views/system/ServerMonitor.vue`：主题切换功能在 setup 顶层先 `refreshAccent()`（内部写 `ACCENT_SOFT.value`），`const ACCENT_SOFT = ref(...)` 声明却在 watch 注册之后。

## 坑 / 模式

`<script setup>` 顶层代码即 setup() 执行体，按声明顺序执行；`const`/`let` 存在暂时性死区（TDZ）。函数声明会提升、看起来「写在后面也能调」，但函数体内引用的 `const` 变量若在调用点之后声明，运行到该行即抛 `ReferenceError: Cannot access 'X' before initialization`。setup 抛错且无组件级错误边界时整个应用卸载 → 路由页**整页空白**（DOM 完全为空，不是"无数据"）。构建与 vue-tsc 均不报错，只有真实进入该路由才暴露——排查「页面白屏」先怀疑 setup 顶层执行顺序/TDZ，再看异步数据。

## 出现次数

1（2026-10-09 服务器监控页白屏，修复：把 `ACCENT_SOFT` 声明移到 `refreshAccent` 定义与调用之前）

## 状态

pending

## 晋升去向

（待晋升）

## 晋升后复发

0

## 记录日期

2026-10-09
