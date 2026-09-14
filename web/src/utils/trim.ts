// 请求文本统一去首尾空格：在 axios 请求拦截器对 body/params 做一次深度 trim，
// 页面层不再逐字段手工 .trim()（幂等，已 trim 的调用不受影响）
const SKIP_KEYS = new Set(['password', 'old_password', 'new_password'])
// 文件名/二进制内容不做处理；Blob/File 理论上不会出现在对象字段里，防御性排除
const SKIP_TYPES = [FormData, Blob, File]

export function deepTrim<T>(value: T): T {
  if (typeof value === 'string') return value.trim() as unknown as T
  if (Array.isArray(value)) return value.map(deepTrim) as unknown as T
  if (value && typeof value === 'object') {
    if (SKIP_TYPES.some((t) => value instanceof t)) return value
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      // 密码类字段尊重用户原始输入（首尾空格可能是密码本身的一部分，trim 会改变语义）
      out[k] = SKIP_KEYS.has(k) ? v : deepTrim(v)
    }
    return out as T
  }
  return value
}
