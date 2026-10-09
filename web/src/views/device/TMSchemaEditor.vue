<!-- 物模型 Schema 结构化编辑器（页面私有组件）：属性/服务/事件三个可编辑表格页签 + JSON 预览。
     父组件通过 ref 调用 build() 取校验后的 Schema；initial 变化或组件重挂载（:key 递增）时重新装载。
     文案走 thingModel.schema.* / thingModel.editor.* -->
<template>
  <div class="tm-schema-editor">
    <n-tabs type="line" size="small">
      <n-tab-pane name="properties" :tab="`${t('thingModel.schema.properties')} (${propRows.length})`">
        <div class="tm-schema-editor__bar">
          <n-button size="tiny" @click="addProp">{{ t('thingModel.editor.addProperty') }}</n-button>
          <span class="tm-schema-editor__hint">{{ t('thingModel.editor.identHint') }}</span>
        </div>
        <n-data-table :columns="propColumns" :data="propRows" :row-key="(r: PropRow) => r._id" size="small" :max-height="320" />
      </n-tab-pane>
      <n-tab-pane name="services" :tab="`${t('thingModel.schema.services')} (${serviceRows.length})`">
        <div class="tm-schema-editor__bar">
          <n-button size="tiny" @click="addService">{{ t('thingModel.editor.addService') }}</n-button>
          <span class="tm-schema-editor__hint">{{ t('thingModel.editor.serviceHint') }}</span>
        </div>
        <n-data-table :columns="serviceColumns" :data="serviceRows" :row-key="(r: DefRow) => r._id" size="small" :max-height="320" />
      </n-tab-pane>
      <n-tab-pane name="events" :tab="`${t('thingModel.schema.events')} (${eventRows.length})`">
        <div class="tm-schema-editor__bar">
          <n-button size="tiny" @click="addEvent">{{ t('thingModel.editor.addEvent') }}</n-button>
          <span class="tm-schema-editor__hint">{{ t('thingModel.editor.eventHint') }}</span>
        </div>
        <n-data-table :columns="eventColumns" :data="eventRows" :row-key="(r: DefRow) => r._id" size="small" :max-height="320" />
      </n-tab-pane>
      <n-tab-pane name="raw" :tab="t('thingModel.schema.jsonPreview')">
        <pre class="tm-schema-editor__raw">{{ previewText }}</pre>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { NButton, NDataTable, NInput, NInputNumber, NSelect, NSwitch, NTabPane, NTabs, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import type { TMParam, TMProperty, TMSchema } from '../../api/types'

const props = defineProps<{ initial?: TMSchema | null }>()
const { t } = useI18n()

// ---- 行模型（enum/min/max 转为编辑友好的文本/可空数值，构建时再还原）----

interface PropRow {
  _id: number
  key: string
  data_type: string
  unit: string
  writable: boolean
  enumText: string
  min: number | null
  max: number | null
  description: string
}
interface ParamRow {
  _id: number
  name: string
  data_type: string
  required: boolean
  enumText: string
  description: string
}
interface DefRow {
  _id: number
  key: string
  call_type: string
  description: string
  params: ParamRow[]
}

let uid = 0
const nid = () => ++uid
const propRows = ref<PropRow[]>([])
const serviceRows = ref<DefRow[]>([])
const eventRows = ref<DefRow[]>([])

const dataTypeOptions = ['int', 'float', 'bool', 'string', 'enum', 'json'].map((v) => ({ label: v, value: v }))
const callTypeOptions = computed(() => [
  { label: t('thingModel.editor.callTypeSync'), value: 'sync' },
  { label: t('thingModel.editor.callTypeAsync'), value: 'async' },
])

// ---- 装载：Schema → 行 ----

function parseEnumText(arr?: string[]): string {
  return (arr ?? []).join(',')
}
function toParamRows(params?: Record<string, TMParam>): ParamRow[] {
  return Object.entries(params ?? {}).map(([name, p]) => ({
    _id: nid(), name, data_type: p.data_type || 'string', required: !!p.required,
    enumText: parseEnumText(p.enum), description: p.description ?? '',
  }))
}

function load(schema: TMSchema | null | undefined) {
  propRows.value = Object.entries(schema?.properties ?? {}).map(([key, p]) => ({
    _id: nid(), key, data_type: p.data_type || 'string', unit: p.unit ?? '', writable: !!p.writable,
    enumText: parseEnumText(p.enum), min: p.min ?? null, max: p.max ?? null,
    description: p.description ?? '',
  }))
  serviceRows.value = Object.entries(schema?.services ?? {}).map(([key, s]) => ({
    _id: nid(), key, call_type: s.call_type || 'sync', description: s.description ?? '',
    params: toParamRows(s.params),
  }))
  eventRows.value = Object.entries(schema?.events ?? {}).map(([key, e]) => ({
    _id: nid(), key, call_type: '', description: e.description ?? '',
    params: toParamRows(e.params),
  }))
}
watch(() => props.initial, load, { immediate: true })

// ---- 新增行 ----

function addProp() {
  propRows.value.push({
    _id: nid(), key: '', data_type: 'string', unit: '', writable: false,
    enumText: '', min: null, max: null, description: '',
  })
}
function addService() {
  serviceRows.value.push({ _id: nid(), key: '', call_type: 'sync', description: '', params: [] })
}
function addEvent() {
  eventRows.value.push({ _id: nid(), key: '', call_type: '', description: '', params: [] })
}
function addParam(def: DefRow) {
  def.params.push({ _id: nid(), name: '', data_type: 'string', required: false, enumText: '', description: '' })
}

// ---- 构建：行 → Schema（含校验）----

const IDENT_RE = /^[A-Za-z_][A-Za-z0-9_]*$/

function splitEnum(text: string): string[] {
  return text.split(/[,，]/).map((s) => s.trim()).filter(Boolean)
}

function buildParams(rows: ParamRow[], label: string, errors: string[]): Record<string, TMParam> | undefined {
  const seen = new Set<string>()
  const out: Record<string, TMParam> = {}
  for (const p of rows) {
    if (!p.name && !p.description && !p.enumText) continue // 整行为空跳过
    if (!p.name) {
      errors.push(t('thingModel.editor.errParamNameEmpty', { at: label }))
      continue
    }
    if (!IDENT_RE.test(p.name)) {
      errors.push(t('thingModel.editor.errParamNameInvalid', { at: label, name: p.name }))
      continue
    }
    if (seen.has(p.name)) {
      errors.push(t('thingModel.editor.errParamNameDup', { at: label, name: p.name }))
      continue
    }
    seen.add(p.name)
    const enumArr = splitEnum(p.enumText)
    if (p.data_type === 'enum' && !enumArr.length) errors.push(t('thingModel.editor.errEnumRequired', { at: label, name: p.name }))
    out[p.name] = {
      data_type: p.data_type,
      required: p.required || undefined,
      enum: enumArr.length ? enumArr : undefined,
      description: p.description || undefined,
    }
  }
  return Object.keys(out).length ? out : undefined
}

function buildDefRows(rows: DefRow[], labelKey: string, withCallType: boolean, errors: string[]) {
  const seen = new Set<string>()
  const out: Record<string, { call_type?: string; params?: Record<string, TMParam>; description?: string }> = {}
  rows.forEach((row, i) => {
    const at = t(labelKey, { n: i + 1 })
    if (!row.key && !row.description) return
    if (!row.key) {
      errors.push(t('thingModel.editor.errIdentEmpty', { at }))
      return
    }
    if (!IDENT_RE.test(row.key)) {
      errors.push(t('thingModel.editor.errIdentInvalid', { at, name: row.key }))
      return
    }
    if (seen.has(row.key)) {
      errors.push(t('thingModel.editor.errIdentDup', { at, name: row.key }))
      return
    }
    seen.add(row.key)
    out[row.key] = {
      ...(withCallType ? { call_type: row.call_type } : {}),
      params: buildParams(row.params, t('thingModel.editor.paramsOf', { name: row.key }), errors),
      description: row.description || undefined,
    }
  })
  return Object.keys(out).length ? out : undefined
}

function build(): { ok: boolean; errors: string[]; schema: TMSchema } {
  const errors: string[] = []
  const schema: TMSchema = {}

  const properties: Record<string, TMProperty> = {}
  const seenProp = new Set<string>()
  propRows.value.forEach((row, i) => {
    const at = t('thingModel.editor.atProperty', { n: i + 1 })
    if (!row.key && !row.description && !row.enumText && !row.unit) return
    if (!row.key) {
      errors.push(t('thingModel.editor.errIdentEmpty', { at }))
      return
    }
    if (!IDENT_RE.test(row.key)) {
      errors.push(t('thingModel.editor.errIdentInvalid', { at, name: row.key }))
      return
    }
    if (seenProp.has(row.key)) {
      errors.push(t('thingModel.editor.errIdentDup', { at, name: row.key }))
      return
    }
    seenProp.add(row.key)
    const enumArr = splitEnum(row.enumText)
    if (row.data_type === 'enum' && !enumArr.length) errors.push(t('thingModel.editor.errEnumRequired', { at, name: row.key }))
    if (row.min !== null && row.max !== null && row.min > row.max) errors.push(t('thingModel.editor.errMinMax', { at, name: row.key }))
    properties[row.key] = {
      data_type: row.data_type,
      unit: row.unit || undefined,
      writable: row.writable,
      enum: enumArr.length ? enumArr : undefined,
      min: row.min ?? undefined,
      max: row.max ?? undefined,
      description: row.description || undefined,
    }
  })
  if (Object.keys(properties).length) schema.properties = properties

  const services = buildDefRows(serviceRows.value, 'thingModel.editor.atService', true, errors)
  if (services) schema.services = services as NonNullable<TMSchema['services']>
  const events = buildDefRows(eventRows.value, 'thingModel.editor.atEvent', false, errors)
  if (events) schema.events = events as NonNullable<TMSchema['events']>

  return { ok: errors.length === 0, errors, schema }
}
defineExpose({ build })

// JSON 预览：按当前行宽松序列化（空标识符行跳过，仅预览不作校验）
const previewText = computed(() => {
  const lenient: TMSchema = {
    properties: Object.fromEntries(propRows.value.filter((r) => r.key).map((r) => [r.key, {
      data_type: r.data_type, unit: r.unit || undefined, writable: r.writable,
      enum: splitEnum(r.enumText).length ? splitEnum(r.enumText) : undefined,
      min: r.min ?? undefined, max: r.max ?? undefined, description: r.description || undefined,
    }])),
    services: Object.fromEntries(serviceRows.value.filter((r) => r.key).map((r) => [r.key, {
      call_type: r.call_type, description: r.description || undefined,
      params: Object.fromEntries(r.params.filter((p) => p.name).map((p) => [p.name, {
        data_type: p.data_type, required: p.required || undefined,
        enum: splitEnum(p.enumText).length ? splitEnum(p.enumText) : undefined,
        description: p.description || undefined,
      }])),
    }])),
    events: Object.fromEntries(eventRows.value.filter((r) => r.key).map((r) => [r.key, {
      description: r.description || undefined,
      params: Object.fromEntries(r.params.filter((p) => p.name).map((p) => [p.name, {
        data_type: p.data_type, required: p.required || undefined,
        enum: splitEnum(p.enumText).length ? splitEnum(p.enumText) : undefined,
        description: p.description || undefined,
      }])),
    }])),
  }
  return JSON.stringify(lenient, null, 2)
})

// ---- 表格列（render 内直接改行对象，n-data-table 依赖其响应式重渲染）----

function opColumn<T extends { _id: number }>(rows: { value: T[] }) {
  return {
    title: t('common.operation'), key: '__op', width: 56,
    render: (row: T) => h(NButton, {
      size: 'tiny', type: 'error', ghost: true,
      onClick: () => { rows.value = rows.value.filter((r) => r._id !== row._id) },
    }, { default: () => t('common.delete') }),
  }
}

const propColumns = computed<DataTableColumns<PropRow>>(() => [
  opColumn(propRows),
  { title: t('thingModel.schema.identifier'), key: 'key', width: 150, render: (r) => h(NInput, { value: r.key, size: 'small', placeholder: t('thingModel.editor.propKeyPh'), onUpdateValue: (v: string) => (r.key = v) }) },
  { title: t('thingModel.schema.dataType'), key: 'data_type', width: 100, render: (r) => h(NSelect, { value: r.data_type, size: 'small', options: dataTypeOptions, onUpdateValue: (v: string) => (r.data_type = v) }) },
  { title: t('thingModel.schema.unit'), key: 'unit', width: 80, render: (r) => h(NInput, { value: r.unit, size: 'small', placeholder: '%', onUpdateValue: (v: string) => (r.unit = v) }) },
  { title: t('thingModel.schema.writable'), key: 'writable', width: 60, render: (r) => h(NSwitch, { value: r.writable, size: 'small', onUpdateValue: (v: boolean) => (r.writable = v) }) },
  { title: t('thingModel.schema.enum'), key: 'enum', width: 140, ellipsis: { tooltip: true }, render: (r) => h(NInput, { value: r.enumText, size: 'small', placeholder: t('thingModel.editor.enumPh'), onUpdateValue: (v: string) => (r.enumText = v) }) },
  { title: 'min', key: 'min', width: 90, render: (r) => h(NInputNumber, { value: r.min, size: 'small', onUpdateValue: (v: number | null) => (r.min = v) }) },
  { title: 'max', key: 'max', width: 90, render: (r) => h(NInputNumber, { value: r.max, size: 'small', onUpdateValue: (v: number | null) => (r.max = v) }) },
  { title: t('thingModel.schema.description'), key: 'description', render: (r) => h(NInput, { value: r.description, size: 'small', onUpdateValue: (v: string) => (r.description = v) }) },
])

const paramColumnsOf = (def: DefRow): DataTableColumns<ParamRow> => [
  {
    title: t('common.operation'), key: '__op', width: 56,
    render: (p) => h(NButton, {
      size: 'tiny', type: 'error', ghost: true,
      onClick: () => { def.params = def.params.filter((x) => x._id !== p._id) },
    }, { default: () => t('common.delete') }),
  },
  { title: t('thingModel.schema.param'), key: 'name', width: 150, render: (p) => h(NInput, { value: p.name, size: 'small', onUpdateValue: (v: string) => (p.name = v) }) },
  { title: t('thingModel.schema.dataType'), key: 'data_type', width: 100, render: (p) => h(NSelect, { value: p.data_type, size: 'small', options: dataTypeOptions, onUpdateValue: (v: string) => (p.data_type = v) }) },
  { title: t('thingModel.schema.required'), key: 'required', width: 60, render: (p) => h(NSwitch, { value: p.required, size: 'small', onUpdateValue: (v: boolean) => (p.required = v) }) },
  { title: t('thingModel.schema.enum'), key: 'enum', width: 140, render: (p) => h(NInput, { value: p.enumText, size: 'small', placeholder: t('thingModel.editor.enumPh'), onUpdateValue: (v: string) => (p.enumText = v) }) },
  { title: t('thingModel.schema.description'), key: 'description', render: (p) => h(NInput, { value: p.description, size: 'small', onUpdateValue: (v: string) => (p.description = v) }) },
]

function paramEditor(def: DefRow) {
  return h('div', { style: 'padding: 4px 0' }, [
    h(NDataTable, {
      columns: paramColumnsOf(def), data: def.params, rowKey: (p: ParamRow) => p._id, size: 'small',
    }),
    h(NButton, { size: 'tiny', style: 'margin-top: 6px', onClick: () => addParam(def) }, { default: () => t('thingModel.editor.addParam') }),
  ])
}

const serviceColumns = computed<DataTableColumns<DefRow>>(() => [
  { type: 'expand', renderExpand: (r) => paramEditor(r) },
  opColumn(serviceRows),
  { title: t('thingModel.schema.identifier'), key: 'key', width: 170, render: (r) => h(NInput, { value: r.key, size: 'small', placeholder: t('thingModel.editor.serviceKeyPh'), onUpdateValue: (v: string) => (r.key = v) }) },
  { title: t('thingModel.schema.callType'), key: 'call_type', width: 130, render: (r) => h(NSelect, { value: r.call_type, size: 'small', options: callTypeOptions.value, onUpdateValue: (v: string) => (r.call_type = v) }) },
  { title: t('thingModel.schema.params'), key: 'params', width: 70, render: (r) => t('thingModel.schema.paramCount', { n: r.params.length }) },
  { title: t('thingModel.schema.description'), key: 'description', render: (r) => h(NInput, { value: r.description, size: 'small', onUpdateValue: (v: string) => (r.description = v) }) },
])

const eventColumns = computed<DataTableColumns<DefRow>>(() => [
  { type: 'expand', renderExpand: (r) => paramEditor(r) },
  opColumn(eventRows),
  { title: t('thingModel.schema.identifier'), key: 'key', width: 170, render: (r) => h(NInput, { value: r.key, size: 'small', placeholder: t('thingModel.editor.eventKeyPh'), onUpdateValue: (v: string) => (r.key = v) }) },
  { title: t('thingModel.schema.params'), key: 'params', width: 70, render: (r) => t('thingModel.schema.paramCount', { n: r.params.length }) },
  { title: t('thingModel.schema.description'), key: 'description', render: (r) => h(NInput, { value: r.description, size: 'small', onUpdateValue: (v: string) => (r.description = v) }) },
])
</script>

<style scoped>
.tm-schema-editor__bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}
.tm-schema-editor__hint {
  font-size: 12px;
  color: var(--sx-muted);
}
.tm-schema-editor__raw {
  background: var(--sx-bg);
  padding: 12px;
  border-radius: 6px;
  font-size: 12px;
  max-height: 420px;
  overflow: auto;
  margin: 0;
}
</style>
