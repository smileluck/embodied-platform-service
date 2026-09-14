import type { Directive } from 'vue'
import { useUserStore } from '../stores/user'

// v-permission="['menu:user']" —— 按钮级权限：用户拥有任一 code 才渲染
export const permissionDirective: Directive<HTMLElement, string[]> = {
  mounted(el, binding) {
    const userStore = useUserStore()
    const need = binding.value || []
    if (need.length && !need.some((code) => userStore.has(code))) {
      el.parentNode?.removeChild(el)
    }
  },
}
