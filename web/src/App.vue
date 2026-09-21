<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides" :locale="naiveLocale" :date-locale="naiveDateLocale">
    <n-message-provider>
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider, zhCN, enUS, dateZhCN, dateEnUS, type GlobalThemeOverrides } from 'naive-ui'
import { useI18n } from 'vue-i18n'

// naive-ui 组件文案随 i18n 语言切换
const { locale } = useI18n()
const naiveLocale = computed(() => (locale.value === 'en-US' ? enUS : zhCN))
const naiveDateLocale = computed(() => (locale.value === 'en-US' ? dateEnUS : dateZhCN))

// 主题单一来源：naive-ui overrides 运行时读取 variables.css 的 CSS 变量，
// 暗色切换（html.dark）时 computed 重算自动跟随，无需维护双份色值
import { darkTheme } from 'naive-ui'
import { isDarkRef } from './stores/theme'

function readVar(name: string, fallback: string): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

const naiveTheme = computed(() => (isDarkRef.value ? darkTheme : null))

const themeOverrides = computed<GlobalThemeOverrides>(() => {
  // 必须显式依赖 isDarkRef：readVar 读的是 DOM CSS 变量（非响应式），
  // 无响应式依赖的 computed 会被永久缓存——暗色下仍套用首次求值的亮色，
  // 表现为卡片等 naive 组件不随主题切换。applyTheme 在 toggle 中同步切
  // html.dark，重算此刻读到的已是新模式色值
  void isDarkRef.value
  const surface = readVar('--sx-surface', '#FFFFFF')
  return {
  common: {
    primaryColor: readVar('--sx-accent', '#3F75AB'),
    primaryColorHover: readVar('--sx-accent-hover', '#518CC8'),
    primaryColorPressed: readVar('--sx-accent-pressed', '#315E8C'),
    primaryColorSuppl: readVar('--sx-accent-hover', '#518CC8'),
    borderRadius: readVar('--sx-radius', '8px'),
    borderRadiusSmall: '5px',
    bodyColor: readVar('--sx-bg', '#F5F7FA'),
    cardColor: surface,
    textColorBase: readVar('--sx-ink', '#151E2B'),
    borderColor: readVar('--sx-line', '#E3E8EF'),
    fontFamily: readVar('--sx-font-body', 'sans-serif'),
  },
  // 弹窗内的卡片/对话框走 colorModal（naive 默认中性灰，与清水蓝主题脱节），统一对齐面板色
  Card: { colorModal: surface },
  Dialog: { colorModal: surface },
  }
})
</script>

<style>
@import './styles/tokens.css';
</style>
