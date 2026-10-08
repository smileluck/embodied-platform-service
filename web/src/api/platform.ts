// 平台（embodied-platform）直调 API：仅保留应用用户（租户端）认证；
// 管理端登录/刷新/登出已改经本服务后端代理（见 api/index.ts 的 login/refreshToken/logout），token 仍双用。
// 平台是唯一身份源——本系统后端不碰密码，只做代理转发 + token 自省 + 本地准入。
// 平台地址来自 VITE_PLATFORM_API（开发默认 http://localhost:27080）；
// 生产部署需在平台 CORS 白名单（cors.allowedOrigins）登记本前端 Origin。
import type { TokenPair } from './types'

export const PLATFORM_API = (import.meta.env.VITE_PLATFORM_API as string) || 'http://localhost:27080'

// 平台统一信封 {code,msg,data}；code!=0 抛错（msg 已本地化，按 Accept-Language）
async function call<T>(path: string, init: RequestInit, acceptLanguage: string): Promise<T> {
  const resp = await fetch(PLATFORM_API + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'Accept-Language': acceptLanguage,
      ...(init.headers || {}),
    },
  })
  let body: any
  try {
    body = await resp.json()
  } catch {
    throw new Error(`platform: http ${resp.status}`)
  }
  if (!body || body.code !== 0) {
    const err: any = new Error(body?.msg || `platform: http ${resp.status}`)
    err.platformStatus = resp.status
    throw err
  }
  return body.data as T
}

// ---- 应用用户认证（/app-auth，租户端登录同走此处；token 双用于平台与本系统 /app-api/v1） ----

// 应用用户登录响应：令牌对 + 用户租户归属（平台租户 ID，与商户租户集的交集视图）。
// tenant_ids 用于登录后选择当前租户（本系统 /app-api/v1 的 X-Tenant-ID 需先于 profile 确定）
export interface AppLoginResult {
  access_token: string
  refresh_token: string
  expires_at: string
  user?: { id: number; username: string; tenant_ids: number[] }
}

// 应用用户登录（无验证码、无设备端会话概念）
export function platformAppLogin(username: string, password: string, acceptLanguage: string) {
  return call<AppLoginResult>('/api/v1/app-auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  }, acceptLanguage)
}

// 应用用户刷新（refresh_token 放 body——跨域 cookie 不可用）。
// 注意：平台 /app-auth 无 logout 端点——登出仅本地清态，会话靠平台侧自然过期
export function platformAppRefresh(refreshToken: string, acceptLanguage: string) {
  return call<TokenPair>('/api/v1/app-auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  }, acceptLanguage)
}
