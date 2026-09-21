// 主题状态：亮/暗/跟随系统（localStorage 持久化 + 系统偏好初始）。
// class 切换在模块加载时同步执行（配合 index.html 防闪脚本双保险）。
import { computed, ref } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'

const KEY = 'sx-theme'

// 切换动效时长：View Transitions 交叉渐变与回退方案的统一过渡保持一致
export const THEME_TRANSITION_MS = 300

const prefersDarkMQ =
  typeof window !== 'undefined' && window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null

// 系统偏好实时镜像：mode=system 时 isDark 跟随它变化
const systemDark = ref<boolean>(!!prefersDarkMQ?.matches)
prefersDarkMQ?.addEventListener?.('change', (e) => {
  systemDark.value = e.matches
  applyTheme()
})

const stored = localStorage.getItem(KEY) as ThemeMode | null

// 主题模式（非生效明暗）：未设置过 = 跟随系统；存量 'dark'/'light' 值兼容
export const mode = ref<ThemeMode>(stored ?? 'system')

// 生效明暗（只读语义）：所有消费方（顶栏图标、图表 watch 等）读这个
export const isDarkRef = computed<boolean>(
  () => mode.value === 'dark' || (mode.value === 'system' && systemDark.value),
)

export function applyTheme() {
  document.documentElement.classList.toggle('dark', isDarkRef.value)
}

// 硬切整帧替换会让各区域以不同节奏跳变（部分元素还带各自过渡），观感割裂。
// 优先 View Transitions：整屏旧新两帧交叉渐变，全部元素同一节奏；不支持的
// 浏览器回退为「切换期间统一给全元素加过渡」；系统设置了减少动效则直接切换。
function withTransition(turn: () => void) {
  if (typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
    turn()
    return
  }
  const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
  if (typeof doc.startViewTransition === 'function') {
    doc.startViewTransition(turn)
    return
  }
  document.documentElement.classList.add('theme-switching')
  turn()
  window.setTimeout(() => document.documentElement.classList.remove('theme-switching'), THEME_TRANSITION_MS)
}

function commit(next: ThemeMode) {
  mode.value = next
  localStorage.setItem(KEY, next)
  applyTheme()
}

// 切换主题模式：生效明暗真正变化时播整屏渐变，否则静默完成（如暗色→跟随系统且系统即暗色）
export function setMode(next: ThemeMode) {
  if (next === mode.value) return
  const willDark = next === 'dark' || (next === 'system' && systemDark.value)
  if (willDark === isDarkRef.value) {
    commit(next)
    return
  }
  withTransition(() => commit(next))
}

// 顶栏快速切换按钮：按当前生效明暗取反（system 模式下也能一键到明确亮/暗）
export function toggleTheme() {
  withTransition(() => commit(isDarkRef.value ? 'light' : 'dark'))
}

applyTheme()
