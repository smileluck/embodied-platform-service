<template>
  <!-- MCP 服务管理：搜索独立卡片 + 列表卡片头右侧「新增服务」；行内「测试」真实握手远端并展示工具清单 -->
  <SearchCard storage-key="mcpServers" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('mcp.searchKw')" clearable style="width: 200px" @keyup.enter="search" />
    <n-select v-model:value="query.transport" :options="transportOptions" :placeholder="t('mcp.transport')" clearable style="width: 160px" />
    <n-select v-model:value="query.status" :options="statusOptions" :placeholder="t('common.status')" clearable style="width: 110px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button v-permission="['mcp:server:create']" type="primary" ghost @click="openCreate">
          {{ t('mcp.newServer') }}
        </n-button>
      </div>
    </template>

    <n-data-table size="small" :columns="columns" :data="rows" :loading="loading" :pagination="pagination" remote :bordered="false" />
  </n-card>

  <!-- 新增/编辑：卡片式弹窗；访问凭证密文回显（编辑留空=保持不变），自定义 Header 动态行 -->
  <n-modal v-model:show="showModal" preset="card" :title="editing ? t('mcp.editServer') : t('mcp.newServer')" style="width: 640px" :bordered="false" size="small">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="110">
      <n-form-item :label="t('mcp.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-word-limit :placeholder="t('mcp.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('mcp.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" show-word-limit :placeholder="t('mcp.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('mcp.transport')" path="transport">
        <n-radio-group v-model:value="form.transport" size="small">
          <n-radio-button value="streamable_http">{{ t('mcp.transportHttp') }}</n-radio-button>
          <n-radio-button value="sse">{{ t('mcp.transportSse') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item :label="t('mcp.baseUrl')" path="base_url">
        <n-input v-model:value="form.base_url" :maxlength="255" :placeholder="t('mcp.baseUrlPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('mcp.token')" path="token">
        <n-input
          v-model:value="form.token" type="password" show-password-on="click" :maxlength="255"
          :placeholder="tokenPlaceholder"
        />
      </n-form-item>
      <n-form-item :label="t('mcp.headers')">
        <div class="headers-block">
          <div v-for="(h, i) in form.headers" :key="i" class="header-row">
            <n-input v-model:value="h.key" :maxlength="64" :placeholder="t('mcp.headerKey')" />
            <n-input v-model:value="h.value" :maxlength="512" :placeholder="t('mcp.headerValue')" />
            <n-button size="small" quaternary type="error" @click="form.headers.splice(i, 1)">✕</n-button>
          </div>
          <n-button size="small" dashed block :disabled="form.headers.length >= 10" @click="form.headers.push({ key: '', value: '' })">
            + {{ t('mcp.addHeader') }}
          </n-button>
          <div class="headers-hint">{{ t('mcp.headersHint') }}</div>
        </div>
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="form.remark" :maxlength="200" show-word-limit :placeholder="t('role.remarkPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('common.status')">
        <n-switch v-model:value="form.status" :checked-value="1" :unchecked-value="0" size="small" />
      </n-form-item>
    </n-form>
    <template #action>
      <div class="modal-actions">
        <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
      </div>
    </template>
  </n-modal>

  <!-- 连通测试结果：服务器信息 + 工具清单（失败展示原因） -->
  <n-modal v-model:show="showTestModal" preset="card" :title="`${t('mcp.testTitle')} · ${testTarget?.name ?? ''}`" style="width: 560px" :bordered="false" size="small">
    <template v-if="testResult">
      <n-alert v-if="!testResult.ok" type="error" :show-icon="true">
        {{ t('mcp.testFailed') }}：{{ testResult.error }}
      </n-alert>
      <template v-else>
        <n-descriptions label-placement="left" :column="2" size="small" bordered>
          <n-descriptions-item :label="t('mcp.testServer')">
            {{ testResult.server_name }}{{ testResult.server_version ? ` v${testResult.server_version}` : '' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('mcp.testLatency')">{{ testResult.latency_ms }} ms</n-descriptions-item>
          <n-descriptions-item :label="t('mcp.testProtocol')" :span="2">
            <span class="mono">{{ testResult.protocol_version || '—' }}</span>
          </n-descriptions-item>
        </n-descriptions>
        <div class="tools-title">{{ t('mcp.testTools') }}（{{ testResult.tools.length }}）</div>
        <div v-if="testResult.tools.length === 0" class="tools-empty">{{ t('mcp.testNoTools') }}</div>
        <div v-else class="tools-list">
          <div v-for="tool in testResult.tools" :key="tool.name" class="tool-item">
            <span class="tool-name mono">{{ tool.name }}</span>
            <span v-if="tool.description" class="tool-desc">{{ tool.description }}</span>
          </div>
        </div>
      </template>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NAlert, NButton, NCard, NDataTable, NDescriptions, NDescriptionsItem, NForm, NFormItem,
  NInput, NModal, NRadioButton, NRadioGroup, NSelect, NSwitch, NTag, NTooltip,
  useDialog, useMessage, type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import { createMcpServer, deleteMcpServer, listMcpServers, testMcpServer, updateMcpServer } from '../../api'
import type { McpServer } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const query = reactive({ kw: '', transport: '', status: null as number | null, page: 1, page_size: 10 })
const rows = ref<McpServer[]>([])
const loading = ref(false)
const { pagination, setTotal, runSearch: search } = usePagination(query, () => load())

const transportOptions = computed(() => [
  { label: t('mcp.transportHttp'), value: 'streamable_http' },
  { label: t('mcp.transportSse'), value: 'sse' },
])
const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 0 },
])

const columns = computed<DataTableColumns<McpServer>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('mcp.name'), key: 'name', minWidth: 130, ellipsis: { tooltip: true } },
  { title: t('mcp.code'), key: 'code', width: 120, render: (row) => h('span', { class: 'mono' }, row.code) },
  {
    title: t('mcp.transport'), key: 'transport', width: 130,
    render: (row) => h(NTag, {
      size: 'small', bordered: false,
      type: row.transport === 'streamable_http' ? 'info' : 'warning',
    }, { default: () => (row.transport === 'streamable_http' ? 'HTTP' : 'SSE') }),
  },
  {
    title: t('mcp.baseUrl'), key: 'base_url', minWidth: 220,
    render: (row) => h(NTooltip, null, { trigger: () => h('span', { class: 'mono url-cell' }, row.base_url), default: () => row.base_url }),
  },
  {
    title: t('mcp.token'), key: 'token', width: 120,
    render: (row) => row.token_mask
      ? h('span', { class: 'mono' }, row.token_mask)
      : h('span', { style: 'color: var(--sx-muted)' }, t('mcp.tokenUnset')),
  },
  {
    title: t('common.status'), key: 'status', width: 76,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 1 ? 'success' : 'default' },
      { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  {
    title: t('common.updateTime'), key: 'updated_at', width: 150,
    render: (row) => row.updated_at?.slice(0, 16).replace('T', ' ') ?? '—',
  },
  {
    title: t('common.actions'), key: 'actions', width: 150,
    render: (row) => {
      const actions = []
      if (userStore.has('mcp:server:update')) actions.push({ label: t('common.edit'), onClick: () => openEdit(row) })
      if (userStore.has('mcp:server:test')) actions.push({ label: t('mcp.test'), onClick: () => runTest(row) })
      if (userStore.has('mcp:server:delete')) actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      return renderActions(actions)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const res = await listMcpServers({
      page: query.page, page_size: query.page_size,
      kw: query.kw.trim() || undefined,
      transport: query.transport || undefined,
      status: query.status ?? undefined,
    })
    rows.value = res.data.data.list
    setTotal(res.data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  Object.assign(query, { kw: '', transport: '', status: null, page: 1 })
  load()
}

// ---- 新增/编辑 ----
const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const editMask = ref('')
const formRef = ref<FormInst>()
const form = reactive({
  name: '', code: '', transport: 'streamable_http' as 'streamable_http' | 'sse',
  base_url: '', token: '', remark: '', status: 1,
  headers: [] as { key: string; value: string }[],
})

const rules = computed<FormRules>(() => ({
  name: [{ required: true, message: t('role.namePlaceholder'), trigger: ['blur', 'input'] }],
  code: [{ required: true, message: t('agent.agent.form.codeRequired'), trigger: ['blur', 'input'] }],
  base_url: [{ required: true, message: t('agent.provider.form.baseUrlRequired'), trigger: ['blur', 'input'] }],
}))

const tokenPlaceholder = computed(() =>
  (editing.value && editMask.value) ? `${t('mcp.tokenKeepHint')}（${editMask.value}）` : t('mcp.tokenPlaceholder'))

function openCreate() {
  editing.value = false
  editMask.value = ''
  Object.assign(form, {
    name: '', code: '', transport: 'streamable_http', base_url: '', token: '', remark: '', status: 1, headers: [],
  })
  showModal.value = true
}

function openEdit(row: McpServer) {
  editing.value = true
  editId.value = row.id
  editMask.value = row.token_mask
  Object.assign(form, {
    name: row.name, code: row.code, transport: row.transport, base_url: row.base_url,
    token: '', remark: row.remark, status: row.status,
    headers: (row.headers ?? []).map((h) => ({ key: h.key, value: h.value })),
  })
  showModal.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    const data = {
      name: form.name.trim(), code: form.code.trim(), transport: form.transport,
      base_url: form.base_url.trim(), token: form.token, remark: form.remark.trim(), status: form.status,
      headers: form.headers.filter((h) => h.key.trim() !== ''),
    }
    if (editing.value) {
      await updateMcpServer(editId.value, data)
    } else {
      await createMcpServer(data)
    }
    message.success(t('common.success'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: McpServer) {
  dialog.warning({
    title: t('common.tips'),
    content: t('mcp.deleteConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteMcpServer(row.id)
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.failed'))
      }
    },
  })
}

// ---- 连通测试 ----
const testingId = ref(0)
const showTestModal = ref(false)
const testTarget = ref<McpServer | null>(null)
const testResult = ref<Awaited<ReturnType<typeof testMcpServer>>['data']['data'] | null>(null)

async function runTest(row: McpServer) {
  testingId.value = row.id
  testTarget.value = row
  testResult.value = null
  showTestModal.value = true
  try {
    const { data } = await testMcpServer(row.id)
    testResult.value = data.data
  } catch (e: any) {
    showTestModal.value = false
    message.error(e?.response?.data?.msg || t('common.failed'))
  } finally {
    testingId.value = 0
  }
}

onMounted(load)
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.mono {
  font-family: var(--sx-font-mono);
}
/* 地址列截断（NEllipsis 须像素宽，这里用 max-width 同效） */
.url-cell {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}

/* 自定义 Header 编辑区 */
.headers-block {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.header-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.header-row .n-input:first-child {
  width: 40%;
}
.header-row .n-input:nth-child(2) {
  flex: 1;
}
.headers-hint {
  font-size: 12px;
  color: var(--sx-muted);
}

/* 测试结果工具清单 */
.tools-title {
  margin: 14px 0 8px;
  font-size: 13px;
  font-weight: 600;
}
.tools-empty {
  color: var(--sx-muted);
  font-size: 12px;
}
.tools-list {
  max-height: 320px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.tool-item {
  display: flex;
  gap: 10px;
  align-items: baseline;
  padding: 6px 10px;
  border-radius: 6px;
  background: var(--sx-bg);
  border: 1px solid var(--sx-line);
}
.tool-name {
  font-size: 12px;
  color: var(--sx-accent);
  flex-shrink: 0;
}
.tool-desc {
  font-size: 12px;
  color: var(--sx-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
