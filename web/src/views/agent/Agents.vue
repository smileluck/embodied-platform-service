<template>
  <!-- 智能体底座 · Agent 配置：标准搜索 + 表格 CRUD；行内「调试」打开无状态 Playground（SSE 流式） -->
  <SearchCard storage-key="agents" @search="search" @reset="resetQuery">
    <n-input v-model:value="query.name" :placeholder="t('agent.agent.searchName')" clearable style="width: 160px" @keyup.enter="search" />
    <n-input v-model:value="query.code" :placeholder="t('agent.agent.searchCode')" clearable style="width: 160px" @keyup.enter="search" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost v-permission="['agent:create']" @click="openCreate">{{ t('agent.agent.newAgent') }}</n-button>
      </div>
    </template>

    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" remote />
  </n-card>

  <!-- 新增/编辑 -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('agent.agent.editAgent') : t('agent.agent.newAgent')" style="width: 560px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="90">
      <n-form-item :label="t('agent.agent.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="20" show-word-limit :placeholder="t('role.namePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('agent.agent.code')" path="code">
        <n-input v-model:value="form.code" :maxlength="64" show-word-limit placeholder="assistant / summarizer / coder" />
      </n-form-item>
      <n-form-item :label="t('agent.agent.provider')" path="provider_id">
        <n-select v-model:value="form.provider_id" :options="providerOptions" :placeholder="t('common.pleaseSelect')" @update:value="form.model_id = null" />
      </n-form-item>
      <n-form-item :label="t('agent.agent.model')" path="model_id">
        <n-select
          v-model:value="form.model_id" :options="modelOptions" filterable
          :placeholder="form.provider_id ? t('common.pleaseSelect') : t('agent.agent.modelPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="t('agent.agent.systemPrompt')" path="system_prompt">
        <n-input
          v-model:value="form.system_prompt" type="textarea" :rows="5" :maxlength="4000" show-word-limit
          :placeholder="t('agent.agent.systemPromptPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="t('agent.agent.temperature')">
        <div class="param-row">
          <n-slider v-model:value="form.temperature" :min="0" :max="2" :step="0.1" />
          <span class="param-val mono">{{ form.temperature.toFixed(1) }}</span>
        </div>
      </n-form-item>
      <n-form-item :label="t('agent.agent.topP')">
        <div class="param-row">
          <n-slider v-model:value="form.top_p" :min="0" :max="1" :step="0.05" />
          <span class="param-val mono">{{ form.top_p.toFixed(2) }}</span>
        </div>
      </n-form-item>
      <n-form-item :label="t('agent.agent.maxTokens')">
        <n-input-number v-model:value="form.max_tokens" :min="0" :max="131072" :step="256" style="width: 100%" :placeholder="t('agent.agent.maxTokensPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('agent.agent.tools')">
        <n-select
          v-model:value="form.tools" :options="toolOptions" multiple clearable
          :placeholder="t('agent.agent.toolsPlaceholder')"
        />
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="form.remark" :maxlength="200" show-word-limit :placeholder="t('role.remarkPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('common.status')">
        <n-switch v-model:value="form.status" :checked-value="1" :unchecked-value="0" size="small" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 调试对话 Playground（无状态，不落库；对话逻辑复用 ChatPanel） -->
  <n-drawer v-model:show="showPlayground" :width="560" :auto-focus="false">
    <n-drawer-content :title="`${t('agent.playground.title')} · ${playAgent?.name ?? ''}`" closable>
      <ChatPanel
        v-if="playAgent" :key="playAgent.id"
        :agent-id="playAgent.id" :model-label="playModelLabel"
      />
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton, NCard, NDataTable, NDrawer, NDrawerContent, NForm, NFormItem, NInput, NInputNumber,
  NModal, NSelect, NSlider, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import {
  createAgent, deleteAgent, listAgentModels, listAgentProviders, listAgentTools, listAgents, updateAgent,
} from '../../api'
import type { AgentInfo, AgentModel, AgentProvider } from '../../api/types'
import ChatPanel from './ChatPanel.vue'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const rows = ref<AgentInfo[]>([])
const query = reactive({ name: '', code: '', page: 1, page_size: 10 })

// 供应商/模型映射（表格展示与表单级联选择用）
const providers = ref<AgentProvider[]>([])
const models = ref<AgentModel[]>([])
const providerName = computed(() => new Map(providers.value.map((p) => [p.id, p.name])))
const modelById = computed(() => new Map(models.value.map((m) => [m.id, m])))

async function loadRefs() {
  const [pRes, mRes] = await Promise.all([
    listAgentProviders({ page: 1, page_size: 0 }),
    listAgentModels({ page: 1, page_size: 0 }),
  ])
  providers.value = pRes.data.data.list
  models.value = mRes.data.data.list
}

const { pagination, setTotal, runSearch: search } = usePagination(query, load)

async function load() {
  loading.value = true
  try {
    const { data } = await listAgents(query)
    rows.value = data.data.list
    pagination.page = query.page
    pagination.pageSize = query.page_size
    setTotal(data.data.page.total)
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.name = ''
  query.code = ''
  query.page = 1
  load()
}

// ---- 表单 ----
const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const form = reactive({
  name: '', code: '',
  // null 表示未选择：n-select 值为 0 时无对应选项会直接显示"0"
  provider_id: null as number | null, model_id: null as number | null, tools: [] as string[],
  system_prompt: '', temperature: 0.7, top_p: 0, max_tokens: 0, remark: '', status: 1,
})
const formRef = ref<FormInst | null>(null)

const rules = computed<FormRules>(() => ({
  name: [
    { required: true, message: t('agent.agent.form.nameRequired'), trigger: ['blur', 'input'] },
    { max: 20, message: t('agent.agent.form.nameMax'), trigger: ['blur', 'input'] },
  ],
  code: [
    { required: true, message: t('agent.agent.form.codeRequired'), trigger: ['blur', 'input'] },
    { max: 64, message: t('agent.agent.form.codeMax'), trigger: ['blur', 'input'] },
  ],
  provider_id: [{ required: true, type: 'number', message: t('agent.agent.form.providerRequired'), trigger: ['blur', 'change'] }],
  model_id: [{ required: true, type: 'number', validator: (_r, v: number | null) => v != null && v > 0, message: t('agent.agent.form.modelRequired'), trigger: ['blur', 'change'] }],
}))

const providerOptions = computed(() =>
  providers.value.map((p) => ({ label: p.name, value: p.id, disabled: p.status !== 1 })))

// 级联：仅显示当前供应商下的模型；禁用未启用模型
const modelOptions = computed(() =>
  models.value
    .filter((m) => m.provider_id === form.provider_id)
    .map((m) => ({ label: `${m.display_name || m.name}（${m.name}）`, value: m.id, disabled: m.status !== 1 })))

function openCreate() {
  editing.value = false
  Object.assign(form, {
    name: '', code: '', provider_id: null, model_id: null, tools: [],
    system_prompt: '', temperature: 0.7, top_p: 0, max_tokens: 0, remark: '', status: 1,
  })
  showModal.value = true
}

function openEdit(row: AgentInfo) {
  editing.value = true
  editId.value = row.id
  const m = modelById.value.get(row.model_id)
  Object.assign(form, {
    name: row.name, code: row.code,
    provider_id: m?.provider_id ?? null, model_id: row.model_id || null, tools: row.tools ?? [],
    system_prompt: row.system_prompt, temperature: row.temperature || 0.7,
    top_p: row.top_p, max_tokens: row.max_tokens, remark: row.remark, status: row.status,
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
    const payload = {
      name: form.name.trim(), code: form.code.trim(), model_id: form.model_id as number, tools: form.tools,
      system_prompt: form.system_prompt.trim(), temperature: form.temperature, top_p: form.top_p,
      max_tokens: form.max_tokens, remark: form.remark.trim(), status: form.status,
    }
    if (editing.value) {
      await updateAgent(editId.value, payload)
    } else {
      await createAgent(payload)
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: AgentInfo) {
  dialog.warning({
    title: t('role.deleteConfirmTitle'),
    content: t('agent.deleteAgentConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteAgent(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('agent.deleteFailed'))
      }
    },
  })
}

const columns = computed<DataTableColumns<AgentInfo>>(() => [
  { title: 'ID', key: 'id', width: 60 },
  { title: t('agent.agent.name'), key: 'name', width: 130 },
  { title: t('agent.agent.code'), key: 'code', width: 120, render: (row) => row.code },
  {
    title: t('agent.agent.model'), key: 'model', minWidth: 180,
    render: (row) => {
      const m = modelById.value.get(row.model_id)
      if (!m) return h('span', { style: 'color: var(--sx-muted)' }, '—')
      const pn = providerName.value.get(m.provider_id) ?? ''
      return h('span', { class: 'mono' }, `${pn} · ${m.display_name || m.name}`)
    },
  },
  {
    title: t('agent.agent.params'), key: 'params', width: 150,
    render: (row) => {
      if (!row.temperature && !row.top_p && !row.max_tokens) return '—'
      const parts: string[] = []
      if (row.temperature) parts.push(`T ${row.temperature}`)
      if (row.top_p) parts.push(`P ${row.top_p}`)
      if (row.max_tokens) parts.push(`M ${row.max_tokens}`)
      return h('span', { class: 'mono', style: 'font-size:12px' }, parts.join(' / '))
    },
  },
  {
    title: t('common.status'), key: 'status', width: 76,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 1 ? 'success' : 'default' },
      { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  {
    title: t('common.operation'), key: 'actions', width: 200,
    render(row) {
      const actions: TableAction[] = []
      if (userStore.has('agent:update')) {
        actions.push({ label: t('common.edit'), accent: true, onClick: () => openEdit(row) })
      }
      if (userStore.has('agent:chat')) {
        actions.push({ label: t('agent.agent.debug'), onClick: () => openPlayground(row) })
      }
      if (userStore.has('agent:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDelete(row) })
      }
      return renderActions(actions)
    },
  },
])

// ---- 调试对话 Playground（抽屉；SSE 流式逻辑复用 ChatPanel 组件） ----
const showPlayground = ref(false)
const playAgent = ref<AgentInfo | null>(null)
const playModelLabel = computed(() => {
  const a = playAgent.value
  if (!a) return ''
  const m = modelById.value.get(a.model_id)
  return m ? `${providerName.value.get(m.provider_id) ?? ''} · ${m.name}` : ''
})

function openPlayground(row: AgentInfo) {
  playAgent.value = row
  showPlayground.value = true
}

// 可绑定的本地工具清单（function calling）
const toolOptions = ref<{ label: string; value: string }[]>([])

onMounted(async () => {
  await loadRefs()
  load()
  if (userStore.has('agent:tool:list')) {
    try {
      const res = await listAgentTools()
      toolOptions.value = (res.data.data ?? []).map((n: string) => ({ label: n, value: n }))
    } catch { /* 工具清单加载失败不阻断页面 */ }
  }
})
</script>

<style scoped>
/* 卡头只放操作按钮（页面标题由顶栏展示） */
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
}

/* 采样参数行：滑块 + 当前值 */
.param-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}
.param-row .n-slider {
  flex: 1;
}
.param-val {
  width: 40px;
  text-align: right;
  font-size: 12px;
  color: var(--sx-muted);
}

.mono {
  font-family: var(--sx-font-mono);
}
</style>
