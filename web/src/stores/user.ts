import { defineStore } from 'pinia'
import { getMenus, getProfile, login as apiLogin, logout as apiLogout, refreshToken as apiRefresh } from '../api'
import type { MenuNode, Permission, UserInfo } from '../api/types'

// 用户 store：平台仍是唯一身份源，但管理端登录/刷新/登出经本服务后端代理
// （浏览器不再直调平台，规避 CORS/可达性耦合）；token 双用不变，存储与旧版一致（localStorage）。
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
    // 登录（验证码参数在后端开启验证码时使用；失败 msg 已由后端本地化）
    async login(username: string, password: string, captchaId = '', captchaCode = '') {
      const { data } = await apiLogin({ username, password, captcha_id: captchaId, captcha_code: captchaCode })
      this.accessToken = data.data.access_token
      this.refreshTokenValue = data.data.refresh_token
      localStorage.setItem('access_token', this.accessToken)
      localStorage.setItem('refresh_token', this.refreshTokenValue)
    },
    // 静默刷新（refresh_token 放 body）；成功返回新 token
    async refresh(): Promise<string | null> {
      if (!this.refreshTokenValue) return null
      try {
        const { data } = await apiRefresh(this.refreshTokenValue)
        this.accessToken = data.data.access_token
        this.refreshTokenValue = data.data.refresh_token
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
    // 供 401 拦截器与路由守卫使用——过期场景下再发 logout 请求只会引发二次 401 竞态。
    // 注意先清 localStorage 再 $reset：$reset 会重跑 state() 工厂读取 localStorage，
    // 顺序颠倒会把旧 token 读回内存
    clearAuth() {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      this.$reset()
    },
    async logout() {
      // 登出吊销服务端会话（token 立即失效）；失败不阻断本地清态
      if (this.accessToken) {
        try {
          await apiLogout()
        } catch {
          // 忽略：本地清态不受影响
        }
      }
      this.clearAuth()
    },
  },
})
