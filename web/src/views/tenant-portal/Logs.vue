<!-- 租户门户审计日志（log:list 权限）：登录/操作两个只读表格，数据走门户独立 axios 实例（api/tenant.ts）。
     只读口径：无清空/导出入口；保留期说明由列表响应 retention_days 驱动。 -->
<template>
  <n-tabs v-model:value="tab" type="line" style="margin-bottom: 8px">
    <n-tab-pane name="login" :tab="t('tenantPortal.logs.tabLogin')">
      <SearchCard :key="'login-search'" storage-key="tenant-portal-login-logs" @search="loginSearch" @reset="resetLoginQuery">
        <n-input v-model:value="loginQuery.username" :placeholder="t('tenantPortal.logs.username')" clearable style="width: 160px" @keyup.enter="loginSearch" />
        <n-input v-model:value="loginQuery.ip" :placeholder="t('tenantPortal.logs.ipPlaceholder')" clearable style="width: 150px" @keyup.enter="loginSearch" />
        <n-select v-model:value="loginQuery.status" :options="statusOptions" clearable :placeholder="t('tenantPortal.logs.statusPlaceholder')" style="width: 120px" />
        <n-date-picker v-model:value="loginRange" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
      </SearchCard>

      <n-card :key="'login-table'">
        <template #header>
          <span class="retention-hint">{{ loginRetentionHint }}</span>
        </template>
        <n-data-table :columns="loginColumns" :data="loginRows" :loading="loginLoading" :pagination="loginPagination" paginate-single-page remote />
      </n-card>
    </n-tab-pane>

    <n-tab-pane name="operation" :tab="t('tenantPortal.logs.tabOperation')">
      <SearchCard :key="'op-search'" storage-key="tenant-portal-operation-logs" @search="opSearch" @reset="resetOpQuery">
        <n-input v-model:value="opQuery.username" :placeholder="t('tenantPortal.logs.username')" clearable style="width: 150px" @keyup.enter="opSearch" />
        <n-select v-model:value="opQuery.method" :options="methodOptions" clearable :placeholder="t('tenantPortal.logs.methodPlaceholder')" style="width: 130px" />
        <n-input v-model:value="opQuery.kw" :placeholder="t('tenantPortal.logs.kwPlaceholder')" clearable style="width: 190px" @keyup.enter="opSearch" />
        <n-date-picker v-model:value="opRange" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
      </SearchCard>

      <n-card :key="'op-table'">
        <template #header>
          <span class="retention-hint">{{ opRetentionHint }}</span>
        </template>
        <n-data-table :columns="opColumns" :data="opRows" :loading="opLoading" :pagination="opPagination" paginate-single-page remote />
      </n-card>
    </n-tab-pane>
  </n-tabs>

  <!-- 操作详情（只读） -->
  <n-modal v-model:show="showDetail" preset="dialog" :title="t('tenantPortal.logs.detailTitle')" style="width: 560px">
    <div v-if="detail" class="detail">
      <div class="detail-row"><span class="detail-label">{{ t('tenantPortal.logs.username') }}</span><span>{{ detail.username || '—' }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('tenantPortal.logs.action') }}</span><span>{{ detail.action }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('tenantPortal.logs.api') }}</span><span class="sx-mono">{{ detail.method }} {{ detail.path }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('tenantPortal.logs.route') }}</span><span class="sx-mono">{{ detail.route || '—' }}</span></div>
      <div class="detail-row"><span class="detail-label">IP</span><span>{{ detail.ip }}</span></div>
      <div class="detail-row"><span class="detail-label">{{ t('tenantPortal.logs.terminal') }}</span><span>{{ detail.user_agent || '—' }}</span></div>
      <div class="detail-row">
        <span class="detail-label">{{ t('tenantPortal.logs.statusCode') }}</span>
        <span>{{ detail.status_code }}（{{ detail.latency_ms }}ms）</span>
      </div>
      <div class="detail-row"><span class="detail-label">{{ t('tenantPortal.logs.time') }}</span><span>{{ detail.created_at }}</span></div>
      <div class="detail-label" style="margin-top: 8px">{{ t('tenantPortal.logs.paramsLabel') }}</div>
      <pre class="detail-params sx-mono">{{ detail.params || t('tenantPortal.logs.paramsEmpty') }}</pre>
    </div>
    <template #action>
      <n-button @click="showDetail = false">{{ t('common.close') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton, NCard, NDataTable, NDatePicker, NEllipsis, NInput, NModal, NSelect, NTabPane, NTabs, NTag,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { listTenantLoginLogs, listTenantOperationLogs } from '../../api/tenant'
import type { TenantLoginLog, TenantOperationLog } from '../../api/tenant'
import { usePagination } from '../../utils/pagination'

const { t } = useI18n()

const tab = ref('login')

const statusOptions = computed(() => [
  { label: t('tenantPortal.logs.success'), value: 1 },
  { label: t('tenantPortal.logs.failed'), value: 0 },
])

const methodOptions = [
  { label: 'POST', value: 'POST' },
  { label: 'PUT', value: 'PUT' },
  { label: 'DELETE', value: 'DELETE' },
  { label: 'PATCH', value: 'PATCH' },
]

// 请求方式标签配色：POST 新增绿、PUT 修改橙、DELETE 删除红
const methodTagType = (m: string): 'success' | 'warning' | 'error' | 'info' =>
  m === 'POST' ? 'success' : m === 'PUT' ? 'warning' : m === 'DELETE' ? 'error' : 'info'

// ---- 登录日志 ----
const loginLoading = ref(false)
const loginRows = ref<TenantLoginLog[]>([])
const loginRetentionDays = ref(0)
const loginQuery = reactive({ username: '', ip: '', status: null as number | null, page: 1, page_size: 10 })
// 时间范围（毫秒时间戳二元组，提交时转秒）
const loginRange = ref<[number, number] | null>(null)

const { pagination: loginPagination, setTotal: setLoginTotal, runSearch: loginSearch } = usePagination(loginQuery, loadLoginLogs)

async function loadLoginLogs() {
  loginLoading.value = true
  try {
    const { data } = await listTenantLoginLogs({
      page: loginQuery.page,
      page_size: loginQuery.page_size,
      username: loginQuery.username || undefined,
      ip: loginQuery.ip || undefined,
      status: loginQuery.status ?? undefined,
      start: loginRange.value ? Math.floor(loginRange.value[0] / 1000) : undefined,
      end: loginRange.value ? Math.floor(loginRange.value[1] / 1000) : undefined,
    })
    loginRows.value = data.data.list
    loginRetentionDays.value = data.data.retention_days
    loginPagination.page = loginQuery.page
    loginPagination.pageSize = loginQuery.page_size
    setLoginTotal(data.data.page.total)
  } finally {
    loginLoading.value = false
  }
}

function resetLoginQuery() {
  loginQuery.username = ''
  loginQuery.ip = ''
  loginQuery.status = null
  loginRange.value = null
  loginQuery.page = 1
  loadLoginLogs()
}

const loginColumns = computed<DataTableColumns<TenantLoginLog>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantPortal.logs.username'), key: 'username', width: 140, render: (row) => row.username || '—' },
  { title: 'IP', key: 'ip', width: 130 },
  {
    title: t('tenantPortal.logs.browser'), key: 'user_agent',
    render: (row) => h(NEllipsis, { style: 'max-width: 200px', tooltip: true }, { default: () => row.user_agent || '—' }),
  },
  {
    title: t('tenantPortal.logs.result'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('tenantPortal.logs.success') : t('tenantPortal.logs.failed')) }),
  },
  { title: t('tenantPortal.logs.msg'), key: 'msg', render: (row) => row.msg || '—' },
  { title: t('tenantPortal.logs.time'), key: 'created_at', width: 170 },
])

// ---- 操作日志（切到该 tab 时首载） ----
const opLoading = ref(false)
const opRows = ref<TenantOperationLog[]>([])
const opRetentionDays = ref(0)
const opQuery = reactive({ username: '', method: null as string | null, kw: '', page: 1, page_size: 10 })
const opRange = ref<[number, number] | null>(null)
const opLoaded = ref(false)
const showDetail = ref(false)
const detail = ref<TenantOperationLog | null>(null)

const { pagination: opPagination, setTotal: setOpTotal, runSearch: opSearch } = usePagination(opQuery, loadOpLogs)

async function loadOpLogs() {
  opLoading.value = true
  try {
    const { data } = await listTenantOperationLogs({
      page: opQuery.page,
      page_size: opQuery.page_size,
      username: opQuery.username || undefined,
      method: opQuery.method || undefined,
      kw: opQuery.kw || undefined,
      start: opRange.value ? Math.floor(opRange.value[0] / 1000) : undefined,
      end: opRange.value ? Math.floor(opRange.value[1] / 1000) : undefined,
    })
    opRows.value = data.data.list
    opRetentionDays.value = data.data.retention_days
    opPagination.page = opQuery.page
    opPagination.pageSize = opQuery.page_size
    setOpTotal(data.data.page.total)
  } finally {
    opLoading.value = false
  }
}

function resetOpQuery() {
  opQuery.username = ''
  opQuery.method = null
  opQuery.kw = ''
  opRange.value = null
  opQuery.page = 1
  loadOpLogs()
}

function openDetail(row: TenantOperationLog) {
  detail.value = row
  showDetail.value = true
}

const opColumns = computed<DataTableColumns<TenantOperationLog>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantPortal.logs.username'), key: 'username', width: 120, render: (row) => row.username || '—' },
  { title: t('tenantPortal.logs.action'), key: 'action', width: 130 },
  {
    title: t('tenantPortal.logs.api'), key: 'path',
    render: (row) =>
      h('span', { style: 'display: inline-flex; align-items: center; gap: 6px; min-width: 0' }, [
        h(NTag, { type: methodTagType(row.method), size: 'small' }, { default: () => row.method }),
        h(NEllipsis, { style: 'max-width: 260px', tooltip: true }, { default: () => row.path }),
      ]),
  },
  { title: 'IP', key: 'ip', width: 125 },
  {
    title: t('tenantPortal.logs.statusCode'), key: 'status_code', width: 90,
    render: (row) =>
      h(NTag, { type: row.status_code < 400 ? 'success' : 'error', size: 'small', bordered: false }, { default: () => String(row.status_code) }),
  },
  { title: t('tenantPortal.logs.latency'), key: 'latency_ms', width: 80, render: (row) => `${row.latency_ms}ms` },
  { title: t('tenantPortal.logs.time'), key: 'created_at', width: 170 },
  {
    title: t('common.operation'), key: 'actions', width: 120,
    render(row) {
      const actions: Array<TableAction> = [{ label: t('common.detail'), accent: true, onClick: () => openDetail(row) }]
      return renderActions(actions)
    },
  },
])

// 保留策略说明（retention_days 由列表响应带回；0=未启用自动清理）
const loginRetentionHint = computed(() =>
  loginRetentionDays.value > 0 ? t('tenantPortal.logs.retentionDays', { days: loginRetentionDays.value }) : t('tenantPortal.logs.retentionForever'),
)
const opRetentionHint = computed(() =>
  opRetentionDays.value > 0 ? t('tenantPortal.logs.retentionDays', { days: opRetentionDays.value }) : t('tenantPortal.logs.retentionForever'),
)

// 操作日志 tab 首次激活时再加载，避免进页即双请求
watch(tab, (v) => {
  if (v === 'operation' && !opLoaded.value) {
    opLoaded.value = true
    loadOpLogs()
  }
})

onMounted(loadLoginLogs)
</script>

<style scoped>
.retention-hint {
  font-size: 13px;
  color: var(--sx-muted);
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
