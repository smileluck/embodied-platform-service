import { defineStore } from 'pinia'
import { getMenus, getProfile } from '../api'
import { platformLogin, platformLogout, platformRefresh } from '../api/platform'
import { getLocale } from '../locales'
import type { MenuNode, Permission, UserInfo } from '../api/types'

// 用户 store：平台是唯一身份源——登录/刷新/登出直调平台（token 双用），
// 本系统后端只做 token 自省 + 本地准入；token 存储与旧版一致（localStorage）。
export const useUserStore = defineStore('user', {
  state: () => ({
    accessToken: localStorage.getItem('access_token') || '',
    refreshTokenValue: localStorage.getItem('refresh_token') || '',
    user: null as UserInfo | null,
    permissions: [] as Permission[],
    menus: [] as MenuNode[],
    routesLoaded: false,
  }),
  getters: {
    codes: (s) => s.permissions.map((p) => p.code),
    has(state) {
      // 'all' 为超管通配权限点，与后端 RBAC 的 */* 语义一致
      return (code: string) => state.permissions.some((p) => p.code === 'all' || p.code === code)
    },
  },
  actions: {
    // 平台登录（验证码参数在平台开启验证码时使用）
    async login(username: string, password: string, captchaId = '', captchaCode = '') {
      const data = await platformLogin(username, password, getLocale(), captchaId, captchaCode)
      this.accessToken = data.access_token
      this.refreshTokenValue = data.refresh_token
      localStorage.setItem('access_token', this.accessToken)
      localStorage.setItem('refresh_token', this.refreshTokenValue)
    },
    // 静默刷新（直调平台，refresh_token 放 body——跨域 cookie 不可用）；成功返回新 token
    async refresh(): Promise<string | null> {
      if (!this.refreshTokenValue) return null
      try {
        const data = await platformRefresh(this.refreshTokenValue, getLocale())
        this.accessToken = data.access_token
        this.refreshTokenValue = data.refresh_token
        localStorage.setItem('access_token', this.accessToken)
        localStorage.setItem('refresh_token', this.refreshTokenValue)
        return this.accessToken
      } catch {
        return null
      }
    },
    // 拉取用户信息 + 菜单（登录后 / 刷新页面后调用）
    async loadUserContext() {
      const [profileRes, menusRes] = await Promise.all([getProfile(), getMenus()])
      this.user = profileRes.data.data.user
      this.permissions = profileRes.data.data.permissions
      this.menus = menusRes.data.data
      this.routesLoaded = true
    },
    // 轻量清态：只清内存与 localStorage，不发网络请求。
    // 供 401 拦截器与路由守卫使用——过期场景下再发 logout 请求只会引发二次 401 竞态
    clearAuth() {
      this.$reset()
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    },
    async logout() {
      // 登出吊销平台会话（token 立即失效）；失败不阻断本地清态
      if (this.accessToken) {
        await platformLogout(this.accessToken, getLocale())
      }
      this.clearAuth()
    },
  },
})
