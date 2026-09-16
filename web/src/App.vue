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

// 主题规范：色值与 src/styles/tokens.css 同源，改主题两处同步调整。
// info 固定为板岩灰（中性信息色）：主色是品牌琥珀后，info 若随主色会与 warning（警示琥珀）混淆。
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#D97706',
    primaryColorHover: '#E8960C',
    primaryColorPressed: '#B8620A',
    primaryColorSuppl: '#E8960C',
    infoColor: '#64748B',
    infoColorHover: '#75879C',
    infoColorPressed: '#5A6B7D',
    infoColorSuppl: '#75879C',
    successColor: '#16A34A',
    successColorHover: '#2DB35F',
    successColorPressed: '#128A3E',
    successColorSuppl: '#2DB35F',
    warningColor: '#E8960C',
    warningColorHover: '#F2A62B',
    warningColorPressed: '#CF820A',
    warningColorSuppl: '#F2A62B',
    errorColor: '#DC2626',
    errorColorHover: '#E7443F',
    errorColorPressed: '#C01E1E',
    errorColorSuppl: '#E7443F',
    borderRadius: '8px',
    borderRadiusSmall: '5px',
    bodyColor: '#F4F4F2',
    cardColor: '#FFFFFF',
    textColorBase: '#17191E',
    borderColor: '#E5E3DE',
    fontFamily: "-apple-system, BlinkMacSystemFont, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Segoe UI', sans-serif",
  },
}
</script>

<style>
@import './styles/tokens.css';
</style>
