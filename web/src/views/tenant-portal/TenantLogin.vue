<template>
  <div class="tenant-login">
    <main class="form-panel">
      <div class="form-box">
        <div class="form-head">
          <div class="seal">T</div>
          <div>
            <h2 class="form-title">{{ t('tenantPortal.login.title') }}</h2>
            <p class="form-sub">{{ t('tenantPortal.login.sub') }}</p>
          </div>
        </div>

        <n-form ref="formRef" :model="form" :rules="rules" :show-label="false">
          <n-form-item path="username">
            <n-input v-model:value="form.username" size="large" :placeholder="t('tenantPortal.login.username')" @keyup.enter="onLogin" />
          </n-form-item>
          <n-form-item path="password">
            <n-input v-model:value="form.password" size="large" type="password" show-password-on="click" :placeholder="t('tenantPortal.login.password')" @keyup.enter="onLogin" />
          </n-form-item>
        </n-form>
        <n-button class="login-btn" type="primary" size="large" block :loading="loading" @click="onLogin">
          {{ t('tenantPortal.login.button') }}
        </n-button>
        <p class="entry-hint mono">{{ t('tenantPortal.login.adminHint') }}</p>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { NButton, NForm, NFormItem, NInput, useMessage, type FormInst, type FormRules } from 'naive-ui'
import { useTenantUserStore } from '../../stores/tenantUser'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const store = useTenantUserStore()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const form = reactive({ username: '', password: '' })
const rules: FormRules = {
  username: { required: true, message: t('tenantPortal.login.usernameRequired'), trigger: 'blur' },
  password: { required: true, message: t('tenantPortal.login.passwordRequired'), trigger: 'blur' },
}

async function onLogin() {
  if (loading.value) return
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    // 登录直调平台 /app-auth（token 双用于平台与本系统 /app-api/v1）；
    // 返回租户归属集用于自举当前租户（X-Tenant-ID 需先于 profile 确定）
    const tenantIds = await store.login(form.username.trim(), form.password)
    if (!tenantIds.length || !store.tenantId) {
      store.clearAuth()
      message.error(t('tenantPortal.login.noTenant'))
      return
    }
    // 平台登录成功 ≠ 可进入租户端：加载上下文验证本地租户同步状态与角色
    try {
      await store.loadContext()
    } catch (e: any) {
      store.clearAuth()
      // 403：token 有效但选定租户未同步/未启用（本地闸门拒绝）
      if (e?.response?.status === 403) {
        message.error(t('tenantPortal.login.tenantNotAccessible'))
      } else {
        message.error(t('tenantPortal.login.contextFailed'))
      }
      return
    }
    message.success(t('tenantPortal.login.success'))
    router.push('/tenant/devices')
  } catch (e: any) {
    const networkFailed = !e?.response && /fetch/i.test(e?.message || '')
    if (networkFailed) {
      message.error(t('tenantPortal.login.platformUnreachable'))
    } else {
      message.error(e?.message || t('tenantPortal.login.failed'))
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (route.query.reason === 'expired') {
    message.warning(t('tenantPortal.login.sessionExpired'))
  }
  if (route.query.reason === 'noTenant') {
    message.warning(t('tenantPortal.login.noTenant'))
  }
})
</script>

<style scoped>
.tenant-login {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--sx-bg);
}
.form-box {
  width: 100%;
  max-width: 380px;
  padding: 40px 32px;
  border: 1px solid var(--sx-line);
  border-radius: var(--sx-radius, 10px);
  background: var(--sx-panel);
}
.form-head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 28px;
}
.seal {
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
  font-family: var(--sx-font-mono);
  font-weight: 700;
  font-size: 21px;
  color: #fff;
  background: var(--sx-accent);
}
.form-title {
  margin: 0;
  font-size: 20px;
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
.entry-hint {
  margin: 20px 0 0;
  font-family: var(--sx-font-mono);
  font-size: 11px;
  color: var(--sx-muted);
  text-align: center;
}
</style>
