// 平台（embodied-platform）直调 API：登录/刷新/登出由浏览器直调平台（token 双用）。
// 平台是唯一身份源——本系统后端不碰密码，只做 token 自省 + 本地准入。
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
  let body: any = null
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

// 平台登录：设备端 web（同端互斥）；验证码开关在平台侧（captchaEnabled）
export function platformLogin(username: string, password: string, acceptLanguage: string, captchaId = '', captchaCode = '') {
  return call<TokenPair>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password, captcha_id: captchaId, captcha_code: captchaCode, device_type: 'web' }),
  }, acceptLanguage)
}

// 平台刷新：跨域 cookie 不可用，refresh_token 放 body
export function platformRefresh(refreshToken: string, acceptLanguage: string) {
  return call<TokenPair>('/api/v1/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  }, acceptLanguage)
}

// 平台登出：吊销当前会话（token 立即失效）；失败不阻断本地清态
export async function platformLogout(accessToken: string, acceptLanguage: string): Promise<void> {
  try {
    await call<null>('/api/v1/auth/logout', {
      method: 'POST',
      headers: { Authorization: `Bearer ${accessToken}` },
    }, acceptLanguage)
  } catch {
    // 忽略：本地清态不受影响
  }
}

// 平台验证码（enabled=false 表示平台侧停用；image 为 PNG base64，无 data: 前缀）
export async function platformCaptcha(acceptLanguage: string): Promise<{ captcha_id: string; captcha_image: string; enabled: boolean }> {
  return call<{ captcha_id: string; captcha_image: string; enabled: boolean }>(
    '/api/v1/auth/captcha', { method: 'GET' }, acceptLanguage,
  )
}
