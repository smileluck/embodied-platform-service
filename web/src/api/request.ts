import axios from 'axios'
import { useUserStore } from '../stores/user'
import router from '../router'
import { getLocale } from '../locales'
import { deepTrim } from '../utils/trim'

const request = axios.create({ baseURL: '/api/v1', timeout: 15000 })

// 请求拦截：附带 token 与当前语言（后端按 Accept-Language 返回本地化 msg/菜单名）；
// 提交文本统一深度去首尾空格（密码类字段与文件除外，见 utils/trim）
request.interceptors.request.use((config) => {
  const userStore = useUserStore()
  if (userStore.accessToken) {
    config.headers.Authorization = `Bearer ${userStore.accessToken}`
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

// 响应拦截：401 时尝试刷新 token 后重放请求
let refreshing: Promise<string | null> | null = null

request.interceptors.response.use(
  (resp) => resp,
  async (error) => {
    const { response, config } = error
    // 认证端点自身的 401 不进刷新流程：refresh 请求若也走这里会形成
    // refreshing 单飞 promise 的自等待死锁（await 等待包含自身的调用链），
    // 路由守卫因此永久挂起 —— 这就是 token 过期白屏的根因
    const isAuthEndpoint = typeof config?.url === 'string' && /\/auth\/(refresh|login|logout)$/.test(config.url)
    if (response?.status === 401 && !config._retried && !isAuthEndpoint) {
      config._retried = true
      refreshing ||= useUserStore().refresh().finally(() => (refreshing = null))
      const token = await refreshing
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
        return request(config)
      }
      // 刷新失败：会话已失效（被顶替/被下线/过期），轻量清态并回登录页携带原因。
      // clearAuth 不发 logout 请求，避免过期场景下二次 401 竞态；
      // 若正处于路由初始化（守卫 await 中），守卫会兜底返回登录页，这里幂等跳过
      useUserStore().clearAuth()
      if (router.currentRoute.value.path !== '/login') {
        router.replace({ path: '/login', query: { reason: 'expired' } })
      }
    }
    return Promise.reject(error)
  },
)

export default request
