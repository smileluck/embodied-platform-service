// 菜单图标统一渲染：本地 @vicons/ionicons5 图标名 或 网络图片 URL
import { h } from 'vue'
import { NIcon } from 'naive-ui'
import * as icons from '@vicons/ionicons5'

const FALLBACK = '📄'

// 旧种子数据使用 element 风格名称，做一层兼容映射
const LEGACY_ALIAS: Record<string, string> = {
  HomeFilled: 'HomeOutline', Setting: 'SettingsOutline', User: 'PersonOutline',
  Avatar: 'IdCardOutline', Lock: 'LockClosedOutline', Menu: 'MenuOutline',
}

// 判断是否为图片地址（网络 URL 或以 / 开头的本地静态路径）
export function isImageIcon(icon?: string): boolean {
  if (!icon) return false
  return /^(https?:\/\/|\/)/i.test(icon)
}

// 图标名 -> ionicons5 组件（不存在返回 null）
export function resolveIcon(name?: string): any | null {
  if (!name) return null
  const key = LEGACY_ALIAS[name] || name
  return (icons as Record<string, any>)[key] ?? null
}

// 渲染为 VNode：图片用 <img>，名称用 NIcon，兜底 📄
export function renderMenuIcon(icon?: string, size = 18) {
  if (isImageIcon(icon)) {
    return h('img', {
      src: icon!,
      style: `width:${size}px;height:${size}px;object-fit:contain;display:block`,
      onError: (e: Event) => {
        // 图片加载失败降级为 📄，避免裂图
        const el = e.target as HTMLElement
        if (el?.parentElement) el.outerHTML = `<span style="font-size:${size - 2}px">${FALLBACK}</span>`
      },
    })
  }
  const comp = resolveIcon(icon)
  if (comp) {
    return h(NIcon, { size }, { default: () => h(comp) })
  }
  return h('span', { style: `font-size:${size - 2}px` }, FALLBACK)
}
