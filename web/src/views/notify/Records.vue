<template>
  <!-- 告警发送记录：筛选 + 清空；行查看 Markdown 内容 -->
  <SearchCard storage-key="notify-records" @search="search" @reset="resetQuery">
    <n-select v-model:value="query.channel_id" :options="channelFilterOptions" :placeholder="t('notify.record.allChannels')" clearable style="width: 180px" />
    <n-select v-model:value="query.source" :options="sourceFilterOptions" :placeholder="t('notify.record.allSources')" clearable style="width: 150px" />
    <n-select v-model:value="query.status" :options="statusFilterOptions" :placeholder="t('notify.record.allStatus')" clearable style="width: 130px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['notify:record:clear']" type="error" ghost @click="confirmClear">
          {{ t('notify.record.clear') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :pagination="pagination" remote :bordered="false" />
  </n-card>

  <n-modal v-model:show="showDetail" preset="card" :title="detail?.title ?? ''" style="width: 640px" :bordered="false" size="small">
    <div class="detail-meta">
      <n-tag size="small" :bordered="false" :type="detail?.status === 'sent' ? 'success' : 'error'">
        {{ detail?.status === 'sent' ? t('notify.record.sent') : t('notify.record.failed') }}
      </n-tag>
      <span>{{ detail?.channel_name }} · {{ detail?.rule_name }} · {{ detail?.created_at?.slice(0, 19).replace('T', ' ') }} · {{ detail?.duration_ms }}ms</span>
    </div>
    <div v-if="detail?.error" class="detail-error">{{ detail.error }}</div>
    <div class="detail-body" v-html="renderedContent" />
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NModal, NSelect, NTag, NTooltip, useDialog, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { clearNotifyRecords, listNotifyChannels, listNotifyRecords } from '../../api'
import type { NotifyRecord } from '../../api/types'
import { renderMarkdown } from '../../utils/markdown'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const query = reactive<{ channel_id: number | null; source: string | null; status: string | null; page: number; page_size: number }>({
  channel_id: null, source: null, status: null, page: 1, page_size: 10,
})
const rows = ref<NotifyRecord[]>([])
const loading = ref(false)
const { pagination, setTotal, runSearch: search } = usePagination(query, () => load())

const sourceLabels = computed(() => ({
  job_failed: t('notify.rule.sourceJobFailed'),
  monitor: t('notify.rule.sourceMonitor'),
  ip_autoban: t('notify.rule.sourceIPAutoban'),
  test: t('notify.record.sourceTest'),
}))

// 渠道类型标签（邮件/通用 Webhook/企业微信/钉钉/飞书）
const channelTypeLabel = (v: string) => ({
  email: t('notify.channel.typeEmail'),
  webhook: t('notify.channel.typeWebhook'),
  wecom: t('notify.channel.typeWecom'),
  dingtalk: t('notify.channel.typeDingtalk'),
  feishu: t('notify.channel.typeFeishu'),
} as Record<string, string>)[v] ?? v

const channels = ref<{ id: number; name: string }[]>([])
const channelFilterOptions = computed(() => channels.value.map((c) => ({ label: c.name, value: c.id })))
const sourceFilterOptions = computed(() => [
  { label: t('notify.rule.sourceJobFailed'), value: 'job_failed' },
  { label: t('notify.rule.sourceMonitor'), value: 'monitor' },
  { label: t('notify.rule.sourceIPAutoban'), value: 'ip_autoban' },
  { label: t('notify.record.sourceTest'), value: 'test' },
])
const statusFilterOptions = computed(() => [
  { label: t('notify.record.sent'), value: 'sent' },
  { label: t('notify.record.failed'), value: 'failed' },
])

const columns = computed<DataTableColumns<NotifyRecord>>(() => [
  { title: t('notify.record.time'), key: 'created_at', width: 150, render: (row) => row.created_at?.slice(0, 19).replace('T', ' ') ?? '—' },
  {
    title: t('notify.record.channel'), key: 'channel_name', width: 150, ellipsis: { tooltip: true },
    render: (row) => `${row.channel_name}（${channelTypeLabel(row.channel_type)}）`,
  },
  { title: t('notify.record.rule'), key: 'rule_name', width: 110, ellipsis: { tooltip: true } },
  { title: t('notify.record.source'), key: 'source', width: 100, render: (row) => sourceLabels.value[row.source] ?? row.source },
  { title: t('notify.record.titleField'), key: 'title', minWidth: 180, ellipsis: { tooltip: true } },
  {
    title: t('notify.record.status'), key: 'status', width: 90,
    render: (row) => {
      const tag = h(NTag, { size: 'small', bordered: false, type: row.status === 'sent' ? 'success' : 'error' },
        { default: () => row.status === 'sent' ? t('notify.record.sent') : t('notify.record.failed') })
      if (row.status === 'failed' && row.error) {
        return h(NTooltip, null, { trigger: () => tag, default: () => row.error })
      }
      return tag
    },
  },
  { title: t('notify.record.duration'), key: 'duration_ms', width: 80, render: (row) => `${row.duration_ms}ms` },
  {
    title: t('common.actions'), key: 'actions', width: 80,
    render: (row) => renderActions([{ label: t('notify.record.view'), onClick: () => openDetail(row) }]),
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listNotifyRecords({
      page: query.page, page_size: query.page_size,
      channel_id: query.channel_id ?? undefined, source: query.source ?? undefined, status: query.status ?? undefined,
    })
    rows.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { channel_id: null, source: null, status: null, page: 1 })
  load()
}

const showDetail = ref(false)
const detail = ref<NotifyRecord>()
const renderedContent = computed(() => (detail.value ? renderMarkdown(detail.value.content || '') : ''))

function openDetail(row: NotifyRecord) {
  detail.value = row
  showDetail.value = true
}

function confirmClear() {
  dialog.warning({
    title: t('common.tips'),
    content: t('notify.record.clearConfirm'),
    positiveText: t('notify.record.clear'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await clearNotifyRecords()
        message.success(t('common.success'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

onMounted(() => {
  listNotifyChannels().then((res) => {
    channels.value = res.data.data.map((c) => ({ id: c.id, name: c.name }))
  }).catch(() => { /* 筛选项加载失败不阻断 */ })
  load()
})
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}
.detail-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  font-size: 13px;
  opacity: 0.75;
}
.detail-error {
  margin-bottom: 10px;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 13px;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
  background: rgba(240, 80, 60, 0.08);
}
.detail-body {
  font-size: 13px;
  line-height: 1.7;
  word-break: break-word;
}
</style>
