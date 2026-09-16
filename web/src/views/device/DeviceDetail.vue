<template>
  <n-card :loading="loading">
    <template #header>
      <div class="head">
        <n-button quaternary size="small" @click="router.back()">← {{ t('common.back') }}</n-button>
        <span class="title">{{ device?.name || `#${id}` }}</span>
        <n-tag v-if="device" :type="device.online ? 'success' : 'default'" size="small">{{ device.online ? t('device.online') : t('device.offline') }}</n-tag>
        <n-tag v-if="device" type="info" size="small">{{ device.status }}</n-tag>
      </div>
    </template>

    <n-descriptions v-if="device" :column="3" label-placement="left" size="small" bordered>
      <n-descriptions-item label="ID">{{ device.id }}</n-descriptions-item>
      <n-descriptions-item :label="t('device.sn')">{{ device.sn }}</n-descriptions-item>
      <n-descriptions-item :label="t('device.model')">#{{ device.model_id }}</n-descriptions-item>
      <n-descriptions-item :label="t('device.tenantId')">{{ device.tenant_id }}</n-descriptions-item>
      <n-descriptions-item :label="t('device.transportCol')">{{ device.transport || t('device.transportDefault') }}</n-descriptions-item>
      <n-descriptions-item :label="t('device.firmware')">{{ device.firmware_version || '—' }}</n-descriptions-item>
      <n-descriptions-item :label="t('device.lastSeen')">{{ device.last_seen_at || '—' }}</n-descriptions-item>
      <n-descriptions-item :label="t('common.createTime')">{{ device.created_at }}</n-descriptions-item>
      <n-descriptions-item label="">{{ }}</n-descriptions-item>
    </n-descriptions>

    <n-tabs type="line" style="margin-top: 16px" v-if="device">
      <n-tab-pane name="shadow" :tab="t('device.shadow')">
        <div class="json-grid">
          <n-card size="small" :title="t('device.shadowDesired')">
            <pre>{{ formatJSON(shadow?.desired) }}</pre>
          </n-card>
          <n-card size="small" :title="t('device.shadowReported')">
            <pre>{{ formatJSON(shadow?.reported) }}</pre>
          </n-card>
        </div>
      </n-tab-pane>

      <n-tab-pane name="commands" :tab="t('device.commands')">
        <div class="cmd-toolbar">
          <n-select v-model:value="cmdForm.command_type" :options="cmdTypeOptions" style="width: 220px" filterable tag :placeholder="t('device.cmdType')" />
          <n-select v-model:value="cmdForm.priority" :options="priorityOptions" style="width: 140px" />
          <n-button type="primary" ghost :loading="issuing" v-permission="['device:commandIssue']" @click="issue">{{ t('device.issue') }}</n-button>
        </div>
        <n-data-table :columns="cmdColumns" :data="commands" :loading="cmdsLoading" :pagination="cmdPagination" size="small" paginate-single-page remote />
      </n-tab-pane>

      <n-tab-pane name="telemetry" :tab="t('device.telemetry')">
        <div class="cmd-toolbar">
          <n-input v-model:value="tlQuery.metric" :placeholder="t('device.metric')" clearable style="width: 160px" />
          <n-button @click="loadTelemetry">{{ t('common.search') }}</n-button>
        </div>
        <n-data-table :columns="tlColumns" :data="telemetry" :loading="tlLoading" size="small" :max-height="420" />
        <div v-if="tlNextMarker" class="more">
          <n-button size="small" quaternary @click="loadTelemetryMore">{{ t('device.loadMore') }}</n-button>
        </div>
      </n-tab-pane>

      <n-tab-pane name="events" :tab="t('device.events')">
        <div class="cmd-toolbar">
          <n-button size="small" @click="loadEvents">{{ t('common.refresh') }}</n-button>
        </div>
        <n-data-table :columns="eventColumns" :data="events" :loading="evLoading" size="small" :max-height="420" />
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>

<script setup lang="ts">
import { h, onMounted, reactive, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NCard, NDataTable, NDescriptions, NDescriptionsItem, NInput, NSelect, NTabPane, NTabs, NTag, useMessage, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import {
  getDevice, getDeviceCommand, getDeviceShadow, getDeviceTelemetry,
  issueDeviceCommand, listDeviceCommands, listDeviceDataEvents,
} from '../../api'
import { usePagination } from '../../utils/pagination'
import type { DataEvent, Device, DeviceCommand, DeviceShadow } from '../../api/types'

const { t } = useI18n()
const message = useMessage()
const route = useRoute()
const router = useRouter()
const id = Number(route.params.id)

const loading = ref(false)
const device = ref<Device | null>(null)
const shadow = ref<DeviceShadow | null>(null)

async function loadDevice() {
  loading.value = true
  try {
    const { data: resp } = await getDevice(id)
    device.value = resp.data
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadShadow() {
  try {
    const { data: resp } = await getDeviceShadow(id)
    shadow.value = resp.data
  } catch { /* 影子不可用时留空 */ }
}

// ---- 命令 ----
const commands = ref<DeviceCommand[]>([])
const cmdsLoading = ref(false)
const issuing = ref(false)
const cmdQuery = reactive({ page: 1, page_size: 10 })
const { pagination: cmdPagination } = usePagination(cmdQuery, () => loadCommands())

const cmdTypeOptions = [
  { label: 'emergency:estop', value: 'emergency:estop' },
]
const priorityOptions = computed(() => [
  { label: t('device.p0'), value: 0 },
  { label: t('device.p1'), value: 1 },
  { label: t('device.p2'), value: 2 },
  { label: t('device.p3'), value: 3 },
])
const cmdForm = reactive({ command_type: null as string | null, priority: 3 })

const statusTagType = (s: string) =>
  s === 'succeeded' ? 'success' : s === 'failed' || s === 'timeout' ? 'error' : s === 'acked' || s === 'dispatched' ? 'info' : 'warning'

const cmdColumns: DataTableColumns<DeviceCommand> = [
  { title: 'ID', key: 'id', width: 80, render: (row) => h(NButton, { text: true, type: 'info', onClick: () => refreshCommand(row.id) }, { default: () => `#${row.id}` }) },
  { title: t('device.cmdType'), key: 'command_type', width: 200, ellipsis: { tooltip: true } },
  { title: t('device.priority'), key: 'priority', width: 70 },
  { title: t('device.cmdStatus'), key: 'status', width: 100, render: (row) => h(NTag, { type: statusTagType(row.status), size: 'small', bordered: false }, { default: () => row.status }) },
  { title: t('device.caller'), key: 'caller', width: 150 },
  { title: t('common.createTime'), key: 'created_at', width: 160 },
]

async function loadCommands() {
  cmdsLoading.value = true
  try {
    const { data: resp } = await listDeviceCommands(id, { page: cmdQuery.page, page_size: cmdQuery.page_size })
    commands.value = resp.data.list || []
    cmdPagination.itemCount = resp.data.page?.total || 0
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    cmdsLoading.value = false
  }
}

async function refreshCommand(cmdId: number) {
  try {
    const { data: resp } = await getDeviceCommand(cmdId)
    const idx = commands.value.findIndex((c) => c.id === cmdId)
    if (idx >= 0) commands.value[idx] = resp.data
    message.success(resp.data.status)
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  }
}

async function issue() {
  if (!cmdForm.command_type) {
    message.warning(t('device.cmdTypeRequired'))
    return
  }
  issuing.value = true
  try {
    // 异步语义：受理即返回，结果在命令列表里轮询刷新
    await issueDeviceCommand(id, { command_type: cmdForm.command_type, priority: cmdForm.priority })
    message.success(t('device.issueAccepted'))
    cmdQuery.page = 1
    loadCommands()
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('device.issueFailed'))
  } finally {
    issuing.value = false
  }
}

// ---- 遥测 ----
const telemetry = ref<{ metric: string; value: number; ts: string }[]>([])
const tlLoading = ref(false)
const tlNextMarker = ref('')
const tlQuery = reactive({ metric: '' })

const tlColumns: DataTableColumns<{ metric: string; value: number; ts: string }> = [
  { title: t('device.metric'), key: 'metric', width: 160 },
  { title: t('device.value'), key: 'value', width: 120 },
  { title: 'TS', key: 'ts' },
]

async function loadTelemetry() {
  tlLoading.value = true
  telemetry.value = []
  try {
    const { data: resp } = await getDeviceTelemetry(id, { metric: tlQuery.metric || undefined, limit: 50 })
    telemetry.value = (resp.data.items || []) as any
    tlNextMarker.value = resp.data.next_marker || ''
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    tlLoading.value = false
  }
}

async function loadTelemetryMore() {
  if (!tlNextMarker.value) return
  tlLoading.value = true
  try {
    const { data: resp } = await getDeviceTelemetry(id, { metric: tlQuery.metric || undefined, limit: 50, marker: tlNextMarker.value })
    telemetry.value.push(...((resp.data.items || []) as any))
    tlNextMarker.value = resp.data.next_marker || ''
  } finally {
    tlLoading.value = false
  }
}

// ---- 事件（游标轮询） ----
const events = ref<DataEvent[]>([])
const evLoading = ref(false)
const sinceId = ref(0)

const eventColumns: DataTableColumns<DataEvent> = [
  { title: 'ID', key: 'id', width: 80 },
  { title: t('device.eventType'), key: 'event_type', width: 160 },
  { title: t('device.payload'), key: 'payload', ellipsis: { tooltip: true } },
  { title: t('device.occurredAt'), key: 'occurred_at', width: 170 },
]

async function loadEvents() {
  evLoading.value = true
  try {
    const { data: resp } = await listDeviceDataEvents(id, { since_id: sinceId.value, limit: 100 })
    const list = resp.data.list || []
    if (list.length) sinceId.value = list[list.length - 1].id
    events.value = [...list.slice().reverse(), ...events.value]
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    evLoading.value = false
  }
}

function formatJSON(text?: string) {
  if (!text) return '—'
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}

onMounted(() => {
  loadDevice()
  loadShadow()
  loadCommands()
  loadTelemetry()
  loadEvents()
})
</script>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.title {
  font-weight: 600;
}
.json-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.json-grid pre {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  font-size: 12px;
}
.cmd-toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}
.more {
  margin-top: 8px;
  text-align: center;
}
</style>
