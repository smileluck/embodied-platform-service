import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '../stores/user'
import { setupDynamicRoutes } from './dynamic'
import { i18n } from '../locales'

// 静态路由：登录页、错误页、主布局壳
// 注意：404 兜底不能静态注册——刷新深链接时动态菜单路由尚未注册，静态 catch-all 会把
// 原始路径吞掉显示 404。兜底在动态路由注册完成后于 ./dynamic.ts 中补充（渲染 404 页）。
// 前端自有路由用 meta.titleKey（i18n key，随语言切换重译）；菜单路由用 meta.title（后端按语言返回的名称）
const staticRoutes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('../views/login/Login.vue'), meta: { titleKey: 'menu.login' } },
  { path: '/404', name: 'not-found-page', component: () => import('../views/error/NotFound.vue'), meta: { titleKey: 'menu.notFound' } },
  { path: '/500', name: 'server-error-page', component: () => import('../views/error/ServerError.vue'), meta: { titleKey: 'menu.serverError' } },
  {
    path: '/',
    name: 'layout-root',
    component: () => import('../layout/AdminLayout.vue'),
    children: [], // 动态菜单路由运行时注入（菜单 path 为绝对路径），见 ./dynamic.ts
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes: staticRoutes,
})

// 无需登录即可访问的路径（错误页允许直接访问排查问题）
const PUBLIC_PATHS = new Set(['/login', '/404', '/500'])

const loginRedirect = () => ({ path: '/login', query: { reason: 'expired' }, replace: true })

// 路由守卫（return 风格）：未登录跳登录页；登录后按菜单动态注册路由。
// 用返回值而非 next()——导航被 401 拦截器的新导航取消时，返回值会被安全忽略，
// 不会像"对已取消导航调 next()"那样抛异常导致白屏。
router.beforeEach(async (to) => {
  const userStore = useUserStore()
  if (PUBLIC_PATHS.has(to.path)) {
    return true
  }
  if (!userStore.accessToken) {
    return loginRedirect()
  }
  if (!userStore.routesLoaded) {
    try {
      const firstPath = await setupDynamicRoutes()
      // 路由注册完成，重导航重新匹配（to 在导航开始时解析，深链接此前无匹配）；
      // '/' 落到第一个菜单
      const target = to.path === '/' ? firstPath : to.path
      return { path: target, query: to.query, replace: true }
    } catch {
      // token/会话失效（过期、被顶替、被下线、JWT 密钥更换）：
      // 轻量清态（不发网络请求），携带原因回登录页
      userStore.clearAuth()
      return loginRedirect()
    }
  }
  return true
})

router.afterEach((to) => {
  const { t } = i18n.global
  const titleKey = to.meta?.titleKey as string | undefined
  const title = (titleKey ? t(titleKey) : (to.meta?.title as string)) || ''
  document.title = title ? `${title} - SmileX Admin` : 'SmileX Admin'
})

export default router
