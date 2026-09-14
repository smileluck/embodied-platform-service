import { h } from 'vue'
import { RouterView, type RouteRecordRaw } from 'vue-router'
import router from './index'
import { useUserStore } from '../stores/user'

// 兼容存量"有子级的菜单"（未迁移为 dir 的旧数据）：菜单本身没有页面组件时，用透传组件渲染子路由。
// 注意必须是组件对象：route.component 若是普通函数会被 vue-router 当作懒加载 loader（要求返回 Promise）。
const Passthrough = { name: 'RouterViewPassthrough', render: () => h(RouterView) }

// 菜单 code -> 页面组件 映射（新增页面在此登记）
const viewModules: Record<string, () => Promise<any>> = {
  'menu:dashboard': () => import('../views/dashboard/Dashboard.vue'),
  'menu:user': () => import('../views/system/Users.vue'),
  'menu:role': () => import('../views/system/Roles.vue'),
  'menu:menu': () => import('../views/system/Menus.vue'),
  'menu:online': () => import('../views/system/Online.vue'),
  'menu:loginLog': () => import('../views/log/LoginLogs.vue'),
  'menu:opLog': () => import('../views/log/OperationLogs.vue'),
  'menu:file': () => import('../views/file/Files.vue'),
  'menu:blacklist': () => import('../views/system/Blacklist.vue'),
  'menu:merchant': () => import('../views/openapi/Merchants.vue'),
  'menu:tenant': () => import('../views/tenant/Tenants.vue'),
  'menu:appUser': () => import('../views/tenant/AppUsers.vue'),
  'menu:merchantLog': () => import('../views/openapi/ApiLogs.vue'),
  'menu:about': () => import('../views/about/About.vue'),
}

// 将后端菜单树转换为路由
export function menuToRoutes(menus: any[]): RouteRecordRaw[] {
  const routes: RouteRecordRaw[] = []
  for (const m of menus ?? []) {
    // 目录（dir）：无页面、无路由，仅作侧栏分组；子菜单提升到当前层级注册
    //（子菜单 path 为绝对路径如 /system/users，提升后 URL 不变）
    if (m.type === 'dir') {
      routes.push(...menuToRoutes(m.children))
      continue
    }
    const comp = m.children?.length && !viewModules[m.code] ? Passthrough : viewModules[m.code]
    const route: RouteRecordRaw = {
      path: m.path,
      name: m.code,
      component: comp,
      meta: { title: m.name, icon: m.icon },
      children: [],
    }
    if (m.children?.length) {
      route.children = menuToRoutes(m.children)
    }
    routes.push(route)
  }
  return routes
}

// 登录后按后端菜单动态注册路由（挂到 layout-root 下），返回首个菜单路径
export async function setupDynamicRoutes(): Promise<string> {
  const userStore = useUserStore()
  await userStore.loadUserContext()
  const dynamic = menuToRoutes(userStore.menus)
  for (const r of dynamic) {
    router.addRoute('layout-root', r)
  }
  // 隐藏路由：个人中心（不在菜单树中，不在侧栏/全局搜索显示）
  if (!router.hasRoute('profile')) {
    router.addRoute('layout-root', {
      path: '/profile',
      name: 'profile',
      component: () => import('../views/profile/Profile.vue'),
      meta: { titleKey: 'menu.profile' },
    })
  }
  // 隐藏路由：导出记录（不在菜单树中，任何登录用户可访问）
  if (!router.hasRoute('export-records')) {
    router.addRoute('layout-root', {
      path: '/exports',
      name: 'export-records',
      component: () => import('../views/export/ExportRecords.vue'),
      meta: { titleKey: 'menu.exportRecords' },
    })
  }
  // 菜单路由注册完成后再挂 404 兜底，避免刷新深链接时原始路径被吞（见 ./index.ts）；
  // 真正的未知路径渲染 404 错误页
  if (!router.hasRoute('not-found')) {
    router.addRoute({
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/error/NotFound.vue'),
      meta: { titleKey: 'menu.notFound' },
    })
  }
  userStore.routesLoaded = true
  return dynamic.length ? dynamic[0].path : '/'
}

// 语言切换后：菜单名由后端按新语言返回，就地更新已注册菜单路由的 meta.title
//（路由记录对象在 currentRoute 的 matched 中被共享，改 meta 即可驱动面包屑/标题刷新）
export function refreshRouteTitles(menus: any[]) {
  const walk = (nodes: any[]) => {
    for (const m of nodes ?? []) {
      if (m.type !== 'dir' && m.code && router.hasRoute(m.code)) {
        const rec = router.getRoutes().find((r) => r.name === m.code)
        if (rec) rec.meta.title = m.name
      }
      if (m.children?.length) walk(m.children)
    }
  }
  walk(menus)
}
