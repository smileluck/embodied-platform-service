// 页面外观偏好：标签栏显隐 / 深色侧边栏 / 内容区定宽居中。
// 与 theme.ts 同款模式：模块级 ref + 手写 localStorage（键风格 sx-*）。
import { ref } from 'vue'

function persistedBool(key: string, fallback: boolean): boolean {
  const v = localStorage.getItem(key)
  if (v === null) return fallback
  return v === '1'
}

function persistBool(key: string, value: boolean) {
  localStorage.setItem(key, value ? '1' : '0')
}

const TABS_KEY = 'sx-tabs-visible'
const SIDER_DARK_KEY = 'sx-sider-dark'
const CONTENT_NARROW_KEY = 'sx-content-narrow'

// 多标签页栏：默认显示（隐藏仅收起视觉栏，路由同步与 keep-alive 缓存不受影响）
export const showTabs = ref<boolean>(persistedBool(TABS_KEY, true))
export function setShowTabs(v: boolean) {
  showTabs.value = v
  persistBool(TABS_KEY, v)
}

// 深色侧边栏：品牌藏蓝面板底（--sx-panel）+ 反色菜单
export const darkSider = ref<boolean>(persistedBool(SIDER_DARK_KEY, false))
export function setDarkSider(v: boolean) {
  darkSider.value = v
  persistBool(SIDER_DARK_KEY, v)
}

// 内容区定宽居中：宽屏下限宽阅读
export const narrowContent = ref<boolean>(persistedBool(CONTENT_NARROW_KEY, false))
export function setNarrowContent(v: boolean) {
  narrowContent.value = v
  persistBool(CONTENT_NARROW_KEY, v)
}
