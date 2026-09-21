<!-- 设备运营总览（首页）：指标全部由现有列表接口的 page.total 聚合（page_size=1），无专用统计接口；
     某块接口失败（含无权限 403）该处显示 — ，不阻塞整页。状态呈现统一用「状态灯语」(.sx-led)。 -->
<template>
  <!-- 首页仪表盘：统计卡 + 7 日登录/操作趋势 + 最近登录 -->
  <div class="dash-page">
    <div class="cards">
      <n-card size="small">
        <n-statistic :label="t('dashboard.users')" :value="stats?.cards.users ?? 0" />
      </n-card>
      <n-card size="small">
        <n-statistic :label="t('dashboard.roles')" :value="stats?.cards.roles ?? 0" />
      </n-card>
      <n-card size="small">
      </n-card>
      <n-card size="small">
        <n-statistic :label="t('dashboard.todayLogins')" :value="stats?.cards.today_logins ?? 0" />
      </n-card>
    </div>

    <n-card size="small" class="chart-card" :title="t('dashboard.trend')">
      <div ref="chartRef" class="chart" />
    </n-card>

    <n-card size="small" :title="t('dashboard.recentLogins')">
      <n-data-table size="small" :columns="columns" :data="stats?.recent_logins ?? []" :loading="loading" :bordered="false" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { NCard, NDataTable, NStatistic, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { getDashboardStats } from '../../api'
import type { DashboardStats } from '../../api/types'
import { isDarkRef } from '../../stores/theme'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const { t, locale } = useI18n()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartRef = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null
let resizeOb: ResizeObserver | null = null

const columns = computed<DataTableColumns<DashboardStats['recent_logins'][number]>>(() => [
  { title: t('user.username'), key: 'username' },
  { title: 'IP', key: 'ip', className: 'mono' },
  {
    title: t('common.status'), key: 'status', width: 90,
    render: (row) => h(NTag, { size: 'small', bordered: false, type: row.status === 'success' ? 'success' : 'error' },
      { default: () => (row.status === 'success' ? t('dashboard.loginOk') : t('dashboard.loginFail')) }),
  },
  {
    title: t('common.createTime'), key: 'created_at', width: 170,
    render: (row) => row.created_at?.slice(0, 19).replace('T', ' ') ?? '—',
  },
])

async function load() {
  loading.value = true
  try {
    const res = await getDashboardStats()
    stats.value = res.data.data
    await nextTick()
    renderChart()
  } finally {
    loading.value = false
  }
}

function renderChart() {
  if (!chartRef.value) return
  if (!chart) {
    chart = echarts.init(chartRef.value)
    resizeOb = new ResizeObserver(() => chart?.resize())
    resizeOb.observe(chartRef.value)
  }
  const login = stats.value?.login_trend ?? []
  const op = stats.value?.op_trend ?? []
  // 轴/图例文字用中性灰（亮暗两套底色都可读）；echarts 默认 #333 在暗色下几乎不可见
  const MUTED = '#6B7787'
  const LINE = 'rgba(107, 119, 135, 0.35)'
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { top: 0, itemWidth: 14, textStyle: { fontSize: 11, color: MUTED }, data: [t('dashboard.loginTotal'), t('dashboard.loginSuccess'), t('dashboard.opCount')] },
    grid: { left: 48, right: 16, top: 36, bottom: 28 },
    xAxis: {
      type: 'category',
      data: login.map((p) => p.date.slice(5)),
      axisLabel: { fontSize: 10, color: MUTED, hideOverlap: true },
      axisLine: { lineStyle: { color: LINE } },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { fontSize: 10, color: MUTED },
      splitLine: { lineStyle: { color: LINE } },
    },
    series: [
      { name: t('dashboard.loginTotal'), type: 'line', smooth: true, symbolSize: 5, itemStyle: { color: '#3F75AB' }, data: login.map((p) => p.total) },
      { name: t('dashboard.loginSuccess'), type: 'line', smooth: true, symbolSize: 5, itemStyle: { color: '#7FB069' }, data: login.map((p) => p.success) },
      { name: t('dashboard.opCount'), type: 'line', smooth: true, symbolSize: 5, itemStyle: { color: '#d9903f' }, data: op.map((p) => p.total) },
    ],
  })
}

onMounted(load)
// 主题切换后重绘：echarts 文字/线色不会随 CSS 变量自动跟随
watch(isDarkRef, () => renderChart())
// 语言切换后重绘：图例/系列名是 setOption 时固化的 t() 文案
watch(locale, () => renderChart())
onBeforeUnmount(() => {
  resizeOb?.disconnect()
  chart?.dispose()
})
</script>

<style scoped>
.dash-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
@media (max-width: 900px) {
  .cards {
    grid-template-columns: repeat(2, 1fr);
  }
}
.chart {
  height: 280px;
}
.mono {
  font-family: var(--sx-font-mono);
}
</style>
