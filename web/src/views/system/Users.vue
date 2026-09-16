<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="users" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.username" :placeholder="t('user.username')" clearable style="width: 180px" @keyup.enter="load" />
    <n-select v-model:value="query.status" :options="statusOptions" :placeholder="t('user.admissionStatus')" clearable style="width: 140px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button ghost :loading="exporting" @click="doExport" v-permission="['user:export']">{{ t('user.export') }}</n-button>
        <n-button type="primary" ghost :loading="syncing" @click="doSync" v-permission="['user:sync']">{{ t('user.sync') }}</n-button>
      </div>
    </template>

    <!-- 说明：账号体系在 embodied-platform 上（平台是唯一身份源），此处只管理「谁能进入本系统」 -->
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

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
import { NCard, NInput, NButton, NDataTable, NModal, NSelect, NSwitch, NTag, useMessage, useDialog, type DataTableColumns } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { createExport, deleteUser, listRoles, listUsers, setUserAdmission, setUserRoles, syncUsersFromPlatform } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { AdmissionUser } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const exporting = ref(false)
const rows = ref<AdmissionUser[]>([])
const query = reactive({ username: '', status: null as number | null, page: 1, page_size: 10 })
const roleOptions = ref<{ label: string; value: number }[]>([])

const statusOptions = [
  { label: t('user.admitted'), value: 1 },
  { label: t('user.suspended'), value: 0 },
]

const { pagination } = usePagination(query, () => load())

const columns: DataTableColumns<AdmissionUser> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('user.username'), key: 'username', width: 140 },
  { title: t('user.nickname'), key: 'nickname', width: 140, ellipsis: { tooltip: true } },
  { title: t('user.email'), key: 'email', width: 200, ellipsis: { tooltip: true } },
  {
    title: t('user.admissionStatus'),
    key: 'enabled',
    width: 100,
    render: (row) =>
      h(NTag, { type: row.enabled ? 'success' : 'warning', size: 'small', bordered: false },
        { default: () => (row.enabled ? t('user.admitted') : t('user.suspended')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 240,
    render: (row) => {
      const actions: TableAction[] = [
        {
          label: t('user.setRoles'),
          permission: 'user:setRoles',
          onClick: () => openRoles(row),
        },
      ]
      actions.push(
        row.enabled
          ? { label: t('user.suspend'), permission: 'user:setAdmission', onClick: () => setAdmission(row, false) }
          : { label: t('user.admit'), permission: 'user:setAdmission', onClick: () => setAdmission(row, true) },
      )
      actions.push({ label: t('common.delete'), permission: 'user:delete', onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
]

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listUsers({
      page: query.page,
      page_size: query.page_size,
      username: query.username || undefined,
      status: query.status ?? undefined,
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
  query.username = ''
  query.status = null
  load()
}

// ---- 准入开关（关闭即同步吊销该用户对本系统的访问授权，即时生效） ----
async function setAdmission(row: AdmissionUser, enabled: boolean) {
  try {
    await setUserAdmission(row.id, enabled)
    row.enabled = enabled
    message.success(enabled ? t('user.admitSuccess') : t('user.suspendSuccess'))
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  }
}

// ---- 分配角色 ----
const showRoles = ref(false)
const roleIds = ref<number[]>([])
const editingId = ref(0)

function openRoles(row: AdmissionUser) {
  editingId.value = row.id
  roleIds.value = [...(row.role_ids || [])]
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

// ---- 移除准入（平台账号不受影响，仅移出本系统） ----
function confirmDelete(row: AdmissionUser) {
  dialog.warning({
    title: t('user.removeConfirmTitle'),
    content: t('user.removeConfirmContent', { username: row.username }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteUser(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.deleteFailed'))
      }
    },
  })
}

// ---- 从平台同步用户（预建投影，默认停用；刷新存量快照） ----
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
    await createExport('users', { username: query.username, status: query.status ?? undefined })
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
    roleOptions.value = (resp.data.list || []).map((r) => ({ label: r.name, value: r.id }))
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
</style>
