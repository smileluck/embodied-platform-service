<!-- 管理端「租户日志」：平台租户门户的登录/操作日志审计（菜单码 menu:tenantLog，菜单名走后端种子）。
     与门户 Logs.vue 同构，差异：多租户下拉（tenant_id 为平台租户 ID 精确筛选）+ 按权限点渲染的清空按钮。 -->
<template>
  <n-tabs v-model:value="tab" type="line" style="margin-bottom: 8px">
    <n-tab-pane name="login" :tab="t('tenantLog.tabLogin')">
      <!-- 搜索栏独立卡片：可折叠，重置/搜索按钮在卡片右下角 -->
      <SearchCard :key="'login-search'" storage-key="tenantLoginLogs" @search="loginSearch" @reset="resetLoginQuery">
        <n-select v-model:value="loginQuery.tenant_id" :options="tenantOptions" clearable filterable :placeholder="t('tenantLog.tenantPlaceholder')" style="width: 180px" />
        <n-input v-model:value="loginQuery.username" :placeholder="t('loginLog.username')" clearable style="width: 160px" @keyup.enter="loginSearch" />
        <n-input v-model:value="loginQuery.ip" :placeholder="t('loginLog.ipPlaceholder')" clearable style="width: 150px" @keyup.enter="loginSearch" />
        <n-select v-model:value="loginQuery.status" :options="statusOptions" clearable :placeholder="t('loginLog.statusPlaceholder')" style="width: 120px" />
        <n-date-picker v-model:value="loginRange" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
      </SearchCard>

      <n-card :key="'login-table'">
        <template #header>
          <div class="page-header">
            <span class="retention-hint">{{ loginRetentionHint }}</span>
            <div class="page-actions">
              <n-button type="error" ghost v-permission="['log:tenantLogin:clear']" @click="confirmClearLogin">{{ t('loginLog.clear') }}</n-button>
            </div>
          </div>
        </template>
        <n-data-table :columns="loginColumns" :data="loginRows" :loading="loginLoading" :pagination="loginPagination" paginate-single-page remote />
      </n-card>
    </n-tab-pane>

    <n-tab-pane name="operation" :tab="t('tenantLog.tabOperation')">
      <SearchCard :key="'op-search'" storage-key="tenantOperationLogs" @search="opSearch" @reset="resetOpQuery">
        <n-select v-model:value="opQuery.tenant_id" :options="tenantOptions" clearable filterable :placeholder="t('tenantLog.tenantPlaceholder')" style="width: 180px" />
        <n-input v-model:value="opQuery.username" :placeholder="t('opLog.username')" clearable style="width: 150px" @keyup.enter="opSearch" />
        <n-select v-model:value="opQuery.method" :options="methodOptions" clearable :placeholder="t('opLog.methodPlaceholder')" style="width: 130px" />
        <n-input v-model:value="opQuery.kw" :placeholder="t('opLog.kwPlaceholder')" clearable style="width: 190px" @keyup.enter="opSearch" />
        <n-date-picker v-model:value="opRange" type="datetimerange" clearable style="width: 340px; max-width: 100%" />
      </SearchCard>

      <n-card :key="'op-table'">
        <template #header>
          <div class="page-header">
            <span class="retention-hint">{{ opRetentionHint }}</span>
            <div class="page-actions">
              <n-button type="error" ghost v-permission="['log:tenantOp:clear']" @click="confirmClearOp">{{ t('opLog.clear') }}</n-button>
            </div>
          </div>
        </template>
        <n-data-table :columns="opColumns" :data="opRows" :loading="opLoading" :pagination="opPagination" paginate-single-page remote />
      </n-card>
    </n-tab-pane>
  </n-tabs>

  <!-- 操作详情 -->
  <n-modal v-model:show="showDetail" preset="dialog" :title="t('opLog.detailTitle')" style="width: 560px">
    <div v-if="detail" class="detail">
      <div class="detail-row"><span class="detail-label">{{ t('tenantLog.tenant') }}</span><span>{{ tenantName(detail.tenant_id) }}</span></div>
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
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton, NCard, NDataTable, NDatePicker, NEllipsis, NInput, NModal, NSelect, NTabPane, NTabs, NTag,
  useDialog, useMessage, type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import {
  clearTenantLoginLogs, clearTenantOperationLogs, listTenantLoginLogs, listTenantOperationLogs, listTenants,
} from '../../api'
import { usePagination } from '../../utils/pagination'
import type { TenantLoginLog, TenantOperationLog } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const tab = ref('login')

const statusOptions = computed(() => [
  { label: t('loginLog.success'), value: 1 },
  { label: t('loginLog.failed'), value: 0 },
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

// 租户下拉（tenant_id 为平台租户 ID 口径：仅已同步 platform_id>0 的租户可筛）
const tenantOptions = ref<{ label: string; value: number }[]>([])
const tenantName = (id: number) => tenantOptions.value.find((o) => o.value === id)?.label || `#${id}`

async function loadTenantOptions() {
  try {
    const { data } = await listTenants({ page: 1, page_size: 0, status: 1 })
    tenantOptions.value = data.data.list
      .filter((x) => x.platform_id > 0)
      .map((x) => ({ label: x.name, value: x.platform_id }))
  } catch {
    // 下拉选项加载失败不阻塞页面，列表照常可用
  }
}

// ---- 登录日志 ----
const loginLoading = ref(false)
const loginRows = ref<TenantLoginLog[]>([])
const loginRetentionDays = ref(0)
const loginQuery = reactive({ tenant_id: null as number | null, username: '', ip: '', status: null as number | null, page: 1, page_size: 10 })
// 时间范围（毫秒时间戳二元组，提交时转秒）
const loginRange = ref<[number, number] | null>(null)

const { pagination: loginPagination, setTotal: setLoginTotal, runSearch: loginSearch } = usePagination(loginQuery, loadLoginLogs)

async function loadLoginLogs() {
  loginLoading.value = true
  try {
    const { data } = await listTenantLoginLogs({
      page: loginQuery.page,
      page_size: loginQuery.page_size,
      tenant_id: loginQuery.tenant_id ?? undefined,
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
  loginQuery.tenant_id = null
  loginQuery.username = ''
  loginQuery.ip = ''
  loginQuery.status = null
  loginRange.value = null
  loginQuery.page = 1
  loadLoginLogs()
}

function confirmClearLogin() {
  // 与后端保留期自动清理同一截止时间（retentionDays 天前）；未启用保留期时退化为清空全部
  const content =
    loginRetentionDays.value > 0
      ? t('loginLog.clearConfirmContent', { days: loginRetentionDays.value })
      : t('loginLog.clearAllConfirmContent')
  dialog.warning({
    title: t('loginLog.clearConfirmTitle'),
    content,
    positiveText: t('loginLog.clear'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const { data } = await clearTenantLoginLogs()
        message.success(t('loginLog.clearSuccess', { n: data.data.deleted }))
        loadLoginLogs()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('loginLog.clearFailed'))
      }
    },
  })
}

const loginColumns = computed<DataTableColumns<TenantLoginLog>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantLog.tenant'), key: 'tenant_id', width: 130, render: (row) => tenantName(row.tenant_id) },
  { title: t('loginLog.username'), key: 'username', width: 140, render: (row) => row.username || '—' },
  { title: 'IP', key: 'ip', width: 130 },
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

// ---- 操作日志（切到该 tab 时首载） ----
const opLoading = ref(false)
const opRows = ref<TenantOperationLog[]>([])
const opRetentionDays = ref(0)
const opQuery = reactive({ tenant_id: null as number | null, username: '', method: null as string | null, kw: '', page: 1, page_size: 10 })
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
      tenant_id: opQuery.tenant_id ?? undefined,
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
  opQuery.tenant_id = null
  opQuery.username = ''
  opQuery.method = null
  opQuery.kw = ''
  opRange.value = null
  opQuery.page = 1
  loadOpLogs()
}

function confirmClearOp() {
  const content =
    opRetentionDays.value > 0
      ? t('opLog.clearConfirmContent', { days: opRetentionDays.value })
      : t('opLog.clearAllConfirmContent')
  dialog.warning({
    title: t('opLog.clearConfirmTitle'),
    content,
    positiveText: t('opLog.clear'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        const { data } = await clearTenantOperationLogs()
        message.success(t('opLog.clearSuccess', { n: data.data.deleted }))
        loadOpLogs()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('opLog.clearFailed'))
      }
    },
  })
}

function openDetail(row: TenantOperationLog) {
  detail.value = row
  showDetail.value = true
}

const opColumns = computed<DataTableColumns<TenantOperationLog>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('tenantLog.tenant'), key: 'tenant_id', width: 120, render: (row) => tenantName(row.tenant_id) },
  { title: t('opLog.username'), key: 'username', width: 120, render: (row) => row.username || '—' },
  { title: t('opLog.action'), key: 'action', width: 130 },
  {
    title: t('opLog.api'), key: 'path',
    render: (row) =>
      h('span', { style: 'display: inline-flex; align-items: center; gap: 6px; min-width: 0' }, [
        h(NTag, { type: methodTagType(row.method), size: 'small' }, { default: () => row.method }),
        h(NEllipsis, { style: 'max-width: 240px', tooltip: true }, { default: () => row.path }),
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
    title: t('common.operation'), key: 'actions', width: 120,
    render(row) {
      const actions: Array<TableAction> = [{ label: t('common.detail'), accent: true, onClick: () => openDetail(row) }]
      return renderActions(actions)
    },
  },
])

// 保留策略说明（与后端保留期自动清理共用同一配置）
const loginRetentionHint = computed(() =>
  loginRetentionDays.value > 0 ? t('loginLog.retentionDays', { days: loginRetentionDays.value }) : t('loginLog.retentionForever'),
)
const opRetentionHint = computed(() =>
  opRetentionDays.value > 0 ? t('opLog.retentionDays', { days: opRetentionDays.value }) : t('opLog.retentionForever'),
)

// 操作日志 tab 首次激活时再加载，避免进页即双请求
watch(tab, (v) => {
  if (v === 'operation' && !opLoaded.value) {
    opLoaded.value = true
    loadOpLogs()
  }
})

onMounted(() => {
  loadLoginLogs()
  loadTenantOptions()
})
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
