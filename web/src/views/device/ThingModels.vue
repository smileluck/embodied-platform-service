<!-- 物模型管理页：节点树（本商户节点可增删改，通用节点只读但可挂子节点）+ 右侧详情/Schema 解析/版本管理三页签。
     写面收敛（读=通用+本商户、写仅本商户 404 不泄露、父链校验 409）由平台开放面保证，错误透传 msg。
     开放面版本列表仅 published 选择器视图，无草稿枚举/详情端点：草稿 vid 仅在创建/回滚响应中返回，
     页面以 localStorage（tm_draft:<nodeId>）跨会话跟踪本会话创建的草稿。 -->
<template>
  <SearchCard storage-key="thing-models" @search="onSearch" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('thingModel.kwPlaceholder')" clearable style="width: 220px" @keyup.enter="onSearch" />
    <n-select v-model:value="query.layer" :options="layerOptions" :placeholder="t('thingModel.layerLabel')" clearable style="width: 140px" />
  </SearchCard>

  <div class="tm-layout">
    <n-card size="small" class="tm-tree-card">
      <template #header>
        <div class="tree-header">
          <span>{{ t('thingModel.nodeTree') }}</span>
          <n-button size="tiny" type="primary" ghost @click="openNodeCreate(null)" v-permission="['thingModel:nodeCreate']">
            {{ t('thingModel.addNode') }}
          </n-button>
        </div>
      </template>
      <n-spin :show="loading">
        <n-tree
          v-if="treeData.length"
          :data="treeData"
          :selected-keys="selectedKeys"
          :render-label="renderNodeLabel"
          :default-expand-all="!!query.kw"
          block-line
          @update:selected-keys="onSelect"
        />
        <n-empty v-else-if="!loading" :description="t('common.noData')" />
      </n-spin>
    </n-card>

    <n-card :title="t('thingModel.nodeDetail')" size="small" class="tm-detail-card">
      <n-empty v-if="!selectedNode" :description="t('thingModel.selectNode')" />
      <n-tabs v-else type="line" size="small">
        <!-- 详情 -->
        <n-tab-pane name="detail" :tab="t('thingModel.tabDetail')">
          <n-descriptions :column="2" label-placement="left" size="small" bordered>
            <n-descriptions-item :label="t('thingModel.code')">{{ selectedNode.code }}</n-descriptions-item>
            <n-descriptions-item :label="t('thingModel.name')">{{ selectedNode.name }}</n-descriptions-item>
            <n-descriptions-item :label="t('thingModel.layerLabel')">
              <n-tag size="small" :bordered="false">{{ layerText(selectedNode.layer) }}</n-tag>
            </n-descriptions-item>
            <n-descriptions-item :label="t('common.status')">
              <n-tag size="small" :type="selectedNode.status === 1 ? 'success' : 'error'">
                {{ selectedNode.status === 1 ? t('common.enabled') : t('common.disabled') }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item :label="t('thingModel.scope')">
              <n-tag size="small" :type="selectedNode.merchant_id === 0 ? 'default' : 'info'">
                {{ selectedNode.merchant_id === 0 ? t('thingModel.scopeCommon') : t('thingModel.scopeMerchant') }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item :label="t('thingModel.parentNode')">{{ parentName(selectedNode) }}</n-descriptions-item>
          </n-descriptions>
        </n-tab-pane>

        <!-- Schema 合并解析（thingModel:view） -->
        <n-tab-pane v-if="userStore.has('thingModel:view')" name="schema" :tab="t('thingModel.tabSchema')">
          <n-spin :show="resolveLoading">
            <div v-if="resolveChain" class="resolve-chain">{{ t('thingModel.chain') }}：{{ resolveChain }}</div>
            <TMSchemaViewer v-if="resolvedSchema" :schema="resolvedSchema" />
            <n-alert v-else-if="resolveHint" type="default" :show-icon="false">{{ resolveHint }}</n-alert>
          </n-spin>
        </n-tab-pane>

        <!-- 版本管理（thingModel:version） -->
        <n-tab-pane v-if="userStore.has('thingModel:version')" name="versions" :tab="t('thingModel.tabVersions')">
          <n-alert v-if="inheritance && !inheritance.up_to_date" type="warning" :show-icon="false" style="margin-bottom: 12px">
            {{ t('thingModel.parentUpdated') }}
            <div class="alert-sub">{{ inheritanceSummary }}</div>
            <div class="alert-sub">{{ t('thingModel.inheritanceRebaseHint') }}</div>
          </n-alert>
          <n-alert v-if="orphanDraft" type="warning" :show-icon="false" style="margin-bottom: 12px">
            {{ t('thingModel.orphanDraftHint') }}
          </n-alert>
          <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center">
            <n-button
              v-if="nodeWritable && !localDraft" type="primary" size="small"
              @click="openDraftCreate" v-permission="['thingModel:draft']"
            >{{ t('thingModel.newDraft') }}</n-button>
            <span v-if="nodeWritable && localDraft" class="alert-sub" style="margin: 0">
              {{ t('thingModel.singleDraftHint', { version: localDraft.version }) }}
            </span>
          </div>
          <n-data-table :columns="versionColumns" :data="versionRows" :loading="versionsLoading" size="small" :pagination="false" />
        </n-tab-pane>
      </n-tabs>
    </n-card>
  </div>

  <!-- 新增/编辑节点 -->
  <n-modal v-model:show="showNodeModal" preset="dialog" :title="nodeEditing ? t('thingModel.editNodeTitle') : t('thingModel.createNodeTitle')" style="width: 480px">
    <n-form ref="nodeFormRef" :model="nodeForm" :rules="nodeRules" label-placement="left" label-width="90">
      <n-form-item :label="t('thingModel.layerLabel')" path="layer">
        <n-select v-model:value="nodeForm.layer" :options="layerOptions" :disabled="nodeEditing" @update:value="onFormLayerChange" />
      </n-form-item>
      <n-form-item v-if="nodeForm.layer !== 'base'" :label="t('thingModel.parentNode')" path="parent_id">
        <n-select
          v-model:value="nodeForm.parent_id" :options="parentOptions" filterable clearable
          :placeholder="t('thingModel.parentPh')" :disabled="nodeEditing" />
      </n-form-item>
      <n-form-item :label="t('thingModel.code')" path="code">
        <n-input v-model:value="nodeForm.code" :maxlength="64" :placeholder="t('thingModel.codePh')" :disabled="nodeEditing" />
      </n-form-item>
      <n-form-item :label="t('thingModel.name')" path="name">
        <n-input v-model:value="nodeForm.name" :maxlength="64" />
      </n-form-item>
      <n-form-item v-if="nodeEditing" :label="t('common.status')">
        <n-select v-model:value="nodeForm.status" :options="statusOptions" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showNodeModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="nodeSaving" @click="saveNode">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>

  <!-- 新建草稿抽屉：结构化 Schema 编辑器（可选预填最新发布版本的本层要素） -->
  <n-drawer v-model:show="showDraftCreate" :width="920">
    <n-drawer-content :title="t('thingModel.createDraftTitle', { name: selectedNode?.name ?? '' })" closable>
      <n-alert type="info" :show-icon="false" style="margin-bottom: 12px">{{ t('thingModel.draftBlankHint') }}</n-alert>
      <n-checkbox
        v-if="versions.length" v-model:checked="draftPrefill" :disabled="prefillLoading"
        style="margin-bottom: 12px" @update:checked="applyPrefill"
      >{{ t('thingModel.prefillLatest') }}</n-checkbox>
      <TMSchemaEditor :key="`create-${draftCreateSeed}`" ref="draftCreateEditor" :initial="draftCreateInitial" />
      <template #footer>
        <div style="display: flex; justify-content: flex-end; gap: 8px; width: 100%">
          <n-button @click="showDraftCreate = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="draftCreateSaving" @click="createDraft">{{ t('thingModel.createDraft') }}</n-button>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>

  <!-- 编辑草稿抽屉 -->
  <n-drawer v-model:show="showDraftEdit" :width="920">
    <n-drawer-content :title="t('thingModel.editDraftTitle', { version: localDraft?.version ?? '', name: selectedNode?.name ?? '' })" closable>
      <TMSchemaEditor :key="`edit-${localDraft?.vid ?? 0}-${draftEditSeed}`" ref="draftEditEditor" :initial="localDraft?.schema ?? null" />
      <template #footer>
        <div style="display: flex; justify-content: flex-end; gap: 8px; width: 100%">
          <n-button @click="showDraftEdit = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="draftEditSaving" @click="saveDraft">{{ t('thingModel.saveDraft') }}</n-button>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>

  <!-- 版本明细：该版本的合并解析结果（含父层继承要素） -->
  <n-drawer v-model:show="showVersionDetail" :width="820">
    <n-drawer-content :title="t('thingModel.versionDetailTitle', { version: detailVersion?.version ?? '' })" closable>
      <n-alert type="default" :show-icon="false" style="margin-bottom: 12px">{{ t('thingModel.mergedHint') }}</n-alert>
      <n-spin :show="detailLoading">
        <div v-if="detailChain" class="resolve-chain">{{ t('thingModel.chain') }}：{{ detailChain }}</div>
        <TMSchemaViewer v-if="detailSchema" :schema="detailSchema" />
      </n-spin>
    </n-drawer-content>
  </n-drawer>

  <!-- 回退确认：追加式回退（复制为新版本），可选直接发布 -->
  <n-modal v-model:show="showRollback" preset="dialog" :title="t('thingModel.rollbackTitle', { version: rollbackTarget?.version ?? '' })" style="width: 480px">
    <n-alert type="info" :show-icon="false" style="margin: 0 0 12px">
      {{ t('thingModel.rollbackHint', { version: rollbackTarget?.version ?? '' }) }}
    </n-alert>
    <n-radio-group v-model:value="rollbackMode">
      <n-space vertical>
        <n-radio value="draft">{{ t('thingModel.rollbackDraftOnly') }}</n-radio>
        <n-radio value="publish">{{ t('thingModel.rollbackPublish') }}</n-radio>
      </n-space>
    </n-radio-group>
    <template #action>
      <n-button @click="showRollback = false">{{ t('common.cancel') }}</n-button>
      <n-button type="warning" :loading="rollbackSaving" style="margin-left: 8px" @click="doRollback">{{ t('thingModel.confirmRollback') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, type VNodeChild } from 'vue'
import {
  NAlert, NButton, NCard, NCheckbox, NDataTable, NDescriptions, NDescriptionsItem, NDrawer, NDrawerContent,
  NEmpty, NForm, NFormItem, NInput, NModal, NRadio, NRadioGroup, NSelect, NSpace, NSpin, NTabPane, NTabs,
  NTag, NTree, useDialog, useMessage,
  type DataTableColumns, type FormInst, type FormRules, type TreeOption,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import TMSchemaViewer from './TMSchemaViewer.vue'
import TMSchemaEditor from './TMSchemaEditor.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { useUserStore } from '../../stores/user'
import { formatDateTime } from '../../utils/datetime'
import {
  createThingModelDraft, createThingModelNode, deleteThingModelNode, deleteThingModelVersion,
  getThingModelInheritanceStatus, listThingModelNodes, listThingModelVersions,
  publishThingModelVersion, resolveThingModel, rollbackThingModelVersion,
  updateThingModelDraft, updateThingModelNode,
} from '../../api'
import type { TMInheritanceStatus, TMNode, TMSchema, TMVersion } from '../../api/types'

const { t } = useI18n()
const userStore = useUserStore()
const message = useMessage()
const dialog = useDialog()

function errMsg(e: any, fallback: string): string {
  return e?.response?.data?.msg || fallback
}

const LAYER_ORDER = ['base', 'category', 'model', 'instance']

const layerOptions = computed(() =>
  LAYER_ORDER.map((l) => ({ label: t(`thingModel.layer.${l}`), value: l })),
)
const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 2 },
])

function layerText(layer: string): string {
  return LAYER_ORDER.includes(layer) ? t(`thingModel.layer.${layer}`) : layer
}

// ---- 节点树 ----
const loading = ref(false)
const allNodes = ref<TMNode[]>([])
const query = reactive({ kw: '', layer: null as string | null })

const nodeById = computed(() => new Map(allNodes.value.map((n) => [n.id, n])))

function parentName(node: TMNode): string {
  const p = nodeById.value.get(node.parent_id)
  return p ? `${p.name}（${p.code}）` : '—'
}

// kw 客户端过滤：命中节点 + 全部祖先（保留树路径）；layer 精确过滤
const visibleNodes = computed(() => {
  let nodes = allNodes.value
  if (query.layer) nodes = nodes.filter((n) => n.layer === query.layer)
  const kw = query.kw.trim().toLowerCase()
  if (!kw) return nodes
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const keep = new Set<number>()
  for (const n of nodes) {
    if (!n.name.toLowerCase().includes(kw) && !n.code.toLowerCase().includes(kw)) continue
    let cur: TMNode | undefined = n
    while (cur && !keep.has(cur.id)) {
      keep.add(cur.id)
      cur = byId.get(cur.parent_id)
    }
  }
  return nodes.filter((n) => keep.has(n.id))
})

function toTreeOptions(nodes: TMNode[]): TreeOption[] {
  const ids = new Set(nodes.map((n) => n.id))
  const byParent = new Map<number, TMNode[]>()
  for (const n of nodes) {
    const arr = byParent.get(n.parent_id) ?? []
    arr.push(n)
    byParent.set(n.parent_id, arr)
  }
  const sorter = (a: TMNode, b: TMNode) =>
    LAYER_ORDER.indexOf(a.layer) - LAYER_ORDER.indexOf(b.layer) || a.id - b.id
  const build = (list: TMNode[]): TreeOption[] =>
    list.slice().sort(sorter).map((n) => ({ key: n.id, label: `${n.name}（${n.code}）`, children: build(byParent.get(n.id) ?? []), node: n }))
  // 根=parent_id 不在当前集合内的节点（parent_id=0 或父节点被过滤）
  return build(nodes.filter((n) => !ids.has(n.parent_id)))
}

const treeData = computed<TreeOption[]>(() => toTreeOptions(visibleNodes.value))

// 行内操作：任意节点可挂本商户子节点（父链规则平台校验）；编辑/删除仅本商户节点
function renderNodeLabel({ option }: { option: TreeOption }): VNodeChild {
  const node = (option as any).node as TMNode
  const btns: VNodeChild[] = []
  if (userStore.has('thingModel:nodeCreate')) {
    btns.push(h(NButton, {
      text: true, size: 'tiny', type: 'primary',
      onClick: (e: MouseEvent) => { e.stopPropagation(); openNodeCreate(node) },
    }, { default: () => t('thingModel.addChildNode') }))
  }
  if (node.merchant_id !== 0) {
    if (userStore.has('thingModel:nodeUpdate')) {
      btns.push(h(NButton, {
        text: true, size: 'tiny',
        onClick: (e: MouseEvent) => { e.stopPropagation(); openNodeEdit(node) },
      }, { default: () => t('common.edit') }))
    }
    if (userStore.has('thingModel:nodeDelete')) {
      btns.push(h(NButton, {
        text: true, size: 'tiny', type: 'error',
        onClick: (e: MouseEvent) => { e.stopPropagation(); confirmDeleteNode(node) },
      }, { default: () => t('common.delete') }))
    }
  }
  return h('span', { style: 'display:inline-flex;align-items:center;gap:6px;flex-wrap:wrap' }, [
    h('span', option.label as string),
    h(NTag, { size: 'tiny', bordered: false }, { default: () => layerText(node.layer) }),
    h(
      NTag,
      { size: 'tiny', bordered: false, type: node.merchant_id === 0 ? 'default' : 'info' },
      { default: () => (node.merchant_id === 0 ? t('thingModel.scopeCommon') : t('thingModel.scopeMerchant')) },
    ),
    ...btns,
  ])
}

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listThingModelNodes()
    allNodes.value = resp.data || []
  } catch (e: any) {
    message.error(errMsg(e, t('common.loadFailed')))
  } finally {
    loading.value = false
  }
}

function onSearch() {
  // 全量已拉取，搜索仅触发客户端过滤（computed 即时生效）
  selectedKeys.value = []
}

function resetQuery() {
  query.kw = ''
  query.layer = null
}

// ---- 节点选择与详情区 ----
const selectedKeys = ref<number[]>([])
const selectedNode = computed(() => allNodes.value.find((n) => n.id === selectedKeys.value[0]) ?? null)
const nodeWritable = computed(() => !!selectedNode.value && selectedNode.value.merchant_id !== 0)

async function onSelect(keys: Array<string | number>) {
  selectedKeys.value = keys as number[]
  versions.value = []
  localDraft.value = null
  resolvedSchema.value = null
  resolveChain.value = ''
  resolveHint.value = ''
  inheritance.value = null
  const node = selectedNode.value
  if (!node) return
  if (userStore.has('thingModel:version')) void reloadVersions(node)
  if (userStore.has('thingModel:view')) {
    void loadResolved(node)
    void loadInheritance(node)
  }
}

// ---- Schema 合并解析 ----
const resolvedSchema = ref<TMSchema | null>(null)
const resolveChain = ref('')
const resolveHint = ref('')
const resolveLoading = ref(false)

async function loadResolved(node: TMNode) {
  resolveLoading.value = true
  try {
    const { data: resp } = await resolveThingModel(node.id)
    resolvedSchema.value = resp.data.schema
    resolveChain.value = (resp.data.chain ?? []).map((l) => `${l.node.name} v${l.version.version}`).join(' → ')
    resolveHint.value = ''
  } catch {
    resolvedSchema.value = null
    resolveChain.value = ''
    resolveHint.value = t('thingModel.resolveFailed')
  } finally {
    resolveLoading.value = false
  }
}

// ---- 继承状态 ----
const inheritance = ref<TMInheritanceStatus | null>(null)

const inheritanceSummary = computed(() =>
  (inheritance.value?.updates ?? [])
    .map((u) => `${u.node.name}: ${u.snapshot_version ? `v${u.snapshot_version.version}` : t('thingModel.floating')} → v${u.latest_version.version}`)
    .join('；'),
)

// 平台侧有草稿但本会话无记录（开放面无草稿枚举端点，只能提示）
const orphanDraft = computed(() => inheritance.value?.baseline === 'draft' && !localDraft.value)

async function loadInheritance(node: TMNode) {
  if (node.layer === 'base') {
    inheritance.value = null
    return
  }
  try {
    const { data: resp } = await getThingModelInheritanceStatus(node.id)
    inheritance.value = resp.data
  } catch {
    inheritance.value = null
  }
}

// ---- 版本列表（published）+ 本地草稿跟踪 ----
const versions = ref<TMVersion[]>([])
const versionsLoading = ref(false)

// 开放面无草稿列表/详情端点：草稿仅以 localStorage 记录（vid/version/schema），跨会话存续；
// 发布或删除后清除；失效记录（404 或已出现在 published 列表）惰性清理
interface DraftRecord { vid: number; version: number; schema: TMSchema }
const DRAFT_KEY = (nodeId: number) => `tm_draft:${nodeId}`
const localDraft = ref<DraftRecord | null>(null)

function loadDraftRecord(nodeId: number): DraftRecord | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY(nodeId))
    return raw ? (JSON.parse(raw) as DraftRecord) : null
  } catch {
    return null
  }
}
function saveDraftRecord(nodeId: number, rec: DraftRecord) {
  localStorage.setItem(DRAFT_KEY(nodeId), JSON.stringify(rec))
}
function clearDraftRecord(nodeId: number) {
  localStorage.removeItem(DRAFT_KEY(nodeId))
  if (selectedNode.value?.id === nodeId) localDraft.value = null
}

type VersionRow = { kind: 'draft'; rec: DraftRecord } | { kind: 'version'; v: TMVersion }

const versionRows = computed<VersionRow[]>(() => {
  const rows: VersionRow[] = versions.value.map((v) => ({ kind: 'version', v }))
  if (localDraft.value) rows.unshift({ kind: 'draft', rec: localDraft.value })
  return rows
})

const versionById = computed(() => new Map(versions.value.map((v) => [v.id, v])))

async function reloadVersions(node: TMNode) {
  versionsLoading.value = true
  try {
    const { data: resp } = await listThingModelVersions(node.id)
    versions.value = resp.data || []
    // 本地草稿记录已发布（vid 出现在 published 列表）→ 清理
    const rec = loadDraftRecord(node.id)
    if (rec && versions.value.some((v) => v.id === rec.vid)) {
      clearDraftRecord(node.id)
    }
    localDraft.value = loadDraftRecord(node.id)
  } catch (e: any) {
    message.error(errMsg(e, t('common.loadFailed')))
  } finally {
    versionsLoading.value = false
  }
}

const versionColumns = computed<DataTableColumns<VersionRow>>(() => [
  {
    title: t('thingModel.version'), key: 'version', width: 200,
    render: (row) => {
      if (row.kind === 'draft') {
        return h('span', { style: 'display:inline-flex;align-items:center;gap:4px' }, [
          `v${row.rec.version}`,
          h(NTag, { size: 'tiny', type: 'warning', bordered: false }, { default: () => t('thingModel.draftLocalTag') }),
        ])
      }
      const v = row.v
      const src = v.revert_of ? versionById.value.get(v.revert_of) : null
      return h('span', { style: 'display:inline-flex;align-items:center;gap:4px' }, [
        `v${v.version}`,
        ...(src ? [h(NTag, { size: 'tiny', bordered: false }, { default: () => t('thingModel.revertFrom', { version: src.version }) })] : []),
      ])
    },
  },
  {
    title: t('common.status'), key: 'status', width: 90,
    render: (row) => row.kind === 'draft'
      ? h(NTag, { size: 'small', type: 'warning' }, { default: () => t('thingModel.statusDraft') })
      : h(NTag, { size: 'small', type: 'success' }, { default: () => t('thingModel.published') }),
  },
  {
    title: t('thingModel.publishedAt'), key: 'published_at', width: 165,
    render: (row) => (row.kind === 'version' ? formatDateTime(row.v.published_at) || '—' : '—'),
  },
  {
    title: t('common.operation'), key: 'actions', width: 250,
    render: (row) => {
      if (row.kind === 'draft') {
        const actions: TableAction[] = [
          { label: t('common.edit'), accent: true, permission: 'thingModel:draftUpdate', onClick: () => openDraftEdit() },
          { label: t('thingModel.publish'), permission: 'thingModel:publish', onClick: () => confirmPublish(row.rec) },
          { label: t('common.delete'), danger: true, permission: 'thingModel:versionDelete', onClick: () => confirmDeleteDraft() },
        ]
        return renderActions(actions)
      }
      const v = row.v
      const actions: TableAction[] = [
        { label: t('thingModel.detail'), permission: 'thingModel:view', onClick: () => openVersionDetail(v) },
      ]
      if (nodeWritable.value) {
        actions.push({ label: t('thingModel.rollbackTo'), permission: 'thingModel:rollback', onClick: () => openRollback(v) })
        actions.push({ label: t('common.delete'), danger: true, permission: 'thingModel:versionDelete', onClick: () => confirmDeleteVersion(v) })
      }
      return renderActions(actions)
    },
  },
])

// ---- 节点新增/编辑/删除 ----
const showNodeModal = ref(false)
const nodeEditing = ref(false)
const nodeSaving = ref(false)
const nodeFormRef = ref<FormInst | null>(null)
const nodeForm = reactive({
  layer: 'base' as 'base' | 'category' | 'model' | 'instance',
  parent_id: null as number | null,
  code: '',
  name: '',
  status: 1,
})
let nodeEditId = 0

const nodeRules = computed<FormRules>(() => ({
  layer: [{ required: true, message: t('thingModel.layerRequired'), trigger: ['change'] }],
  parent_id: [{ required: nodeForm.layer !== 'base', type: 'number', message: t('thingModel.parentRequired'), trigger: ['change'] }],
  code: [{ required: true, message: t('thingModel.codeRequired'), trigger: ['blur', 'input'] }],
  name: [{ required: true, message: t('thingModel.nameRequired'), trigger: ['blur', 'input'] }],
}))

// 父节点候选：layer 联动（category→base、model→category、instance→model；通用与本商户节点均可作父）
const parentOptions = computed(() => {
  const parentLayer = LAYER_ORDER[LAYER_ORDER.indexOf(nodeForm.layer) - 1]
  if (!parentLayer) return []
  return allNodes.value
    .filter((n) => n.layer === parentLayer && n.id !== nodeEditId)
    .map((n) => ({ label: `${n.name}（${n.code}）`, value: n.id }))
})

function onFormLayerChange() {
  nodeForm.parent_id = null
}

function openNodeCreate(parent: TMNode | null) {
  nodeEditing.value = false
  nodeEditId = 0
  const layer = parent
    ? (LAYER_ORDER[LAYER_ORDER.indexOf(parent.layer) + 1] as typeof nodeForm.layer | undefined) ?? 'instance'
    : 'base'
  Object.assign(nodeForm, { layer, parent_id: parent?.id ?? null, code: '', name: '', status: 1 })
  showNodeModal.value = true
}

function openNodeEdit(node: TMNode) {
  nodeEditing.value = true
  nodeEditId = node.id
  Object.assign(nodeForm, {
    layer: node.layer as typeof nodeForm.layer,
    parent_id: node.parent_id || null,
    code: node.code, name: node.name, status: node.status,
  })
  showNodeModal.value = true
}

async function saveNode() {
  try {
    await nodeFormRef.value?.validate()
  } catch {
    return
  }
  nodeSaving.value = true
  try {
    if (nodeEditing.value) {
      await updateThingModelNode(nodeEditId, { name: nodeForm.name, status: nodeForm.status })
    } else {
      await createThingModelNode({
        layer: nodeForm.layer,
        parent_id: nodeForm.layer === 'base' ? undefined : nodeForm.parent_id ?? undefined,
        code: nodeForm.code,
        name: nodeForm.name,
      })
    }
    message.success(t('common.saveSuccess'))
    showNodeModal.value = false
    load()
  } catch (e: any) {
    message.error(errMsg(e, t('common.failed')))
  } finally {
    nodeSaving.value = false
  }
}

function confirmDeleteNode(node: TMNode) {
  dialog.warning({
    title: t('thingModel.deleteNodeTitle'),
    content: t('thingModel.deleteNodeContent', { name: node.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteThingModelNode(node.id)
        message.success(t('common.deleteSuccess'))
        if (selectedNode.value?.id === node.id) selectedKeys.value = []
        load()
      } catch (e: any) {
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

// ---- 草稿（创建/编辑/发布/删除） ----
const showDraftCreate = ref(false)
const draftCreateSeed = ref(0)
const draftCreateSaving = ref(false)
const draftPrefill = ref(true)
const prefillLoading = ref(false)
const draftCreateInitial = ref<TMSchema | null>(null)
const draftCreateEditor = ref<InstanceType<typeof TMSchemaEditor> | null>(null)

const showDraftEdit = ref(false)
const draftEditSeed = ref(0)
const draftEditSaving = ref(false)
const draftEditEditor = ref<InstanceType<typeof TMSchemaEditor> | null>(null)

// 预填：节点基线合并解析 - 父节点合并解析 = 本层自有要素（同名同义视为继承，剔除）
function stableStringify(v: any): string {
  if (Array.isArray(v)) return `[${v.map(stableStringify).join(',')}]`
  if (v && typeof v === 'object') {
    return `{${Object.keys(v).sort().map((k) => `${JSON.stringify(k)}:${stableStringify(v[k])}`).join(',')}}`
  }
  return JSON.stringify(v)
}

function subtractSchema(merged: TMSchema | null, inherited: TMSchema | null): TMSchema {
  const out: TMSchema = {}
  for (const section of ['properties', 'services', 'events'] as const) {
    const m = merged?.[section] ?? {}
    const inh = inherited?.[section] ?? {}
    const entries = Object.entries(m).filter(
      ([k, v]) => !(k in inh && stableStringify((inh as any)[k]) === stableStringify(v)),
    )
    if (entries.length) (out as any)[section] = Object.fromEntries(entries)
  }
  return out
}

async function computePrefillSchema(node: TMNode): Promise<TMSchema | null> {
  const { data: resp } = await resolveThingModel(node.id)
  const merged = resp.data.schema
  if (!node.parent_id) return merged
  try {
    const { data: parentResp } = await resolveThingModel(node.parent_id)
    return subtractSchema(merged, parentResp.data.schema)
  } catch {
    return merged
  }
}

async function applyPrefill(checked: boolean) {
  const node = selectedNode.value
  if (!node) return
  if (!checked) {
    draftCreateInitial.value = null
    draftCreateSeed.value++
    return
  }
  prefillLoading.value = true
  try {
    draftCreateInitial.value = await computePrefillSchema(node)
  } catch {
    draftCreateInitial.value = null
  } finally {
    prefillLoading.value = false
    draftCreateSeed.value++ // 重挂载编辑器装载预填内容
  }
}

function openDraftCreate() {
  draftPrefill.value = versions.value.length > 0
  draftCreateInitial.value = null
  draftCreateSeed.value++
  showDraftCreate.value = true
  if (draftPrefill.value) void applyPrefill(true)
}

async function createDraft() {
  const node = selectedNode.value
  if (!node || !draftCreateEditor.value) return
  const r = draftCreateEditor.value.build()
  if (!r.ok) {
    message.error(r.errors.join('；'))
    return
  }
  draftCreateSaving.value = true
  try {
    const { data: resp } = await createThingModelDraft(node.id)
    const draft = resp.data
    const schema = r.schema
    if (schema.properties || schema.services || schema.events) {
      await updateThingModelDraft(draft.id, schema)
    }
    saveDraftRecord(node.id, { vid: draft.id, version: draft.version, schema })
    localDraft.value = loadDraftRecord(node.id)
    message.success(t('thingModel.draftCreated', { version: draft.version }))
    showDraftCreate.value = false
    void loadInheritance(node)
  } catch (e: any) {
    message.error(errMsg(e, t('common.failed')))
  } finally {
    draftCreateSaving.value = false
  }
}

function openDraftEdit() {
  draftEditSeed.value++
  showDraftEdit.value = true
}

async function saveDraft() {
  const node = selectedNode.value
  const draft = localDraft.value
  if (!node || !draft || !draftEditEditor.value) return
  const r = draftEditEditor.value.build()
  if (!r.ok) {
    message.error(r.errors.join('；'))
    return
  }
  draftEditSaving.value = true
  try {
    await updateThingModelDraft(draft.vid, r.schema)
    saveDraftRecord(node.id, { ...draft, schema: r.schema })
    localDraft.value = loadDraftRecord(node.id)
    message.success(t('thingModel.draftSaved', { version: draft.version }))
    showDraftEdit.value = false
  } catch (e: any) {
    if (e?.response?.status === 404) clearDraftRecord(node.id)
    message.error(errMsg(e, t('common.failed')))
  } finally {
    draftEditSaving.value = false
  }
}

function confirmPublish(rec: DraftRecord) {
  dialog.warning({
    title: t('thingModel.publishConfirmTitle'),
    content: t('thingModel.publishConfirmContent', { version: rec.version }),
    positiveText: t('thingModel.publish'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const node = selectedNode.value
      if (!node) return
      try {
        await publishThingModelVersion(rec.vid)
        message.success(t('thingModel.versionPublished', { version: rec.version }))
        clearDraftRecord(node.id)
        showDraftEdit.value = false
        await reloadVersions(node)
        void loadResolved(node)
        void loadInheritance(node)
      } catch (e: any) {
        if (e?.response?.status === 404) {
          clearDraftRecord(node.id)
          await reloadVersions(node)
        }
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

function confirmDeleteDraft() {
  const rec = localDraft.value
  const node = selectedNode.value
  if (!rec || !node) return
  dialog.warning({
    title: t('thingModel.deleteVersionTitle'),
    content: t('thingModel.deleteDraftContent', { version: rec.version }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteThingModelVersion(rec.vid)
        message.success(t('thingModel.versionDeleted', { version: rec.version }))
      } catch (e: any) {
        if (e?.response?.status !== 404) {
          message.error(errMsg(e, t('common.failed')))
          return
        }
      }
      clearDraftRecord(node.id)
      showDraftEdit.value = false
      void loadInheritance(node)
    },
  })
}

function confirmDeleteVersion(v: TMVersion) {
  dialog.warning({
    title: t('thingModel.deleteVersionTitle'),
    content: t('thingModel.deleteVersionContent', { version: v.version }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const node = selectedNode.value
      try {
        await deleteThingModelVersion(v.id)
        message.success(t('thingModel.versionDeleted', { version: v.version }))
        if (node) await reloadVersions(node)
      } catch (e: any) {
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

// ---- 版本明细（合并解析 pin 到该版本） ----
const showVersionDetail = ref(false)
const detailVersion = ref<TMVersion | null>(null)
const detailSchema = ref<TMSchema | null>(null)
const detailChain = ref('')
const detailLoading = ref(false)

async function openVersionDetail(v: TMVersion) {
  const node = selectedNode.value
  if (!node) return
  detailVersion.value = v
  detailSchema.value = null
  detailChain.value = ''
  showVersionDetail.value = true
  detailLoading.value = true
  try {
    const { data: resp } = await resolveThingModel(node.id, v.id)
    detailSchema.value = resp.data.schema
    detailChain.value = (resp.data.chain ?? []).map((l) => `${l.node.name} v${l.version.version}`).join(' → ')
  } catch (e: any) {
    message.error(errMsg(e, t('common.loadFailed')))
  } finally {
    detailLoading.value = false
  }
}

// ---- 回退（追加式）----
const showRollback = ref(false)
const rollbackTarget = ref<TMVersion | null>(null)
const rollbackMode = ref<'draft' | 'publish'>('draft')
const rollbackSaving = ref(false)

function openRollback(v: TMVersion) {
  rollbackTarget.value = v
  rollbackMode.value = 'draft'
  showRollback.value = true
}

async function doRollback() {
  const node = selectedNode.value
  const target = rollbackTarget.value
  if (!node || !target) return
  rollbackSaving.value = true
  try {
    const publish = rollbackMode.value === 'publish'
    const { data: resp } = await rollbackThingModelVersion(target.id, publish)
    showRollback.value = false
    if (publish) {
      message.success(t('thingModel.rollbackPublished', { version: resp.data.version, src: target.version }))
      await reloadVersions(node)
      void loadResolved(node)
      void loadInheritance(node)
      return
    }
    // 回滚草稿：响应含完整版本与复制来的 schema，记入本地跟踪并直接进入编辑态
    message.success(t('thingModel.rollbackDraftCreated', { version: resp.data.version, src: target.version }))
    saveDraftRecord(node.id, { vid: resp.data.id, version: resp.data.version, schema: resp.data.schema ?? {} })
    localDraft.value = loadDraftRecord(node.id)
    void loadInheritance(node)
    openDraftEdit()
  } catch (e: any) {
    message.error(errMsg(e, t('common.failed')))
  } finally {
    rollbackSaving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.tm-layout {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}
.tm-tree-card {
  width: 460px;
  flex-shrink: 0;
}
.tree-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.tm-detail-card {
  flex: 1;
  min-width: 0;
}
.resolve-chain {
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--sx-muted);
}
.alert-sub {
  font-size: 12px;
  color: var(--sx-muted);
  margin-top: 2px;
}
</style>
