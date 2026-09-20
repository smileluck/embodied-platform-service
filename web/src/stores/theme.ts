// 主题状态：亮/暗切换（localStorage 持久化 + 系统偏好初始）。
// class 切换在模块加载时同步执行（配合 index.html 防闪脚本双保险）。
import { ref } from 'vue'

export type ThemeMode = 'light' | 'dark'

const KEY = 'sx-theme'

const stored = localStorage.getItem(KEY) as ThemeMode | null
const prefersDark = typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches

export const isDarkRef = ref<boolean>(stored ? stored === 'dark' : !!prefersDark)

export function applyTheme() {
  document.documentElement.classList.toggle('dark', isDarkRef.value)
}

export function toggleTheme() {
  isDarkRef.value = !isDarkRef.value
  localStorage.setItem(KEY, isDarkRef.value ? 'dark' : 'light')
  applyTheme()
}

applyTheme()
