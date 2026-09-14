<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="online" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.username" :placeholder="t('online.username')" clearable style="width: 180px" @keyup.enter="load" />
    <n-select v-model:value="query.device" :options="deviceOptions" clearable :placeholder="t('online.device')" style="width: 140px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button ghost @click="load">
          <template #icon><n-icon :component="RefreshOutline" /></template>
          {{ t('common.refresh') }}
        </n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NDataTable, NEllipsis, NIcon, NInput, NSelect, NTag, useMessage, useDialog, type DataTableColumns } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { kickOnlineSession, kickUserSessions, listOnlineUsers } from '../../api'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import type { OnlineSession } from '../../api/types'

const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()
const { t } = useI18n()

// admin（id=1）超管账号：仅其本人可见操作按钮（与用户管理保护规则一致）
const SUPER_ADMIN_ID = 1
const canOperate = (row: OnlineSession) => row.user_id !== SUPER_ADMIN_ID || userStore.user?.id === SUPER_ADMIN_ID

const loading = ref(false)
const rows = ref<OnlineSession[]>([])
const query = reactive({ username: '', device: null as string | null, page: 1, page_size: 10 })

const deviceOptions = computed(() => [
  { label: t('online.web'), value: 'web' },
  { label: t('online.app'), value: 'app' },
])

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listOnlineUsers({
      page: query.page,
      page_size: query.page_size,
      username: query.username || undefined,
      device: query.device || undefined,
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
  query.username = ''
  query.device = null
  query.page = 1
  load()
}

// 下线单个会话（该用户此设备端）
function confirmKick(row: OnlineSession) {
  const self = row.is_current ? t('online.kickSelfNote') : ''
  dialog.warning({
    title: t('online.kickTitle'),
    content: t('online.kickConfirm', { username: row.username, device: deviceLabel(row.device), self }),
    positiveText: t('online.kick'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await kickOnlineSession(row.sid)
        message.success(t('online.kicked'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

// 下线某用户全部端
function confirmKickUser(row: OnlineSession) {
  dialog.warning({
    title: t('online.kickUserTitle'),
    content: t('online.kickUserConfirm', { username: row.username }),
    positiveText: t('online.kickAll'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await kickUserSessions(row.user_id)
        message.success(t('online.allKicked'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

function deviceLabel(device: string) {
  return device === 'app' ? t('online.app') : t('online.web')
}

// 操作列依赖按钮权限，computed 使权限变化后重新渲染
const columns = computed<DataTableColumns<OnlineSession>>(() => [
  {
    title: t('online.username'), key: 'username', width: 160,
    render: (row) =>
      h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, [
        row.username,
        row.is_current ? h(NTag, { type: 'primary', size: 'small', bordered: false }, { default: () => t('online.currentSession') }) : null,
      ]),
  },
  { title: t('online.nickname'), key: 'nickname', width: 120, render: (row) => row.nickname || '—' },
  {
    title: t('online.device'), key: 'device', width: 90,
    render: (row) =>
      h(NTag, { type: row.device === 'app' ? 'success' : 'info', size: 'small' }, { default: () => deviceLabel(row.device) }),
  },
  { title: 'IP', key: 'ip', width: 130 },
  {
    title: t('online.deviceInfo'), key: 'user_agent',
    render: (row) => h(NEllipsis, { style: 'max-width: 220px', tooltip: true }, { default: () => row.user_agent || '—' }),
  },
  { title: t('online.loginTime'), key: 'login_at', width: 170 },
  { title: t('online.lastActive'), key: 'last_active_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 150,
    render(row) {
      if (!canOperate(row)) {
        return renderActions([])
      }
      const actions: Array<TableAction> = []
      if (userStore.has('online:kick')) {
        actions.push({ label: t('online.kick'), danger: true, onClick: () => confirmKick(row) })
      }
      if (userStore.has('online:kickUser')) {
        actions.push({ label: t('online.kickAll'), danger: true, onClick: () => confirmKickUser(row) })
      }
      return renderActions(actions)
    },
  },
])

onMounted(() => { load() })
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
}
</style>
