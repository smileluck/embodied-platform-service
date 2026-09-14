<template>
  <n-config-provider :theme="null" :theme-overrides="themeOverrides" :locale="naiveLocale" :date-locale="naiveDateLocale">
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

// 主题规范：色值与 src/styles/tokens.css 同源，改主题两处同步调整
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#3F75AB',
    primaryColorHover: '#518CC8',
    primaryColorPressed: '#315E8C',
    primaryColorSuppl: '#518CC8',
    borderRadius: '8px',
    borderRadiusSmall: '5px',
    bodyColor: '#F5F7FA',
    cardColor: '#FFFFFF',
    textColorBase: '#151E2B',
    borderColor: '#E3E8EF',
    fontFamily: "-apple-system, BlinkMacSystemFont, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Segoe UI', sans-serif",
  },
}
</script>

<style>
@import './styles/tokens.css';
</style>
