import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './locales'
import { permissionDirective } from './directives/permission'

const app = createApp(App)
app.use(createPinia())
app.use(i18n)
app.use(router)
app.directive('permission', permissionDirective)

// 首次导航就绪（含动态菜单注册、首个页面 chunk 解析）后再挂载：
// 挂载前 index.html 的 #app-loading 全屏动画全程可见，
// 消除两段白屏空档（bundle 加载期 + 挂载后等路由期）。
// 10s 兜底：路由异常卡死时也强制挂载，避免永远停在加载动画上。
void Promise.race([
  router.isReady().catch(() => {}),
  new Promise((resolve) => setTimeout(resolve, 10000)),
]).then(() => {
  app.mount('#app')
  // 淡出静态加载层后移除节点
  const boot = document.getElementById('app-loading')
  if (boot) {
    boot.classList.add('is-done')
    setTimeout(() => boot.remove(), 400)
  }
})
