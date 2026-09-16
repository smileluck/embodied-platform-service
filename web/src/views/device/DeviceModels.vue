<template>
  <SearchCard storage-key="device-models" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('model.kwPlaceholder')" clearable style="width: 200px" @keyup.enter="load" />
    <n-select v-model:value="query.status" :options="statusOptions" :placeholder="t('common.status')" clearable style="width: 130px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openCreate" v-permission="['model:create']">{{ t('model.newModel') }}</n-button>
      </div>
    </template>

    <!-- 型号真源在 embodied-platform 管理面（服务账号代理）；创建须绑物模型节点+已发布版本 -->
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 新增/编辑 -->
  <n-modal v-model:show="showModal" preset="dialog" :title="editing ? t('model.editModel') : t('model.newModel')" style="width: 520px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="110">
      <n-form-item :label="t('model.code')" path="code" v-if="!editing">
        <n-input v-model:value="form.code" :maxlength="64" :placeholder="t('model.codePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('model.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('model.tmNode')" path="tm_node_id">
        <n-select v-model:value="form.tm_node_id" :options="tmNodeOptions" filterable :placeholder="t('model.tmNodePlaceholder')" :disabled="editing" @update:value="onNodeChange" />
      </n-form-item>
      <n-form-item :label="t('model.tmVersion')" path="tm_version_id">
        <n-select v-model:value="form.tm_version_id" :options="tmVersionOptions" :placeholder="t('model.tmVersionPlaceholder')" :loading="versionsLoading" />
      </n-form-item>
      <n-form-item :label="t('model.manufacturer')">
        <n-input v-model:value="form.manufacturer" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('common.remark')">
        <n-input v-model:value="form.description" type="textarea" :maxlength="255" :autosize="{ minRows: 2, maxRows: 4 }" />
      </n-form-item>
      <n-form-item :label="t('device.transport')">
        <n-select v-model:value="form.transport" :options="transportOptions" clearable :placeholder="t('device.transportDefault')" />
      </n-form-item>
      <n-form-item :label="t('common.status')" v-if="editing">
        <n-select v-model:value="form.status" :options="statusOptions" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NCard, NInput, NButton, NDataTable, NModal, NForm, NFormItem, NSelect, NTag, useMessage, useDialog, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import {
  createDeviceModel, deleteDeviceModel, listDeviceModels,
  listThingModelNodes, listThingModelVersions, updateDeviceModel,
} from '../../api'
import { usePagination } from '../../utils/pagination'
import type { DeviceModel, TMNode, TMVersion } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const saving = ref(false)
const rows = ref<DeviceModel[]>([])
const query = reactive({ kw: '', status: null as number | null, page: 1, page_size: 10 })

const statusOptions = computed(() => [
  { label: t('common.enabled'), value: 1 },
  { label: t('common.disabled'), value: 2 },
])
const transportOptions = [
  { label: 'Socket', value: 'socket' },
  { label: 'MQTT', value: 'mqtt' },
]

const { pagination } = usePagination(query, () => load())

const columns: DataTableColumns<DeviceModel> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('model.code'), key: 'code', width: 140 },
  { title: t('model.name'), key: 'name', minWidth: 120, ellipsis: { tooltip: true } },
  {
    title: t('model.tmBinding'), key: 'tm_node_name', width: 200,
    render: (row) => `${row.tm_node_name || `#${row.tm_node_id}`} · v${row.tm_version_no ?? '?'}`,
  },
  { title: t('model.manufacturer'), key: 'manufacturer', width: 140, render: (row) => row.manufacturer || '—' },
  { title: t('device.transportCol'), key: 'transport', width: 90, render: (row) => row.transport || '—' },
  {
    title: t('common.status'), key: 'status', width: 80,
    render: (row) => h(NTag, { type: row.status === 1 ? 'success' : 'error', size: 'small' }, { default: () => (row.status === 1 ? t('common.enabled') : t('common.disabled')) }),
  },
  { title: t('common.createTime'), key: 'created_at', width: 160 },
  {
    title: t('common.operation'), key: 'actions', width: 130,
    render: (row) => {
      const actions: TableAction[] = [
        { label: t('common.edit'), accent: true, permission: 'model:update', onClick: () => openEdit(row) },
        { label: t('common.delete'), danger: true, permission: 'model:delete', onClick: () => confirmDelete(row) },
      ]
      return renderActions(actions)
    },
  },
]

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listDeviceModels({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
      status: query.status ?? undefined,
    })
    rows.value = resp.data.list || []
    pagination.itemCount = resp.data.page?.total || 0
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('model.listFailed'))
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.kw = ''
  query.status = null
  load()
}

// ---- 物模型选择器（只读；layer=model 为型号可绑定的层） ----
const tmNodeOptions = ref<{ label: string; value: number }[]>([])
const tmVersionOptions = ref<{ label: string; value: number }[]>([])
const versionsLoading = ref(false)

async function loadTMNodes() {
  try {
    const { data: resp } = await listThingModelNodes({ layer: 'model' })
    tmNodeOptions.value = (resp.data || []).map((n: TMNode) => ({ label: `${n.name}（${n.code}）`, value: n.id }))
  } catch { /* 选择器加载失败不阻断列表 */ }
}

async function onNodeChange(nodeId: number) {
  form.tm_version_id = null
  tmVersionOptions.value = []
  if (!nodeId) return
  versionsLoading.value = true
  try {
    const { data: resp } = await listThingModelVersions(nodeId)
    tmVersionOptions.value = (resp.data || []).map((v: TMVersion) => ({
      label: `v${v.version}（${t('model.publishedAt')} ${v.published_at || '—'}）`,
      value: v.id,
    }))
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('model.listFailed'))
  } finally {
    versionsLoading.value = false
  }
}

// ---- 新增/编辑 ----
const showModal = ref(false)
const editing = ref(false)
const editId = ref(0)
const formRef = ref<FormInst | null>(null)
const form = reactive({
  code: '', name: '', tm_node_id: null as number | null, tm_version_id: null as number | null,
  manufacturer: '', description: '', transport: null as string | null, status: 1,
})

const rules: FormRules = {
  code: [{ required: true, message: t('model.codeRequired'), trigger: ['blur', 'input'] }],
  name: [{ required: true, message: t('model.nameRequired'), trigger: ['blur', 'input'] }],
  tm_node_id: [{ required: true, type: 'number', message: t('model.tmNodeRequired'), trigger: ['blur', 'change'] }],
  tm_version_id: [{ required: true, type: 'number', message: t('model.tmVersionRequired'), trigger: ['blur', 'change'] }],
}

function openCreate() {
  editing.value = false
  Object.assign(form, { code: '', name: '', tm_node_id: null, tm_version_id: null, manufacturer: '', description: '', transport: null, status: 1 })
  tmVersionOptions.value = []
  showModal.value = true
}

function openEdit(row: DeviceModel) {
  editing.value = true
  editId.value = row.id
  Object.assign(form, {
    code: row.code, name: row.name, tm_node_id: row.tm_node_id, tm_version_id: row.tm_version_id,
    manufacturer: row.manufacturer, description: row.description,
    transport: row.transport || null, status: row.status,
  })
  // 编辑时回填当前节点的版本下拉
  if (row.tm_node_id) {
    onNodeChange(row.tm_node_id).then(() => {
      form.tm_version_id = row.tm_version_id
    })
  }
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
    if (editing.value) {
      await updateDeviceModel(editId.value, {
        name: form.name.trim(),
        tm_version_id: form.tm_version_id!,
        status: form.status,
        manufacturer: form.manufacturer.trim(),
        description: form.description.trim(),
        transport: form.transport ?? undefined,
      })
    } else {
      await createDeviceModel({
        code: form.code.trim(),
        name: form.name.trim(),
        tm_node_id: form.tm_node_id!,
        tm_version_id: form.tm_version_id!,
        manufacturer: form.manufacturer.trim(),
        description: form.description.trim(),
        transport: form.transport ?? undefined,
      })
    }
    message.success(t('common.saveSuccess'))
    showModal.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row: DeviceModel) {
  dialog.warning({
    title: t('model.deleteConfirmTitle'),
    content: t('model.deleteConfirmContent', { name: row.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await deleteDeviceModel(row.id)
        message.success(t('common.deleteSuccess'))
        load()
      } catch (e: any) {
        message.error(e?.response?.data?.msg || t('common.deleteFailed'))
      }
    },
  })
}

onMounted(() => {
  load()
  loadTMNodes()
})
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
}
</style>
