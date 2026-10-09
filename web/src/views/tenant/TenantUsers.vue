<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="tenant-users" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('tenantUser.keywordPlaceholder')" clearable style="width: 180px" @keyup.enter="search" />
    <n-input v-model:value="query.phone" :placeholder="t('tenantUser.phonePlaceholder')" clearable style="width: 160px" @keyup.enter="search" />
    <n-select v-model:value="query.status" :options="statusOptions" clearable :placeholder="t('tenantUser.statusPlaceholder')" style="width: 120px" />
    <n-select v-model:value="query.tenant_id" :options="tenantOptions" clearable filterable :placeholder="t('tenantUser.tenantPlaceholder')" style="width: 180px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate" v-permission="['tenantUser:create']">{{ t('tenantUser.newUser') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑（username/所属租户创建后不可修改；单租户绑定） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('tenantUser.editUser') : t('tenantUser.newUser')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="90">
      <n-form-item :label="t('tenantUser.tenant')" path="tenant_id">
        <n-select v-model:value="form.tenant_id" :options="tenantOptions" filterable :disabled="editing" :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
      <n-form-item :label="t('tenantUser.username')" path="username">
        <n-input v-model:value="form.username" :maxlength="64" :disabled="editing" :placeholder="t('tenantUser.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item v-if="!editing" :label="t('tenantUser.password')" path="password">
        <n-input v-model:value="form.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('tenantUser.passwordPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenantUser.roles')" path="role_ids">
        <n-select v-model:value="form.role_ids" :options="roleOptions" multiple filterable clearable :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
      <n-form-item :label="t('tenantUser.nickname')" path="nickname">
        <n-input v-model:value="form.nickname" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenantUser.phone')" path="phone">
        <n-input v-model:value="form.phone" :maxlength="32" />
      </n-form-item>
      <n-form-item :label="t('tenantUser.email')" path="email">
        <n-input v-model:value="form.email" :maxlength="128" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 设置角色：多选该用户归属租户下的角色（全量替换） -->
  <n-modal v-model:show="showRole" preset="dialog" :title="t('tenantUser.setRolesTitle')" style="width: 440px">
    <n-form label-placement="left" label-width="90">
      <n-form-item :label="t('tenantUser.username')">
        <span>{{ roleTarget?.username }}</span>
      </n-form-item>
      <n-form-item :label="t('tenantUser.roles')">
        <n-select v-model:value="roleForm.role_ids" :options="roleOptions" multiple filterable :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showRole = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="doSetRoles">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 重置密码：管理员指定新密码，旧密码立即失效 -->
  <n-modal v-model:show="showReset" preset="dialog" :title="t('tenantUser.resetPasswordTitle')" style="width: 420px">
    <n-alert type="warning" :show-icon="true" style="margin-bottom: 12px">{{ t('tenantUser.resetConfirmContent', { name: resetTarget?.username }) }}</n-alert>
    <n-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-placement="left" label-width="90">
      <n-form-item :label="t('tenantUser.newPassword')" path="password">
        <n-input v-model:value="resetForm.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('tenantUser.newPasswordPlaceholder')" />
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
import { createTenantUser, deleteTenantUser, listTenantRoles, listTenantUsers, listTenants, resetTenantUserPassword, setTenantUserRoles, setTenantUserStatus, updateTenantUser } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { TenantRole, TenantUserAccount } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<TenantUserAccount[]>([])
const query = reactive({ kw: '', phone: '', status: null as number | null, tenant_id: null as number | null, page: 1, page_size: 10 })

// 租户下拉选项（启用状态已同步租户，平台租户 ID 口径；筛选/表单共用）
const tenantOptions = ref<{ label: string; value: number }[]>([])

// 当前筛选租户下的角色选项（设置角色/创建表单用；切租户时重载）
const rolesOfTenant = ref<TenantRole[]>([])

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({ tenant_id: null as number | null, username: '', password: '', nickname: '', phone: '', email: '', role_ids: [] as number[] })
const formRef = ref<FormInst | null>(null)

// 角色选项：跟随表单/弹窗目标用户的归属租户（创建表单跟随所选租户）
const roleTargetTenantId = ref<number | null>(null)
const roleOptions = computed(() =>
  rolesOfTenant.value
    .filter((r) => !roleTargetTenantId.value || r.tenant_id === roleTargetTenantId.value)
    .map((r) => ({ label: r.name, value: r.id })))

const showRole = ref(false)
const roleTarget = ref<TenantUserAccount | null>(null)
const roleForm = reactive({ role_ids: [] as number[] })

const showReset = ref(false)
const resetTarget = ref<TenantUserAccount | null>(null)
const resetForm = reactive({ password: '' })
const resetFormRef = ref<FormInst | null>(null)

// 租户用户状态：1 启用 0 禁用（禁用后门户登录/token 校验即时失败）
const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 0 },
])

const rules = computed<FormRules>(() => ({
  tenant_id: [{ required: true, type: 'number', message: t('tenantUser.form.tenantRequired'), trigger: ['blur', 'change'] }],
  username: [
    { required: true, message: t('tenantUser.form.usernameRequired'), trigger: ['blur', 'input'] },
    { min: 3, max: 64, message: t('tenantUser.form.usernameLength'), trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: t('tenantUser.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('tenantUser.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
  email: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
      message: t('tenantUser.form.emailInvalid'),
    },
  ],
}))

const resetRules = computed<FormRules>(() => ({
  password: [
    { required: true, message: t('tenantUser.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('tenantUser.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
}))

const { pagination, setTotal, runSearch: search } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listTenantUsers({
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
    const { data } = await listTenants({ page: 1, page_size: 0, status: 1 })
    // 平台租户 ID 口径：仅已同步（platform_id>0）租户可挂载租户用户
    tenantOptions.value = data.data.list
      .filter((x) => x.platform_id > 0)
      .map((x) => ({ label: x.name, value: x.platform_id }))
  } catch {
    // 下拉选项加载失败不阻塞页面，列表照常可用
  }
}

// 拉取商户全部租户角色（page_size=0 全量），按 tenant_id 过滤生成选项
async function loadRoles() {
  try {
    const { data } = await listTenantRoles({ page: 1, page_size: 0 })
    rolesOfTenant.value = data.data.list
  } catch {
    rolesOfTenant.value = []
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
  Object.assign(form, { tenant_id: query.tenant_id, username: '', password: '', nickname: '', phone: '', email: '', role_ids: [] })
  roleTargetTenantId.value = form.tenant_id
  showModal.value = true
}

function openEdit(row: TenantUserAccount) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { tenant_id: row.tenant_id, username: row.username, password: '', nickname: row.nickname, phone: row.phone, email: row.email, role_ids: [...row.role_ids] })
  roleTargetTenantId.value = row.tenant_id
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
      await updateTenantUser(editId.value, {
        nickname: form.nickname.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
      })
    } else {
      await createTenantUser({
        tenant_id: form.tenant_id!,
        username: form.username.trim(),
        password: form.password,
        nickname: form.nickname.trim() || undefined,
        phone: form.phone.trim() || undefined,
        email: form.email.trim() || undefined,
        role_ids: form.role_ids,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantUser.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openSetRoles(row: TenantUserAccount) {
  roleTarget.value = row
  roleForm.role_ids = [...row.role_ids]
  roleTargetTenantId.value = row.tenant_id
  showRole.value = true
}

async function doSetRoles() {
  if (!roleTarget.value) return
  saving.value = true
  try {
    await setTenantUserRoles(roleTarget.value.id, roleForm.role_ids)
    message.success(t('common.saveSuccess'))
    showRole.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantUser.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openReset(row: TenantUserAccount) {
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
    await resetTenantUserPassword(resetTarget.value.id, resetForm.password)
    message.success(t('common.saveSuccess'))
    showReset.value = false
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantUser.resetFailed'))
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row: TenantUserAccount) {
  try {
    await setTenantUserStatus(row.id, row.status === 1 ? 0 : 1)
    message.success(t('tenantUser.statusUpdated'))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantUser.statusFailed'))
  }
}

function confirmDelete(row: TenantUserAccount) {
  dialog.warning({
    title: t('tenantUser.deleteConfirmTitle'),
    content: t('tenantUser.deleteConfirmContent', { name: row.username }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteTenantUser(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('tenantUser.deleteFailed'))
      }
    },
  })
}

const roleName = (id: number) => rolesOfTenant.value.find((r) => r.id === id)?.name || `#${id}`

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<TenantUserAccount>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('tenantUser.username'), key: 'username', width: 140 },
  { title: t('tenantUser.tenant'), key: 'tenant_name', width: 140, render: (row) => row.tenant_name || `#${row.tenant_id}` },
  {
    title: t('tenantUser.roles'), key: 'role_ids', width: 180,
    render: (row) => row.role_ids.length
      ? h('span', { style: 'display:inline-flex;gap:4px;flex-wrap:wrap' }, row.role_ids.map((id) => h(NTag, { size: 'small', bordered: false }, { default: () => roleName(id) })))
      : '—',
  },
  { title: t('tenantUser.nickname'), key: 'nickname', width: 110, render: (row) => row.nickname || '—' },
  { title: t('tenantUser.phone'), key: 'phone', width: 120, render: (row) => row.phone || '—' },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 220,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('tenantUser:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('tenantUser:setRoles')) {
        actions.push({ label: t('tenantUser.setRoles'), onClick: () => openSetRoles(row) })
      }
      if (userStore.has('tenantUser:resetPwd')) {
        actions.push({ label: t('tenantUser.resetPassword'), onClick: () => openReset(row) })
      }
      if (userStore.has('tenantUser:status')) {
        actions.push({ label: row.status === 1 ? t('common.disable') : t('common.enable'), onClick: () => toggleStatus(row) })
      }
      if (userStore.has('tenantUser:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(() => {
  load()
  loadTenantOptions()
  loadRoles()
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
