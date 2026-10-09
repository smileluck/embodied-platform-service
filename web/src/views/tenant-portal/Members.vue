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

  <!-- 新增成员（username 创建后不可改；归属固定当前租户，跨租户归属走商户管理端） -->
  <n-modal v-model:show="showModal" preset="dialog" :title="t('tenantPortal.members.newMember')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('tenantPortal.members.username')" path="username">
        <n-input v-model:value="form.username" :maxlength="64" :placeholder="t('tenantPortal.members.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.password')" path="password">
        <n-input v-model:value="form.password" type="password" show-password-on="click" :maxlength="20" :placeholder="t('tenantPortal.members.passwordPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.roles')" path="role_ids">
        <n-select v-model:value="form.role_ids" :options="roleOptions" multiple filterable clearable :placeholder="t('common.pleaseSelect')" />
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

  <!-- 编辑成员资料（username/角色编辑入口在行内「设置角色」） -->
  <n-modal v-model:show="showEdit" preset="dialog" :title="t('tenantPortal.members.editMember')" style="width: 440px">
    <n-form :model="editForm" label-placement="left" label-width="80">
      <n-form-item :label="t('tenantPortal.members.username')">
        <span>{{ editTarget?.username }}</span>
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.nickname')">
        <n-input v-model:value="editForm.nickname" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.phone')">
        <n-input v-model:value="editForm.phone" :maxlength="32" />
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.email')">
        <n-input v-model:value="editForm.email" :maxlength="128" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showEdit = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="doEdit">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 设置角色（全量替换本租户角色；本人不可自改——后端守卫兜底） -->
  <n-modal v-model:show="showRoles" preset="dialog" :title="t('tenantPortal.members.setRolesTitle')" style="width: 440px">
    <n-form label-placement="left" label-width="80">
      <n-form-item :label="t('tenantPortal.members.username')">
        <span>{{ rolesTarget?.username }}</span>
      </n-form-item>
      <n-form-item :label="t('tenantPortal.members.roles')">
        <n-select v-model:value="rolesForm.role_ids" :options="roleOptions" multiple filterable :placeholder="t('common.pleaseSelect')" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showRoles = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="doSetRoles">{{ t('common.confirm') }}</n-button>
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
  createTenantMember, listTenantMemberRoles, listTenantMembers, removeTenantMember,
  resetTenantMemberPassword, setTenantMemberRoles, setTenantMemberStatus, updateTenantMember,
} from '../../api/tenant'
import { usePagination } from '../../utils/pagination'
import { useTenantUserStore } from '../../stores/tenantUser'
import type { TenantMember, TenantRoleOption } from '../../api/tenant'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const store = useTenantUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<TenantMember[]>([])
const query = reactive({ kw: '', page: 1, page_size: 10 })

// 本租户角色选项（经门户 /members/roles 拉取）
const roleOptions = ref<{ label: string; value: number }[]>([])
const rolesById = ref<Map<number, TenantRoleOption>>(new Map())

const showModal = ref(false)
const form = reactive({ username: '', password: '', nickname: '', phone: '', email: '', role_ids: [] as number[] })
const formRef = ref<FormInst | null>(null)

const showEdit = ref(false)
const editTarget = ref<TenantMember | null>(null)
const editForm = reactive({ nickname: '', phone: '', email: '' })

const showRoles = ref(false)
const rolesTarget = ref<TenantMember | null>(null)
const rolesForm = reactive({ role_ids: [] as number[] })

const showReset = ref(false)
const resetTarget = ref<TenantMember | null>(null)
const resetForm = reactive({ password: '' })
const resetFormRef = ref<FormInst | null>(null)

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

async function loadRoles() {
  try {
    const { data } = await listTenantMemberRoles()
    const list = data.data || []
    rolesById.value = new Map(list.map((r) => [r.id, r]))
    roleOptions.value = list.map((r) => ({ label: r.name, value: r.id }))
  } catch {
    roleOptions.value = []
    rolesById.value = new Map()
  }
}

function resetQuery() {
  query.kw = ''
  query.page = 1
  load()
}

function openCreate() {
  Object.assign(form, { username: '', password: '', nickname: '', phone: '', email: '', role_ids: [] })
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
    await createTenantMember({
      username: form.username.trim(),
      password: form.password,
      nickname: form.nickname.trim() || undefined,
      phone: form.phone.trim() || undefined,
      email: form.email.trim() || undefined,
      role_ids: form.role_ids,
    })
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openEdit(row: TenantMember) {
  editTarget.value = row
  Object.assign(editForm, { nickname: row.nickname, phone: row.phone, email: row.email })
  showEdit.value = true
}

async function doEdit() {
  if (!editTarget.value) return
  saving.value = true
  try {
    await updateTenantMember(editTarget.value.id, {
      nickname: editForm.nickname.trim() || undefined,
      phone: editForm.phone.trim() || undefined,
      email: editForm.email.trim() || undefined,
    })
    message.success(t('common.saveSuccess'))
    showEdit.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.saveFailed'))
  } finally {
    saving.value = false
  }
}

function openSetRoles(row: TenantMember) {
  rolesTarget.value = row
  rolesForm.role_ids = [...row.role_ids]
  showRoles.value = true
}

async function doSetRoles() {
  if (!rolesTarget.value) return
  saving.value = true
  try {
    await setTenantMemberRoles(rolesTarget.value.id, rolesForm.role_ids)
    message.success(t('common.saveSuccess'))
    showRoles.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('tenantPortal.members.roleFailed'))
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

const roleName = (id: number) => rolesById.value.get(id)?.name || `#${id}`

const columns = computed<DataTableColumns<TenantMember>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantPortal.members.username'), key: 'username', width: 140 },
  { title: t('tenantPortal.members.nickname'), key: 'nickname', width: 110, render: (row) => row.nickname || '—' },
  {
    title: t('tenantPortal.members.roles'), key: 'role_ids', width: 170,
    render: (row) => row.role_ids.length
      ? h('span', { style: 'display:inline-flex;gap:4px;flex-wrap:wrap' }, row.role_ids.map((id) => h(NTag, { size: 'small', bordered: false }, { default: () => roleName(id) })))
      : '—',
  },
  { title: t('tenantPortal.members.phone'), key: 'phone', width: 120, render: (row) => row.phone || '—' },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  {
    title: t('common.operation'), key: 'actions', width: 250,
    render(row) {
      const actions: TableAction[] = []
      const isSelf = row.id === store.user?.id
      actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      // 本人不可变更自己的角色/启停/移除（后端守卫兜底，前端隐藏入口减少无效请求）
      if (!isSelf) {
        actions.push({ label: t('tenantPortal.members.setRoles'), onClick: () => openSetRoles(row) })
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

onMounted(() => {
  load()
  loadRoles()
})
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
