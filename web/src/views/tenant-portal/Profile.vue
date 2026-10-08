<template>
  <n-card :title="t('tenantPortal.profile.title')">
    <n-descriptions v-if="store.user" :column="2" label-placement="left" size="small" bordered>
      <n-descriptions-item :label="t('tenantPortal.profile.username')">{{ store.user.username }}</n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.profile.nickname')">{{ store.user.nickname || '—' }}</n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.profile.tenant')">{{ store.profile?.tenant?.name || '—' }}</n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.profile.role')">
        {{ store.isAdmin ? t('tenantPortal.role.admin') : t('tenantPortal.role.member') }}
      </n-descriptions-item>
    </n-descriptions>

    <n-divider />

    <h4 class="section-title">{{ t('tenantPortal.profile.changePassword') }}</h4>
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="100" style="max-width: 420px">
      <n-form-item :label="t('tenantPortal.profile.oldPassword')" path="old_password">
        <n-input v-model:value="form.old_password" type="password" show-password-on="click" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.profile.newPassword')" path="new_password">
        <n-input v-model:value="form.new_password" type="password" show-password-on="click" :maxlength="20" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.profile.confirmPassword')" path="confirm">
        <n-input v-model:value="form.confirm" type="password" show-password-on="click" :maxlength="20" />
      </n-form-item>
      <n-button type="primary" :loading="saving" @click="submit">{{ t('common.confirm') }}</n-button>
    </n-form>
  </n-card>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { NButton, NCard, NDescriptions, NDescriptionsItem, NDivider, NForm, NFormItem, NInput, useMessage, type FormInst, type FormRules } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useTenantUserStore } from '../../stores/tenantUser'
import { changeTenantPassword } from '../../api/tenant'

const { t } = useI18n()
const message = useMessage()
const store = useTenantUserStore()

const formRef = ref<FormInst | null>(null)
const saving = ref(false)
const form = reactive({ old_password: '', new_password: '', confirm: '' })

const rules = computed<FormRules>(() => ({
  old_password: { required: true, message: t('tenantPortal.profile.form.oldRequired'), trigger: 'blur' },
  new_password: [
    { required: true, message: t('tenantPortal.profile.form.newRequired'), trigger: 'blur' },
    { min: 6, max: 20, message: t('tenantPortal.profile.form.newLength'), trigger: 'blur' },
  ],
  confirm: [
    { required: true, message: t('tenantPortal.profile.form.confirmRequired'), trigger: 'blur' },
    {
      trigger: 'blur',
      validator: (_rule, value: string) => value === form.new_password,
      message: t('tenantPortal.profile.form.confirmMismatch'),
    },
  ],
}))

// 改密经本服务持本人 token 代理平台（与 B 端个人中心同模式；旧密码由平台校验）
async function submit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    await changeTenantPassword({ old_password: form.old_password, new_password: form.new_password })
    message.success(t('tenantPortal.profile.passwordChanged'))
    form.old_password = ''
    form.new_password = ''
    form.confirm = ''
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.profile.changeFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.section-title {
  margin: 0 0 16px;
  font-size: 15px;
  color: var(--sx-ink);
}
</style>
