import { defineStore } from 'pinia'
import { platformAppLogin, platformAppRefresh } from '../api/platform'
import { getTenantProfile } from '../api/tenant'
import { getLocale } from '../locales'
import type { TenantProfile } from '../api/tenant'

// 租户端用户 store：与商户管理端（stores/user.ts）完全隔离——token 独立存储键，
// 登录/刷新直调平台 /app-auth（token 双用于平台与本系统 /app-api/v1）。
// 当前租户（X-Tenant-ID 口径的平台租户 ID）来自登录后的租户选择，随请求头携带。
export const TENANT_ACCESS_KEY = 'tenant_access_token'
export const TENANT_REFRESH_KEY = 'tenant_refresh_token'
export const TENANT_TENANT_KEY = 'tenant_selected_id'

export const useTenantUserStore = defineStore('tenantUser', {
  state: () => ({
    accessToken: localStorage.getItem(TENANT_ACCESS_KEY) || '',
    refreshTokenValue: localStorage.getItem(TENANT_REFRESH_KEY) || '',
    // 当前租户平台 ID（0=尚未选择，登录后由选择页/恢复值设定）
    tenantId: Number(localStorage.getItem(TENANT_TENANT_KEY)) || 0,
    profile: null as TenantProfile | null,
    contextLoaded: false,
  }),
  getters: {
    // 当前租户内角色（tenant_admin | member；profile 未加载时视为 member）
    role: (s) => s.profile?.role || 'member',
    isAdmin(): boolean {
      return this.role === 'tenant_admin'
    },
    user: (s) => s.profile?.user || null,
    accessibleTenants: (s) => s.profile?.accessible_tenants || [],
  },
  actions: {
    // 平台应用用户登录（租户端）：token 落键后按登录响应的 tenant_ids 自举当前
    // 租户（本系统 /app-api/v1 的 X-Tenant-ID 需先于 profile 确定；已选租户仍在
    // 归属集内则保持，否则回落第一个）
    async login(username: string, password: string) {
      const data = await platformAppLogin(username, password, getLocale())
      this.accessToken = data.access_token
      this.refreshTokenValue = data.refresh_token
      localStorage.setItem(TENANT_ACCESS_KEY, this.accessToken)
      localStorage.setItem(TENANT_REFRESH_KEY, this.refreshTokenValue)
      const tenantIds = data.user?.tenant_ids || []
      if (!tenantIds.includes(this.tenantId)) {
        this.switchTenant(tenantIds[0] || 0)
      }
      return tenantIds
    },
    // 静默刷新（直调平台 app-auth/refresh）；成功返回新 access token
    async refresh(): Promise<string | null> {
      if (!this.refreshTokenValue) return null
      try {
        const data = await platformAppRefresh(this.refreshTokenValue, getLocale())
        this.accessToken = data.access_token
        this.refreshTokenValue = data.refresh_token
        localStorage.setItem(TENANT_ACCESS_KEY, this.accessToken)
        localStorage.setItem(TENANT_REFRESH_KEY, this.refreshTokenValue)
        return this.accessToken
      } catch {
        return null
      }
    },
    // 拉取租户端上下文（本人身份 + 可访问租户 + 本租户内角色）。
    // 需先选定租户（X-Tenant-ID）；未选定时由登录流程引导选择
    async loadContext() {
      const { data: resp } = await getTenantProfile()
      this.profile = resp.data
      this.contextLoaded = true
      // 选定租户已被移出归属集（profile 的 accessible 不含）时回落第一个可用租户
      if (!resp.data.accessible_tenants.some((x) => x.platform_id === this.tenantId)) {
        const first = resp.data.accessible_tenants[0]
        this.switchTenant(first ? first.platform_id : 0)
      }
    },
    // 切换当前租户（平台租户 ID）：写存储并重载上下文（角色随租户变化）
    switchTenant(tenantId: number) {
      this.tenantId = tenantId
      localStorage.setItem(TENANT_TENANT_KEY, String(tenantId))
    },
    // 轻量清态：仅清内存与 localStorage（平台 /app-auth 无 logout 端点，会话靠平台侧过期）
    clearAuth() {
      localStorage.removeItem(TENANT_ACCESS_KEY)
      localStorage.removeItem(TENANT_REFRESH_KEY)
      localStorage.removeItem(TENANT_TENANT_KEY)
      this.$reset()
    },
    // 登出 = 本地清态（无平台侧吊销端点）
    async logout() {
      this.clearAuth()
    },
  },
})
