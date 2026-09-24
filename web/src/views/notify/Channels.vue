<template>
  <!-- 通知渠道：邮件 SMTP / Webhook，密钥密文存储（编辑留空=保持原值） -->
  <SearchCard storage-key="notify-channels" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('notify.channel.kwPlaceholder')" clearable style="width: 200px" @keyup.enter="load" />
    <n-select v-model:value="query.type" :options="typeFilterOptions" :placeholder="t('notify.channel.allTypes')" clearable style="width: 140px" />
    <n-select v-model:value="query.status" :options="statusFilterOptions" :placeholder="t('notify.channel.allStatus')" clearable style="width: 130px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['notify:channel:create']" type="primary" ghost @click="openCreate">
          {{ t('notify.channel.title') }} · {{ t('common.add') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :bordered="false" />
  </n-card>

  <n-modal v-model:show="showModal" preset="card" :title="editing ? t('common.edit') : t('common.add')" style="width: 640px" :bordered="false" size="small">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="130">
      <n-form-item :label="t('notify.channel.type')">
        <n-radio-group v-model:value="form.type" size="small" :disabled="editing">
          <n-radio-button value="email">{{ t('notify.channel.typeEmail') }}</n-radio-button>
          <n-radio-button value="webhook">{{ t('notify.channel.typeWebhook') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item :label="t('notify.channel.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-count />
      </n-form-item>
      <n-form-item :label="t('notify.channel.status')">
        <n-switch v-model:value="form.status" :checked-value="1" :unchecked-value="0" />
      </n-form-item>

      <template v-if="form.type === 'email'">
        <n-form-item :label="t('notify.channel.smtpHost')" path="smtpHost">
          <n-input v-model:value="form.smtpHost" :maxlength="128" :placeholder="t('notify.channel.smtpHost')" style="width: 60%" />
          <n-input-number v-model:value="form.smtpPort" :min="1" :max="65535" :show-button="false" style="width: 35%; margin-left: 5%" :placeholder="t('notify.channel.smtpPort')" />
        </n-form-item>
        <n-form-item :label="t('notify.channel.smtpUser')">
          <n-input v-model:value="form.smtpUser" :maxlength="128" />
        </n-form-item>
        <n-form-item :label="t('notify.channel.smtpPassword')" path="smtpPassword">
          <n-input v-model:value="form.smtpPassword" type="password" show-password-on="click" :maxlength="128" :placeholder="editing && form.smtpPasswordMask ? t('notify.channel.keepPassword') : ''" />
          <template v-if="editing && form.smtpPasswordMask" #feedback>{{ form.smtpPasswordMask }}</template>
        </n-form-item>
        <n-form-item :label="t('notify.channel.smtpFrom')" path="smtpFrom">
          <n-input v-model:value="form.smtpFrom" :maxlength="128" />
        </n-form-item>
        <n-form-item :label="t('notify.channel.recipients')" path="recipients">
          <n-input v-model:value="form.recipientsText" type="textarea" :rows="3" :placeholder="t('notify.channel.recipients')" />
        </n-form-item>
      </template>

      <template v-else>
        <n-form-item :label="t('notify.channel.webhookUrl')" path="webhookUrl">
          <n-input v-model:value="form.webhookUrl" :maxlength="512" placeholder="https://example.com/hook" />
        </n-form-item>
        <n-form-item :label="t('notify.channel.webhookSecret')">
          <n-input v-model:value="form.webhookSecret" type="password" show-password-on="click" :maxlength="128" :placeholder="editing && form.webhookSecretMask ? t('notify.channel.keepSecret') : ''" />
          <template v-if="editing && form.webhookSecretMask" #feedback>{{ form.webhookSecretMask }}</template>
        </n-form-item>
        <div class="secret-tip">{{ t('notify.channel.secretTip') }}</div>
      </template>
    </n-form>
    <template #action>
      <div class="modal-actions">
        <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NForm, NFormItem, NInput, NInputNumber, NModal, NSelect,
  NRadioButton, NRadioGroup, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { useUserStore } from '../../stores/user'
import { createNotifyChannel, deleteNotifyChannel, listNotifyChannels, testNotifyChannel, updateNotifyChannel } from '../../api'
import type { NotifyChannel } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const rows = ref<NotifyChannel[]>([])
const loading = ref(false)

// 搜索筛选：名称关键词全模糊 + 类型/状态精确；null = 不筛（n-select 清空归 null）
const query = reactive<{ kw: string; type: string | null; status: number | null }>({ kw: '', type: null, status: null })
const typeFilterOptions = computed(() => [
  { label: t('notify.channel.typeEmail'), value: 'email' },
  { label: t('notify.channel.typeWebhook'), value: 'webhook' },
])
const statusFilterOptions = computed(() => [
  { label: t('notify.channel.enabled'), value: 1 },
  { label: t('notify.channel.disabled'), value: 0 },
])

function resetQuery() {
  Object.assign(query, { kw: '', type: null, status: null })
  load()
}

const columns = computed<DataTableColumns<NotifyChannel>>(() => [
  { title: t('notify.channel.name'), key: 'name', width: 140, ellipsis: { tooltip: true } },
  {
    title: t('notify.channel.type'), key: 'type', width: 90,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.type === 'email' ? 'info' : 'success' },
      { default: () => row.type === 'email' ? t('notify.channel.typeEmail') : t('notify.channel.typeWebhook') }),
  },
  {
    title: t('notify.channel.config'), key: 'config', minWidth: 220, ellipsis: { tooltip: true },
    render: (row) => {
      if (row.type === 'email') {
        return `${row.smtp_host}:${row.smtp_port} → ${(row.recipients ?? []).join(', ')}`
      }
      return row.webhook_url ?? ''
    },
  },
  {
    title: t('notify.channel.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 1 ? 'success' : 'default' },
      { default: () => row.status === 1 ? t('notify.channel.enabled') : t('notify.channel.disabled') }),
  },
  { title: t('common.updateTime'), key: 'updated_at', width: 150, render: (row) => row.updated_at?.slice(0, 19).replace('T', ' ') ?? '—' },
  {
    title: t('common.actions'), key: 'actions', width: 200,
    render: (row) => {
      const actions: { label: string; onClick: () => void; danger?: boolean }[] = []
      if (userStore.has('notify:channel:test')) actions.push({ label: testingId.value === row.id ? t('notify.channel.testing') : t('notify.channel.test'), onClick: () => testRow(row) })
      if (userStore.has('notify:channel:update')) actions.push({ label: t('common.edit'), onClick: () => openEdit(row) })
      if (userStore.has('notify:channel:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listNotifyChannels({
      kw: query.kw.trim() || undefined, type: query.type ?? undefined, status: query.status ?? undefined,
    })
    rows.value = res.data.data
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    loading.value = false
  }
}

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const testingId = ref(0)
const formRef = ref<FormInst>()
const form = reactive({
  type: 'email' as 'email' | 'webhook',
  name: '',
  status: 1,
  smtpHost: '',
  smtpPort: 465,
  smtpUser: '',
  smtpPassword: '',
  smtpPasswordMask: '',
  smtpFrom: '',
  recipientsText: '',
  webhookUrl: '',
  webhookSecret: '',
  webhookSecretMask: '',
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('notify.channel.nameRequired'), trigger: ['blur', 'change'] }],
  smtpHost: form.type === 'email' ? [{ required: true, message: t('notify.channel.hostRequired'), trigger: ['blur', 'change'] }] : [],
  smtpFrom: form.type === 'email' ? [{ required: true, message: t('notify.channel.fromRequired'), trigger: ['blur', 'change'] }] : [],
  webhookUrl: form.type === 'webhook' ? [{ required: true, message: t('notify.channel.urlRequired'), trigger: ['blur', 'change'] }] : [],
}))

function openCreate() {
  editing.value = false
  Object.assign(form, {
    type: 'email', name: '', status: 1, smtpHost: '', smtpPort: 465, smtpUser: '', smtpPassword: '',
    smtpPasswordMask: '', smtpFrom: '', recipientsText: '', webhookUrl: '', webhookSecret: '', webhookSecretMask: '',
  })
  showModal.value = true
}

function openEdit(row: NotifyChannel) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, {
    type: row.type,
    name: row.name,
    status: row.status,
    smtpHost: row.smtp_host ?? '',
    smtpPort: row.smtp_port || 465,
    smtpUser: row.smtp_user ?? '',
    smtpPassword: '',
    smtpPasswordMask: row.smtp_password_mask ?? '',
    smtpFrom: row.smtp_from ?? '',
    recipientsText: (row.recipients ?? []).join('\n'),
    webhookUrl: row.webhook_url ?? '',
    webhookSecret: '',
    webhookSecretMask: row.webhook_secret_mask ?? '',
  })
  showModal.value = true
}

function parseRecipients(): string[] {
  return form.recipientsText.split(/[\n,，;；]/).map((s) => s.trim()).filter(Boolean)
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  const isEmail = form.type === 'email'
  if (isEmail && parseRecipients().length === 0) {
    message.warning(t('notify.channel.recipientsRequired'))
    return
  }
  saving.value = true
  try {
    const data: Record<string, unknown> = {
      name: form.name.trim(), type: form.type, status: form.status,
      smtp_host: form.smtpHost.trim(), smtp_port: form.smtpPort, smtp_user: form.smtpUser.trim(),
      smtp_password: form.smtpPassword.trim(), smtp_from: form.smtpFrom.trim(),
      recipients: isEmail ? parseRecipients() : [],
      webhook_url: form.webhookUrl.trim(), webhook_secret: form.webhookSecret.trim(),
    }
    if (editing.value) {
      await updateNotifyChannel(editId.value, data)
    } else {
      await createNotifyChannel(data)
    }
    message.success(t('common.success'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    saving.value = false
  }
}

async function testRow(row: NotifyChannel) {
  testingId.value = row.id
  try {
    const res = await testNotifyChannel(row.id)
    const rec = res.data.data
    if (rec.status === 'sent') {
      message.success(t('notify.channel.testOk', { ms: rec.duration_ms }))
    } else {
      dialog.error({ title: t('notify.channel.testFailTitle'), content: rec.error || '-' })
    }
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    testingId.value = 0
  }
}

function confirmDelete(row: NotifyChannel) {
  dialog.warning({
    title: t('common.tips'),
    content: t('notify.channel.deleteConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteNotifyChannel(row.id)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

onMounted(load)
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.secret-tip {
  margin: -8px 0 8px 130px;
  font-size: 12px;
  opacity: 0.6;
  line-height: 1.5;
}
</style>
