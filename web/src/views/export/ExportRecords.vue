<template>
  <n-card>
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, onBeforeUnmount, reactive, ref, type VNode } from 'vue'
import { NButton, NCard, NDataTable, NTag, NTooltip, useMessage, useDialog, type DataTableColumns, type TagProps } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { useI18n } from 'vue-i18n'
import { listExportRecords, getExportBlob, deleteExport } from '../../api'
import { usePagination } from '../../utils/pagination'
import { saveBlob, parseDispositionFilename } from '../../utils/download'
import type { ExportRecord } from '../../api/types'

const { t } = useI18n()

const message = useMessage()
const dialog = useDialog()
const loading = ref(false)
const rows = ref<ExportRecord[]>([])
const downloadingId = ref(0)
const query = reactive({ page: 1, page_size: 10 })

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listExportRecords(query)
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
  syncPolling()
}

// 有进行中任务时每 5s 自动刷新当前页；无进行中任务即停表
let pollTimer: number | undefined
function syncPolling() {
  window.clearInterval(pollTimer)
  pollTimer = undefined
  if (rows.value.some((r) => r.status === 'pending' || r.status === 'running')) {
    pollTimer = window.setInterval(load, 5000)
  }
}

// 文件名优先取 Content-Disposition（RFC5987），取不到用记录名加 .csv 兜底
async function download(row: ExportRecord) {
  downloadingId.value = row.id
  try {
    const resp = await getExportBlob(row.id)
    const filename = parseDispositionFilename(resp.headers['content-disposition']) || `${row.name}.csv`
    saveBlob(resp.data, filename)
  } catch {
    message.error(t('exportRecords.downloadFailed'))
  } finally {
    downloadingId.value = 0
  }
}

function confirmDelete(row: ExportRecord) {
  dialog.warning({
    title: t('exportRecords.deleteConfirmTitle'),
    content: t('exportRecords.deleteConfirmContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteExport(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('exportRecords.deleteFailed'))
      }
    },
  })
}

const bizNames = computed<Record<string, string>>(() => ({
  user: t('exportRecords.biz.user'), login_log: t('exportRecords.biz.loginLog'), op_log: t('exportRecords.biz.opLog'),
}))

const statusType = (s: ExportRecord['status']): TagProps['type'] =>
  s === 'pending' ? 'info' : s === 'running' ? 'warning' : s === 'done' ? 'success' : 'error'
const statusText = computed<Record<ExportRecord['status'], string>>(() => ({
  pending: t('exportRecords.status.pending'),
  running: t('exportRecords.status.running'),
  done: t('exportRecords.status.done'),
  failed: t('exportRecords.status.failed'),
}))

function fmtSize(n: number) {
  if (!n) return '—'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

// 状态标签：失败行 hover 显示错误原因
function renderStatus(row: ExportRecord): VNode {
  const tag = h(NTag, { type: statusType(row.status), size: 'small' }, { default: () => statusText.value[row.status] })
  if (row.status === 'failed' && row.error) {
    return h(NTooltip, null, { trigger: () => tag, default: () => row.error })
  }
  return tag
}

const columns = computed<DataTableColumns<ExportRecord>>(() => [
  { title: t('exportRecords.name'), key: 'name', ellipsis: { tooltip: true } },
  {
    title: t('exportRecords.bizType'), key: 'biz', width: 110,
    render: (row) => h(NTag, { size: 'small', bordered: false }, { default: () => bizNames.value[row.biz] || row.biz }),
  },
  { title: t('common.status'), key: 'status', width: 90, render: renderStatus },
  {
    title: t('exportRecords.rows'), key: 'rows', width: 120,
    render: (row) =>
      h('span', { style: 'display: inline-flex; align-items: center; gap: 6px' }, [
        String(row.rows),
        row.truncated
          ? h(NTag, { type: 'warning', size: 'tiny', bordered: false }, { default: () => t('exportRecords.truncated') })
          : null,
      ]),
  },
  { title: t('exportRecords.size'), key: 'size', width: 100, render: (row) => fmtSize(row.size) },
  { title: t('common.createTime'), key: 'created_at', width: 170 },
  { title: t('exportRecords.finishTime'), key: 'finished_at', width: 170, render: (row) => row.finished_at || '—' },
  {
    title: t('common.operation'), key: 'actions', width: 110,
    render(row) {
      const actions: Array<TableAction | VNode> = []
      if (row.status === 'done') {
        actions.push({
          label: downloadingId.value === row.id ? t('exportRecords.downloading') : t('common.download'),
          accent: true,
          onClick: () => { if (!downloadingId.value) download(row) },
        })
      }
      actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

onMounted(load)
onBeforeUnmount(() => window.clearInterval(pollTimer))
</script>
