<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="users" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.username" :placeholder="t('user.username')" clearable style="width: 180px" @keyup.enter="load" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button ghost :loading="exporting" @click="doExport" v-permission="['user:export']">{{ t('user.export') }}</n-button>
        <n-button type="primary" ghost @click="openCreate" v-permission="['user:create']">{{ t('user.newUser') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑 -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('user.editUser') : t('user.newUser')" style="width: 480px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="80">
      <n-form-item :label="t('user.username')" path="username" v-if="!editing">
        <n-input v-model:value="form.username" :placeholder="t('user.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('user.password')" path="password" v-if="!editing">
        <n-input v-model:value="form.password" type="password" :maxlength="20" :placeholder="t('user.passwordPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('user.nickname')" path="nickname">
        <n-input v-model:value="form.nickname" :maxlength="20" show-word-limit :placeholder="t('user.nicknamePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('user.phone')" path="phone">
        <n-input v-model:value="form.phone" :maxlength="32" :placeholder="t('user.phonePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('user.email')" path="email"><n-input v-model:value="form.email" :placeholder="t('user.emailPlaceholder')" /></n-form-item>
      <n-form-item :label="t('user.role')">
        <n-select v-model:value="form.role_ids" multiple :options="roleOptions" :disabled="editing && !userStore.has('user:setRoles')" />
      </n-form-item>
      <n-form-item :label="t('common.status')" v-if="editing">
        <n-switch v-model:value="form.statusOn" :checked-value="1" :unchecked-value="0" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 重置密码 -->
  <n-modal v-model:show="showPwd" preset="dialog" :title="t('user.resetPassword')" style="width: 420px">
    <n-input v-model:value="newPassword" type="password" :maxlength="20" :placeholder="t('user.newPasswordPlaceholder')" />
    <template #action>
      <n-button @click="showPwd = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" @click="savePwd">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, type VNode } from 'vue'
import { NCard, NInput, NButton, NDataTable, NModal, NForm, NFormItem, NSelect, NSwitch, NTag, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createUser, createExport, deleteUser, listRoles, listUsers, resetPassword, setUserRoles, updateUser } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { UserInfo } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

// admin（id=1）超管账号：仅其本人可见操作按钮
const SUPER_ADMIN_ID = 1
const canOperate = (row: UserInfo) => row.id !== SUPER_ADMIN_ID || userStore.user?.id === SUPER_ADMIN_ID
const loading = ref(false)
const saving = ref(false)
const rows = ref<UserInfo[]>([])
const query = reactive({ username: '', page: 1, page_size: 10 })
const roleOptions = ref<{ label: string; value: number }[]>([])

const showModal = ref(false)
const showPwd = ref(false)
const editing = ref(false)
const editId = ref(0)
const newPassword = ref('')
const form = reactive({ username: '', password: '', nickname: '', phone: '', email: '', role_ids: [] as number[], statusOn: 1 })
const formRef = ref<FormInst | null>(null)

// 与后端 binding 规则保持一致：用户名 3-64、密码 6-20、昵称最长 20、手机号选填最长 32、邮箱格式（选填）
const rules = computed<FormRules>(() => ({
  username: [
    { required: true, message: t('user.form.usernameRequired'), trigger: ['blur', 'input'] },
    { min: 3, max: 64, message: t('user.form.usernameLength'), trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: t('user.form.passwordRequired'), trigger: ['blur', 'input'] },
    { min: 6, max: 20, message: t('user.form.passwordLength'), trigger: ['blur', 'input'] },
  ],
  nickname: [
    { max: 20, message: t('user.form.nicknameLength'), trigger: ['blur', 'input'] },
  ],
  phone: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^1[3-9]\d{9}$/.test(value),
      message: t('user.form.phoneInvalid'),
    },
  ],
  email: [
    {
      trigger: ['blur', 'input'],
      validator: (_rule, value: string) => !value || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value),
      message: t('user.form.emailInvalid'),
    },
  ],
}))

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listUsers(query)
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.username = ''
  query.page = 1
  load()
}

// 异步导出：提交当前过滤条件（page/page_size 与空值由 createExport 剔除）
const exporting = ref(false)
async function doExport() {
  exporting.value = true
  try {
    await createExport('users', { ...query })
    message.success(t('user.exportQueued'))
  } catch (e: any) {
    if (e?.response?.status === 429) {
      message.warning(t('user.exportTooMany'))
    } else {
      message.error(e?.response?.data?.msg || t('user.exportFailed'))
    }
  } finally {
    exporting.value = false
  }
}

async function loadRoles() {
  const { data } = await listRoles({ page: 1, page_size: 100 })
  roleOptions.value = data.data.list.map((r) => ({ label: r.name, value: r.id }))
}

function openCreate() {
  editing.value = false
  Object.assign(form, { username: '', password: '', nickname: '', phone: '', email: '', role_ids: [], statusOn: 1 })
  showModal.value = true
}

function openEdit(row: UserInfo) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, { username: row.username, password: '', nickname: row.nickname, phone: row.phone, email: row.email, role_ids: row.role_ids ?? [], statusOn: row.status })
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
      await updateUser(editId.value, { nickname: form.nickname, phone: form.phone.trim(), email: form.email, status: form.statusOn })
      // 分配角色是独立接口权限，未授权时不发起该调用（角色保持不变）
      if (userStore.has('user:setRoles')) {
        await setUserRoles(editId.value, form.role_ids)
      }
    } else {
      await createUser({ username: form.username.trim(), password: form.password, nickname: form.nickname.trim(), phone: form.phone.trim(), email: form.email.trim(), role_ids: form.role_ids })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    await load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('user.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: UserInfo) {
  if (row.id === SUPER_ADMIN_ID) { message.error(t('user.superAdminNoDelete')); return }
  dialog.warning({
    title: t('user.deleteConfirmTitle'),
    content: t('user.deleteConfirmContent', { username: row.username }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteUser(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('user.deleteFailed'))
      }
    },
  })
}

async function savePwd() {
  if (newPassword.value.length < 6 || newPassword.value.length > 20) { message.warning(t('user.pwdLengthWarn')); return }
  try {
    await resetPassword(editId.value, newPassword.value)
    message.success(t('user.pwdResetSuccess'))
    showPwd.value = false
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('user.pwdResetFailed'))
  }
}

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<UserInfo>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('user.username'), key: 'username' },
  { title: t('user.nickname'), key: 'nickname', render: (row) => row.nickname || '—' },
  { title: t('user.phone'), key: 'phone', width: 130, render: (row) => row.phone || '—' },
  { title: t('user.email'), key: 'email', render: (row) => row.email || '—' },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 160,
    render(row) {
      if (!canOperate(row)) {
        return renderActions([])
      }
      const actions: Array<TableAction | VNode> = []
      if (userStore.has('user:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('user:resetPassword')) {
        actions.push({ label: t('user.resetPassword'), onClick: () => { editId.value = row.id; newPassword.value = ''; showPwd.value = true } })
      }
      // 超管账号禁止删除（即使超管本人），不展示删除按钮
      if (row.id !== SUPER_ADMIN_ID && userStore.has('user:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(() => { load(); loadRoles() })
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
