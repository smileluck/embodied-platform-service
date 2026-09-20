<template>
  <div class="monitor-page">
    <!-- 顶部：主机信息 + 轮询控制（标题由顶栏展示，卡内只放信息与操作） -->
    <n-card size="small" class="host-card">
      <div class="host-bar">
        <div class="host-item">
          <span class="host-label">{{ t('monitor.hostname') }}</span>
          <span class="host-value mono">{{ status?.host?.hostname || '—' }}</span>
        </div>
        <div class="host-item">
          <span class="host-label">{{ t('monitor.platform') }}</span>
          <span class="host-value mono">{{ platformText }}</span>
        </div>
        <div class="host-item">
          <span class="host-label">{{ t('monitor.kernel') }}</span>
          <span class="host-value mono">{{ kernelText }}</span>
        </div>
        <div class="host-item">
          <span class="host-label">{{ t('monitor.uptime') }}</span>
          <span class="host-value mono">{{ fmtUptime(status?.uptime) }}</span>
        </div>
        <div class="host-item">
          <span class="host-label">{{ t('monitor.lastUpdate') }}</span>
          <span class="host-value mono">{{ lastUpdateText }}</span>
        </div>
        <div class="host-actions">
          <n-select v-model:value="intervalSec" :options="intervalOptions" size="small" class="interval-select" />
          <n-button size="small" @click="togglePause">
            {{ paused ? t('monitor.resume') : t('monitor.pause') }}
          </n-button>
          <n-button size="small" :loading="loading" @click="refreshNow">
            {{ t('monitor.refreshNow') }}
          </n-button>
        </div>
      </div>
    </n-card>

    <!-- 主体：左右 4:6 固定比例分栏，任何窗口宽度都并排 -->
    <div class="cols">
      <!-- 左栏：CPU 曲线与每核占用 + Go 进程运行时 -->
      <div class="col col-left">
        <n-card :title="t('monitor.cpuTitle')" class="cpu-card">
          <template #header-extra>
            <span class="cpu-now mono" :class="textLevel(status?.cpu.percent)">{{ cpuNow }}</span>
          </template>
          <div class="cpu-model mono">{{ cpuModelText }}</div>
          <div ref="cpuChartRef" class="chart" />
          <div class="cores">
            <div v-for="(p, i) in status?.cpu?.per_core || []" :key="i" class="core-row">
              <span class="core-label mono">C{{ i }}</span>
              <div class="core-bar">
                <div class="core-fill" :class="fillLevel(p)" :style="{ width: Math.min(p, 100) + '%' }" />
              </div>
              <span class="core-val mono">{{ p.toFixed(0) }}%</span>
            </div>
          </div>
        </n-card>

        <n-card :title="t('monitor.goTitle')" size="small" class="go-card">
          <div class="go-grid">
            <div v-for="item in goItems" :key="item.label" class="go-item">
              <span class="host-label">{{ item.label }}</span>
              <span class="go-val mono">{{ item.value }}</span>
            </div>
          </div>
        </n-card>
      </div>

      <!-- 右栏：内存曲线与统计 + 磁盘分区 + 网络接口 -->
      <div class="col col-right">
        <n-card :title="t('monitor.memTitle')" size="small" class="mem-card">
          <div ref="memChartRef" class="chart" />
          <div class="mem-stats">
            <div class="mem-stat">
              <span class="host-label">{{ t('monitor.memUsed') }}</span>
              <span class="go-val mono">{{ fmtBytes(status?.memory?.used) }} / {{ fmtBytes(status?.memory?.total) }}</span>
            </div>
            <div class="mem-stat">
              <span class="host-label">{{ t('monitor.memAvailable') }}</span>
              <span class="go-val mono">{{ fmtBytes(status?.memory?.available) }}</span>
            </div>
            <div class="mem-stat">
              <span class="host-label">{{ t('monitor.swap') }}</span>
              <span class="go-val mono">{{ swapText }}</span>
            </div>
          </div>
        </n-card>

        <n-card :title="t('monitor.diskTitle')" size="small" class="list-card">
          <div v-if="!status?.disks?.length" class="empty">{{ t('monitor.noData') }}</div>
          <div v-for="d in status?.disks || []" :key="d.mount" class="row">
            <div class="row-main">
              <span class="row-title mono" :title="d.device">{{ d.mount }}</span>
              <span class="row-sub">{{ fmtBytes(d.used) }} / {{ fmtBytes(d.total) }}</span>
            </div>
            <div class="row-bar">
              <div class="row-fill" :class="fillLevel(d.used_percent)" :style="{ width: Math.min(d.used_percent, 100) + '%' }" />
            </div>
            <span class="row-val mono" :class="textLevel(d.used_percent)">{{ d.used_percent.toFixed(1) }}%</span>
          </div>
        </n-card>

        <n-card :title="t('monitor.netTitle')" size="small" class="list-card">
          <div v-if="!sortedNet.length" class="empty">{{ t('monitor.noData') }}</div>
          <div v-for="n in sortedNet" :key="n.name" class="net-row">
            <span class="row-title mono">{{ n.name }}</span>
            <span class="net-dir down mono">↓ {{ fmtRate(n.recv_rate) }}</span>
            <span class="net-dir up mono">↑ {{ fmtRate(n.send_rate) }}</span>
            <span class="row-sub">Σ↓{{ fmtBytes(n.bytes_recv) }} ↑{{ fmtBytes(n.bytes_sent) }}</span>
          </div>
        </n-card>
      </div>
    </div>

    <!-- 历史回看：60s 一点，最长 72h -->
    <n-card size="small" class="history-card">
      <template #header>
        <span class="card-title">{{ t('monitor.historyTitle') }}</span>
      </template>
      <template #header-extra>
        <n-radio-group v-model:value="historyHours" size="small" @update:value="loadHistory">
          <n-radio-button :value="6">6h</n-radio-button>
          <n-radio-button :value="24">24h</n-radio-button>
          <n-radio-button :value="72">72h</n-radio-button>
        </n-radio-group>
      </template>
      <div v-if="!history.length" class="empty" style="padding: 24px 0">{{ t('monitor.noHistory') }}</div>
      <div v-show="history.length" ref="histChartRef" class="chart" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NRadioButton, NRadioGroup } from 'naive-ui'
import { getMonitorHistory } from '../../api'
import type { MonitorHistoryPoint } from '../../api/types'
import { NButton, NCard, NSelect, useMessage } from 'naive-ui'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getServerStatus } from '../../api'
import type { ServerStatus } from '../../api/types'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

// 曲线配色与 styles/tokens.css 同源（清水蓝主题）
// 主题色从 CSS 变量读取（亮/暗切换后刷新图表）
const ACCENT = ref('#3F75AB')
const ACCENT_SOFT = ref('#8FC5E8')
function refreshAccent() {
  const cs = getComputedStyle(document.documentElement)
  ACCENT.value = cs.getPropertyValue('--sx-accent').trim() || '#3F75AB'
  ACCENT_SOFT.value = cs.getPropertyValue('--sx-accent-bright').trim() || '#8FC5E8'
}
refreshAccent()
watch(isDarkRef, () => {
  refreshAccent()
  renderHistory()
  refreshNow() // 立即重拉一帧并重绘实时图表（配色已更新）
})
const MUTED = '#6B7787'
const DANGER = '#C2453A'
// 曲线滚动窗口：5s 轮询下约覆盖最近 5 分钟
const MAX_POINTS = 60

const { t } = useI18n()
const message = useMessage()

const status = ref<ServerStatus | null>(null)
const loading = ref(false)
const paused = ref(false)
const intervalSec = ref(5)
const intervalOptions = [5, 10, 30, 60].map(s => ({ label: `${s}s`, value: s }))

// ---- 派生展示 ----
const platformText = computed(() => {
  const h = status.value?.host
  if (!h) return '—'
  return `${h.platform} ${h.platform_version} (${h.kernel_arch})`
})
const kernelText = computed(() => status.value?.host?.kernel_version || '—')
const cpuNow = computed(() => `${(status.value?.cpu?.percent ?? 0).toFixed(1)}%`)
const cpuModelText = computed(() => {
  const c = status.value?.cpu
  if (!c?.model_name && !c?.cores) return ''
  return `${c.model_name} · ${t('monitor.cores', { n: c.cores })}`
})
const swapText = computed(() => {
  const m = status.value?.memory
  if (!m || !m.swap_total) return t('monitor.swapNone')
  return `${fmtBytes(m.swap_used)} / ${fmtBytes(m.swap_total)} (${m.swap_percent.toFixed(0)}%)`
})
const lastUpdateText = computed(() =>
  status.value ? new Date(status.value.time * 1000).toLocaleTimeString() : '—')
const sortedNet = computed(() =>
  [...(status.value?.net || [])].sort((a, b) => (b.recv_rate + b.send_rate) - (a.recv_rate + a.send_rate)))
const goItems = computed(() => {
  const s = status.value
  const g = s?.go
  if (!g) return []
  return [
    { label: t('monitor.goVersion'), value: g.version },
    { label: t('monitor.goroutines'), value: String(g.goroutines) },
    { label: t('monitor.gcCount'), value: String(g.gc_count) },
    { label: t('monitor.gcPause'), value: `${g.gc_pause_ms.toFixed(1)} ms` },
    { label: t('monitor.heapAlloc'), value: fmtBytes(g.heap_alloc) },
    { label: t('monitor.sysMemory'), value: fmtBytes(g.sys_memory) },
    { label: t('monitor.processUptime'), value: fmtUptime(s!.time - g.process_start) },
  ]
})

// ---- 格式化 ----
function fmtBytes(n?: number): string {
  if (n == null || isNaN(n)) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`
}
function fmtRate(n?: number): string {
  if (n == null || isNaN(n)) return '—'
  return `${fmtBytes(n)}/s`
}
function fmtUptime(sec?: number): string {
  if (sec == null || sec < 0) return '—'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${Math.floor(sec % 60)}s`
}
// 占用分级：≥85% 危险、≥60% 警告、其余主色（文本/进度条各取所需 class）
function textLevel(p?: number): string {
  if (p == null) return ''
  if (p >= 85) return 'text-danger'
  if (p >= 60) return 'text-warn'
  return 'text-ok'
}
function fillLevel(p?: number): string {
  if (p == null) return ''
  if (p >= 85) return 'fill-danger'
  if (p >= 60) return 'fill-warn'
  return 'fill-ok'
}

// ---- 图表 ----
const cpuChartRef = ref<HTMLElement>()
const memChartRef = ref<HTMLElement>()
let cpuChart: echarts.ECharts | null = null
let memChart: echarts.ECharts | null = null
let ro: ResizeObserver | null = null

const cpuHistory: [number, number][] = []
const memHistory: [number, number | null][] = []
const swapHistory: [number, number | null][] = []

function baseAxis() {
  return {
    grid: { left: 44, right: 12, top: 28, bottom: 22 },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v: unknown) => (typeof v === 'number' ? `${v.toFixed(1)}%` : '—'),
    },
    xAxis: {
      type: 'time' as const,
      axisLabel: { fontSize: 10, color: MUTED, hideOverlap: true },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value' as const,
      min: 0,
      max: 100,
      axisLabel: { fontSize: 10, color: MUTED, formatter: '{value}%' },
      splitLine: { lineStyle: { color: '#E3E8EF' } },
    },
  }
}

function initCharts() {
  cpuChart = echarts.init(cpuChartRef.value!)
  cpuChart.setOption({
    ...baseAxis(),
    series: [{
      name: 'CPU', type: 'line' as const, smooth: true, showSymbol: false,
      data: cpuHistory, animation: false,
      lineStyle: { color: ACCENT, width: 1.5 },
      itemStyle: { color: ACCENT },
      areaStyle: { color: 'rgba(63, 117, 171, 0.12)' },
    }],
  })
  memChart = echarts.init(memChartRef.value!)
  memChart.setOption({
    ...baseAxis(),
    legend: { top: 0, right: 0, itemWidth: 14, textStyle: { fontSize: 11, color: MUTED } },
    series: [
      {
        name: t('monitor.memLegend'), type: 'line' as const, smooth: true, showSymbol: false,
        data: memHistory, animation: false,
        lineStyle: { color: ACCENT, width: 1.5 },
        itemStyle: { color: ACCENT },
        areaStyle: { color: 'rgba(63, 117, 171, 0.12)' },
      },
      {
        name: t('monitor.swap'), type: 'line' as const, smooth: true, showSymbol: false,
        data: swapHistory, animation: false,
        lineStyle: { color: ACCENT_SOFT, width: 1.5, type: 'dashed' as const },
        itemStyle: { color: ACCENT_SOFT },
      },
    ],
  })
  ro = new ResizeObserver(() => { cpuChart?.resize(); memChart?.resize() })
  ro.observe(cpuChartRef.value!)
  ro.observe(memChartRef.value!)
}

function pushWindow<T>(arr: T[], point: T) {
  arr.push(point)
  if (arr.length > MAX_POINTS) arr.shift()
}

function applyStatus(s: ServerStatus) {
  status.value = s
  const ts = s.time * 1000
  const m = s.memory
  pushWindow(cpuHistory, [ts, +s.cpu.percent.toFixed(1)])
  pushWindow(memHistory, [ts, m.total > 0 ? +m.used_percent.toFixed(1) : null])
  pushWindow(swapHistory, [ts, m.swap_total > 0 ? +m.swap_percent.toFixed(1) : null])
  cpuChart?.setOption({ series: [{ data: cpuHistory }] })
  memChart?.setOption({ series: [{ data: memHistory }, { data: swapHistory }] })
}

// ---- 轮询（setTimeout 链防请求堆积；页面隐藏自动暂停，可见时恢复） ----
let timer: number | undefined
let wasFailing = false

function clearTimer() {
  if (timer != null) { window.clearTimeout(timer); timer = undefined }
}
function schedule() {
  clearTimer()
  if (paused.value || document.hidden) return
  timer = window.setTimeout(async () => {
    await load()
    schedule()
  }, intervalSec.value * 1000)
}
async function load() {
  loading.value = true
  try {
    const { data } = await getServerStatus()
    applyStatus(data.data)
    wasFailing = false
  } catch (e: any) {
    // 连续轮询失败只提示一次，避免打扰
    if (!wasFailing) message.error(e?.response?.data?.msg || t('monitor.loadFailed'))
    wasFailing = true
  } finally {
    loading.value = false
  }
}
function togglePause() {
  paused.value = !paused.value
  if (paused.value) clearTimer()
  else void load().then(schedule)
}
function refreshNow() {
  if (paused.value) paused.value = false
  void load().then(schedule)
}
function onVisibility() {
  if (document.hidden) clearTimer()
  else void load().then(schedule)
}
watch(intervalSec, () => schedule())

// ---- 历史回看 ----
const historyHours = ref(24)
const history = ref<MonitorHistoryPoint[]>([])
const histChartRef = ref<HTMLElement | null>(null)
let histChart: echarts.ECharts | null = null
let histResizeOb: ResizeObserver | null = null

async function loadHistory() {
  try {
    const res = await getMonitorHistory(historyHours.value)
    history.value = res.data.data
    await nextTick()
    renderHistory()
  } catch { /* 历史加载失败不影响实时区 */ }
}

function renderHistory() {
  if (!histChartRef.value || !history.value.length) return
  if (!histChart) {
    histChart = echarts.init(histChartRef.value)
    histResizeOb = new ResizeObserver(() => histChart?.resize())
    histResizeOb.observe(histChartRef.value)
  }
  const pts = history.value
  const times = pts.map((p) => new Date(p.ts * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }))
  histChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['CPU %', t('monitor.memTitle') + ' %', 'NET'] },
    grid: { left: 48, right: 48, top: 36, bottom: 28 },
    xAxis: { type: 'category', data: times },
    yAxis: [
      { type: 'value', max: 100, name: '%' },
      { type: 'value', name: 'B/s', splitLine: { show: false } },
    ],
    series: [
      { name: 'CPU %', type: 'line', showSymbol: false, itemStyle: { color: ACCENT }, data: pts.map((p) => p.cpu_percent.toFixed(1)) },
      { name: t('monitor.memTitle') + ' %', type: 'line', showSymbol: false, itemStyle: { color: '#7FB069' }, data: pts.map((p) => p.mem_percent.toFixed(1)) },
      { name: 'NET', type: 'line', yAxisIndex: 1, showSymbol: false, itemStyle: { color: '#d9903f' }, data: pts.map((p) => Math.round(p.net_recv_rate + p.net_send_rate)) },
    ],
  })
}

onMounted(() => {
  loadHistory()
  initCharts()
  document.addEventListener('visibilitychange', onVisibility)
  void load().then(schedule)
})
onBeforeUnmount(() => {
  histResizeOb?.disconnect()
  histChart?.dispose()
  clearTimer()
  document.removeEventListener('visibilitychange', onVisibility)
  ro?.disconnect()
  cpuChart?.dispose()
  memChart?.dispose()
})
</script>

<style scoped>
.history-card {
  margin-top: 12px;
}

/* 撑满一屏：100vh - 顶栏 64px - 内容区上下 padding（与 Profile 页同范式） */
.monitor-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: calc(100vh - 64px - 16px);
  min-height: 0;
}
.host-card :deep(.n-card-content) {
  padding: 10px 16px;
}
.host-bar {
  display: flex;
  align-items: center;
  gap: 24px;
  flex-wrap: wrap;
}
.host-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.host-label {
  font-size: 11px;
  color: var(--sx-muted);
  white-space: nowrap;
}
.host-value {
  font-size: 13px;
  color: var(--sx-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 260px;
}
.host-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}
.interval-select {
  width: 84px;
}
/* 左右 4:6 固定比例分栏，任何窗口宽度都并排；小高度时栏内滚动兜底 */
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
.cpu-card {
  flex: 1 1 auto;
  min-height: 420px;
  display: flex;
  flex-direction: column;
}
.cpu-card :deep(.n-card-content) {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: auto;
}
.cpu-now {
  font-size: 15px;
  font-weight: 700;
}
.cpu-model {
  font-size: 11px;
  color: var(--sx-muted);
  margin-bottom: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chart {
  width: 100%;
  height: 190px;
  flex: none;
}
.cores {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.core-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.core-label {
  width: 34px;
  font-size: 11px;
  color: var(--sx-muted);
  flex: none;
}
.core-bar {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: var(--sx-accent-soft);
  overflow: hidden;
}
.core-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.6s ease;
}
.core-val {
  width: 36px;
  text-align: right;
  font-size: 11px;
  color: var(--sx-muted);
  flex: none;
}
.go-card :deep(.n-card-content) {
  padding: 12px 16px;
}
.go-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px 16px;
}
.go-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.go-val {
  font-size: 13px;
  color: var(--sx-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.mem-card .mem-stats {
  display: flex;
  gap: 24px;
  margin-top: 6px;
  flex-wrap: wrap;
}
.mem-stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.list-card :deep(.n-card-content) {
  padding: 12px 16px;
}
.empty {
  font-size: 12px;
  color: var(--sx-muted);
  text-align: center;
  padding: 8px 0;
}
.row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 5px 0;
}
.row-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
  width: 55%;
  min-width: 0;
}
.row-title {
  font-size: 13px;
  color: var(--sx-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.row-sub {
  font-size: 11px;
  color: var(--sx-muted);
  white-space: nowrap;
}
.row-bar {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: var(--sx-accent-soft);
  overflow: hidden;
}
.row-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.6s ease;
}
.row-val {
  width: 52px;
  text-align: right;
  font-size: 12px;
  flex: none;
}
.net-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 5px 0;
  flex-wrap: wrap;
}
.net-row .row-title {
  width: 90px;
  flex: none;
}
.net-dir {
  font-size: 13px;
}
.net-dir.down { color: var(--sx-accent); }
.net-dir.up { color: #7A9E57; }
.net-row .row-sub {
  margin-left: auto;
}
/* 占用分级：正常用主色，≥60% 警告，≥85% 危险（文本与进度条分色） */
.text-ok { color: var(--sx-accent); }
.text-warn { color: #B77E1F; }
.text-danger { color: var(--sx-danger); }
.fill-ok { background-color: var(--sx-accent); }
.fill-warn { background-color: #E0A63D; }
.fill-danger { background-color: var(--sx-danger); }
</style>
