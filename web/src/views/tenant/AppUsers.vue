<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="app-users" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('appUser.keywordPlaceholder')" clearable style="width: 180px" @keyup.enter="load" />
    <n-input v-model:value="query.phone" :placeholder="t('appUser.phonePlaceholder')" clearable style="width: 160px" @keyup.enter="load" />
    <n-select v-model:value="query.status" :options="statusOptions" clearable :placeholder="t('appUser.statusPlaceholder')" style="width: 120px" />
    <n-select v-model:value="query.tenant_id" :options="tenantOptions" clearable filterable :placeholder="t('appUser.tenantPlaceholder')" style="width: 180px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate" v-permission="['appUser:create']">{{ t('appUser.newAppUser') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑（username 创建后不可修改；tenant_ids 全量替换） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('appUser.editAppUser') : t('appUser.newAppUser')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('appUser.username')" path="username">
        <n-input v-model:value="form.username" :maxlength="64" :disabled="editing" :placeholder="t('appUser.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item v-if="!editing" :label="t('appUser.password')" path="password">
        <n-input v-model:value="form.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('appUser.passwordPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('appUser.nickname')" path="nickname">
        <n-input v-model:value="form.nickname" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('appUser.phone')" path="phone">
        <n-input v-model:value="form.phone" :maxlength="32" />
      </n-form-item>
      <n-form-item :label="t('appUser.email')" path="email">
        <n-input v-model:value="form.email" :maxlength="128" />
      </n-form-item>
      <n-form-item :label="t('appUser.tenants')" path="tenant_ids">
        <n-select v-model:value="form.tenant_ids" :options="tenantOptions" multiple filterable clearable :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 重置密码：管理员指定新密码，旧密码立即失效 -->
  <n-modal v-model:show="showReset" preset="dialog" :title="t('appUser.resetPasswordTitle')" style="width: 420px">
    <n-alert type="warning" :show-icon="true" style="margin-bottom: 12px">{{ t('appUser.resetConfirmContent', { name: resetTarget?.username }) }}</n-alert>
    <n-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-placement="left" label-width="80">
      <n-form-item :label="t('appUser.newPassword')" path="password">
        <n-input v-model:value="resetForm.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('appUser.newPasswordPlaceholder')" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showReset = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="doResetPassword">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NAlert, NCard, NInput, NButton, NDataTable, NModal, NForm, NFormItem, NSelect, NTag, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createAppUser, deleteAppUser, listAppUsers, listTenants, resetAppUserPassword, setAppUserStatus, updateAppUser } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { AppUser } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<AppUser[]>([])
const query = reactive({ kw: '', phone: '', status: null as number | null, tenant_id: null as number | null, page: 1, page_size: 10 })

// 租户下拉选项（启用状态租户，页面挂载时加载一次，筛选/表单共用）
const tenantOptions = ref<{ label: string; value: number }[]>([])

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({ username: '', password: '', nickname: '', phone: '', email: '', tenant_ids: [] as number[] })
const formRef = ref<FormInst | null>(null)

const showReset = ref(false)
const resetTarget = ref<AppUser | null>(null)
const resetForm = reactive({ password: '' })
const resetFormRef = ref<FormInst | null>(null)

// 应用用户状态：1 启用 0 禁用
const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 0 },
])

const rules = computed<FormRules>(() => ({
  username: [
    { required: true, message: t('appUser.form.usernameRequired'), trigger: ['blur', 'input'] },
    { min: 3, max: 64, message: t('appUser.form.usernameLength'), trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: t('appUser.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('appUser.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
  email: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
      message: t('appUser.form.emailInvalid'),
    },
  ],
}))

const resetRules = computed<FormRules>(() => ({
  password: [
    { required: true, message: t('appUser.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('appUser.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
}))

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listAppUsers({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
      phone: query.phone || undefined,
      status: query.status ?? undefined,
      tenant_id: query.tenant_id ?? undefined,
    })
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

async function loadTenantOptions() {
  try {
    const { data } = await listTenants({ page: 1, page_size: 100, status: 1 })
    tenantOptions.value = data.data.list.map((x) => ({ label: x.name, value: x.id }))
  } catch {
    // 下拉选项加载失败不阻塞页面，列表照常可用
  }
}

function resetQuery() {
  query.kw = ''
  query.phone = ''
  query.status = null
  query.tenant_id = null
  query.page = 1
  load()
}

function openCreate() {
  editing.value = false
  Object.assign(form, { username: '', password: '', nickname: '', phone: '', email: '', tenant_ids: [] })
  showModal.value = true
}

function openEdit(row: AppUser) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { username: row.username, password: '', nickname: row.nickname, phone: row.phone, email: row.email, tenant_ids: [...row.tenant_ids] })
  showModal.value = true
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
      await updateAppUser(editId.value, {
        nickname: form.nickname.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
        tenant_ids: form.tenant_ids,
      })
    } else {
      await createAppUser({
        username: form.username.trim(),
        password: form.password,
        nickname: form.nickname.trim() || undefined,
        phone: form.phone.trim() || undefined,
        email: form.email.trim() || undefined,
        tenant_ids: form.tenant_ids.length ? form.tenant_ids : undefined,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('appUser.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openReset(row: AppUser) {
  resetTarget.value = row
  resetForm.password = ''
  showReset.value = true
}

async function doResetPassword() {
  try {
    await resetFormRef.value?.validate()
  } catch {
    return
  }
  if (!resetTarget.value) return
  saving.value = true
  try {
    await resetAppUserPassword(resetTarget.value.id, resetForm.password)
    message.success(t('common.saveSuccess'))
    showReset.value = false
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('appUser.resetFailed'))
  } finally {
    saving.value = false
  }
}

// 状态切换走更新接口（PUT /app-users/:id {status}），与编辑共用 appUser:update 权限
async function toggleStatus(row: AppUser) {
  try {
    await setAppUserStatus(row.id, row.status === 1 ? 0 : 1)
    message.success(t('appUser.statusUpdated'))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('appUser.statusFailed'))
  }
}

function confirmDelete(row: AppUser) {
  dialog.warning({
    title: t('appUser.deleteConfirmTitle'),
    content: t('appUser.deleteConfirmContent', { name: row.username }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteAppUser(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('appUser.deleteFailed'))
      }
    },
  })
}

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<AppUser>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('appUser.username'), key: 'username', width: 140 },
  { title: t('appUser.nickname'), key: 'nickname', width: 120, render: (row) => row.nickname || '—' },
  { title: t('appUser.phone'), key: 'phone', width: 130, render: (row) => row.phone || '—' },
  {
    title: t('appUser.tenants'), key: 'tenant_names', width: 200,
    render: (row) => row.tenant_names.length
      ? h('span', { style: 'display:inline-flex;gap:4px;flex-wrap:wrap' }, row.tenant_names.map((n) => h(NTag, { size: 'small', bordered: false }, { default: () => n })))
      : '—',
  },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 260,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('appUser:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('appUser:resetPwd')) {
        actions.push({ label: t('appUser.resetPassword'), onClick: () => openReset(row) })
      }
      if (userStore.has('appUser:update')) {
        actions.push({ label: row.status === 1 ? t('common.disable') : t('common.enable'), onClick: () => toggleStatus(row) })
      }
      if (userStore.has('appUser:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(() => {
  load()
  loadTenantOptions()
})
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
