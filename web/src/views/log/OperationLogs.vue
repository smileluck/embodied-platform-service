<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="operationLogs" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.username" :placeholder="t('opLog.username')" clearable style="width: 150px" @keyup.enter="load" />
    <n-select v-model:value="query.method" :options="methodOptions" clearable :placeholder="t('opLog.methodPlaceholder')" style="width: 130px" />
    <n-input v-model:value="query.kw" :placeholder="t('opLog.kwPlaceholder')" clearable style="width: 190px" @keyup.enter="load" />
    <n-date-picker v-model:value="range" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-header">
        <span class="retention-hint">{{ retentionHint }}</span>
        <div class="page-actions">
          <n-button ghost :loading="exporting" v-permission="['log:op:export']" @click="doExport">{{ t('opLog.export') }}</n-button>
          <n-button type="error" ghost v-permission="['log:op:clear']" @click="confirmClear">{{ t('opLog.clear') }}</n-button>
        </div>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 详情 -->
  <n-modal v-model:show="showDetail" preset="dialog" :title="t('opLog.detailTitle')" style="width: 560px">
    <div v-if="detail" class="detail">
      <div class="detail-row"><span class="detail-label">{{ t('opLog.username') }}</span><span>{{ detail.username || '—' }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('opLog.action') }}</span><span>{{ detail.action }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('opLog.api') }}</span><span class="sx-mono">{{ detail.method }} {{ detail.path }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('opLog.route') }}</span><span class="sx-mono">{{ detail.route || '—' }}</span></div>
      <div class="detail-row"><span class="detail-label">IP</span><span>{{ detail.ip }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('opLog.terminal') }}</span><span>{{ detail.user_agent || '—' }}</span></div>
      <div class="detail-row">
        <span class="detail-label">{{ t('opLog.statusCode') }}</span>
        <span>{{ detail.status_code }}（{{ detail.latency_ms }}ms）</span>
      </div>
      <div class="detail-row"><span class="detail-label">{{ t('opLog.time') }}</span><span>{{ detail.created_at }}</span></div>
      <div class="detail-label" style="margin-top: 8px">{{ t('opLog.paramsLabel') }}</div>
      <pre class="detail-params sx-mono">{{ detail.params || t('opLog.paramsEmpty') }}</pre>
    </div>
    <template #action>
      <n-button @click="showDetail = false">{{ t('common.close') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NButton, NCard, NDataTable, NDatePicker, NEllipsis, NInput, NModal, NSelect, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { clearOperationLogs, createExport, listOperationLogs } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { OperationLogInfo } from '../../api/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const rows = ref<OperationLogInfo[]>([])
const retentionDays = ref(0)
const query = reactive({ username: '', method: null as string | null, kw: '', page: 1, page_size: 10 })
// 时间范围（毫秒时间戳二元组，提交时转秒）
const range = ref<[number, number] | null>(null)
const showDetail = ref(false)
const detail = ref<OperationLogInfo | null>(null)

const methodOptions = [
  { label: 'POST', value: 'POST' },
  { label: 'PUT', value: 'PUT' },
  { label: 'DELETE', value: 'DELETE' },
  { label: 'PATCH', value: 'PATCH' },
]

// 请求方式标签配色：POST 新增绿、PUT 修改橙、DELETE 删除红
const methodTagType = (m: string): 'success' | 'warning' | 'error' | 'info' =>
  m === 'POST' ? 'success' : m === 'PUT' ? 'warning' : m === 'DELETE' ? 'error' : 'info'

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listOperationLogs({
      page: query.page,
      page_size: query.page_size,
      username: query.username || undefined,
      method: query.method || undefined,
      kw: query.kw || undefined,
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
  query.method = null
  query.kw = ''
  range.value = null
  query.page = 1
  load()
}

// 异步导出：提交当前过滤条件（与列表查询一致，剔除分页参数）
const exporting = ref(false)
async function doExport() {
  exporting.value = true
  try {
    await createExport('operation-logs', {
      username: query.username,
      method: query.method,
      kw: query.kw,
      start: range.value ? Math.floor(range.value[0] / 1000) : undefined,
      end: range.value ? Math.floor(range.value[1] / 1000) : undefined,
    })
    message.success(t('opLog.exportQueued'))
  } catch (e: any) {
    if (e?.response?.status === 429) {
      message.warning(t('opLog.exportTooMany'))
    } else {
      message.error(e?.response?.data?.msg || t('opLog.exportFailed'))
    }
  } finally {
    exporting.value = false
  }
}

function confirmClear() {
  // 与后端保留期自动清理同一截止时间（retentionDays 天前）；未启用保留期时退化为清空全部
  const content =
    retentionDays.value > 0
      ? t('opLog.clearConfirmContent', { days: retentionDays.value })
      : t('opLog.clearAllConfirmContent')
  dialog.warning({
    title: t('opLog.clearConfirmTitle'),
    content,
    positiveText: t('opLog.clear'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const { data } = await clearOperationLogs()
        message.success(t('opLog.clearSuccess', { n: data.data.deleted }))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('opLog.clearFailed'))
      }
    },
  })
}

function openDetail(row: OperationLogInfo) {
  detail.value = row
  showDetail.value = true
}

const columns = computed<DataTableColumns<OperationLogInfo>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('opLog.username'), key: 'username', width: 120, render: (row) => row.username || '—' },
  { title: t('opLog.action'), key: 'action', width: 130 },
  {
    title: t('opLog.api'), key: 'path',
    render: (row) =>
      h('span', { style: 'display: inline-flex; align-items: center; gap: 6px; min-width: 0' }, [
        h(NTag, { type: methodTagType(row.method), size: 'small' }, { default: () => row.method }),
        h(NEllipsis, { style: 'max-width: 260px', tooltip: true }, { default: () => row.path }),
      ]),
  },
  { title: 'IP', key: 'ip', width: 125 },
  {
    title: t('opLog.statusCode'), key: 'status_code', width: 90,
    render: (row) =>
      h(NTag, { type: row.status_code < 400 ? 'success' : 'error', size: 'small', bordered: false }, { default: () => String(row.status_code) }),
  },
  { title: t('opLog.latency'), key: 'latency_ms', width: 80, render: (row) => `${row.latency_ms}ms` },
  { title: t('opLog.time'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 70,
    render(row) {
      const actions: Array<TableAction> = [{ label: t('common.detail'), accent: true, onClick: () => openDetail(row) }]
      return renderActions(actions)
    },
  },
])

// 保留策略说明（与后端保留期自动清理共用同一配置）
const retentionHint = computed(() =>
  retentionDays.value > 0 ? t('opLog.retentionDays', { days: retentionDays.value }) : t('opLog.retentionForever'),
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
/* 详情弹窗字段行 */
.detail-row {
  display: flex;
  gap: 12px;
  padding: 4px 0;
  word-break: break-all;
}
.detail-label {
  flex: 0 0 60px;
  color: var(--sx-muted);
}
.detail-params {
  margin: 4px 0 0;
  padding: 10px;
  max-height: 240px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--sx-bg);
  border: 1px solid var(--sx-line);
  border-radius: var(--sx-radius);
  font-size: 12px;
}
</style>
