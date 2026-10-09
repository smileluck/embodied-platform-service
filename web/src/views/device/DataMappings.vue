<!-- 数据映射完整管理页（平台开放面代理）。
     读=通用+本商户，写仅本商户独立资源：通用映射（merchant_id=0）行不渲染写操作（平台写面统一 404）。
     版本管理=单草稿制 + 发布不可变 + 追加式回滚（publish=true 一步完成）。 -->
<template>
  <SearchCard storage-key="data-mappings" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('dataMapping.kwPlaceholder')" clearable style="width: 200px" @keyup.enter="load" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openDefCreate" v-permission="['mapping:create']">{{ t('dataMapping.newMapping') }}</n-button>
      </div>
    </template>
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 版本管理抽屉：版本列表 + 新建草稿/编辑/发布/回退/删除（编辑在独立抽屉） -->
  <n-drawer v-model:show="showVersions" :width="820">
    <n-drawer-content :title="t('dataMapping.versionManageTitle', { name: currentDef?.name ?? '' })" closable>
      <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center">
        <n-button
          v-if="defWritable && !draftVersion" type="primary" size="small"
          @click="openDraftCreate" v-permission="['mapping:draft']"
        >{{ t('dataMapping.newDraft') }}</n-button>
        <span v-if="defWritable && draftVersion" style="font-size: 12px; color: var(--sx-muted)">
          {{ t('dataMapping.singleDraftHint', { version: draftVersion.version }) }}
        </span>
      </div>
      <n-data-table :columns="versionColumns" :data="versions" :loading="versionsLoading" size="small" :pagination="false" />
    </n-drawer-content>
  </n-drawer>

  <!-- 草稿编辑抽屉：新建映射资源 / 新建草稿 / 编辑草稿 三态共用 -->
  <n-drawer v-model:show="showEditor" :width="760">
    <n-drawer-content :title="editorTitle" closable>
      <n-form label-placement="left" label-width="90">
        <n-form-item v-if="editorMode === 'create-def'" :label="t('dataMapping.mappingName')" required>
          <n-input v-model:value="editorForm.name" :maxlength="64" :placeholder="t('dataMapping.mappingNamePlaceholder')" />
        </n-form-item>
        <n-form-item v-else :label="t('dataMapping.version')">
          <n-tag size="small">{{ editingVersion ? `v${editingVersion.version}` : t('dataMapping.versionAuto') }}</n-tag>
        </n-form-item>
        <n-form-item :label="t('dataMapping.label')">
          <n-input v-model:value="editorForm.label" :maxlength="64" :placeholder="t('dataMapping.labelPlaceholder')" />
        </n-form-item>
      </n-form>
      <n-alert v-if="editorMode === 'create-draft' && draftBaseVersion" type="info" :show-icon="false" style="margin-bottom: 12px">
        {{ t('dataMapping.prefillHint', { version: draftBaseVersion.version }) }}
      </n-alert>
      <div style="font-size: 12px; color: var(--sx-muted); margin-bottom: 10px">
        {{ t('dataMapping.topicHint', tmPlaceholders) }}
      </div>
      <n-space vertical>
        <div v-for="(m, i) in editorForm.mappings" :key="i" class="mapping-card">
          <n-space align="center" style="margin-bottom: 8px; width: 100%" justify="space-between">
            <span style="font-weight: 600; font-size: 13px">{{ t('dataMapping.mappingItem', { n: i + 1 }) }}</span>
            <n-button size="tiny" type="error" ghost @click="editorForm.mappings.splice(i, 1)">{{ t('common.delete') }}</n-button>
          </n-space>
          <n-space vertical size="small">
            <n-input v-model:value="m.source_topic" :placeholder="t('dataMapping.sourceTopicPh', tmPlaceholders)" />
            <n-space>
              <n-select v-model:value="m.topic_match" :options="topicMatchOptions" clearable style="width: 200px" />
              <n-select v-model:value="m.data_category" :options="categoryOptions" style="width: 170px" />
              <n-input v-model:value="m.data_type" :placeholder="t('dataMapping.dataTypePh')" style="width: 260px" />
              <n-input-number v-model:value="m.sample_rate_hz" :placeholder="t('dataMapping.sampleRatePh')" :min="0" clearable style="width: 170px" />
            </n-space>
            <!-- field_extract：目标字段 → 提取表达式 两列可增删 -->
            <n-dynamic-input v-model:value="m.field_extract" :on-create="() => ({ key: '', value: '' })">
              <template #default="{ value }">
                <div style="display: flex; gap: 8px; width: 100%">
                  <n-input v-model:value="value.key" :placeholder="t('dataMapping.fieldExtractKeyPh')" />
                  <n-input v-model:value="value.value" :placeholder="t('dataMapping.fieldExtractValuePh')" />
                </div>
              </template>
            </n-dynamic-input>
            <n-input v-model:value="m.fields" :placeholder="t('dataMapping.fieldsPh')" />
            <n-input v-model:value="m.trigger" type="textarea" :autosize="{ minRows: 1, maxRows: 3 }" :placeholder="t('dataMapping.triggerPh')" />
          </n-space>
        </div>
        <n-button dashed block @click="addMapping">{{ t('dataMapping.addMapping') }}</n-button>
      </n-space>
      <template #footer>
        <div style="display: flex; justify-content: flex-end; gap: 8px; width: 100%">
          <n-button @click="showEditor = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="savingEditor" @click="saveEditor">{{ t('dataMapping.saveDraft') }}</n-button>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>

  <!-- 版本明细：只读查看版本的映射规则（数据来自版本列表，无需二次请求） -->
  <n-drawer v-model:show="showDetail" :width="760">
    <n-drawer-content :title="t('dataMapping.detailTitle', { name: currentDef?.name ?? '' })" closable>
      <n-descriptions :column="2" label-placement="left" size="small" bordered style="margin-bottom: 12px">
        <n-descriptions-item :label="t('dataMapping.version')">
          v{{ detailVersion?.version ?? '-' }}
          <n-tag v-if="detailRevertFrom" size="tiny" :bordered="false" style="margin-left: 4px">
            {{ t('dataMapping.revertFrom', { version: detailRevertFrom }) }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item :label="t('common.status')">
          <n-tag v-if="detailVersion" size="small" :type="detailVersion.status === 'published' ? 'success' : 'warning'">
            {{ detailVersion.status === 'published' ? t('dataMapping.statusPublished') : t('dataMapping.statusDraft') }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item :label="t('dataMapping.label')">{{ detailVersion?.label || '—' }}</n-descriptions-item>
        <n-descriptions-item :label="t('dataMapping.publishedAt')">
          {{ formatDateTime(detailVersion?.published_at) || '—' }}
        </n-descriptions-item>
      </n-descriptions>
      <n-data-table :columns="detailMappingColumns" :data="detailVersion?.mappings ?? []" size="small" :pagination="false" />
    </n-drawer-content>
  </n-drawer>

  <!-- 回退确认：追加式回退（复制为新版本），可选直接发布 -->
  <n-modal v-model:show="showRollback" preset="dialog" :title="t('dataMapping.rollbackTitle', { version: rollbackTarget?.version ?? '' })" style="width: 480px">
    <n-alert type="info" :show-icon="false" style="margin: 0 0 12px">
      {{ t('dataMapping.rollbackHint', { version: rollbackTarget?.version ?? '' }) }}
    </n-alert>
    <n-radio-group v-model:value="rollbackMode">
      <n-space vertical>
        <n-radio value="draft">{{ t('dataMapping.rollbackDraftOnly') }}</n-radio>
        <n-radio value="publish">{{ t('dataMapping.rollbackPublish') }}</n-radio>
      </n-space>
    </n-radio-group>
    <template #action>
      <n-button @click="showRollback = false">{{ t('common.cancel') }}</n-button>
      <n-button type="warning" :loading="rollbackSaving" style="margin-left: 8px" @click="doRollback">{{ t('dataMapping.confirmRollback') }}</n-button>
    </template>
  </n-modal>

  <!-- 绑定管理抽屉：已绑型号列表 + 绑定/解绑（通用映射只读展示） -->
  <n-drawer v-model:show="showBindings" :width="640">
    <n-drawer-content :title="t('dataMapping.bindingTitle', { name: bindingDef?.name ?? '' })" closable>
      <n-space vertical>
        <n-space v-if="bindingDefWritable" align="center">
          <n-select
            v-model:value="bindModelId" :options="bindableModelOptions" filterable
            :placeholder="t('dataMapping.selectModel')" style="width: 320px" />
          <n-button type="primary" :disabled="!bindModelId" :loading="binding" @click="bind" v-permission="['mapping:bind']">
            {{ t('dataMapping.bind') }}
          </n-button>
        </n-space>
        <n-data-table :columns="boundColumns" :data="boundModels" :loading="bindingsLoading" size="small" :pagination="false" />
      </n-space>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NAlert, NButton, NCard, NDataTable, NDescriptions, NDescriptionsItem, NDrawer, NDrawerContent, NDynamicInput,
  NForm, NFormItem, NInput, NInputNumber, NModal, NRadio, NRadioGroup, NSelect, NSpace, NTag,
  useDialog, useMessage, type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import SearchCard from '../../components/SearchCard.vue'
import { renderActions, type TableAction } from '../../utils/tableActions'
import { usePagination } from '../../utils/pagination'
import { formatDateTime } from '../../utils/datetime'
import {
  bindDataMappingModel, createDataMapping, createDataMappingDraft, deleteDataMapping, deleteDataMappingVersion,
  listDataMappingBindings, listDataMappings, listDataMappingVersions, listDeviceModels,
  publishDataMapping, rollbackDataMapping, unbindDataMappingModel, updateDataMappingDraft,
} from '../../api'
import type { DataMappingDef, DataMappingVersion, DeviceModel, Mapping } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

function errMsg(e: any, fallback: string): string {
  return e?.response?.data?.msg || fallback
}

// source_topic 占位符字面值（locale 文案中的 {sn} 等需经插值参数原样输出，避免 vue-i18n 当作缺失变量）
const tmPlaceholders = { sn: '{sn}', device_id: '{device_id}', name: '{name}', model_id: '{model_id}' }

// ---- 资源列表 ----
const loading = ref(false)
const rows = ref<DataMappingDef[]>([])
const query = reactive({ kw: '', page: 1, page_size: 10 })

const { pagination } = usePagination(query, () => load())

const columns = computed<DataTableColumns<DataMappingDef>>(() => [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('dataMapping.mappingName'), key: 'name', minWidth: 160, ellipsis: { tooltip: true } },
  {
    title: t('dataMapping.scope'), key: 'merchant_id', width: 90,
    render: (row) => h(NTag, { type: row.merchant_id === 0 ? 'default' : 'info', size: 'small' }, { default: () => (row.merchant_id === 0 ? t('dataMapping.scopeCommon') : t('dataMapping.scopeMerchant')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 165, render: (row) => formatDateTime(row.created_at) },
  { title: t('common.updateTime'), key: 'updated_at', width: 165, render: (row) => formatDateTime(row.updated_at) },
  {
    title: t('common.operation'), key: 'actions', width: 210,
    render: (row) => {
      // 通用映射（merchant_id=0）平台写面 404：仅保留版本/绑定只读入口
      const actions: TableAction[] = [
        { label: t('dataMapping.versionManage'), accent: true, permission: 'mapping:versionList', onClick: () => openVersions(row) },
        { label: t('dataMapping.bindings'), permission: 'mapping:bindingList', onClick: () => openBindings(row) },
      ]
      if (row.merchant_id !== 0) {
        actions.push({ label: t('common.delete'), danger: true, permission: 'mapping:delete', onClick: () => confirmDeleteDef(row) })
      }
      return renderActions(actions)
    },
  },
])

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listDataMappings({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
    })
    rows.value = resp.data.list || []
    pagination.itemCount = resp.data.page?.total || 0
  } catch (e: any) {
    message.error(errMsg(e, t('dataMapping.listFailed')))
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.kw = ''
  load()
}

function confirmDeleteDef(row: DataMappingDef) {
  dialog.warning({
    title: t('dataMapping.deleteDefTitle'),
    content: t('dataMapping.deleteDefContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteDataMapping(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

// ---- 版本管理 ----
const showVersions = ref(false)
const currentDef = ref<DataMappingDef | null>(null)
const versions = ref<DataMappingVersion[]>([])
const versionsLoading = ref(false)

const defWritable = computed(() => !!currentDef.value && currentDef.value.merchant_id !== 0)
const draftVersion = computed(() => versions.value.find((v) => v.status === 'draft') ?? null)
const versionById = computed(() => new Map(versions.value.map((v) => [v.id, v])))

async function openVersions(row: DataMappingDef) {
  currentDef.value = row
  showVersions.value = true
  await reloadVersions()
}

async function reloadVersions() {
  if (!currentDef.value) return
  versionsLoading.value = true
  try {
    const { data: resp } = await listDataMappingVersions(currentDef.value.id)
    versions.value = resp.data.list || []
  } catch (e: any) {
    message.error(errMsg(e, t('common.loadFailed')))
  } finally {
    versionsLoading.value = false
  }
}

const versionColumns = computed<DataTableColumns<DataMappingVersion>>(() => [
  {
    title: t('dataMapping.version'), key: 'version', width: 170,
    render: (v) => {
      const src = v.revert_of ? versionById.value.get(v.revert_of) : null
      if (!src) return `v${v.version}`
      return h('span', { style: 'display: inline-flex; align-items: center; gap: 4px' }, [
        `v${v.version}`,
        h(NTag, { size: 'tiny', bordered: false }, { default: () => t('dataMapping.revertFrom', { version: src.version }) }),
      ])
    },
  },
  { title: t('dataMapping.label'), key: 'label', width: 120, render: (v) => v.label || '—' },
  {
    title: t('common.status'), key: 'status', width: 90,
    render: (v) => h(NTag, { size: 'small', type: v.status === 'published' ? 'success' : 'warning' }, { default: () => (v.status === 'published' ? t('dataMapping.statusPublished') : t('dataMapping.statusDraft')) }),
  },
  { title: t('dataMapping.publishedAt'), key: 'published_at', width: 165, render: (v) => formatDateTime(v.published_at) || '—' },
  {
    title: t('common.operation'), key: 'actions', width: 260,
    render: (v) => {
      const actions: TableAction[] = [
        { label: t('dataMapping.detail'), permission: 'mapping:versionView', onClick: () => openDetail(v) },
      ]
      if (defWritable.value && v.status === 'draft') {
        actions.push(
          { label: t('common.edit'), permission: 'mapping:updateDraft', onClick: () => openDraftEdit(v) },
          { label: t('dataMapping.publish'), accent: true, permission: 'mapping:publish', onClick: () => confirmPublish(v) },
        )
      }
      if (defWritable.value && v.status === 'published') {
        actions.push({ label: t('dataMapping.rollbackTo'), permission: 'mapping:rollback', onClick: () => openRollback(v) })
      }
      if (defWritable.value) {
        actions.push({ label: t('common.delete'), danger: true, permission: 'mapping:deleteVersion', onClick: () => confirmDeleteVersion(v) })
      }
      return renderActions(actions)
    },
  },
])

function confirmPublish(v: DataMappingVersion) {
  dialog.warning({
    title: t('dataMapping.publishConfirmTitle'),
    content: t('dataMapping.publishConfirmContent', { version: v.version }),
    positiveText: t('dataMapping.publish'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await publishDataMapping(v.id)
        message.success(t('dataMapping.published', { version: v.version }))
        await reloadVersions()
      } catch (e: any) {
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

function confirmDeleteVersion(v: DataMappingVersion) {
  dialog.warning({
    title: t('dataMapping.deleteDefTitle'),
    content: v.status === 'draft'
      ? t('dataMapping.deleteVersionContentDraft', { version: v.version })
      : t('dataMapping.deleteVersionContentPublished', { version: v.version }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteDataMappingVersion(v.id)
        message.success(t('dataMapping.versionDeleted', { version: v.version }))
        if (detailVersion.value?.id === v.id) showDetail.value = false
        await reloadVersions()
      } catch (e: any) {
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

// ---- 版本明细（只读）----
const showDetail = ref(false)
const detailVersion = ref<DataMappingVersion | null>(null)
const detailRevertFrom = computed(() => {
  const src = detailVersion.value?.revert_of ? versionById.value.get(detailVersion.value.revert_of) : null
  return src ? src.version : null
})

function openDetail(v: DataMappingVersion) {
  detailVersion.value = v
  showDetail.value = true
}

const detailMappingColumns = computed<DataTableColumns<Mapping>>(() => [
  { title: 'source_topic', key: 'source_topic', minWidth: 170 },
  {
    title: t('dataMapping.dataCategory'), key: 'data_category', width: 100,
    render: (m) => h(NTag, { size: 'small' }, { default: () => m.data_category }),
  },
  { title: 'data_type', key: 'data_type', minWidth: 140 },
  { title: t('dataMapping.sampleRate'), key: 'sample_rate_hz', width: 90, render: (m) => m.sample_rate_hz || '—' },
  {
    title: t('dataMapping.extractFieldsTrigger'), key: 'detail', minWidth: 220,
    render: (m) => {
      const parts: string[] = []
      if (m.field_extract && Object.keys(m.field_extract).length) {
        parts.push('extract: ' + Object.entries(m.field_extract).map(([k, v]) => `${k}=${v}`).join(', '))
      }
      if (m.fields?.length) parts.push('fields: ' + m.fields.join(','))
      if (m.trigger) parts.push('trigger: ' + m.trigger)
      return parts.length ? parts.join('；') : '—'
    },
  },
])

// ---- 草稿编辑器（create-def / create-draft / edit-draft 三态）----
interface EditRule {
  source_topic: string
  topic_match: 'exact' | 'glob' | null
  data_category: string
  data_type: string
  sample_rate_hz: number | null
  field_extract: { key: string; value: string }[]
  fields: string
  trigger: string
}

type EditorMode = 'create-def' | 'create-draft' | 'edit-draft'

const showEditor = ref(false)
const editorMode = ref<EditorMode>('create-def')
const editingVersion = ref<DataMappingVersion | null>(null)
const draftBaseVersion = ref<DataMappingVersion | null>(null) // create-draft 预填来源
const savingEditor = ref(false)
const editorForm = reactive({ name: '', label: '', mappings: [] as EditRule[] })

const categoryOptions = computed(() =>
  ['shadow', 'telemetry', 'event', 'alarm', 'media'].map((c) => ({
    label: t(`dataMapping.category.${c}`),
    value: c,
  })),
)

const topicMatchOptions = computed(() => [
  { label: t('dataMapping.topicMatchExact'), value: 'exact' },
  { label: t('dataMapping.topicMatchGlob'), value: 'glob' },
])

const editorTitle = computed(() => {
  if (editorMode.value === 'create-def') return t('dataMapping.createDefTitle')
  if (editorMode.value === 'create-draft') return t('dataMapping.createDraftTitle', { name: currentDef.value?.name ?? '' })
  return t('dataMapping.editDraftTitle', { version: editingVersion.value?.version ?? '', name: currentDef.value?.name ?? '' })
})

function toEditRule(r: Mapping): EditRule {
  return {
    source_topic: r.source_topic ?? '',
    topic_match: (r.topic_match as 'exact' | 'glob') || null,
    data_category: r.data_category || 'telemetry',
    data_type: r.data_type ?? '',
    sample_rate_hz: r.sample_rate_hz || null,
    field_extract: Object.entries(r.field_extract ?? {}).map(([key, value]) => ({ key, value: String(value) })),
    fields: (r.fields ?? []).join(','),
    trigger: r.trigger ?? '',
  }
}

function fromEditRule(r: EditRule): Mapping {
  const rule: Mapping = {
    source_topic: r.source_topic,
    data_category: r.data_category,
    data_type: r.data_type,
  }
  if (r.topic_match) rule.topic_match = r.topic_match
  if (r.sample_rate_hz != null) rule.sample_rate_hz = r.sample_rate_hz
  const fe = r.field_extract.filter((p) => p.key)
  if (fe.length) rule.field_extract = Object.fromEntries(fe.map((p) => [p.key, p.value]))
  const fields = r.fields.split(',').map((s) => s.trim()).filter(Boolean)
  if (fields.length) rule.fields = fields
  if (r.trigger) rule.trigger = r.trigger
  return rule
}

function addMapping() {
  editorForm.mappings.push({ source_topic: '', topic_match: null, data_category: 'telemetry', data_type: '', sample_rate_hz: null, field_extract: [], fields: '', trigger: '' })
}

function resetEditorForm(mappings: EditRule[] = []) {
  Object.assign(editorForm, { name: '', label: '', mappings })
  if (!editorForm.mappings.length) addMapping()
}

function openDefCreate() {
  editorMode.value = 'create-def'
  editingVersion.value = null
  draftBaseVersion.value = null
  resetEditorForm()
  showEditor.value = true
}

// 新建草稿：预填当前最新已发布版本的规则（无已发布则空白）
function openDraftCreate() {
  if (!currentDef.value) return
  editorMode.value = 'create-draft'
  editingVersion.value = null
  draftBaseVersion.value = versions.value.find((v) => v.status === 'published') ?? null
  resetEditorForm((draftBaseVersion.value?.mappings ?? []).map(toEditRule))
  showEditor.value = true
}

function openDraftEdit(v: DataMappingVersion) {
  editorMode.value = 'edit-draft'
  editingVersion.value = v
  draftBaseVersion.value = null
  resetEditorForm((v.mappings ?? []).map(toEditRule))
  editorForm.label = v.label ?? ''
  showEditor.value = true
}

async function saveEditor() {
  if (editorMode.value === 'create-def' && !editorForm.name.trim()) {
    message.warning(t('dataMapping.mappingNameRequired'))
    return
  }
  const rules = editorForm.mappings
  if (!rules.length || rules.some((r) => !r.source_topic.trim() || !r.data_type.trim())) {
    message.warning(t('dataMapping.ruleIncomplete'))
    return
  }
  const mappings = rules.map(fromEditRule)
  savingEditor.value = true
  try {
    if (editorMode.value === 'create-def') {
      const { data: resp } = await createDataMapping({ name: editorForm.name, label: editorForm.label || undefined, mappings })
      message.success(t('dataMapping.defCreated', { version: resp.data.version.version }))
      showEditor.value = false
      load()
      // 顺势打开版本管理，便于继续编辑/发布首版
      await openVersions(resp.data.def)
    } else if (editorMode.value === 'create-draft') {
      if (!currentDef.value) return
      const { data: resp } = await createDataMappingDraft(currentDef.value.id, { label: editorForm.label || undefined, mappings })
      message.success(t('dataMapping.draftCreated', { version: resp.data.version }))
      showEditor.value = false
      await reloadVersions()
    } else {
      if (!editingVersion.value) return
      await updateDataMappingDraft(editingVersion.value.id, { label: editorForm.label, mappings })
      message.success(t('dataMapping.draftSaved', { version: editingVersion.value.version }))
      showEditor.value = false
      await reloadVersions()
    }
  } catch (e: any) {
    message.error(errMsg(e, t('common.failed')))
  } finally {
    savingEditor.value = false
  }
}

// ---- 回退（追加式）----
const showRollback = ref(false)
const rollbackTarget = ref<DataMappingVersion | null>(null)
const rollbackMode = ref<'draft' | 'publish'>('draft')
const rollbackSaving = ref(false)

function openRollback(v: DataMappingVersion) {
  rollbackTarget.value = v
  rollbackMode.value = 'draft'
  showRollback.value = true
}

async function doRollback() {
  const target = rollbackTarget.value
  if (!target) return
  rollbackSaving.value = true
  try {
    const publish = rollbackMode.value === 'publish'
    const { data: resp } = await rollbackDataMapping(target.id, publish)
    showRollback.value = false
    if (publish) {
      message.success(t('dataMapping.rollbackPublished', { version: resp.data.version, src: target.version }))
    } else {
      message.success(t('dataMapping.rollbackDraftCreated', { version: resp.data.version, src: target.version }))
    }
    await reloadVersions()
  } catch (e: any) {
    message.error(errMsg(e, t('common.failed')))
  } finally {
    rollbackSaving.value = false
  }
}

// ---- 型号绑定 ----
const showBindings = ref(false)
const bindingDef = ref<DataMappingDef | null>(null)
const boundModels = ref<DeviceModel[]>([])
const bindingsLoading = ref(false)
const allModels = ref<DeviceModel[]>([])
const bindModelId = ref<number | null>(null)
const binding = ref(false)

const bindingDefWritable = computed(() => !!bindingDef.value && bindingDef.value.merchant_id !== 0)

const bindableModelOptions = computed(() =>
  allModels.value
    .filter((m) => !boundModels.value.some((b) => b.id === m.id))
    .map((m) => ({ label: `${m.name}（${m.code}）`, value: m.id })),
)

async function loadModels() {
  try {
    // page_size=0 全量：绑定选择器候选
    const { data: resp } = await listDeviceModels({ page: 1, page_size: 0 })
    allModels.value = resp.data.list || []
  } catch { /* 选择器加载失败不阻断列表 */ }
}

async function loadBindings() {
  if (!bindingDef.value) return
  bindingsLoading.value = true
  try {
    const { data: resp } = await listDataMappingBindings(bindingDef.value.id)
    boundModels.value = resp.data.list || []
    bindModelId.value = null
  } catch (e: any) {
    message.error(errMsg(e, t('common.loadFailed')))
  } finally {
    bindingsLoading.value = false
  }
}

function openBindings(row: DataMappingDef) {
  bindingDef.value = row
  showBindings.value = true
  loadBindings()
}

async function bind() {
  if (!bindingDef.value || !bindModelId.value) return
  binding.value = true
  try {
    await bindDataMappingModel(bindingDef.value.id, bindModelId.value)
    message.success(t('dataMapping.bindSuccess'))
    loadBindings()
  } catch (e: any) {
    message.error(errMsg(e, t('common.failed')))
  } finally {
    binding.value = false
  }
}

function unbind(m: DeviceModel) {
  if (!bindingDef.value) return
  dialog.warning({
    title: t('dataMapping.unbindTitle'),
    content: t('dataMapping.unbindContent', { name: m.name, code: m.code }),
    positiveText: t('dataMapping.unbind'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await unbindDataMappingModel(bindingDef.value!.id, m.id)
        message.success(t('dataMapping.unbindSuccess'))
        loadBindings()
      } catch (e: any) {
        message.error(errMsg(e, t('common.failed')))
      }
    },
  })
}

const boundColumns = computed<DataTableColumns<DeviceModel>>(() => [
  { title: t('model.code'), key: 'code', width: 140 },
  { title: t('model.name'), key: 'name', minWidth: 160 },
  { title: t('model.manufacturer'), key: 'manufacturer', width: 140, render: (m) => m.manufacturer || '—' },
  {
    title: t('common.operation'), key: 'actions', width: 90,
    render: (m) => {
      if (!bindingDefWritable.value) return '—'
      return renderActions([
        { label: t('dataMapping.unbind'), danger: true, permission: 'mapping:unbind', onClick: () => unbind(m) },
      ])
    },
  },
])

onMounted(() => {
  load()
  loadModels()
})
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
}
.mapping-card {
  border: 1px solid var(--sx-line);
  border-radius: 4px;
  padding: 10px;
}
</style>
