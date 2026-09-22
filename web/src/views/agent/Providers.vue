<template>
  <!-- 智能体底座 · 供应商管理：左右 4:6 固定分栏，任何窗口宽度都并排（同 ServerMonitor 范式） -->
  <div class="agent-page">
    <div class="cols">
      <!-- 左栏：供应商列表（数量有限，滚动浏览不分页） -->
      <div class="col col-left">
        <n-card size="small" class="provider-card">
          <template #header>
            <span class="card-title">{{ t('agent.provider.newList') }}</span>
          </template>
          <template #header-extra>
            <n-button size="small" type="primary" ghost v-permission="['agent:provider:create']" @click="openCreate">
              {{ t('agent.provider.newProvider') }}
            </n-button>
          </template>
          <div class="provider-toolbar">
            <n-input v-model:value="query.kw" size="small" :placeholder="t('agent.provider.searchPlaceholder')" clearable @keyup.enter="searchProviders" />
          </div>
          <div v-if="loading" class="provider-loading"><n-spin size="small" /></div>
          <n-empty v-else-if="!providers.length" size="small" :description="t('agent.provider.unselectHint')" style="padding: 24px 0" />
          <div v-else class="provider-list">
            <div
              v-for="p in providers" :key="p.id"
              class="provider-item" :class="{ active: selected?.id === p.id }"
              @click="selectProvider(p)"
            >
              <div class="provider-row">
                <span class="provider-name">{{ p.name }}</span>
                <n-tag size="small" :type="p.status === 1 ? 'success' : 'default'" :bordered="false">
                  {{ p.status === 1 ? t('common.enabled') : t('common.disabled') }}
                </n-tag>
              </div>
              <div class="provider-meta mono">{{ p.code }} · {{ p.api_key_mask || t('agent.provider.apiKeyUnset') }}</div>
              <div class="provider-actions">
                <n-button text size="tiny" type="primary" :loading="testingId === p.id" v-permission="['agent:provider:test']" @click.stop="testProvider(p)">
                  {{ t('agent.provider.test') }}
                </n-button>
                <n-button text size="tiny" type="primary" v-permission="['agent:provider:update']" @click.stop="openEdit(p)">
                  {{ t('common.edit') }}
                </n-button>
                <n-button text size="tiny" type="error" v-permission="['agent:provider:delete']" @click.stop="confirmDeleteProvider(p)">
                  {{ t('common.delete') }}
                </n-button>
              </div>
            </div>
          </div>
        </n-card>
      </div>

      <!-- 右栏：供应商详情 + 模型管理 -->
      <div class="col col-right">
        <n-card size="small">
          <template #header>
            <span class="card-title">{{ t('agent.provider.title') }}</span>
          </template>
          <template v-if="selected" #header-extra>
            <n-button size="small" :loading="testingId === selected.id" v-permission="['agent:provider:test']" @click="testProvider(selected)">
              {{ t('agent.provider.test') }}
            </n-button>
          </template>
          <n-descriptions v-if="selected" :column="2" size="small" label-placement="left" bordered>
            <n-descriptions-item :label="t('agent.provider.name')">{{ selected.name }}</n-descriptions-item>
            <n-descriptions-item :label="t('agent.provider.code')"><span class="mono">{{ selected.code }}</span></n-descriptions-item>
            <n-descriptions-item :label="t('agent.provider.baseUrl')" :span="2"><span class="mono">{{ selected.base_url }}</span></n-descriptions-item>
            <n-descriptions-item :label="t('agent.provider.apiKey')">
              <span class="mono">{{ selected.api_key_mask || '—' }}</span>
            </n-descriptions-item>
            <n-descriptions-item :label="t('agent.provider.protocol')"><span class="mono">{{ selected.protocol }}</span></n-descriptions-item>
            <n-descriptions-item :label="t('common.remark')" :span="2">{{ selected.remark || '—' }}</n-descriptions-item>
          </n-descriptions>
          <n-empty v-else size="small" :description="t('agent.provider.unselectHint')" style="padding: 24px 0" />
        </n-card>

        <n-card size="small">
          <template #header>
            <span class="card-title">{{ t('agent.model.title') }}<span v-if="selected" class="model-count">（{{ selected.name }}）</span></span>
          </template>
          <template #header-extra>
            <n-button size="small" type="primary" ghost :disabled="!selected" v-permission="['agent:model:create']" @click="openModelCreate">
              {{ t('agent.model.newModel') }}
            </n-button>
          </template>
          <n-data-table
            size="small" :columns="modelColumns" :data="models" :loading="modelsLoading"
            :pagination="modelPagination" remote
          />
        </n-card>
      </div>
    </div>

    <!-- 供应商 新增/编辑 -->
    <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('agent.provider.editProvider') : t('agent.provider.newProvider')" style="width: 480px">
      <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="90">
        <n-form-item :label="t('agent.provider.preset')">
          <!-- 厂商预设：一键填充名称/编码/接口地址，避免手填错 base_url 导致连接超时 -->
          <n-select
            :value="presetPick" :options="presetOptions" clearable
            :placeholder="t('agent.provider.presetPlaceholder')" @update:value="applyPreset"
          />
        </n-form-item>
        <n-form-item :label="t('agent.provider.name')" path="name">
          <n-input v-model:value="form.name" :maxlength="20" show-word-limit :placeholder="t('agent.model.form.nameRequired')" />
        </n-form-item>
        <n-form-item :label="t('agent.provider.code')" path="code">
          <n-input v-model:value="form.code" :maxlength="64" show-word-limit placeholder="zhipu / openai / ollama" />
        </n-form-item>
        <n-form-item :label="t('agent.provider.baseUrl')" path="base_url">
          <n-input v-model:value="form.base_url" :placeholder="t('agent.provider.baseUrlPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('agent.provider.apiKey')" path="api_key">
          <n-input
            v-model:value="form.api_key" type="password" show-password-on="click"
            :maxlength="512" :placeholder="editing ? t('agent.provider.apiKeyPlaceholder') : t('agent.provider.apiKey')"
          />
          <!-- 自定义反馈仅在编辑态展示"保持原密钥"提示；新增态留给必填校验文案（占插槽会吞掉校验报错） -->
          <template v-if="editing" #feedback>{{ apiKeyFeedback }}</template>
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

    <!-- 模型 新增/编辑 -->
    <n-modal v-model:show="showModelModal" preset="dialog" :title="modelEditing ? t('agent.model.editModel') : t('agent.model.newModel')" style="width: 480px">
      <n-form ref="modelFormRef" :model="modelForm" :rules="modelRules" label-placement="left" label-width="90">
        <n-form-item :label="t('agent.model.name')" path="name">
          <div class="model-name-row">
            <n-select
              v-model:value="modelForm.name" filterable tag :options="remoteOptions"
              :placeholder="t('agent.model.namePlaceholder')" :loading="remoteLoading"
            />
            <n-button
              size="small" :loading="remoteLoading" v-permission="['agent:provider:remoteModels']"
              @click="fetchRemoteModels"
            >{{ t('agent.model.fetchRemote') }}</n-button>
          </div>
        </n-form-item>
        <n-form-item :label="t('agent.model.displayName')" path="display_name">
          <n-input v-model:value="modelForm.display_name" :maxlength="20" show-word-limit :placeholder="t('role.namePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('agent.model.contextWindow')">
          <n-input-number v-model:value="modelForm.context_window" :min="0" :step="1000" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('agent.model.maxOutput')">
          <n-input-number v-model:value="modelForm.max_output" :min="0" :step="256" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('agent.model.supportsTools')">
          <n-switch v-model:value="modelForm.supports_tools" size="small" />
        </n-form-item>
        <n-form-item :label="t('agent.model.inputPrice')">
          <n-input-number v-model:value="modelForm.input_price" :min="0" :step="0.001" :precision="4" :placeholder="t('agent.model.pricePlaceholder')" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('agent.model.outputPrice')">
          <n-input-number v-model:value="modelForm.output_price" :min="0" :step="0.001" :precision="4" :placeholder="t('agent.model.pricePlaceholder')" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('common.remark')">
          <n-input v-model:value="modelForm.remark" :maxlength="200" show-word-limit :placeholder="t('role.remarkPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('common.status')">
          <n-switch v-model:value="modelForm.status" :checked-value="1" :unchecked-value="0" size="small" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-button @click="showModelModal = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="modelSaving" @click="saveModel">{{ t('common.confirm') }}</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton, NCard, NDataTable, NDescriptions, NDescriptionsItem, NEmpty, NForm, NFormItem,
  NInput, NInputNumber, NModal, NSelect, NSpin, NSwitch, NTag, useDialog, useMessage,
  type DataTableColumns, type FormInst, type FormRules,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { useUserStore } from '../../stores/user'
import {
  createAgentModel, createAgentProvider, deleteAgentModel, deleteAgentProvider,
  listAgentModels, listAgentProviders, listAgentRemoteModels, testAgentProvider,
  testAgentModel as testAgentModelApi,
  updateAgentModel, updateAgentProvider,
} from '../../api'
import type { AgentModel, AgentProvider, AgentTestResult } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

// ---- 供应商列表（左栏） ----
const loading = ref(false)
const providers = ref<AgentProvider[]>([])
const selected = ref<AgentProvider | null>(null)
const query = reactive({ kw: '' })
const testingId = ref(0)

async function loadProviders(keepSelection = true) {
  loading.value = true
  try {
    const kw = query.kw.trim()
    const { data } = await listAgentProviders({
      page: 1, page_size: 0,
      ...(kw ? { name: kw, code: kw } : {}),
    })
    providers.value = data.data.list
    if (keepSelection && selected.value) {
      selected.value = providers.value.find((p) => p.id === selected.value!.id) ?? null
    }
    if (!selected.value && providers.value.length) {
      selected.value = providers.value[0]
    }
  } finally {
    loading.value = false
  }
}

function searchProviders() {
  selected.value = null
  loadProviders()
}

function selectProvider(p: AgentProvider) {
  selected.value = p
}

// 选中供应商变化 -> 重置模型分页并加载
watch(() => selected.value?.id, () => {
  modelQuery.page = 1
  loadModels()
})

// ---- 供应商表单 ----
const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const saving = ref(false)
const form = reactive({ name: '', code: '', base_url: '', api_key: '', remark: '', status: 1 })
const formRef = ref<FormInst | null>(null)

// ---- 厂商预设（主要厂商；base_url 为对应 OpenAI 兼容入口，模型列表接口同源可用） ----
// 拉取模型列表走 GET {base_url}/models，五家均兼容 OpenAI /models 响应结构
const presetOptions = [
  { label: '智谱 GLM', value: 'zhipu' },
  { label: 'Kimi（月之暗面）', value: 'kimi' },
  { label: 'MiniMax', value: 'minimax' },
  { label: 'OpenAI', value: 'openai' },
  { label: '通义千问 Qwen（阿里云百炼）', value: 'qwen' },
]
const presetDefaults: Record<string, { name: string; code: string; base_url: string }> = {
  zhipu: { name: '智谱 GLM', code: 'zhipu', base_url: 'https://open.bigmodel.cn/api/paas/v4' },
  kimi: { name: 'Kimi', code: 'moonshot', base_url: 'https://api.moonshot.cn/v1' },
  minimax: { name: 'MiniMax', code: 'minimax', base_url: 'https://api.minimaxi.com/v1' },
  openai: { name: 'OpenAI', code: 'openai', base_url: 'https://api.openai.com/v1' },
  qwen: { name: '通义千问 Qwen', code: 'qwen', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
}
// 预设选中值：表单与预设一致时回显（编辑打开时为空，不干扰既有供应商）
const presetPick = computed(() => {
  const hit = Object.entries(presetDefaults).find(([, v]) =>
    v.code === form.code.trim() && v.base_url === form.base_url.trim())
  return hit ? hit[0] : null
})
function applyPreset(key: string | null) {
  if (!key) return
  const d = presetDefaults[key]
  if (!d) return
  // 名称仅在新增时覆盖（编辑场景保留运营者自定义名称）；编码/地址始终填充
  if (!editing.value) form.name = d.name
  form.code = d.code
  form.base_url = d.base_url
}

const rules = computed<FormRules>(() => ({
  name: [
    { required: true, message: t('agent.provider.form.nameRequired'), trigger: ['blur', 'input'] },
    { max: 20, message: t('agent.provider.form.nameMax'), trigger: ['blur', 'input'] },
  ],
  code: [
    { required: true, message: t('agent.provider.form.codeRequired'), trigger: ['blur', 'input'] },
    { max: 64, message: t('agent.provider.form.codeMax'), trigger: ['blur', 'input'] },
  ],
  base_url: [{ required: true, message: t('agent.provider.form.baseUrlRequired'), trigger: ['blur', 'input'] }],
  // 新增时 API Key 必填；编辑留空表示保持原密钥不校验
  api_key: editing.value
    ? [{ max: 512, message: t('agent.provider.form.apiKeyMax'), trigger: ['blur', 'input'] }]
    : [
        { required: true, message: t('agent.provider.form.apiKeyRequired'), trigger: ['blur', 'input'] },
        { max: 512, message: t('agent.provider.form.apiKeyMax'), trigger: ['blur', 'input'] },
      ],
}))

const apiKeyFeedback = computed(() =>
  editing.value && selected.value?.api_key_mask
    ? `${t('agent.provider.apiKeyKeepHint')}（${selected.value.api_key_mask}）`
    : '',
)

function openCreate() {
  editing.value = false
  Object.assign(form, { name: '', code: '', base_url: '', api_key: '', remark: '', status: 1 })
  showModal.value = true
}

function openEdit(p: AgentProvider) {
  editing.value = true
  editId.value = p.id
  Object.assign(form, {
    name: p.name, code: p.code, base_url: p.base_url, api_key: '', remark: p.remark, status: p.status,
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
      name: form.name.trim(), code: form.code.trim(), base_url: form.base_url.trim(),
      api_key: form.api_key.trim(), remark: form.remark.trim(), status: form.status,
    }
    if (editing.value) {
      await updateAgentProvider(editId.value, payload)
    } else {
      await createAgentProvider(payload)
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    loadProviders()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmDeleteProvider(p: AgentProvider) {
  dialog.warning({
    title: t('role.deleteConfirmTitle'),
    content: t('agent.deleteProviderConfirm', { name: p.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteAgentProvider(p.id)
        message.success(t('common.deleteSuccess'))
        if (selected.value?.id === p.id) selected.value = null
        loadProviders()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('agent.deleteFailed'))
      }
    },
  })
}

// ---- 连通性测试 ----
function showTestResult(result: AgentTestResult) {
  dialog.success({
    title: t('agent.provider.testOkTitle'),
    content: () =>
      h('div', { style: 'line-height:1.9' }, [
        h('div', [
          h('span', { style: 'color:var(--sx-muted)' }, `${t('agent.model.name')}：`),
          h('span', { class: 'mono' }, result.model),
        ]),
        h('div', [
          h('span', { style: 'color:var(--sx-muted)' }, `${t('agent.provider.latency')}：`),
          h('span', { class: 'mono' }, `${result.latency_ms} ms`),
        ]),
        h('div', [
          h('span', { style: 'color:var(--sx-muted)' }, `${t('agent.playground.tokenUsage')}：`),
          h('span', { class: 'mono' }, String(result.usage?.total_tokens ?? '—')),
        ]),
        h('div', [
          h('span', { style: 'color:var(--sx-muted)' }, `${t('agent.provider.testReplyLabel')}：`),
          h('span', null, result.content || '—'),
        ]),
      ]),
    positiveText: t('common.confirm'),
  })
}

async function testProvider(p: AgentProvider) {
  testingId.value = p.id
  try {
    const { data } = await testAgentProvider(p.id)
    showTestResult(data.data)
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.playground.error'))
  } finally {
    testingId.value = 0
  }
}

// ---- 模型管理（右栏表格） ----
const models = ref<AgentModel[]>([])
const modelsLoading = ref(false)
const modelQuery = reactive({ page: 1, page_size: 10 })
const { pagination: modelPagination, setTotal: setModelTotal } = usePagination(modelQuery, loadModels)

async function loadModels() {
  if (!selected.value) {
    models.value = []
    setModelTotal(0)
    return
  }
  modelsLoading.value = true
  try {
    const { data } = await listAgentModels({
      page: modelQuery.page, page_size: modelQuery.page_size, provider_id: selected.value.id,
    })
    models.value = data.data.list
    modelPagination.page = modelQuery.page
    modelPagination.pageSize = modelQuery.page_size
    setModelTotal(data.data.page.total)
  } finally {
    modelsLoading.value = false
  }
}

const modelColumns = computed<DataTableColumns<AgentModel>>(() => [
  { title: 'ID', key: 'id', width: 56 },
  { title: t('agent.model.name'), key: 'name', render: (row) => h('span', { class: 'mono' }, row.name) },
  { title: t('agent.model.displayName'), key: 'display_name', render: (row) => row.display_name || '—' },
  { title: t('agent.model.contextWindow'), key: 'context_window', width: 110, render: (row) => (row.context_window ? row.context_window.toLocaleString() : '—') },
  {
    title: t('agent.model.supportsTools'), key: 'supports_tools', width: 80,
    render: (row) => (row.supports_tools ? h(NTag, { size: 'small', bordered: false }, { default: () => t('agent.model.toolsYes') }) : '—'),
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
      if (userStore.has('agent:model:test')) {
        actions.push({ label: testingModelId.value === row.id ? t('agent.provider.testing') : t('agent.provider.test'), accent: true, onClick: () => testModel(row) })
      }
      if (userStore.has('agent:model:update')) {
        actions.push({ label: t('common.edit'), onClick: () => openModelEdit(row) })
      }
      if (userStore.has('agent:model:delete')) {
        actions.push({ label: t('common.delete'), danger: true, onClick: () => confirmDeleteModel(row) })
      }
      return renderActions(actions)
    },
  },
])

const testingModelId = ref(0)

async function testModel(row: AgentModel) {
  testingModelId.value = row.id
  try {
    const { data } = await testAgentModelApi(row.id)
    showTestResult(data.data)
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.playground.error'))
  } finally {
    testingModelId.value = 0
  }
}

// ---- 模型表单 ----
const showModelModal = ref(false)
const modelEditing = ref(false)
const modelEditId = ref(0)
const modelSaving = ref(false)
const modelForm = reactive({
  name: '', display_name: '', context_window: 0, max_output: 0, supports_tools: false,
  input_price: null, output_price: null, remark: '', status: 1,
})
const modelFormRef = ref<FormInst | null>(null)
const remoteOptions = ref<{ label: string; value: string }[]>([])
const remoteLoading = ref(false)

const modelRules = computed<FormRules>(() => ({
  name: [
    { required: true, message: t('agent.model.form.nameRequired'), trigger: ['blur', 'input'] },
    { max: 128, message: t('agent.model.form.nameMax'), trigger: ['blur', 'input'] },
  ],
  display_name: [{ max: 20, message: t('agent.model.form.displayNameMax'), trigger: ['blur', 'input'] }],
}))

function openModelCreate() {
  if (!selected.value) return
  modelEditing.value = false
  Object.assign(modelForm, {
    name: '', display_name: '', context_window: 0, max_output: 0, supports_tools: false,
  input_price: null, output_price: null, remark: '', status: 1,
  })
  showModelModal.value = true
}

function openModelEdit(row: AgentModel) {
  modelEditing.value = true
  modelEditId.value = row.id
  Object.assign(modelForm, {
    name: row.name, display_name: row.display_name, context_window: row.context_window,
    max_output: row.max_output, supports_tools: row.supports_tools,
    input_price: row.input_price || null, output_price: row.output_price || null,
    remark: row.remark, status: row.status,
  })
  showModelModal.value = true
}

async function fetchRemoteModels() {
  if (!selected.value) return
  remoteLoading.value = true
  try {
    const { data } = await listAgentRemoteModels(selected.value.id)
    remoteOptions.value = data.data.map((m) => ({ label: m, value: m }))
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.model.fetchRemoteFailed'))
  } finally {
    remoteLoading.value = false
  }
}

async function saveModel() {
  try {
    await modelFormRef.value?.validate()
  } catch {
    return
  }
  modelSaving.value = true
  try {
    const payload = {
      provider_id: selected.value!.id, name: (modelForm.name || '').trim(), display_name: modelForm.display_name.trim(),
      context_window: modelForm.context_window, max_output: modelForm.max_output,
      supports_tools: modelForm.supports_tools,
      input_price: modelForm.input_price ?? undefined, output_price: modelForm.output_price ?? undefined,
      remark: modelForm.remark.trim(), status: modelForm.status,
    }
    if (modelEditing.value) {
      // 编辑时 provider_id 不可变（后端按空值保持原供应商，此处显式传当前供应商）
      await updateAgentModel(modelEditId.value, payload)
    } else {
      await createAgentModel(payload)
    }
    message.success(t('common.saveSuccess'))
    showModelModal.value = false
    loadModels()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('agent.saveFailed'))
  } finally {
    modelSaving.value = false
  }
}

function confirmDeleteModel(row: AgentModel) {
  dialog.warning({
    title: t('role.deleteConfirmTitle'),
    content: t('agent.deleteModelConfirm', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteAgentModel(row.id)
        message.success(t('common.deleteSuccess'))
        loadModels()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('agent.deleteFailed'))
      }
    },
  })
}

onMounted(() => loadProviders(false))
</script>

<style scoped>
/* 撑满一屏：100vh - 顶栏 64px - 内容区上下 padding（与 ServerMonitor 同范式） */
.agent-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 64px - 16px);
  min-height: 0;
}
.cols {
  flex: 1;
  display: flex;
  gap: 12px;
  min-height: 0;
}
.col {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  min-width: 0;
}
.col-left {
  flex: 4 1 0;
  overflow: auto;
}
.col-right {
  flex: 6 1 0;
  overflow: auto;
}

.card-title {
  font-weight: 600;
}
.model-count {
  color: var(--sx-muted);
  font-weight: 400;
  font-size: 13px;
}

/* 左栏供应商卡片：卡内列表滚动 */
.provider-card {
  display: flex;
  flex-direction: column;
}
.provider-card :deep(.n-card__content) {
  flex: 1;
  overflow: auto;
  min-height: 0;
}
.provider-toolbar {
  margin-bottom: 8px;
}
.provider-loading {
  padding: 24px 0;
  text-align: center;
}
.provider-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.provider-item {
  padding: 8px 10px;
  border: 1px solid var(--sx-line);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}
.provider-item:hover {
  border-color: var(--sx-accent);
}
.provider-item.active {
  border-color: var(--sx-accent);
  background-color: rgba(63, 117, 171, 0.08);
}
.provider-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.provider-name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.provider-meta {
  margin-top: 2px;
  font-size: 12px;
  color: var(--sx-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.provider-actions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
}

.model-name-row {
  display: flex;
  gap: 8px;
  width: 100%;
}

.mono {
  font-family: var(--sx-font-mono);
}
</style>
