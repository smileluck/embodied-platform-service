// 主题状态：亮/暗切换（localStorage 持久化 + 系统偏好初始）。
// class 切换在模块加载时同步执行（配合 index.html 防闪脚本双保险）。
import { ref } from 'vue'

export type ThemeMode = 'light' | 'dark'

const KEY = 'sx-theme'

// 切换动效时长：View Transitions 交叉渐变与回退方案的统一过渡保持一致
export const THEME_TRANSITION_MS = 300

const stored = localStorage.getItem(KEY) as ThemeMode | null
const prefersDark = typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches
const prefersReducedMotion = typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

export const isDarkRef = ref<boolean>(stored ? stored === 'dark' : !!prefersDark)

export function applyTheme() {
  document.documentElement.classList.toggle('dark', isDarkRef.value)
}

function applyToggle() {
  isDarkRef.value = !isDarkRef.value
  localStorage.setItem(KEY, isDarkRef.value ? 'dark' : 'light')
  applyTheme()
}

// 硬切整帧替换会让各区域以不同节奏跳变（部分元素还带各自过渡），观感割裂。
// 优先 View Transitions：整屏旧新两帧交叉渐变，全部元素同一节奏；不支持的
// 浏览器回退为「切换期间统一给全元素加过渡」；系统设置了减少动效则直接切换。
export function toggleTheme() {
  if (prefersReducedMotion) {
    applyToggle()
    return
  }
  const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
  if (typeof doc.startViewTransition === 'function') {
    doc.startViewTransition(applyToggle)
    return
  }
  document.documentElement.classList.add('theme-switching')
  applyToggle()
  window.setTimeout(() => document.documentElement.classList.remove('theme-switching'), THEME_TRANSITION_MS)
}

applyTheme()
