<!-- 物模型 Schema 只读查看（页面私有组件）：属性/服务/事件三页签表格 + 关键字过滤 + 原始 JSON。
     服务/事件参数以行展开呈现。文案走 thingModel.schema.* -->
<template>
  <div v-if="schema" class="tm-schema-viewer">
    <div class="tm-schema-viewer__toolbar">
      <n-input v-model:value="keyword" size="small" clearable :placeholder="t('thingModel.schema.filterPh')" style="width: 220px" />
    </div>
    <n-tabs type="line" size="small">
      <n-tab-pane name="properties" :tab="`${t('thingModel.schema.properties')} (${propRows.length})`">
        <n-data-table :columns="propColumns" :data="filteredRows(propRows)" :row-key="(r: PropRow) => r.key" size="small" :max-height="360" />
      </n-tab-pane>
      <n-tab-pane name="services" :tab="`${t('thingModel.schema.services')} (${serviceRows.length})`">
        <n-data-table :columns="serviceColumns" :data="filteredRows(serviceRows)" :row-key="(r: DefRow) => r.key" size="small" :max-height="360" />
      </n-tab-pane>
      <n-tab-pane name="events" :tab="`${t('thingModel.schema.events')} (${eventRows.length})`">
        <n-data-table :columns="eventColumns" :data="filteredRows(eventRows)" :row-key="(r: DefRow) => r.key" size="small" :max-height="360" />
      </n-tab-pane>
      <n-tab-pane name="raw" :tab="t('thingModel.schema.rawJson')">
        <pre class="tm-schema-viewer__raw">{{ rawText }}</pre>
      </n-tab-pane>
    </n-tabs>
  </div>
  <n-empty v-else :description="t('thingModel.schema.empty')" size="small" style="padding: 16px 0" />
</template>

<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { NDataTable, NEmpty, NInput, NTabPane, NTabs, NTag, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import type { TMParam, TMSchema } from '../../api/types'

const props = defineProps<{ schema: TMSchema | null }>()
const { t } = useI18n()

const keyword = ref('')

interface PropRow {
  key: string
  data_type?: string
  unit?: string
  writable?: boolean
  enum?: string[]
  min?: number
  max?: number
  description?: string
}
interface DefRow {
  key: string
  call_type?: string
  params?: Record<string, TMParam>
  description?: string
}

const propRows = computed<PropRow[]>(() =>
  Object.entries(props.schema?.properties ?? {}).map(([key, p]) => ({ key, ...p })),
)
const serviceRows = computed<DefRow[]>(() =>
  Object.entries(props.schema?.services ?? {}).map(([key, s]) => ({ key, ...s })),
)
const eventRows = computed<DefRow[]>(() =>
  Object.entries(props.schema?.events ?? {}).map(([key, e]) => ({ key, ...e })),
)
const rawText = computed(() => JSON.stringify(props.schema ?? {}, null, 2))

// 标识符/描述关键字过滤（对三个页签统一生效）
function filteredRows<T extends PropRow | DefRow>(rows: T[]): T[] {
  const kw = keyword.value.trim()
  if (!kw) return rows
  return rows.filter((r) => r.key.includes(kw) || (r.description ?? '').includes(kw))
}

// 参数子表格（服务/事件展开行共用）
const paramColumns = computed<DataTableColumns<{ name: string; def: TMParam }>>(() => [
  { title: t('thingModel.schema.param'), key: 'name', width: 140, render: (r) => h('code', null, r.name) },
  { title: t('thingModel.schema.dataType'), key: 'data_type', width: 100 },
  {
    title: t('thingModel.schema.required'), key: 'required', width: 70,
    render: (r) => h(NTag, { size: 'small', type: r.def.required ? 'error' : 'default', bordered: false }, { default: () => (r.def.required ? t('thingModel.schema.requiredYes') : t('thingModel.schema.requiredNo')) }),
  },
  { title: t('thingModel.schema.enum'), key: 'enum', render: (r) => r.def.enum?.join('、') ?? '—' },
  { title: t('thingModel.schema.description'), key: 'description', ellipsis: { tooltip: true }, render: (r) => r.def.description ?? '—' },
])

function paramTable(params?: Record<string, TMParam>) {
  const rows = Object.entries(params ?? {}).map(([name, def]) => ({ name, def }))
  if (!rows.length) return h('span', { style: 'color: var(--sx-muted); font-size: 12px' }, t('thingModel.schema.noParams'))
  return h(NDataTable, { columns: paramColumns.value, data: rows, rowKey: (r: { name: string }) => r.name, size: 'small' })
}

const propColumns = computed<DataTableColumns<PropRow>>(() => [
  { title: t('thingModel.schema.identifier'), key: 'key', width: 160, render: (r) => h('code', null, r.key) },
  { title: t('thingModel.schema.dataType'), key: 'data_type', width: 90 },
  { title: t('thingModel.schema.unit'), key: 'unit', width: 70, render: (r) => r.unit ?? '—' },
  {
    title: t('thingModel.schema.writable'), key: 'writable', width: 70,
    render: (r) => r.writable === undefined
      ? '—'
      : h(NTag, { size: 'small', type: r.writable ? 'success' : 'default', bordered: false }, { default: () => (r.writable ? t('thingModel.schema.writableYes') : t('thingModel.schema.writableNo')) }),
  },
  {
    title: t('thingModel.schema.enum'), key: 'enum', width: 140, ellipsis: { tooltip: true },
    render: (r) => r.enum?.join('、') ?? '—',
  },
  {
    title: t('thingModel.schema.range'), key: 'range', width: 110,
    render: (r) => (r.min === undefined && r.max === undefined ? '—' : `${r.min ?? '—'} ~ ${r.max ?? '—'}`),
  },
  { title: t('thingModel.schema.description'), key: 'description', ellipsis: { tooltip: true }, render: (r) => r.description ?? '—' },
])

const serviceColumns = computed<DataTableColumns<DefRow>>(() => [
  { type: 'expand', renderExpand: (r) => paramTable(r.params) },
  { title: t('thingModel.schema.identifier'), key: 'key', width: 180, render: (r) => h('code', null, r.key) },
  { title: t('thingModel.schema.callType'), key: 'call_type', width: 100, render: (r) => r.call_type ?? '—' },
  { title: t('thingModel.schema.params'), key: 'params', width: 80, render: (r) => t('thingModel.schema.paramCount', { n: Object.keys(r.params ?? {}).length }) },
  { title: t('thingModel.schema.description'), key: 'description', ellipsis: { tooltip: true }, render: (r) => r.description ?? '—' },
])

const eventColumns = computed<DataTableColumns<DefRow>>(() => [
  { type: 'expand', renderExpand: (r) => paramTable(r.params) },
  { title: t('thingModel.schema.identifier'), key: 'key', width: 180, render: (r) => h('code', null, r.key) },
  { title: t('thingModel.schema.params'), key: 'params', width: 80, render: (r) => t('thingModel.schema.paramCount', { n: Object.keys(r.params ?? {}).length }) },
  { title: t('thingModel.schema.description'), key: 'description', ellipsis: { tooltip: true }, render: (r) => r.description ?? '—' },
])
</script>

<style scoped>
.tm-schema-viewer__toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 4px;
}
.tm-schema-viewer__raw {
  background: var(--sx-bg);
  padding: 12px;
  border-radius: 6px;
  font-size: 12px;
  max-height: 420px;
  overflow: auto;
  margin: 0;
}
</style>
