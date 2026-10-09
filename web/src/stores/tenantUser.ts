import { defineStore } from 'pinia'
import { tenantPortalLogin, tenantPortalRefresh, getTenantProfile } from '../api/tenant'
import type { TenantProfile } from '../api/tenant'

// 租户门户用户 store：与商户管理端（stores/user.ts）完全隔离——token 独立存储键。
// 2026-10-08 起门户身份=平台第四套身份 tenant_users（单租户绑定）：登录/刷新经本服务
// 后端代理平台 /tenant-api/v1/auth（token 双用于平台与本系统 /tenant-api/v1）；
// 租户上下文在 token 内（tid 自省解析），无租户切换概念；授权=平台租户 RBAC
// perm_codes（member:* 即成员自治能力）。
export const TENANT_ACCESS_KEY = 'tenant_access_token'
export const TENANT_REFRESH_KEY = 'tenant_refresh_token'

export const useTenantUserStore = defineStore('tenantUser', {
  state: () => ({
    accessToken: localStorage.getItem(TENANT_ACCESS_KEY) || '',
    refreshTokenValue: localStorage.getItem(TENANT_REFRESH_KEY) || '',
    profile: null as TenantProfile | null,
    contextLoaded: false,
  }),
  getters: {
    user: (s) => s.profile?.user || null,
    tenant: (s) => s.profile?.tenant || null,
    // tid（平台租户 ID）取自 profile（TenantAuth 自省 token 得来，单租户绑定）
    tenantId(): number {
      return this.profile?.user?.tenant_id || this.tenant?.platform_id || 0
    },
    permCodes: (s) => s.profile?.perm_codes || [],
    // 权限码精确匹配（与后端 RequireTenantPerm 同口径，无通配）
    hasPerm(): (code: string) => boolean {
      return (code: string) => this.permCodes.includes(code)
    },
    // 成员自治能力（菜单显隐沿用 isAdmin 语义：可管成员即视为门户管理员）
    isAdmin(): boolean {
      return this.hasPerm('member:user:list')
    },
  },
  actions: {
    // 门户登录（经本服务代理平台）：token 落键；租户归属由 profile 自省带回
    async login(username: string, password: string) {
      const { data: resp } = await tenantPortalLogin({ username, password })
      this.accessToken = resp.data.access_token
      this.refreshTokenValue = resp.data.refresh_token
      localStorage.setItem(TENANT_ACCESS_KEY, this.accessToken)
      localStorage.setItem(TENANT_REFRESH_KEY, this.refreshTokenValue)
    },
    // 静默刷新（经本服务代理平台）；成功返回新 access token
    async refresh(): Promise<string | null> {
      if (!this.refreshTokenValue) return null
      try {
        const { data: resp } = await tenantPortalRefresh(this.refreshTokenValue)
        this.accessToken = resp.data.access_token
        this.refreshTokenValue = resp.data.refresh_token
        localStorage.setItem(TENANT_ACCESS_KEY, this.accessToken)
        localStorage.setItem(TENANT_REFRESH_KEY, this.refreshTokenValue)
        return this.accessToken
      } catch {
        return null
      }
    },
    // 拉取门户上下文（本人身份 + 当前租户 + 权限码集合）
    async loadContext() {
      const { data: resp } = await getTenantProfile()
      this.profile = resp.data
      this.contextLoaded = true
    },
    // 轻量清态：仅清内存与 localStorage（平台门户无 logout 端点，会话靠平台侧过期）
    clearAuth() {
      localStorage.removeItem(TENANT_ACCESS_KEY)
      localStorage.removeItem(TENANT_REFRESH_KEY)
      this.$reset()
    },
    // 登出 = 本地清态（无平台侧吊销端点）
    async logout() {
      this.clearAuth()
    },
  },
})
