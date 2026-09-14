<template>
  <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
  <SearchCard storage-key="merchantApiLogs" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.app_key" :placeholder="t('merchant.log.appKeyPlaceholder')" clearable style="width: 200px" @keyup.enter="load" />
    <n-input v-model:value="query.path" :placeholder="t('merchant.log.pathPlaceholder')" clearable style="width: 180px" @keyup.enter="load" />
    <n-input-number v-model:value="query.status_code" :placeholder="t('merchant.log.statusCodePlaceholder')" clearable :min="100" :max="599" :show-button="false" style="width: 120px" />
    <n-date-picker v-model:value="range" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
  </SearchCard>

  <n-card>
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NCard, NDataTable, NDatePicker, NEllipsis, NInput, NInputNumber, NTag, useMessage, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { listMerchantAPILogs } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { MerchantAPILog } from '../../api/types'

const { t } = useI18n()
const message = useMessage()

const loading = ref(false)
const rows = ref<MerchantAPILog[]>([])
const query = reactive({ app_key: '', path: '', status_code: null as number | null, page: 1, page_size: 10 })
// 时间范围（毫秒时间戳二元组，提交时转秒）
const range = ref<[number, number] | null>(null)

const { pagination, setTotal } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listMerchantAPILogs({
      page: query.page,
      page_size: query.page_size,
      app_key: query.app_key || undefined,
      path: query.path || undefined,
      status_code: query.status_code ?? undefined,
      start: range.value ? Math.floor(range.value[0] / 1000) : undefined,
      end: range.value ? Math.floor(range.value[1] / 1000) : undefined,
    })
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.app_key = ''
  query.path = ''
  query.status_code = null
  range.value = null
  query.page = 1
  load()
}

const columns = computed<DataTableColumns<MerchantAPILog>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('merchant.appKey'), key: 'app_key', width: 200 },
  { title: t('merchant.log.method'), key: 'method', width: 80 },
  {
    title: t('merchant.log.path'), key: 'path',
    render: (row) => h(NEllipsis, { style: 'max-width: 240px', tooltip: true }, { default: () => row.path }),
  },
  { title: t('merchant.log.ip'), key: 'ip', width: 130 },
  {
    title: t('merchant.log.statusCode'), key: 'status_code', width: 90,
    render: (row) => h(NTag, { type: row.status_code >= 200 && row.status_code < 300 ? 'success' : 'error', size: 'small' }, { default: () => row.status_code }),
  },
  { title: t('merchant.log.latency'), key: 'latency_ms', width: 90, render: (row) => `${row.latency_ms} ms` },
  {
    title: t('merchant.log.msg'), key: 'msg',
    render: (row) => h(NEllipsis, { style: 'max-width: 200px', tooltip: true }, { default: () => row.msg || '—' }),
  },
  { title: t('merchant.log.time'), key: 'created_at', width: 170 },
])

onMounted(load)
</script>
