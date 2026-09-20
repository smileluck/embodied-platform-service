<template>
  <!-- 用量统计：汇总卡 + 按日 token 柱状图 + Top Agent 表；天数可选 7/14/30 -->
  <div class="usage-page">
    <n-card size="small">
      <div class="head">
        <span class="card-title">{{ t('agent.usage.title') }}</span>
        <n-radio-group v-model:value="days" size="small" @update:value="load">
          <n-radio-button :value="7">7</n-radio-button>
          <n-radio-button :value="14">14</n-radio-button>
          <n-radio-button :value="30">30</n-radio-button>
        </n-radio-group>
      </div>

      <div class="summary">
        <n-card size="small" class="stat">
          <n-statistic :label="t('agent.usage.calls')" :value="stats?.calls ?? 0" />
        </n-card>
        <n-card size="small" class="stat">
          <n-statistic :label="t('agent.usage.tokens')" :value="stats?.tokens ?? 0" />
        </n-card>
        <n-card size="small" class="stat">
          <n-statistic :label="t('agent.usage.cost')" :value="stats?.cost ?? 0" :precision="2" />
        </n-card>
      </div>
    </n-card>

    <n-card size="small" class="chart-card" :title="t('agent.usage.dailyChart')">
      <div ref="chartRef" class="chart" />
    </n-card>

    <n-card size="small" :title="t('agent.usage.topAgents')">
      <n-data-table size="small" :columns="columns" :data="stats?.agents ?? []" :bordered="false" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { NCard, NDataTable, NRadioButton, NRadioGroup, NStatistic } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { getAgentUsage } from '../../api'
import type { UsageAgentPoint, UsageStats } from '../../api/types'

echarts.use([BarChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const { t } = useI18n()
const days = ref(7)
const stats = ref<UsageStats | null>(null)
const chartRef = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null
let resizeOb: ResizeObserver | null = null

const columns = computed<DataTableColumns<UsageAgentPoint>>(() => [
  { title: t('agent.usage.agent'), key: 'agent_name', render: (row) => row.agent_name || `#${row.agent_id}` },
  { title: t('agent.usage.calls'), key: 'calls', width: 120 },
  {
    title: t('agent.usage.tokens'), key: 'total_tokens', width: 160,
    render: (row) => row.total_tokens.toLocaleString(),
  },
  { title: t('agent.usage.cost'), key: 'cost', width: 140, render: (row) => row.cost.toFixed(4) },
])

async function load() {
  const res = await getAgentUsage(days.value)
  stats.value = res.data.data
  await nextTick()
  renderChart()
}

function renderChart() {
  if (!chartRef.value) return
  if (!chart) {
    chart = echarts.init(chartRef.value)
    resizeOb = new ResizeObserver(() => chart?.resize())
    resizeOb.observe(chartRef.value)
  }
  const pts = stats.value?.days ?? []
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: [t('agent.usage.promptTokens'), t('agent.usage.completionTokens')] },
    grid: { left: 56, right: 16, top: 36, bottom: 28 },
    xAxis: { type: 'category', data: pts.map((p) => p.date.slice(5)) },
    yAxis: { type: 'value' },
    series: [
      { name: t('agent.usage.promptTokens'), type: 'bar', stack: 'tok', barMaxWidth: 28, itemStyle: { color: '#3F75AB' }, data: pts.map((p) => p.prompt_tokens) },
      { name: t('agent.usage.completionTokens'), type: 'bar', stack: 'tok', barMaxWidth: 28, itemStyle: { color: '#7FB069' }, data: pts.map((p) => p.completion_tokens) },
    ],
  })
}

onMounted(load)
onBeforeUnmount(() => {
  resizeOb?.disconnect()
  chart?.dispose()
})
</script>

<style scoped>
.usage-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.card-title {
  font-weight: 600;
}
.summary {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-top: 12px;
}
@media (max-width: 720px) {
  .summary {
    grid-template-columns: 1fr;
  }
}
.chart {
  height: 300px;
}
</style>
