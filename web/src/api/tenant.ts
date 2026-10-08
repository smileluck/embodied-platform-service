// 租户端 API：独立 axios 实例（/app-api/v1），与管理端（api/request.ts）隔离。
// 请求自动附带 Bearer app-access token + X-Tenant-ID（平台租户 ID，全链路唯一口径）；
// 401 时单飞刷新（直调平台 /app-auth/refresh）后重放。
import axios from 'axios'
import { useTenantUserStore } from '../stores/tenantUser'
import { getLocale } from '../locales'
import { deepTrim } from '../utils/trim'
import { createDiscreteApi } from 'naive-ui'
import { i18n } from '../locales'
import type { PageResult, R } from './types'

declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
    _retried?: boolean
  }
}

const { message: globalMessage } = createDiscreteApi(['message'])

const tenantRequest = axios.create({
  baseURL: '/app-api/v1',
  timeout: 15000,
})

tenantRequest.interceptors.request.use((config) => {
  const store = useTenantUserStore()
  if (store.accessToken) {
    config.headers.Authorization = `Bearer ${store.accessToken}`
  }
  if (store.tenantId) {
    config.headers['X-Tenant-ID'] = String(store.tenantId)
  }
  config.headers['Accept-Language'] = getLocale()
  if (config.data && typeof config.data === 'object') {
    config.data = deepTrim(config.data)
  }
  if (config.params) {
    config.params = deepTrim(config.params)
  }
  return config
})

// 401 单飞刷新（与管理端同款防死锁：认证端点自身 401 不进刷新流程）
let refreshing: Promise<string | null> | null = null

tenantRequest.interceptors.response.use(
  (resp) => resp,
  async (error) => {
    const { response, config } = error
    if (response?.status === 401 && !config._retried) {
      config._retried = true
      refreshing ||= useTenantUserStore().refresh().finally(() => (refreshing = null))
      const token = await refreshing
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
        return tenantRequest(config)
      }
      useTenantUserStore().clearAuth()
      if (!window.location.pathname.startsWith('/tenant/login')) {
        window.location.href = '/tenant/login?reason=expired'
      }
    }
    if (!config?.silent && response?.status !== 401 && (response?.status === undefined || response.status >= 500)) {
      const t = i18n.global.t
      globalMessage.error(
        response?.data?.msg || (response ? t('common.serverError') : t('common.networkError')) + (error.code === 'ECONNABORTED' ? `（${t('common.timeout')}）` : ''),
      )
    }
    return Promise.reject(error)
  },
)

export default tenantRequest

// ---- 类型 ----

// 租户端身份（GET /profile）：本人 + 当前租户 + 可访问租户 + 本租户内角色
export interface TenantProfile {
  user: { id: number; username: string; nickname: string }
  tenant: { platform_id: number; local_id: number; name: string; code: string; status: number } | null
  accessible_tenants: { platform_id: number; local_id: number; name: string; code: string; status: number }[]
  role: 'tenant_admin' | 'member'
}

// 租户成员（/members；role 为本租户内角色）
export interface TenantMember {
  id: number
  username: string
  nickname: string
  phone: string
  email: string
  status: number
  role: 'tenant_admin' | 'member'
  created_at: string
  updated_at: string
}

// 设备（与管理端 Device 同构，走租户面只读口径）
export interface TenantDevice {
  id: number
  sn: string
  name: string
  model_id: number
  tenant_id: number
  status: string
  online: boolean
  transport: string
  firmware_version?: string
  hardware_version?: string
  last_seen_at: string
  created_at: string
  updated_at: string
}

export interface DeviceShadow {
  device_id: number
  desired: string
  reported: string
  updated_at: string
}

export interface TelemetryItem {
  metric: string
  value: number
  ts: string
}

export interface TelemetryHistory {
  items: TelemetryItem[]
  next_marker: string
}

// ---- 身份与个人中心 ----

export function getTenantProfile() {
  return tenantRequest.get<R<TenantProfile>>('/profile')
}

export function changeTenantPassword(data: { old_password: string; new_password: string }) {
  return tenantRequest.put<R<null>>('/profile/password', data)
}

// ---- 成员自助管理（tenant_admin） ----

export function listTenantMembers(params: { page: number; page_size: number; kw?: string }) {
  return tenantRequest.get<R<PageResult<TenantMember>>>('/members', { params })
}

export function createTenantMember(data: {
  username: string
  password: string
  nickname?: string
  phone?: string
  email?: string
  role?: 'tenant_admin' | 'member'
}) {
  return tenantRequest.post<R<TenantMember>>('/members', data)
}

export function updateTenantMember(id: number, data: { nickname?: string; phone?: string; email?: string }) {
  return tenantRequest.put<R<null>>(`/members/${id}`, data)
}

export function setTenantMemberStatus(id: number, status: number) {
  return tenantRequest.put<R<null>>(`/members/${id}`, { status })
}

export function resetTenantMemberPassword(id: number, password: string) {
  return tenantRequest.put<R<null>>(`/members/${id}`, { password })
}

export function setTenantMemberRole(id: number, role: 'tenant_admin' | 'member') {
  return tenantRequest.put<R<null>>(`/members/${id}`, { role })
}

export function removeTenantMember(id: number) {
  return tenantRequest.delete<R<null>>(`/members/${id}`)
}

// ---- 设备只读 ----

export function listTenantDevices(params: {
  page: number
  page_size: number
  kw?: string
  status?: string
  online?: boolean
}) {
  return tenantRequest.get<R<PageResult<TenantDevice>>>('/devices', { params })
}

export function getTenantDevice(id: number) {
  return tenantRequest.get<R<TenantDevice>>(`/devices/${id}`)
}

export function getTenantDeviceShadow(id: number) {
  return tenantRequest.get<R<DeviceShadow>>(`/devices/${id}/shadow`)
}

export function getTenantDeviceTelemetry(id: number, params: { metric?: string; marker?: string; limit?: number }) {
  return tenantRequest.get<R<TelemetryHistory>>(`/devices/${id}/telemetry`, { params })
}
