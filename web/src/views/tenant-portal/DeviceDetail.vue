<template>
  <n-card :loading="loading">
    <template #header>
      <div class="head">
        <n-button quaternary size="small" @click="router.back()">← {{ t('common.back') }}</n-button>
        <div class="unit-plate">
          <p class="mono unit-code">unit · sn {{ device?.sn || `#${id}` }}</p>
          <div class="unit-row">
            <span class="unit-name">{{ device?.name || `#${id}` }}</span>
            <span v-if="device" class="sx-led" :class="device.online ? 'sx-led--ok sx-led--live' : 'sx-led--off'">
              <i></i>{{ device.online ? t('tenantPortal.devices.online') : t('tenantPortal.devices.offline') }}
            </span>
            <n-tag v-if="device" size="small" :bordered="false">{{ device.status }}</n-tag>
            <n-tag v-if="device" size="small" :bordered="false">{{ device.transport || t('tenantPortal.devices.transportDefault') }}</n-tag>
          </div>
        </div>
      </div>
    </template>

    <n-descriptions v-if="device" :column="3" label-placement="left" size="small" bordered>
      <n-descriptions-item label="ID">{{ device.id }}</n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.devices.sn')"><span class="mono-cell">{{ device.sn }}</span></n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.devices.model')">#{{ device.model_id }}</n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.devices.firmware')">{{ device.firmware_version || '—' }}</n-descriptions-item>
      <n-descriptions-item :label="t('tenantPortal.devices.lastSeen')">{{ device.last_seen_at || '—' }}</n-descriptions-item>
      <n-descriptions-item :label="t('common.createTime')">{{ device.created_at }}</n-descriptions-item>
    </n-descriptions>

    <n-tabs v-if="device" type="line" style="margin-top: 16px">
      <n-tab-pane name="shadow" :tab="t('tenantPortal.devices.shadow')">
        <div class="json-grid">
          <n-card size="small" :title="t('tenantPortal.devices.shadowDesired')">
            <pre>{{ formatJSON(shadow?.desired) }}</pre>
          </n-card>
          <n-card size="small" :title="t('tenantPortal.devices.shadowReported')">
            <pre>{{ formatJSON(shadow?.reported) }}</pre>
          </n-card>
        </div>
      </n-tab-pane>

      <n-tab-pane name="telemetry" :tab="t('tenantPortal.devices.telemetry')">
        <div class="cmd-toolbar">
          <n-input v-model:value="tlQuery.metric" :placeholder="t('tenantPortal.devices.metric')" clearable style="width: 160px" />
          <n-button @click="loadTelemetry">{{ t('common.search') }}</n-button>
        </div>
        <n-data-table :columns="tlColumns" :data="telemetry" :loading="tlLoading" size="small" :max-height="420" />
        <div v-if="tlNextMarker" class="more">
          <n-button size="small" quaternary @click="loadTelemetryMore">{{ t('tenantPortal.devices.loadMore') }}</n-button>
        </div>
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton, NCard, NDataTable, NDescriptions, NDescriptionsItem, NInput, NTabPane, NTabs, NTag, useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { getTenantDevice, getTenantDeviceShadow, getTenantDeviceTelemetry, type DeviceShadow, type TenantDevice, type TelemetryItem } from '../../api/tenant'

const { t } = useI18n()
const message = useMessage()
const route = useRoute()
const router = useRouter()
const id = Number(route.params.id)

const loading = ref(false)
const device = ref<TenantDevice | null>(null)
const shadow = ref<DeviceShadow | null>(null)

async function loadDevice() {
  loading.value = true
  try {
    const { data: resp } = await getTenantDevice(id)
    device.value = resp.data
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadShadow() {
  try {
    const { data: resp } = await getTenantDeviceShadow(id)
    shadow.value = resp.data
  } catch { /* 影子不可用时留空 */ }
}

function formatJSON(raw?: string) {
  if (!raw) return '—'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

// ---- 遥测（marker 游标增量） ----
const telemetry = ref<TelemetryItem[]>([])
const tlLoading = ref(false)
const tlNextMarker = ref('')
const tlQuery = reactive({ metric: '' })

async function loadTelemetry() {
  telemetry.value = []
  tlNextMarker.value = ''
  tlLoading.value = true
  try {
    const { data: resp } = await getTenantDeviceTelemetry(id, {
      metric: tlQuery.metric || undefined,
      limit: 100,
    })
    telemetry.value = resp.data.items || []
    tlNextMarker.value = resp.data.next_marker || ''
  } catch (e: any) {
    message.error(e?.response?.data?.msg || t('common.loadFailed'))
  } finally {
    tlLoading.value = false
  }
}

async function loadTelemetryMore() {
  if (!tlNextMarker.value) return
  const { data: resp } = await getTenantDeviceTelemetry(id, {
    metric: tlQuery.metric || undefined,
    marker: tlNextMarker.value,
    limit: 100,
  })
  telemetry.value.push(...(resp.data.items || []))
  tlNextMarker.value = resp.data.next_marker || ''
}

const tlColumns = computed<DataTableColumns<TelemetryItem>>(() => [
  { title: t('tenantPortal.devices.metric'), key: 'metric', width: 180 },
  { title: t('tenantPortal.devices.value'), key: 'value', width: 140 },
  { title: t('tenantPortal.devices.ts'), key: 'ts', width: 200 },
])

onMounted(() => {
  loadDevice()
  loadShadow()
})
</script>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 16px;
}
.unit-plate {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.unit-code {
  margin: 0;
  font-family: var(--sx-font-mono);
  font-size: 11px;
  letter-spacing: 0.1em;
  color: var(--sx-muted);
}
.unit-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.unit-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--sx-ink);
}
.json-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.json-grid pre {
  margin: 0;
  max-height: 420px;
  overflow: auto;
  font-family: var(--sx-font-mono);
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
.mono-cell {
  font-family: var(--sx-font-mono);
}
</style>
