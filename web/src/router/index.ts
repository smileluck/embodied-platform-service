import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '../stores/user'
import { useTenantUserStore } from '../stores/tenantUser'
import { setupDynamicRoutes, firstAccessiblePath } from './dynamic'
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
  // ---- 租户端（企业租户）：独立登录页 + 独立壳，静态路由按角色过滤菜单（无后端菜单面） ----
  // 路由前缀用 /tenant-portal 与管理端「租户中心」菜单（/tenant/tenants、/tenant/app-users）
  // 物理隔离——曾用 /tenant 前缀导致管理端点「租户管理」被守卫误判为租户端而踢去登录页
  { path: '/tenant-portal/login', name: 'tenant-login', component: () => import('../views/tenant-portal/TenantLogin.vue'), meta: { titleKey: 'tenantPortal.menu.login' } },
  {
    path: '/tenant-portal',
    name: 'tenant-layout-root',
    component: () => import('../layout/TenantLayout.vue'),
    redirect: '/tenant-portal/devices',
    children: [
      { path: 'devices', name: 'tenant-devices', component: () => import('../views/tenant-portal/Devices.vue'), meta: { titleKey: 'tenantPortal.menu.devices' } },
      { path: 'devices/:id', name: 'tenant-device-detail', component: () => import('../views/tenant-portal/DeviceDetail.vue'), meta: { titleKey: 'tenantPortal.menu.deviceDetail', hideInMenu: true } },
      { path: 'members', name: 'tenant-members', component: () => import('../views/tenant-portal/Members.vue'), meta: { titleKey: 'tenantPortal.menu.members', adminOnly: true } },
      { path: 'logs', name: 'tenant-logs', component: () => import('../views/tenant-portal/Logs.vue'), meta: { titleKey: 'tenantPortal.menu.logs', perm: 'log:list' } },
      { path: 'profile', name: 'tenant-profile', component: () => import('../views/tenant-portal/Profile.vue'), meta: { titleKey: 'tenantPortal.menu.profile' } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes: staticRoutes,
})

// 无需登录即可访问的路径（错误页允许直接访问排查问题）
const PUBLIC_PATHS = new Set(['/login', '/tenant-portal/login', '/404', '/500'])

const loginRedirect = () => ({ path: '/login', query: { reason: 'expired' }, replace: true })
const tenantLoginRedirect = () => ({ path: '/tenant-portal/login', query: { reason: 'expired' }, replace: true })

// 路由守卫（return 风格）：未登录跳登录页；登录后按菜单动态注册路由。
// 用返回值而非 next()——导航被 401 拦截器的新导航取消时，返回值会被安全忽略，
// 不会像"对已取消导航调 next()"那样抛异常导致白屏。
router.beforeEach(async (to) => {
  // ---- 租户端分流：/tenant-portal/* 用租户门户 token（与管理端完全隔离；
  // 前缀与管理端 /tenant/* 菜单不重叠，见 staticRoutes 注释。
  // 2026-10-08 起门户身份=平台 tenant_users：tid 单租户绑定在 token 内，无租户选择） ----
  if (to.path.startsWith('/tenant-portal')) {
    const store = useTenantUserStore()
    if (to.path === '/tenant-portal/login') return true
    if (!store.accessToken) {
      return tenantLoginRedirect()
    }
    if (!store.contextLoaded) {
      try {
        await store.loadContext()
        return { path: to.path, query: to.query, replace: true }
      } catch {
        store.clearAuth()
        return tenantLoginRedirect()
      }
    }
    // 无成员自治权限者访问 admin-only 页面（成员管理）跳回设备列表
    if (to.meta?.adminOnly && !store.isAdmin) {
      return { path: '/tenant-portal/devices', replace: true }
    }
    // 权限码级页面（如日志 log:list）：无对应权限码跳回设备列表
    const needPerm = to.meta?.perm as string | undefined
    if (needPerm && !store.hasPerm(needPerm)) {
      return { path: '/tenant-portal/devices', replace: true }
    }
    return true
  }

  // ---- 管理端（原有守卫不变） ----
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
    } catch (e: any) {
      // 403 = 平台身份有效但本系统准入未开启（区别于 401 会话失效）；
      // 清态回登录页并提示需管理员开启准入
      if (e?.response?.status === 403) {
        userStore.clearAuth()
        return { path: '/login', query: { reason: 'notAdmitted' }, replace: true }
      }
      // token/会话失效（过期、被吊销、平台不可达）：轻量清态（不发网络请求），携带原因回登录页
      userStore.clearAuth()
      return loginRedirect()
    }
  }
  // 已加载状态下访问 '/'（404/500 页"返回首页"等）：layout-root 无自身页面，
  // 与首载分支一致地落到第一个可访问菜单，避免渲染空壳
  if (to.path === '/') {
    return { path: firstAccessiblePath(userStore.menus), replace: true }
  }
  return true
})

router.afterEach((to) => {
  const { t } = i18n.global
  const titleKey = to.meta?.titleKey as string | undefined
  const title = (titleKey ? t(titleKey) : (to.meta?.title as string)) || ''
  const brand = to.path.startsWith('/tenant-portal') ? 'Tenant Portal' : 'SmileX Admin'
  document.title = title ? `${title} - ${brand}` : brand
})

export default router
