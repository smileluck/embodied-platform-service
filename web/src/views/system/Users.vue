<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="users" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('user.searchKw')" clearable style="width: 220px" @keyup.enter="load" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openAdd" v-permission="['user:add']">{{ t('user.add') }}</n-button>
        <n-button ghost :loading="exporting" @click="doExport" v-permission="['user:export']">{{ t('user.export') }}</n-button>
        <n-button ghost :loading="syncing" @click="doSync" v-permission="['user:sync']">{{ t('user.sync') }}</n-button>
      </div>
    </template>

    <!-- 说明：账号本体在 embodied-platform 上（统一身份源）；本页管理「本商户成员」——
         列表实时来自平台绑定集，新增/删除均推送平台，准入开关与角色是本系统本地授权 -->
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增成员 -->
  <n-modal v-model:show="showAdd" preset="dialog" :title="t('user.addTitle')" style="width: 480px">
    <n-form label-placement="left" label-width="86">
      <n-form-item :label="t('user.username')" required>
        <n-input v-model:value="addForm.username" :placeholder="t('user.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('user.nickname')">
        <n-input v-model:value="addForm.nickname" />
      </n-form-item>
      <n-form-item :label="t('user.password')">
        <n-input v-model:value="addForm.password" type="password" show-password-on="click" :placeholder="t('user.passwordHint')" />
      </n-form-item>
      <n-form-item :label="t('user.role')">
        <n-select v-model:value="addForm.role_ids" multiple :options="roleOptions" />
      </n-form-item>
      <p class="modal-hint">{{ t('user.addHint') }}</p>
    </n-form>
    <template #action>
      <n-button @click="showAdd = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="saveAdd">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 分配角色 -->
  <n-modal v-model:show="showRoles" preset="dialog" :title="t('user.setRoles')" style="width: 440px">
    <p class="modal-hint">{{ t('user.rolesHint') }}</p>
    <n-select v-model:value="roleIds" multiple :options="roleOptions" />
    <template #action>
      <n-button @click="showRoles = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="saveRoles">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import { NCard, NForm, NFormItem, NInput, NButton, NDataTable, NModal, NSelect, NTag, useMessage, useDialog, type DataTableColumns } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { addUser, createExport, deleteUser, listRoles, listUsers, setUserAdmission, setUserRoles, syncUsersFromPlatform } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { MemberRow } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const exporting = ref(false)
const rows = ref<MemberRow[]>([])
const query = reactive({ kw: '', page: 1, page_size: 10 })
const roleOptions = ref<{ label: string; value: number }[]>([])
// 内置锁定角色（2=商户管理员，平台标记驱动）不参与手工分配
const LOCKED_ROLE_IDS = new Set([2])


const columns: DataTableColumns<MemberRow> = [
  { title: 'ID', key: 'platform_user_id', width: 70 },
  { title: t('user.username'), key: 'username', width: 140 },
  { title: t('user.nickname'), key: 'nickname', width: 140, ellipsis: { tooltip: true } },
  {
    title: t('user.platformStatus'),
    key: 'platform_status',
    width: 100,
    render: (row) =>
      h(NTag, { type: row.platform_status === 1 ? 'success' : 'error', size: 'small', bordered: false },
        { default: () => (row.platform_status === 1 ? t('user.platformActive') : t('user.platformDisabled')) }),
  },
  {
    // 准入状态：平台侧事实源（本端开/关即写回平台）；无本地投影=尚未准入本系统
    title: t('user.admissionStatus'),
    key: 'admission',
    width: 110,
    render: (row) => {
      if (!row.projection) {
        return h(NTag, { type: 'default', size: 'small', bordered: false }, { default: () => t('user.notAdmitted') })
      }
      return h(NTag, { type: row.admitted ? 'success' : 'warning', size: 'small', bordered: false },
        { default: () => (row.admitted ? t('user.admitted') : t('user.suspended')) })
    },
  },
  {
    // 角色：商户管理员标签=平台侧实时标记（事实源）；其余为本地已分配角色（分配角色弹窗维护）
    title: t('user.role'),
    key: 'roles',
    width: 200,
    render: (row) => {
      const tags = []
      if (row.is_admin) {
        tags.push(h(NTag, { type: 'info', size: 'small', bordered: false }, { default: () => t('user.merchantAdmin') }))
      }
      const roleIDs = row.projection?.role_ids || []
      for (const id of roleIDs) {
        if (LOCKED_ROLE_IDS.has(id)) continue // 内置角色已由上方管理员标签表达
        const opt = roleOptions.value.find((o) => o.value === id)
        if (opt) tags.push(h(NTag, { size: 'small', bordered: false }, { default: () => opt.label }))
      }
      return tags.length
        ? h('div', { style: 'display: flex; gap: 4px; flex-wrap: wrap' }, tags)
        : h('span', { style: 'color: var(--sx-muted)' }, '—')
    },
  },
  { title: t('user.boundAt'), key: 'bound_at', width: 170 },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 240,
    render: (row) => {
      const actions: TableAction[] = []
      if (row.projection) {
        actions.push({
          label: t('user.setRoles'),
          permission: 'user:setRoles',
          onClick: () => openRoles(row),
        })
      actions.push(
        row.projection!.enabled
          ? { label: t('user.suspend'), permission: 'user:setAdmission', onClick: () => setAdmission(row, false) }
          : { label: t('user.admit'), permission: 'user:setAdmission', onClick: () => setAdmission(row, true) },
      )
      }
      actions.push({ label: t('common.delete'), permission: 'user:delete', onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
]
const { pagination, setTotal, runSearch: search } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listUsers({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
    })
    rows.value = resp.data.list || []
    pagination.itemCount = resp.data.page?.total || 0
  } catch {
    message.error(t('user.listFailed'))
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.kw = ''
  load()
}

// ---- 新增成员（推送平台：无此账号则创建并绑定本商户，有则仅绑定；本地建准入投影） ----
const showAdd = ref(false)
const addForm = reactive({ username: '', nickname: '', password: '', role_ids: [] as number[] })

function openAdd() {
  addForm.username = ''
  addForm.nickname = ''
  addForm.password = ''
  addForm.role_ids = []
  showAdd.value = true
}

async function saveAdd() {
  if (!addForm.username.trim()) {
    message.warning(t('user.usernameRequired'))
    return
  }
  saving.value = true
  try {
    const { data: resp } = await addUser({
      username: addForm.username.trim(),
      nickname: addForm.nickname.trim() || undefined,
      password: addForm.password || undefined,
      enabled: true,
      role_ids: addForm.role_ids,
    })
    message.success(resp.data.existed ? t('user.addBoundExisted') : t('user.addCreated'))
    showAdd.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('user.addFailed'))
  } finally {
    saving.value = false
  }
}

// ---- 准入开关（关闭即同步吊销该用户对本系统的访问授权，即时生效） ----
async function setAdmission(row: MemberRow, enabled: boolean) {
  try {
    await setUserAdmission(row.platform_user_id, enabled)
    if (row.projection) row.projection.enabled = enabled
    message.success(enabled ? t('user.admitSuccess') : t('user.suspendSuccess'))
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  }
}

// ---- 分配角色 ----
const showRoles = ref(false)
const roleIds = ref<number[]>([])
const editingId = ref(0)

function openRoles(row: MemberRow) {
  editingId.value = row.platform_user_id
  // 内置锁定角色不在可选项内，回填时剔除（后端替换时也会保留现持有的锁定绑定）
  roleIds.value = [...(row.projection?.role_ids || [])].filter((id) => !LOCKED_ROLE_IDS.has(id))
  showRoles.value = true
}

async function saveRoles() {
  saving.value = true
  try {
    await setUserRoles(editingId.value, roleIds.value)
    message.success(t('common.saveSuccess'))
    showRoles.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

// ---- 移除成员（解除平台侧关联并删除本地投影；平台账号本体保留） ----
function confirmDelete(row: MemberRow) {
  dialog.warning({
    title: t('user.removeConfirmTitle'),
    content: t('user.removeConfirmContent', { username: row.username }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteUser(row.platform_user_id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.deleteFailed'))
      }
    },
  })
}

// ---- 从平台同步成员（补建缺失投影，成员=准入开启；刷新快照） ----
async function doSync() {
  syncing.value = true
  try {
    const { data: resp } = await syncUsersFromPlatform()
    message.success(t('user.syncDone', { created: resp.data.created, refreshed: resp.data.refreshed }))
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('user.syncFailed'))
  } finally {
    syncing.value = false
  }
}

async function doExport() {
  exporting.value = true
  try {
    // 导出对象=本地准入投影（含角色列），沿用 username 前缀过滤口径
    await createExport('users', { username: query.kw })
    message.success(t('user.exportQueued'))
  } catch (e: any) {
    const status = e?.response?.status
    message.error(status === 429 ? t('user.exportTooMany') : t('user.exportFailed'))
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  load()
  try {
    const { data: resp } = await listRoles({ page: 1, page_size: 0 })
    roleOptions.value = (resp.data.list || []).filter((r) => !LOCKED_ROLE_IDS.has(r.id)).map((r) => ({ label: r.name, value: r.id }))
  } catch { /* 角色加载失败不阻断列表 */ }
})
</script>

<style scoped>
.page-actions {
  display: flex;
  gap: 12px;
}
.modal-hint {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--n-text-color-3, #999);
}
.masked-hint {
  width: 100%;
  font-size: 11px;
  color: var(--sx-muted);
}
</style>
