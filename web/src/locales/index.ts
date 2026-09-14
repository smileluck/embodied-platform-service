import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import enUS from './en-US'

export type AppLocale = 'zh-CN' | 'en-US'

const LOCALE_KEY = 'app_locale'
const DEFAULT_LOCALE: AppLocale = 'zh-CN'

// 初始语言：localStorage 优先，默认中文
function initialLocale(): AppLocale {
  const saved = localStorage.getItem(LOCALE_KEY)
  return saved === 'en-US' ? 'en-US' : DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: { 'zh-CN': zhCN, 'en-US': enUS },
})

export function getLocale(): AppLocale {
  return i18n.global.locale.value as AppLocale
}

// 切换语言：持久化到 localStorage，刷新后保持
export function setLocale(l: AppLocale) {
  localStorage.setItem(LOCALE_KEY, l)
  i18n.global.locale.value = l
}
