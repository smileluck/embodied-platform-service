<template>
  <SearchCard storage-key="devices" @search="load" @reset="resetQuery">
    <n-input v-model:value="query.kw" :placeholder="t('device.kwPlaceholder')" clearable style="width: 200px" @keyup.enter="load" />
    <n-select v-model:value="query.model_id" :options="modelOptions" :placeholder="t('device.model')" clearable filterable style="width: 180px" />
    <n-select v-model:value="query.status" :options="statusOptions" :placeholder="t('device.status')" clearable style="width: 130px" />
    <n-select v-model:value="query.online" :options="onlineOptions" :placeholder="t('device.online')" clearable style="width: 110px" />
  </SearchCard>

  <n-card>
    <template #header>
      <div class="page-actions">
        <n-button type="primary" ghost @click="openRegister" v-permission="['device:register']">{{ t('device.register') }}</n-button>
      </div>
    </template>

    <!-- 设备真源在 embodied-platform（开放面代理）；租户范围由平台按商户绑定服务端收敛 -->
    <n-data-table :columns="columns" :data="rows" :loading="loading" :pagination="pagination" paginate-single-page remote />
  </n-card>

  <!-- 注册设备 -->
  <n-modal v-model:show="showRegister" preset="dialog" :title="t('device.register')" style="width: 520px">
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="110">
      <n-form-item :label="t('device.sn')" path="sn">
        <n-input v-model:value="form.sn" :maxlength="128" :placeholder="t('device.snPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('device.name')" path="name">
        <n-input v-model:value="form.name" :maxlength="64" />
      </n-form-item>
      <n-form-item :label="t('device.model')" path="model_id">
        <n-select v-model:value="form.model_id" :options="modelOptions" filterable clearable />
      </n-form-item>
      <n-form-item :label="t('device.tenant')" path="tenant_id">
        <n-select v-model:value="form.tenant_id" :options="tenantOptions" filterable :placeholder="t('device.tenantPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('device.transport')">
        <n-select v-model:value="form.transport" :options="transportOptions" clearable :placeholder="t('device.transportDefault')" />
      </n-form-item>
    </n-form>
    <template #action>
      <n-button @click="showRegister = false">{{ t('common.cancel') }}</n-button>
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.confirm') }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NInput, NButton, NDataTable, NModal, NForm, NFormItem, NSelect, NTag, useMessage, type DataTableColumns, type FormInst, type FormRules } from 'naive-ui'
import { renderActions, type TableAction } from '../../utils/tableActions'
import SearchCard from '../../components/SearchCard.vue'
import { useI18n } from 'vue-i18n'
import { listDeviceModels, listDevices, listTenants, registerDevice } from '../../api'
import { usePagination } from '../../utils/pagination'
import type { Device } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const router = useRouter()

const loading = ref(false)
const saving = ref(false)
const rows = ref<Device[]>([])
const query = reactive({
  kw: '', model_id: null as number | null, status: null as string | null,
  online: null as string | null, page: 1, page_size: 10,
})

const modelOptions = ref<{ label: string; value: number }[]>([])
const tenantOptions = ref<{ label: string; value: number }[]>([])

const statusOptions = [
  { label: t('device.statusInactive'), value: 'inactive' },
  { label: t('device.statusActive'), value: 'active' },
  { label: t('device.statusDisabled'), value: 'disabled' },
  { label: t('device.statusRetired'), value: 'retired' },
]
const onlineOptions = [
  { label: t('device.online'), value: 'true' },
  { label: t('device.offline'), value: 'false' },
]
const transportOptions = [
  { label: 'Socket', value: 'socket' },
  { label: 'MQTT', value: 'mqtt' },
]

const { pagination } = usePagination(query, () => load())

const columns: DataTableColumns<Device> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: t('device.sn'), key: 'sn', width: 160, ellipsis: { tooltip: true }, render: (row) => h('span', { class: 'mono-cell' }, row.sn) },
  { title: t('device.name'), key: 'name', minWidth: 120, ellipsis: { tooltip: true } },
  { title: t('device.model'), key: 'model_id', width: 80, render: (row) => `#${row.model_id}` },
  {
    title: t('device.online'), key: 'online', width: 110,
    // 状态灯语（.sx-led 定义于 tokens.css）：在线=信号绿、离线=空心点
    render: (row) => h('span', { class: ['sx-led', row.online ? 'sx-led--ok' : 'sx-led--off'] }, [h('i'), row.online ? t('device.online') : t('device.offline')]),
  },
  {
    title: t('device.status'), key: 'status', width: 90,
    render: (row) => h(NTag, { type: row.status === 'active' ? 'info' : row.status === 'disabled' ? 'error' : 'warning', size: 'small' }, { default: () => row.status }),
  },
  { title: t('device.transportCol'), key: 'transport', width: 90, render: (row) => row.transport || '—' },
  { title: t('device.lastSeen'), key: 'last_seen_at', width: 160 },
  {
    title: t('common.operation'), key: 'actions', width: 90,
    render: (row) =>
      renderActions([
        { label: t('common.detail'), accent: true, permission: 'device:view', onClick: () => router.push(`/device/devices/${row.id}`) },
      ]),
  },
]

async function load() {
  loading.value = true
  try {
    const { data: resp } = await listDevices({
      page: query.page,
      page_size: query.page_size,
      kw: query.kw || undefined,
      model_id: query.model_id ?? undefined,
      status: query.status ?? undefined,
      online: query.online ?? undefined,
    })
    rows.value = resp.data.list || []
    pagination.itemCount = resp.data.page?.total || 0
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('device.listFailed'))
  } finally {
    loading.value = false
  }
}

function resetQuery() {
  query.kw = ''
  query.model_id = null
  query.status = null
  query.online = null
  load()
}

// ---- 注册 ----
const showRegister = ref(false)
const formRef = ref<FormInst | null>(null)
const form = reactive({ sn: '', name: '', model_id: null as number | null, tenant_id: null as number | null, transport: null as string | null })

const rules: FormRules = {
  sn: [{ required: true, message: t('device.snRequired'), trigger: ['blur', 'input'] }],
  name: [{ required: true, message: t('device.nameRequired'), trigger: ['blur', 'input'] }],
  tenant_id: [{ required: true, type: 'number', message: t('device.tenantRequired'), trigger: ['blur', 'change'] }],
}

function openRegister() {
  Object.assign(form, { sn: '', name: '', model_id: null, tenant_id: null, transport: null })
  showRegister.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    const dev = await registerDevice({
      sn: form.sn.trim(),
      name: form.name.trim(),
      model_id: form.model_id ?? undefined,
      tenant_id: form.tenant_id!,
      transport: form.transport ?? undefined,
    })
    message.success(t('device.registerDone', { id: dev.data.data.id }))
    showRegister.value = false
    load()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('device.registerFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  load()
  // 型号下拉（平台侧启用态）与本地已同步租户下拉
  try {
    const { data: resp } = await listDeviceModels({ page: 1, page_size: 100, status: 1 })
    modelOptions.value = (resp.data.list || []).map((m) => ({ label: `${m.name}（${m.code}）`, value: m.id }))
  } catch { /* 下拉加载失败不阻断列表 */
  }
  try {
    const { data: resp } = await listTenants({ page: 1, page_size: 100, status: 1 })
    tenantOptions.value = (resp.data.list || [])
      .filter((tn) => tn.platform_id > 0) // 仅已同步租户可注册设备
      .map((tn) => ({ label: `${tn.name}（${tn.code}）`, value: tn.platform_id }))
  } catch { /* 同上 */ }
})
</script>

<style scoped>
.page-actions {
  width: 100%;
  display: flex;
  justify-content: flex-end;
}
.mono-cell {
  font-family: var(--sx-font-mono);
  font-size: 12px;
  letter-spacing: 0.02em;
  font-variant-numeric: tabular-nums;
}
</style>
