<template>
  <SearchCard storage-key="tenant-members" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('tenantPortal.members.kwPlaceholder')" clearable style="width: 200px" @keyup.enter="search" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate">{{ t('tenantPortal.members.newMember') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑成员（username 创建后不可改；归属固定当前租户，跨租户归属走商户管理端） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('tenantPortal.members.editMember') : t('tenantPortal.members.newMember')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('tenantPortal.members.username')" path="username">
        <n-input v-model:value="form.username" :maxlength="64" :disabled="editing" :placeholder="t('tenantPortal.members.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item v-if="!editing" :label="t('tenantPortal.members.password')" path="password">
        <n-input v-model:value="form.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('tenantPortal.members.passwordPlaceholder')" />
      </n-form-item>
      <n-form-item v-if="!editing" :label="t('tenantPortal.members.role')" path="role">
        <n-select v-model:value="form.role" :options="roleOptions" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.nickname')" path="nickname">
        <n-input v-model:value="form.nickname" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.phone')" path="phone">
        <n-input v-model:value="form.phone" :maxlength="32" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.email')" path="email">
        <n-input v-model:value="form.email" :maxlength="128" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 重置密码：管理员指定新密码，旧密码立即失效 -->
  <n-modal v-model:show="showReset" preset="dialog" :title="t('tenantPortal.members.resetPasswordTitle')" style="width: 420px">
    <n-alert type="warning" :show-icon="true" style="margin-bottom: 12px">
      {{ t('tenantPortal.members.resetConfirmContent', { name: resetTarget?.username }) }}
    </n-alert>
    <n-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-placement="left" label-width="80">
      <n-form-item :label="t('tenantPortal.members.newPassword')" path="password">
        <n-input v-model:value="resetForm.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('tenantPortal.members.newPasswordPlaceholder')" />
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
import {
  NAlert, NButton, NCard, NDataTable, NForm, NFormItem, NInput, NModal, NSelect, NTag,
  useDialog, useMessage, type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import {
  createTenantMember, listTenantMembers, removeTenantMember, resetTenantMemberPassword,
  setTenantMemberRole, setTenantMemberStatus, updateTenantMember,
} from '../../api/tenant'
import { usePagination } from '../../utils/pagination'
import { useTenantUserStore } from '../../stores/tenantUser'
import type { TenantMember } from '../../api/tenant'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const store = useTenantUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<TenantMember[]>([])
const query = reactive({ kw: '', page: 1, page_size: 10 })

const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({ username: '', password: '', nickname: '', phone: '', email: '', role: 'member' as 'tenant_admin' | 'member' })
const formRef = ref<FormInst | null>(null)

const showReset = ref(false)
const resetTarget = ref<TenantMember | null>(null)
const resetForm = reactive({ password: '' })
const resetFormRef = ref<FormInst | null>(null)

const roleOptions = computed(() => [
  { label: t('tenantPortal.role.member'), value: 'member' },
  { label: t('tenantPortal.role.admin'), value: 'tenant_admin' },
])

const rules = computed<FormRules>(() => ({
  username: [
    { required: true, message: t('tenantPortal.members.form.usernameRequired'), trigger: ['blur', 'input'] },
    { min: 3, max: 64, message: t('tenantPortal.members.form.usernameLength'), trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: t('tenantPortal.members.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('tenantPortal.members.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
  email: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
      message: t('tenantPortal.members.form.emailInvalid'),
    },
  ],
}))

const resetRules = computed<FormRules>(() => ({
  password: [
    { required: true, message: t('tenantPortal.members.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('tenantPortal.members.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
}))

const { pagination, setTotal, runSearch: search } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listTenantMembers({ page: query.page, page_size: query.page_size, kw: query.kw || undefined })
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.kw = ''
  query.page = 1
  load()
}

function openCreate() {
  editing.value = false
  Object.assign(form, { username: '', password: '', nickname: '', phone: '', email: '', role: 'member' })
  showModal.value = true
}

function openEdit(row: TenantMember) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { username: row.username, password: '', nickname: row.nickname, phone: row.phone, email: row.email, role: row.role })
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateTenantMember(editId.value, {
        nickname: form.nickname.trim() || undefined,
        phone: form.phone.trim() || undefined,
        email: form.email.trim() || undefined,
      })
    } else {
      await createTenantMember({
        username: form.username.trim(),
        password: form.password,
        nickname: form.nickname.trim() || undefined,
        phone: form.phone.trim() || undefined,
        email: form.email.trim() || undefined,
        role: form.role,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openReset(row: TenantMember) {
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
    await resetTenantMemberPassword(resetTarget.value.id, resetForm.password)
    message.success(t('common.saveSuccess'))
    showReset.value = false
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.resetFailed'))
  } finally {
    saving.value = false
  }
}

// 角色切换（本人不可变更——后端守卫兜底，前端提前禁用入口）
async function toggleRole(row: TenantMember) {
  try {
    await setTenantMemberRole(row.id, row.role === 'tenant_admin' ? 'member' : 'tenant_admin')
    message.success(t('common.saveSuccess'))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.roleFailed'))
  }
}

async function toggleStatus(row: TenantMember) {
  try {
    await setTenantMemberStatus(row.id, row.status === 1 ? 0 : 1)
    message.success(t('tenantPortal.members.statusUpdated'))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.statusFailed'))
  }
}

function confirmRemove(row: TenantMember) {
  dialog.warning({
    title: t('tenantPortal.members.removeConfirmTitle'),
    // 移除=出本租户（不删账号，保留其他租户归属）
    content: t('tenantPortal.members.removeConfirmContent', { name: row.username }),
    positiveText: t('tenantPortal.members.remove'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await removeTenantMember(row.id)
        message.success(t('tenantPortal.members.removed'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('tenantPortal.members.removeFailed'))
      }
    },
  })
}

const columns = computed<DataTableColumns<TenantMember>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantPortal.members.username'), key: 'username', width: 140 },
  { title: t('tenantPortal.members.nickname'), key: 'nickname', width: 120, render: (row) => row.nickname || '—' },
  { title: t('tenantPortal.members.phone'), key: 'phone', width: 130, render: (row) => row.phone || '—' },
  {
    title: t('tenantPortal.members.role'), key: 'role', width: 110,
    render: (row) => h(NTag, {
      type: row.role === 'tenant_admin' ? 'primary' : 'default', size: 'small', bordered: false,
    }, { default: () => (row.role === 'tenant_admin' ? t('tenantPortal.role.admin') : t('tenantPortal.role.member')) }),
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
      const isSelf = row.id === store.user?.id
      actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      // 本人不可变更自己的角色/启停/移除（后端守卫兜底，前端隐藏入口减少无效请求）
      if (!isSelf) {
        actions.push({ label: row.role === 'tenant_admin' ? t('tenantPortal.members.demote') : t('tenantPortal.members.promote'), onClick: () => toggleRole(row) })
        actions.push({ label: row.status === 1 ? t('common.disable') : t('common.enable'), onClick: () => toggleStatus(row) })
      }
      actions.push({ label: t('tenantPortal.members.resetPassword'), onClick: () => openReset(row) })
      if (!isSelf) {
        actions.push({ label: t('tenantPortal.members.remove'), danger: true, onClick: () => confirmRemove(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(load)
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
