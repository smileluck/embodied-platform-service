<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="merchants" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.name" :placeholder="t('merchant.name')" clearable style="width: 180px" @keyup.enter="load" />
    <n-input v-model:value="query.code" :placeholder="t('merchant.code')" clearable style="width: 160px" @keyup.enter="load" />
    <n-input v-model:value="query.app_key" :placeholder="t('merchant.appKey')" clearable style="width: 180px" @keyup.enter="load" />
    <n-select v-model:value="query.status" :options="statusOptions" clearable :placeholder="t('merchant.statusPlaceholder')" style="width: 120px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate" v-permission="['merchant:create']">{{ t('merchant.newMerchant') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑 -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('merchant.editMerchant') : t('merchant.newMerchant')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('merchant.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="64" :placeholder="t('merchant.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('merchant.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" :disabled="editing" :placeholder="t('merchant.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('merchant.contactName')" path="contact_name">
        <n-input v-model:value="form.contact_name" :maxlength="64" :placeholder="editing ? t('merchant.contactKeepPlaceholder') : ''" />
      </n-form-item>
      <n-form-item :label="t('merchant.contactPhone')" path="contact_phone">
        <n-input v-model:value="form.contact_phone" :maxlength="32" :placeholder="editing ? t('merchant.contactKeepPlaceholder') : ''" />
      </n-form-item>
      <n-form-item :label="t('merchant.contactEmail')" path="contact_email">
        <n-input v-model:value="form.contact_email" :maxlength="128" :placeholder="editing ? t('merchant.contactKeepPlaceholder') : ''" />
      </n-form-item>
      <n-form-item :label="t('common.remark')" path="remark">
        <n-input v-model:value="form.remark" type="textarea" :maxlength="255" :autosize="{ minRows: 2, maxRows: 4 }" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 一次性密钥展示：创建成功 / 重置成功后弹出，关闭后无法再次查看 -->
  <n-modal v-model:show="showSecret" preset="dialog" :title="t('merchant.secretModalTitle')" style="width: 480px">
    <n-alert type="warning" :show-icon="true" style="margin-bottom: 12px">{{ t('merchant.secretWarning') }}</n-alert>
    <div class="secret-row">
      <span class="secret-label">{{ t('merchant.appKey') }}</span>
      <n-input :value="secretInfo.app_key" readonly />
      <n-button size="small" @click="copyText(secretInfo.app_key)">{{ t('merchant.copy') }}</n-button>
    </div>
    <div class="secret-row">
      <span class="secret-label">{{ t('merchant.appSecret') }}</span>
      <n-input :value="secretInfo.app_secret" readonly />
      <n-button size="small" @click="copyText(secretInfo.app_secret)">{{ t('merchant.copy') }}</n-button>
    </div>
    <template #action>
      <n-button type="primary" @click="showSecret = false">{{ t('common.close') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NAlert, NCard, NInput, NButton, NDataTable, NModal, NForm, NFormItem, NSelect, NTag, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createMerchant, deleteMerchant, listMerchants, resetMerchantSecret, setMerchantStatus, updateMerchant } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { Merchant } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<Merchant[]>([])
const query = reactive({ name: '', code: '', app_key: '', status: null as number | null, page: 1, page_size: 10 })

const showModal = ref(false)
const showSecret = ref(false)
const editing = ref(false)
const editId = ref(0)
const secretInfo = reactive({ app_key: '', app_secret: '' })
const form = reactive({ name: '', code: '', contact_name: '', contact_phone: '', contact_email: '', remark: '' })
const formRef = ref<FormInst | null>(null)

const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 2 },
])

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('merchant.form.nameRequired'), trigger: ['blur', 'input'] }],
  code: [{ required: true, message: t('merchant.form.codeRequired'), trigger: ['blur', 'input'] }],
  contact_email: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
      message: t('merchant.form.emailInvalid'),
    },
  ],
}))

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listMerchants({
      page: query.page,
      page_size: query.page_size,
      name: query.name || undefined,
      code: query.code || undefined,
      app_key: query.app_key || undefined,
      status: query.status ?? undefined,
    })
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.name = ''
  query.code = ''
  query.app_key = ''
  query.status = null
  query.page = 1
  load()
}

function openCreate() {
  editing.value = false
  Object.assign(form, { name: '', code: '', contact_name: '', contact_phone: '', contact_email: '', remark: '' })
  showModal.value = true
}

// 联系人字段后端已脱敏，编辑态留空表示不修改（不可将脱敏值原样回传）
function openEdit(row: Merchant) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { name: row.name, code: row.code, contact_name: '', contact_phone: '', contact_email: '', remark: row.remark })
  showModal.value = true
}

function openSecretModal(appKey: string, appSecret: string) {
  secretInfo.app_key = appKey
  secretInfo.app_secret = appSecret
  showSecret.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return // 校验失败，错误已在表单项上展示
  }
  saving.value = true
  try {
    if (editing.value) {
      const payload: { name: string; contact_name?: string; contact_phone?: string; contact_email?: string; remark?: string } = {
        name: form.name.trim(),
        remark: form.remark.trim(),
      }
      if (form.contact_name.trim()) payload.contact_name = form.contact_name.trim()
      if (form.contact_phone.trim()) payload.contact_phone = form.contact_phone.trim()
      if (form.contact_email.trim()) payload.contact_email = form.contact_email.trim()
      await updateMerchant(editId.value, payload)
      message.success(t('common.saveSuccess'))
    } else {
      const { data } = await createMerchant({
        name: form.name.trim(),
        code: form.code.trim(),
        contact_name: form.contact_name.trim() || undefined,
        contact_phone: form.contact_phone.trim() || undefined,
        contact_email: form.contact_email.trim() || undefined,
        remark: form.remark.trim() || undefined,
      })
      message.success(t('common.saveSuccess'))
      // 密钥仅此一次明文返回，立即弹出展示
      openSecretModal(data.data.app_key, data.data.app_secret)
    }
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('merchant.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmResetSecret(row: Merchant) {
  dialog.warning({
    title: t('merchant.resetSecretConfirmTitle'),
    content: t('merchant.resetSecretConfirmContent', { name: row.name }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const { data } = await resetMerchantSecret(row.id)
        openSecretModal(data.data.app_key, data.data.app_secret)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('merchant.resetFailed'))
      }
    },
  })
}

async function toggleStatus(row: Merchant) {
  try {
    await setMerchantStatus(row.id, row.status === 1 ? 2 : 1)
    message.success(t('merchant.statusUpdated'))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('merchant.statusFailed'))
  }
}

function confirmDelete(row: Merchant) {
  dialog.warning({
    title: t('merchant.deleteConfirmTitle'),
    content: t('merchant.deleteConfirmContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteMerchant(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('merchant.deleteFailed'))
      }
    },
  })
}

// 复制到剪贴板：优先 navigator.clipboard，降级 textarea 方案（非安全上下文/权限被拒时）
async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  message.success(t('merchant.copySuccess'))
}

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<Merchant>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('merchant.name'), key: 'name' },
  { title: t('merchant.code'), key: 'code', width: 140 },
  { title: t('merchant.appKey'), key: 'app_key', width: 200 },
  {
    title: t('merchant.contact'), key: 'contact_name', width: 180,
    render: (row) => [row.contact_name, row.contact_phone].filter(Boolean).join(' / ') || '—',
  },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 220,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('merchant:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('merchant:resetSecret')) {
        actions.push({ label: t('merchant.resetSecret'), onClick: () => confirmResetSecret(row) })
      }
      if (userStore.has('merchant:status')) {
        actions.push({ label: row.status === 1 ? t('common.disable') : t('common.enable'), onClick: () => toggleStatus(row) })
      }
      if (userStore.has('merchant:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(load)
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.secret-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.secret-label {
  flex-shrink: 0;
  width: 80px;
  font-size: 13px;
  color: var(--sx-muted);
}
</style>
