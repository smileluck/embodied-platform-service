<template>
  <div class="login-page">
    <!-- 品牌面板：签名元素「具身单元」 -->
    <aside class="brand-panel">
      <!-- 标注版 AMR 机器人示意：右偏上留白区的动态点缀 -->
      <div class="unit-flow" aria-hidden="true">
        <div class="geo-flow-label">unit-01 · patrol</div>
        <EgoUnit />
      </div>

      <div class="brand-top">
        <div class="seal">S</div>
        <span class="brand-name">SmileX Admin</span>
      </div>

      <div class="brand-copy">
        <p class="mono-label">embodied console</p>
        <h1 class="headline">{{ t('login.brandHeadline1') }}<br />{{ t('login.brandHeadline2') }}</h1>
        <p class="sub">{{ t('login.brandSub') }}</p>
      </div>

      <!-- 遥测脉搏 -->
      <div class="pulse">
        <div class="pulse-head mono-label">telemetry pulse</div>
        <PulseWave />
        <div class="pulse-meta mono">
          <span>bat 78% · charging</span>
          <span>link mqtt · 12ms</span>
          <span class="live"><i></i>operational</span>
        </div>
      </div>

      <div class="brand-foot mono">© 2026 SmileX · internal use only</div>
    </aside>

    <!-- 表单面板 -->
    <main class="form-panel">
      <div class="form-box">
        <div class="form-head">
          <!-- 左：品牌图标 + 标题 -->
          <div class="form-head-left">
            <div class="seal seal--sm">S</div>
            <div>
              <h2 class="form-title">{{ t('login.formTitle') }}</h2>
              <p class="form-sub">{{ t('login.formSub') }}</p>
            </div>
          </div>
          <!-- 中：占位，撑开左右两块 -->
          <div class="form-head-center"></div>
          <!-- 右：语言切换，与主页面顶栏同款 -->
          <n-dropdown class="locale-switch" :options="localeOptions" @select="onLocaleChange">
            <n-button quaternary circle :focusable="false" :aria-label="t('layout.language')">
              <template #icon>
                <n-icon :component="LanguageOutline" />
              </template>
            </n-button>
          </n-dropdown>
        </div>

        <n-form ref="formRef" :model="form" :rules="rules" :show-label="false">
          <n-form-item path="username">
            <n-input v-model:value="form.username" size="large" :placeholder="t('login.username')" @keyup.enter="onLogin" />
          </n-form-item>
          <n-form-item path="password">
            <n-input v-model:value="form.password" size="large" type="password" show-password-on="click" :placeholder="t('login.password')"
              @keyup.enter="onLogin" />
          </n-form-item>
          <n-form-item v-if="captchaEnabled" path="captchaCode">
            <div class="captcha-row">
              <n-input v-model:value="form.captchaCode" size="large" :placeholder="t('login.captcha')" @keyup.enter="onLogin" />
              <img v-if="captchaImage" class="captcha-img" :src="captchaImage" :alt="t('login.captcha')" :title="t('login.captchaClickRefresh')"
                draggable="false" @click="loadCaptcha" />
              <div v-else class="captcha-img captcha-img--empty" @click="loadCaptcha">{{ t('common.refresh') }}</div>
            </div>
          </n-form-item>
        </n-form>
        <div class="form-extra">
          <n-checkbox v-model:checked="remember">{{ t('login.remember') }}</n-checkbox>
        </div>
        <n-button class="login-btn" type="primary" size="large" block :loading="loading" @click="onLogin">
          {{ t('login.loginButton') }}
        </n-button>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NForm, NFormItem, NInput, NButton, NCheckbox, NDropdown, NIcon, useMessage, type FormInst, type DropdownOption } from 'naive-ui'
import { LanguageOutline } from '@vicons/ionicons5'
import { platformCaptcha } from '../../api/platform'
import { setupDynamicRoutes } from '../../router/dynamic'
import { useUserStore } from '../../stores/user'
import { getLocale, setLocale, type AppLocale } from '../../locales'
import EgoUnit from './EgoUnit.vue'
import PulseWave from './PulseWave.vue'
const REMEMBER_KEY = 'remember_account'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const message = useMessage()
const { t } = useI18n()

// 登录页语言切换（未登录无接口依赖，仅切前端文案；登录后请求自动带上新语言）
const localeOptions: DropdownOption[] = [
  { label: '中文', key: 'zh-CN' },
  { label: 'English', key: 'en-US' },
]

function onLocaleChange(key: string | number) {
  setLocale(String(key) as AppLocale)
}
const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const form = reactive({ username: 'admin', password: '', captchaCode: '' })
// 服务端 auth.captchaEnabled=false 时隐藏验证码表单并跳过其校验
const captchaEnabled = ref(true)
const rules = computed(() => ({
  username: { required: true, message: t('login.usernameRequired'), trigger: 'blur' },
  password: { required: true, message: t('login.passwordRequired'), trigger: 'blur' },
  ...(captchaEnabled.value
    ? { captchaCode: { required: true, message: t('login.captchaRequired'), trigger: 'blur' } }
    : {}),
}))

// 验证码：id 不参与表单校验，随登录请求提交
const captchaId = ref('')
const captchaImage = ref('')
const remember = ref(false)

async function loadCaptcha() {
  form.captchaCode = ''
  try {
    const resp = await platformCaptcha(getLocale())
    // 平台停用验证码：隐藏表单，登录时提交空验证码即可
    if (resp.enabled === false) {
      captchaEnabled.value = false
      captchaId.value = ''
      captchaImage.value = ''
      return
    }
    captchaEnabled.value = true
    captchaId.value = resp.captcha_id
    captchaImage.value = resp.captcha_image
  } catch {
    captchaId.value = ''
    captchaImage.value = ''
    captchaEnabled.value = false // 平台不可达时隐藏验证码，让登录错误自己暴露问题
  }
}

// 记住密码：base64 混淆存储（可逆，仅本地便利用途；需 encodeURIComponent 避免 btoa 非 Latin1 报错）
function restoreRemembered() {
  try {
    const raw = localStorage.getItem(REMEMBER_KEY)
    if (!raw) return
    const saved = JSON.parse(decodeURIComponent(atob(raw)))
    if (saved?.username) {
      form.username = saved.username
      form.password = saved.password || ''
      remember.value = true
    }
  } catch { /* 本地数据损坏则忽略 */ }
}

function persistRemembered() {
  if (remember.value) {
    localStorage.setItem(REMEMBER_KEY, btoa(encodeURIComponent(JSON.stringify({
      username: form.username,
      password: form.password,
    }))))
  } else {
    localStorage.removeItem(REMEMBER_KEY)
  }
}

async function onLogin() {
  // 防重复提交：连续两次登录会让首次的导航因平台会话同端互斥被顶替而异常
  if (loading.value) return
  try {
    await formRef.value?.validate()
  } catch {
    return // 校验失败，表单项已提示，静默返回
  }
  loading.value = true
  try {
    // 登录直调平台（token 双用：既调本系统也直调平台）
    await userStore.login(form.username, form.password, captchaId.value, form.captchaCode)
    persistRemembered()
    // 平台登录成功 ≠ 可进入本系统：先加载本系统上下文（profile/菜单/动态路由）验证准入，
    // 403（非本商户成员/未准入）时清态留在登录页——绝不先弹「登录成功」再被拦下
    let firstPath: string
    try {
      firstPath = await setupDynamicRoutes()
    } catch (e: any) {
      userStore.clearAuth()
      if (e?.response?.status === 403) {
        message.error(t('login.notAdmitted'))
      } else {
        // 本系统后端不可用（平台身份本身有效）
        message.error(t('login.contextLoadFailed'))
      }
      loadCaptcha()
      return
    }
    message.success(t('login.loginSuccess'))
    // 路由已在此注册完成，直接进入首个菜单（守卫对 routesLoaded=true 不再转换 '/'）
    router.push(firstPath)
  } catch (e: any) {
    // 平台直调 fetch 失败（平台未启动/CORS 拒绝）：无 response 且 message 为浏览器原生 fetch 报错，
    // 显示本地化的可行动提示而非 "Failed to fetch" 原文
    const networkFailed = !e?.response && /fetch/i.test(e?.message || '')
    const msg: string = e?.response?.data?.msg || e?.message || t('login.loginFailed')
    if (networkFailed) {
      message.error(t('login.platformUnreachable'))
    } else {
      message.error(/captcha|验证码/i.test(msg) ? t('login.captchaError') : msg)
    }
    // 验证码一次性，登录失败（无论原因）后必须换新
    loadCaptcha()
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  restoreRemembered()
  loadCaptcha()
  // 会话失效被踢回登录页（平台侧吊销/被顶替/长期未活跃过期）
  if (route.query.reason === 'expired') {
    message.warning(t('login.sessionExpired'))
  }
  // 平台身份有效但本系统准入未开启：提示联系管理员开启
  if (route.query.reason === 'notAdmitted') {
    message.warning(t('login.notAdmitted'))
  }
})
</script>

<style scoped>
.login-page {
  display: flex;
  min-height: 100vh;
  background: var(--sx-bg);
}

/* ---- 品牌面板（签名区域，唯一的大胆处）---- */
.brand-panel {
  position: relative;
  flex: 7;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 40px 48px;
  background: var(--sx-ink);
  color: var(--sx-shell-text);
  overflow: hidden;
}
/* 面板内的工程网格 */
.brand-panel::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.045) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse at 30% 40%, black 20%, transparent 80%);
  pointer-events: none;
}

/* 具身单元示意：右偏上，绝对定位不参与纵向弹性分布 */
.unit-flow {
  position: absolute;
  top: 84px;
  right: 0;
  width: min(44vw, 520px);
  display: flex;
  flex-direction: column;
  pointer-events: none;
}
.geo-flow-label {
  margin: 0 48px 10px; /* 与面板右内边距对齐 */
  font-family: var(--sx-font-mono);
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgba(233, 231, 226, 0.4);
}

.brand-top {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
}
.seal {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  font-family: var(--sx-font-mono);
  font-weight: 700;
  font-size: 20px;
  color: var(--sx-ink);
  background: var(--sx-accent-bright);
}
.brand-name {
  font-family: var(--sx-font-mono);
  font-size: 14px;
  letter-spacing: 0.06em;
  color: rgba(233, 231, 226, 0.85);
}

.brand-copy {
  position: relative;
}
.mono-label {
  font-family: var(--sx-font-mono);
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--sx-accent-bright);
  margin: 0 0 16px;
}
.headline {
  font-size: clamp(28px, 3.2vw, 40px);
  font-weight: 700;
  line-height: 1.3;
  margin: 0 0 14px;
  letter-spacing: 0.01em;
}
.sub {
  margin: 0;
  font-size: 15px;
  color: rgba(233, 231, 226, 0.6);
}

/* 系统脉搏 */
.pulse {
  position: relative;
  margin-top: 48px;
}
.pulse-head {
  color: rgba(233, 231, 226, 0.5);
  margin-bottom: 10px;
}
/* 波形本体（描线/光束/转折点）见 PulseWave.vue */
.pulse-meta {
  display: flex;
  gap: 20px;
  margin-top: 12px;
  font-family: var(--sx-font-mono);
  font-size: 11px;
  color: rgba(233, 231, 226, 0.45);
}
.live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--sx-ok-bright);
}
.live i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--sx-ok-bright);
  animation: blink 2.4s ease-in-out infinite;
}
@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.25; }
}

.brand-foot {
  position: relative;
  font-size: 11px;
  color: rgba(233, 231, 226, 0.3);
}

/* ---- 表单面板（克制、安静）---- */
.form-panel {
  flex: 3;
  min-width: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
}
.form-box {
  width: 100%;
  max-width: 360px;
}
.form-head {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  margin-bottom: 32px;
}
/* 左：品牌图标 + 标题 */
.form-head-left {
  display: flex;
  align-items: center;
  gap: 14px;
}
/* 中：占位，撑满剩余空间，把语言切换顶到最右 */
.form-head-center {
  flex: 1;
}
/* 右：语言切换图标按钮，与主页面顶栏一致 */
.locale-switch {
  flex-shrink: 0;
  color: var(--sx-muted);
}
.seal--sm {
  width: 44px;
  height: 44px;
  border-radius: 11px;
  background: var(--sx-accent);
  color: #fff;
  font-size: 21px;
}
.form-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--sx-ink);
}
.form-sub {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--sx-muted);
}

.login-btn {
  margin-top: 8px;
  font-weight: 600;
  letter-spacing: 4px;
}

/* 验证码：输入框 + 图片并排，图片与 large 输入框（40px）同高 */
.captcha-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.captcha-img {
  width: 132px;
  height: 40px;
  flex-shrink: 0;
  display: block;
  object-fit: cover;
  border: 1px solid var(--sx-line);
  border-radius: var(--sx-radius);
  cursor: pointer;
  background: var(--sx-accent-soft);
  /* 禁止拖拽成幽灵图（Safari 需前缀属性，其余浏览器 draggable=false 已足够） */
  -webkit-user-drag: none;
  user-select: none;
}
.captcha-img--empty {
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--sx-font-mono);
  font-size: 11px;
  letter-spacing: 0.1em;
  color: var(--sx-muted);
}

.form-extra {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 2px 0 10px;
}

/* 中等屏：右上留白不足时收起具身单元 */
@media (max-width: 1180px) {
  .unit-flow {
    display: none;
  }
}

/* 响应式：窄屏时品牌面板退化为顶部条 */
@media (max-width: 860px) {
  .login-page {
    flex-direction: column;
  }
  .brand-panel {
    flex: none;
    padding: 28px 32px;
  }
  .brand-copy {
    display: none;
  }
  .pulse {
    display: none;
  }
  .brand-foot {
    display: none;
  }
}
</style>
