<!-- last-updated: 2026-10-09 -->
<!-- lesson-meta: status=pending count=1 post=0 target= -->
# vue-i18n 文案中的字面 `{xxx}` 会被当作插值变量

> 路径：`aiDoc/memory/lessons/2026-10-09-vue-i18n-literal-braces.md`

## 情境

数据映射页（web/src/views/device/DataMappings.vue）locale 文案需要原样展示 source_topic 占位符 `{sn}`/`{device_id}` 等。

## 坑 / 模式

vue-i18n 把消息中的 `{sn}` 解析为命名插值：调用处不传同名参数时渲染异常并打 console warning。凡是 locale 文案要展示字面花括号（技术占位符、示例表达式），调用处必须传同名字面参数（如 `t('dataMapping.topicHint', { sn: '{sn}', ... })`），DataMappings.vue 用 `tmPlaceholders` 常量统一承载。容易再犯：写文案的人只看 zh 文本，不会意识到 `{` 是 i18n 语法。

## 出现次数

1

## 状态

pending

## 晋升去向

（待第 2 次出现或用户确认后晋升到 frontend-rules.md 国际化规范节）

## 晋升后复发

0

## 记录日期

2026-10-09
