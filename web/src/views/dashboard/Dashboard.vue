<!-- 设备运营总览（首页）：指标全部由现有列表接口的 page.total 聚合（page_size=1），无专用统计接口；
     某块接口失败（含无权限 403）该处显示 — ，不阻塞整页。状态呈现统一用「状态灯语」(.sx-led)。 -->
<template>
  <div class="dash">
    <!-- 铭牌欢迎条 -->
    <n-card class="plate-card" :bordered="false">
      <div class="plate-bar">
        <div class="plate-copy">
          <p class="sx-mono plate-eyebrow">smilex admin · {{ todayLabel }}</p>
          <h2 class="plate-title">{{ t('dashboard.greeting', { name: userName }) }}</h2>
          <p class="plate-sub">{{ t('dashboard.subtitle') }}</p>
        </div>
        <span class="sx-led sx-led--ok sx-led--live"><i></i>{{ t('dashboard.systemNormal') }}</span>
      </div>
    </n-card>

    <!-- 指标行 -->
    <div class="stat-row">
      <n-card v-for="card in statCards" :key="card.key" class="stat-card" :bordered="false">
        <p class="sx-mono stat-label">{{ card.label }}</p>
        <p class="stat-value">{{ card.value }}</p>
        <p class="stat-sub">
          <span v-if="card.led" class="sx-led" :class="card.led"><i></i>{{ card.ledText }}</span>
          <span v-else-if="card.hint" class="sx-mono stat-hint">{{ card.hint }}</span>
        </p>
      </n-card>
    </div>

    <div class="mid-row">
      <!-- 设备状态构成 -->
      <n-card :bordered="false">
        <template #header>
          <div class="sx-plate">
            <span class="sx-plate-eyebrow">fleet</span>
            <span class="card-title">{{ t('dashboard.breakdown') }}</span>
          </div>
        </template>
        <div class="seg-bar">
          <template v-if="stats.devices">
            <span v-for="seg in segments" :key="seg.key" class="seg"
              :style="{ width: seg.pct + '%', background: seg.color }" :title="`${seg.label} ${seg.value}`" />
          </template>
          <span v-else class="seg seg--empty" style="width: 100%" />
        </div>
        <div class="legend">
          <div v-for="row in legendRows" :key="row.key" class="legend-row">
            <span class="sx-led" :class="row.led"><i></i>{{ row.label }}</span>
            <span class="legend-count">{{ row.value }}</span>
          </div>
        </div>
      </n-card>

      <!-- 系统概览 -->
      <n-card :bordered="false">
        <template #header>
          <div class="sx-plate">
            <span class="sx-plate-eyebrow">system</span>
            <span class="card-title">{{ t('dashboard.sysOverview') }}</span>
          </div>
        </template>
        <div class="ov-row" v-for="row in overviewRows" :key="row.label">
          <span class="ov-label">{{ row.label }}</span>
          <span class="ov-value">{{ row.value }}</span>
        </div>
      </n-card>
    </div>

    <!-- 最近设备 -->
    <n-card :bordered="false">
      <template #header>
        <div class="card-head">
          <div class="sx-plate">
            <span class="sx-plate-eyebrow">recent</span>
            <span class="card-title">{{ t('dashboard.recentDevices') }}</span>
          </div>
          <n-button v-if="canViewDevice" text type="primary" size="small" @click="router.push('/device/devices')">
            {{ t('dashboard.viewAll') }}
          </n-button>
        </div>
      </template>
      <n-data-table :columns="recentColumns" :data="recent" :loading="recentLoading" size="small"
        :row-props="rowProps" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NCard, NDataTable, type DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { useUserStore } from '../../stores/user'
import { listDevices, listDeviceModels, listTenants, listUsers, listRoles } from '../../api'
import type { Device } from '../../api/types'

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()

const userName = computed(() => userStore.user?.nickname || userStore.user?.username || '')
const canViewDevice = computed(() => userStore.codes.includes('device:view'))

// mono 日期戳：2026-09-16 TUE
const WEEK = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT']
const now = new Date()
const pad = (n: number) => String(n).padStart(2, '0')
const todayLabel = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${WEEK[now.getDay()]}`

// ---- 指标聚合：全部取列表接口 page.total ----
const stats = reactive({
  devices: null as number | null,
  online: null as number | null,
  active: null as number | null,
  inactive: null as number | null,
  disabled: null as number | null,
  retired: null as number | null,
  models: null as number | null,
  tenants: null as number | null,
  users: null as number | null,
  roles: null as number | null,
})

const fmt = (n: number | null) => (n === null ? '—' : String(n))

// 单项失败静默降级为 null（403/网络异常等，展示为 —）
async function safe(p: Promise<unknown>): Promise<any> {
  try {
    return await p
  } catch {
    return null
  }
}
const totalOf = (r: any): number | null => r?.data?.data?.page?.total ?? null

const statCards = computed(() => [
  { key: 'devices', label: t('dashboard.statDevices'), value: fmt(stats.devices), led: '', ledText: '', hint: '' },
  {
    key: 'online', label: t('dashboard.statOnline'), value: fmt(stats.online),
    led: stats.online === null ? '' : stats.online > 0 ? 'sx-led--ok sx-led--live' : 'sx-led--off',
    ledText: t('device.online'), hint: '',
  },
  {
    key: 'active', label: t('dashboard.statActive'), value: fmt(stats.active), led: '', ledText: '',
    hint: stats.inactive === null ? '' : t('dashboard.inactiveHint', { n: stats.inactive }),
  },
  {
    key: 'models', label: t('dashboard.statModelTenant'),
    value: `${fmt(stats.models)} / ${fmt(stats.tenants)}`, led: '', ledText: '', hint: '',
  },
])

// 状态构成：分段条 + LED 图例
interface StatusSeg { key: string; led: string; label: string; value: number | null; color: string }

const statusSegs = computed<StatusSeg[]>(() => [
  { key: 'active', led: 'sx-led--ok', label: t('device.statusActive'), value: stats.active, color: 'var(--sx-ok)' },
  { key: 'inactive', led: 'sx-led--warn', label: t('device.statusInactive'), value: stats.inactive, color: 'var(--sx-warn)' },
  { key: 'disabled', led: 'sx-led--danger', label: t('device.statusDisabled'), value: stats.disabled, color: 'var(--sx-danger)' },
  { key: 'retired', led: 'sx-led--off', label: t('device.statusRetired'), value: stats.retired, color: '#9AA0A8' },
])
const legendRows = statusSegs
const segments = computed(() =>
  statusSegs.value
    .map((s) => ({ ...s, pct: stats.devices ? Math.max((s.value || 0) / stats.devices * 100, s.value ? 1.5 : 0) : 0 }))
    .filter((s) => s.pct > 0),
)

const overviewRows = computed(() => [
  { label: t('dashboard.admissionUsers'), value: fmt(stats.users) },
  { label: t('dashboard.roles'), value: fmt(stats.roles) },
  { label: t('dashboard.permCodes'), value: String(userStore.permissions.length) },
])

// ---- 最近设备 ----
const recent = ref<Device[]>([])
const recentLoading = ref(false)

const ledRender = (on: boolean, onText: string, offText: string) =>
  h('span', { class: ['sx-led', on ? 'sx-led--ok' : 'sx-led--off'] }, [h('i'), on ? onText : offText])

const statusLed = (s: string) =>
  s === 'active' ? 'sx-led--ok' : s === 'inactive' ? 'sx-led--warn' : s === 'disabled' ? 'sx-led--danger' : 'sx-led--off'

const recentColumns: DataTableColumns<Device> = [
  { title: t('device.sn'), key: 'sn', width: 180, ellipsis: { tooltip: true }, render: (row) => h('span', { class: 'mono-cell' }, row.sn) },
  { title: t('device.name'), key: 'name', minWidth: 140, ellipsis: { tooltip: true } },
  { title: t('device.model'), key: 'model_id', width: 80, render: (row) => `#${row.model_id}` },
  {
    title: t('device.online'), key: 'online', width: 110,
    render: (row) => ledRender(row.online, t('device.online'), t('device.offline')),
  },
  {
    title: t('device.status'), key: 'status', width: 130,
    render: (row) => h('span', { class: ['sx-led', statusLed(row.status)] }, [h('i'), row.status]),
  },
  { title: t('device.lastSeen'), key: 'last_seen_at', width: 170 },
]

// 行点击进详情（有 device:view 权限才可点）
const rowProps = (row: Device) =>
  canViewDevice.value
    ? { style: 'cursor: pointer', onClick: () => router.push(`/device/devices/${row.id}`) }
    : {}

onMounted(async () => {
  recentLoading.value = true
  const [devAll, devOnline, devActive, devInactive, devDisabled, devRetired, devRecent, models, tenants, users, roles] =
    await Promise.all([
      safe(listDevices({ page: 1, page_size: 1 })),
      safe(listDevices({ page: 1, page_size: 1, online: 'true' })),
      safe(listDevices({ page: 1, page_size: 1, status: 'active' })),
      safe(listDevices({ page: 1, page_size: 1, status: 'inactive' })),
      safe(listDevices({ page: 1, page_size: 1, status: 'disabled' })),
      safe(listDevices({ page: 1, page_size: 1, status: 'retired' })),
      safe(listDevices({ page: 1, page_size: 5 })),
      safe(listDeviceModels({ page: 1, page_size: 1 })),
      safe(listTenants({ page: 1, page_size: 1 })),
      safe(listUsers({ page: 1, page_size: 1 })),
      safe(listRoles({ page: 1, page_size: 1 })),
    ])
  stats.devices = totalOf(devAll)
  stats.online = totalOf(devOnline)
  stats.active = totalOf(devActive)
  stats.inactive = totalOf(devInactive)
  stats.disabled = totalOf(devDisabled)
  stats.retired = totalOf(devRetired)
  stats.models = totalOf(models)
  stats.tenants = totalOf(tenants)
  stats.users = totalOf(users)
  stats.roles = totalOf(roles)
  recent.value = devRecent?.data?.data?.list ?? []
  recentLoading.value = false
})
</script>

<style scoped>
.dash {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 铭牌欢迎条：琥珀刻度 + mono 日期戳 */
.plate-card :deep(.n-card__content) {
  padding: 20px 24px;
}
.plate-bar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}
.plate-copy {
  padding-left: 12px;
  border-left: 3px solid var(--sx-accent);
}
.plate-eyebrow {
  margin: 0 0 8px;
}
.plate-title {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 700;
  line-height: 1.2;
}
.plate-sub {
  margin: 0;
  font-size: 13px;
  color: var(--sx-muted);
}

/* 指标行 */
.stat-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}
.stat-card :deep(.n-card__content) {
  padding: 16px 20px;
}
.stat-label {
  margin: 0 0 10px;
}
.stat-value {
  margin: 0 0 10px;
  font-family: var(--sx-font-mono);
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0.01em;
  font-variant-numeric: tabular-nums;
  color: var(--sx-ink);
}
.stat-sub {
  margin: 0;
  min-height: 12px;
}
.stat-hint {
  text-transform: none;
}

/* 中排：状态构成 + 系统概览 */
.mid-row {
  display: grid;
  grid-template-columns: 3fr 2fr;
  gap: 8px;
}
.card-title {
  font-size: 14px;
  font-weight: 600;
}
.card-head {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.seg-bar {
  display: flex;
  gap: 2px;
  height: 10px;
  border-radius: 5px;
  overflow: hidden;
  margin-bottom: 14px;
  background: var(--sx-bg);
}
.seg {
  display: block;
  min-width: 2px;
  transition: width 0.4s ease;
}
.seg--empty {
  background: var(--sx-line);
}
.legend {
  display: flex;
  flex-direction: column;
}
.legend-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 0;
  border-bottom: 1px solid var(--sx-line);
}
.legend-row:last-child {
  border-bottom: none;
}
.legend-count {
  font-family: var(--sx-font-mono);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--sx-ink);
}

.ov-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 0;
  border-bottom: 1px solid var(--sx-line);
}
.ov-row:last-child {
  border-bottom: none;
}
.ov-label {
  font-size: 13px;
  color: var(--sx-muted);
}
.ov-value {
  font-family: var(--sx-font-mono);
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--sx-ink);
}

/* 最近设备表格内的 mono 单元格 */
.mono-cell {
  font-family: var(--sx-font-mono);
  font-size: 12px;
  letter-spacing: 0.02em;
  font-variant-numeric: tabular-nums;
}

/* 窄屏：指标 2 列、中排单列 */
@media (max-width: 1100px) {
  .stat-row {
    grid-template-columns: repeat(2, 1fr);
  }
  .mid-row {
    grid-template-columns: 1fr;
  }
}
</style>
