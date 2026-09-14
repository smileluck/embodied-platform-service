<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="loginLogs" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.username" :placeholder="t('loginLog.username')" clearable style="width: 160px" @keyup.enter="load" />
    <n-input v-model:value="query.ip" :placeholder="t('loginLog.ipPlaceholder')" clearable style="width: 150px" @keyup.enter="load" />
    <n-select v-model:value="query.status" :options="statusOptions" clearable :placeholder="t('loginLog.statusPlaceholder')" style="width: 120px" />
    <n-date-picker v-model:value="range" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-header">
        <span class="retention-hint">{{ retentionHint }}</span>
        <div class="page-actions">
          <n-button ghost :loading="exporting" v-permission="['log:login:export']" @click="doExport">{{ t('loginLog.export') }}</n-button>
          <n-button type="error" ghost v-permission="['log:login:clear']" @click="confirmClear">{{ t('loginLog.clear') }}</n-button>
        </div>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NDataTable, NDatePicker, NEllipsis, NInput, NSelect, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { clearLoginLogs, createExport, listLoginLogs } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { LoginLogInfo } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const rows = ref<LoginLogInfo[]>([])
const retentionDays = ref(0)
const query = reactive({ username: '', ip: '', status: null as number | null, page: 1, page_size: 10 })
// 时间范围（毫秒时间戳二元组，提交时转秒）
const range = ref<[number, number] | null>(null)

const statusOptions = computed(() => [
  { label: t('loginLog.success'), value: 1 },
  { label: t('loginLog.failed'), value: 0 },
])

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listLoginLogs({
      page: query.page,
      page_size: query.page_size,
      username: query.username || undefined,
      ip: query.ip || undefined,
      status: query.status ?? undefined,
      start: range.value ? Math.floor(range.value[0] / 1000) : undefined,
      end: range.value ? Math.floor(range.value[1] / 1000) : undefined,
    })
    rows.value = data.data.list
    retentionDays.value = data.data.retention_days
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.username = ''
  query.ip = ''
  query.status = null
  range.value = null
  query.page = 1
  load()
}

// 异步导出：提交当前过滤条件（与列表查询一致，剔除分页参数）
const exporting = ref(false)
async function doExport() {
  exporting.value = true
  try {
    await createExport('login-logs', {
      username: query.username,
      ip: query.ip,
      status: query.status,
      start: range.value ? Math.floor(range.value[0] / 1000) : undefined,
      end: range.value ? Math.floor(range.value[1] / 1000) : undefined,
    })
    message.success(t('loginLog.exportQueued'))
  } catch (e: any) {
    if (e?.response?.status === 429) {
      message.warning(t('loginLog.exportTooMany'))
    } else {
      message.error(e?.response?.data?.msg || t('loginLog.exportFailed'))
    }
  } finally {
    exporting.value = false
  }
}

function confirmClear() {
  // 与后端保留期自动清理同一截止时间（retentionDays 天前）；未启用保留期时退化为清空全部
  const content =
    retentionDays.value > 0
      ? t('loginLog.clearConfirmContent', { days: retentionDays.value })
      : t('loginLog.clearAllConfirmContent')
  dialog.warning({
    title: t('loginLog.clearConfirmTitle'),
    content,
    positiveText: t('loginLog.clear'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const { data } = await clearLoginLogs()
        message.success(t('loginLog.clearSuccess', { n: data.data.deleted }))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('loginLog.clearFailed'))
      }
    },
  })
}

function deviceLabel(device: string) {
  return device === 'app' ? t('loginLog.deviceApp') : t('loginLog.deviceWeb')
}

const columns = computed<DataTableColumns<LoginLogInfo>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('loginLog.username'), key: 'username', width: 140, render: (row) => row.username || '—' },
  { title: 'IP', key: 'ip', width: 130 },
  {
    title: t('loginLog.device'), key: 'device', width: 90,
    render: (row) => (row.device ? h(NTag, { type: row.device === 'app' ? 'success' : 'info', size: 'small' }, { default: () => deviceLabel(row.device) }) : '—'),
  },
  {
    title: t('loginLog.browser'), key: 'user_agent',
    render: (row) => h(NEllipsis, { style: 'max-width: 200px', tooltip: true }, { default: () => row.user_agent || '—' }),
  },
  {
    title: t('loginLog.result'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('loginLog.success') : t('loginLog.failed')) }),
  },
  { title: t('loginLog.msg'), key: 'msg', render: (row) => row.msg || '—' },
  { title: t('loginLog.time'), key: 'created_at', width: 170 },
])

// 保留策略说明（与后端保留期自动清理共用同一配置）
const retentionHint = computed(() =>
  retentionDays.value > 0 ? t('loginLog.retentionDays', { days: retentionDays.value }) : t('loginLog.retentionForever'),
)

onMounted(load)
</script>

<style scoped>
/* 卡头左侧保留说明、右侧清空按钮（页面标题由顶栏展示） */
.page-header {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.retention-hint {
  font-size: 13px;
  color: var(--sx-muted);
}
.page-actions {
  display: flex;
  gap: 10px;
}
</style>
