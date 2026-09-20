<template>
  <!-- 智能体底座 · Agent 配置：标准搜索 + 表格 CRUD；行内「调试」打开无状态 Playground（SSE 流式） -->
  <SearchCard storage-key="agents" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.name" :placeholder="t('agent.agent.searchName')" clearable style="width: 160px" @keyup.enter="load" />
    <n-input v-model:value="query.code" :placeholder="t('agent.agent.searchCode')" clearable style="width: 160px" @keyup.enter="load" />
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
        <n-select v-model:value="form.provider_id" :options="providerOptions" :placeholder="t('common.pleaseSelect')" @update:value="form.model_id = 0" />
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

  <!-- 调试对话 Playground（无状态，不落库） -->
  <n-drawer v-model:show="showPlayground" :width="560" :auto-focus="false">
    <n-drawer-content :title="`${t('agent.playground.title')} · ${playAgent?.name ?? ''}`" closable>
      <div class="pg">
        <div v-if="metaModel" class="pg-model mono">{{ metaModel }} · {{ t('agent.playground.subtitle') }}</div>
        <div ref="pgListRef" class="pg-list">
          <div v-if="!chat.length" class="pg-empty">{{ t('agent.playground.empty') }}</div>
          <div v-for="(m, i) in chat" :key="i" class="pg-msg" :class="m.role">
            <div class="pg-bubble">
              <div class="pg-text">{{ m.content || (m.role === 'assistant' && m.streaming ? t('agent.playground.streaming') : '') }}</div>
              <div v-if="m.error" class="pg-error">{{ m.error }}</div>
              <div v-else-if="m.role === 'assistant' && !m.streaming && m.usage" class="pg-usage mono">
                {{ t('agent.playground.tokenUsage') }} {{ m.usage.total_tokens }}
              </div>
            </div>
          </div>
        </div>
        <div class="pg-input">
          <n-input
            v-model:value="chatInput" type="textarea" :rows="3" :maxlength="4000"
            :placeholder="t('agent.playground.inputPlaceholder')" :disabled="chatSending"
            @keydown.enter.exact.prevent="send"
          />
          <div class="pg-actions">
            <n-button size="small" quaternary :disabled="!chat.length || chatSending" @click="clearChat">{{ t('agent.playground.clear') }}</n-button>
            <n-button v-if="chatSending" size="small" type="error" ghost @click="stopChat">{{ t('agent.playground.stop') }}</n-button>
            <n-button v-else size="small" type="primary" :disabled="!chatInput.trim()" @click="send">{{ t('agent.playground.send') }}</n-button>
          </div>
        </div>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onMounted, reactive, ref, watch } from 'vue'
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
  createAgent, deleteAgent, listAgentModels, listAgentProviders, listAgents, updateAgent,
} from '../../api'
import type { AgentInfo, AgentModel, AgentProvider } from '../../api/types'

const { t, locale } = useI18n()
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
    listAgentProviders({ page: 1, page_size: 100 }),
    listAgentModels({ page: 1, page_size: 100 }),
  ])
  providers.value = pRes.data.data.list
  models.value = mRes.data.data.list
}

const { pagination, setTotal } = usePagination(query, load)

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
  name: '', code: '', provider_id: 0, model_id: 0,
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
  model_id: [{ required: true, type: 'number', validator: (_r, v: number) => v > 0, message: t('agent.agent.form.modelRequired'), trigger: ['blur', 'change'] }],
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
    name: '', code: '', provider_id: 0, model_id: 0,
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
    provider_id: m?.provider_id ?? 0, model_id: row.model_id,
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
      name: form.name.trim(), code: form.code.trim(), model_id: form.model_id,
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
    title: t('common.operation'), key: 'actions', width: 170,
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

// ---- 调试对话 Playground（SSE 流式） ----
interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
  error?: string
  usage?: { total_tokens: number }
}

const showPlayground = ref(false)
const playAgent = ref<AgentInfo | null>(null)
const metaModel = ref('')
const chat = ref<ChatMsg[]>([])
const chatInput = ref('')
const chatSending = ref(false)
const abortCtrl = ref<AbortController | null>(null)
const pgListRef = ref<HTMLElement | null>(null)

function openPlayground(row: AgentInfo) {
  playAgent.value = row
  const m = modelById.value.get(row.model_id)
  metaModel.value = m ? `${providerName.value.get(m.provider_id) ?? ''} · ${m.name}` : ''
  chat.value = []
  chatInput.value = ''
  showPlayground.value = true
}

function clearChat() {
  chat.value = []
}

function stopChat() {
  abortCtrl.value?.abort()
}

// 新消息/流式增量后滚动到底部
watch(
  () => chat.value.map((m) => m.content.length).join(','),
  () => nextTick(() => {
    const el = pgListRef.value
    if (el) el.scrollTop = el.scrollHeight
  }),
)

async function send() {
  const text = chatInput.value.trim()
  if (!text || chatSending.value || !playAgent.value) return
  chatInput.value = ''
  chat.value.push({ role: 'user', content: text })
  // 历史消息（system prompt 由后端按 Agent 配置注入）：排除出错的与空回复的 assistant
  const history = chat.value
    .filter((m) => !m.error && (m.role === 'user' || m.content))
    .map((m) => ({ role: m.role, content: m.content }))
  const assistant: ChatMsg = { role: 'assistant', content: '', streaming: true }
  chat.value.push(assistant)
  chatSending.value = true
  const ctrl = new AbortController()
  abortCtrl.value = ctrl
  try {
    const resp = await fetch(`/api/v1/agents/${playAgent.value.id}/chat`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.accessToken}`,
        'Accept-Language': locale.value,
      },
      body: JSON.stringify({ messages: history }),
      signal: ctrl.signal,
    })
    if (!resp.ok || !resp.body) {
      // 非 200：JSON 错误信封（RBAC 拒绝 / 参数错误 / 上游错误等）
      let msg = t('agent.playground.error')
      try {
        const j = await resp.json()
        if (j?.msg) msg = j.msg
      } catch { /* 忽略解析失败 */ }
      throw new Error(msg)
    }
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      let idx: number
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const frame = buf.slice(0, idx)
        buf = buf.slice(idx + 2)
        handleSSEFrame(frame, assistant)
      }
    }
  } catch (e: any) {
    if (e?.name === 'AbortError') {
      assistant.content += '\n（已停止）'
    } else {
      assistant.error = e?.message || t('agent.playground.error')
    }
  } finally {
    assistant.streaming = false
    chatSending.value = false
    abortCtrl.value = null
  }
}

// 解析一帧 SSE：event: xxx\ndata: {...}
function handleSSEFrame(frame: string, assistant: ChatMsg) {
  let event = 'message'
  const dataLines: string[] = []
  for (const line of frame.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (!dataLines.length) return
  let data: any
  try {
    data = JSON.parse(dataLines.join('\n'))
  } catch {
    return
  }
  if (event === 'meta') {
    if (data?.model) metaModel.value = String(data.model)
  } else if (event === 'delta') {
    if (data?.delta) assistant.content += data.delta
    if (data?.usage) assistant.usage = data.usage
  } else if (event === 'error') {
    assistant.error = data?.message || t('agent.playground.error')
  }
}

onMounted(async () => {
  await loadRefs()
  load()
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

/* ---- Playground ---- */
.pg {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.pg-model {
  font-size: 12px;
  color: var(--sx-muted);
}
.pg-list {
  flex: 1;
  overflow: auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px;
}
.pg-empty {
  color: var(--sx-muted);
  font-size: 13px;
  text-align: center;
  padding: 40px 0;
}
.pg-msg {
  display: flex;
}
.pg-msg.user {
  justify-content: flex-end;
}
.pg-bubble {
  max-width: 86%;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}
.pg-msg.user .pg-bubble {
  background: rgba(63, 117, 171, 0.12);
}
.pg-msg.assistant .pg-bubble {
  background: var(--sx-surface);
  border: 1px solid var(--sx-line);
}
.pg-error {
  margin-top: 4px;
  font-size: 12px;
  color: var(--sx-danger);
}
.pg-usage {
  margin-top: 4px;
  font-size: 11px;
  color: var(--sx-muted);
}
.pg-input {
  border-top: 1px solid var(--sx-line);
  padding-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.pg-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
