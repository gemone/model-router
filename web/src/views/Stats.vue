<template>
  <div class="stats">
    <div class="page-header">
      <h1 class="page-title">{{ $t('stats.title') }}</h1>
      <el-radio-group v-model="timeRange" @change="fetchData" size="small">
        <el-radio-button label="today">{{ $t('stats.today') }}</el-radio-button>
        <el-radio-button label="7d">{{ $t('stats.last7Days') }}</el-radio-button>
        <el-radio-button label="30d">{{ $t('stats.last30Days') }}</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 统计卡片 -->
    <el-row :gutter="16" class="stats-row">
      <el-col :xs="12" :sm="12" :md="6" v-for="stat in statCards" :key="stat.key">
        <el-card class="stat-card" shadow="hover">
          <div class="stat-content">
            <div class="stat-icon" :style="{ backgroundColor: stat.color }">
              <el-icon :size="20"><component :is="stat.icon" /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ stat.value }}</div>
              <div class="stat-label">{{ stat.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="16" class="charts-row">
      <el-col :xs="24" :lg="12">
        <el-card class="chart-card">
          <template #header>
            <span class="card-title">{{ $t('stats.requestStats') }}</span>
          </template>
          <div class="chart-container">
            <v-chart :option="requestChartOption" autoresize />
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card class="chart-card">
          <template #header>
            <span class="card-title">{{ $t('stats.tokenStats') }}</span>
          </template>
          <div class="chart-container">
            <v-chart :option="tokenChartOption" autoresize />
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card class="chart-card">
          <template #header>
            <span class="card-title">{{ $t('stats.latencyStats') }}</span>
          </template>
          <div class="chart-container">
            <v-chart :option="latencyChartOption" autoresize />
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card class="chart-card">
          <template #header>
            <span class="card-title">{{ $t('stats.errorRate') }}</span>
          </template>
          <div class="chart-container pie-container">
            <v-chart :option="errorRateChartOption" autoresize />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 供应商统计 -->
    <el-row class="table-row">
      <el-col :span="24">
        <el-card class="table-card">
          <template #header>
            <span class="card-title">{{ $t('stats.providerStats') }}</span>
          </template>
          <el-table :data="providerStats" stripe size="small" class="data-table">
            <el-table-column prop="name" :label="$t('provider.name')" min-width="140" />
            <el-table-column :label="$t('stats.requests')" align="right" width="110">
              <template #default="{ row }">{{ row.requests?.toLocaleString() || 0 }}</template>
            </el-table-column>
            <el-table-column :label="$t('stats.tokens')" align="right" width="110">
              <template #default="{ row }">{{ row.tokens?.toLocaleString() || 0 }}</template>
            </el-table-column>
            <el-table-column :label="$t('stats.latency')" align="right" width="100">
              <template #default="{ row }">{{ row.avg_latency?.toFixed(0) || 0 }}ms</template>
            </el-table-column>
            <el-table-column :label="$t('stats.errorRate')" align="right" width="100">
              <template #default="{ row }">
                <el-tag
                  :type="row.error_rate < 0.05 ? 'success' : row.error_rate < 0.1 ? 'warning' : 'danger'"
                  size="small"
                >
                  {{ (row.error_rate * 100).toFixed(1) }}%
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- 模型统计 -->
    <el-row class="table-row">
      <el-col :span="24">
        <el-card class="table-card">
          <template #header>
            <span class="card-title">{{ $t('stats.modelStats') }}</span>
          </template>
          <el-table :data="modelStats" stripe size="small" class="data-table">
            <el-table-column prop="name" :label="$t('model.name')" min-width="140" />
            <el-table-column :label="$t('stats.requests')" align="right" width="110">
              <template #default="{ row }">{{ row.requests?.toLocaleString() || 0 }}</template>
            </el-table-column>
            <el-table-column :label="$t('stats.tokens')" align="right" width="110">
              <template #default="{ row }">{{ row.tokens?.toLocaleString() || 0 }}</template>
            </el-table-column>
            <el-table-column :label="$t('stats.latency')" align="right" width="100">
              <template #default="{ row }">{{ row.avg_latency?.toFixed(0) || 0 }}ms</template>
            </el-table-column>
            <el-table-column :label="$t('stats.errorRate')" align="right" width="100">
              <template #default="{ row }">
                <el-tag
                  :type="row.error_rate < 0.05 ? 'success' : row.error_rate < 0.1 ? 'warning' : 'danger'"
                  size="small"
                >
                  {{ (row.error_rate * 100).toFixed(1) }}%
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
} from 'echarts/components'
import VChart from 'vue-echarts'
import { useAppStore } from '@/stores/app'

use([
  CanvasRenderer,
  LineChart,
  BarChart,
  PieChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
])

const { t } = useI18n()
const store = useAppStore()

const timeRange = ref('7d')
const statsData = ref({})

// 统计卡片
const statCards = computed(() => {
  const s = store.stats
  return [
    {
      key: 'requests',
      value: (s.total_requests_24h || 0).toLocaleString(),
      label: t('stats.requests'),
      icon: 'DataLine',
      color: '#409EFF',
    },
    {
      key: 'tokens',
      value: ((s.total_tokens || 0) / 1000).toFixed(1) + 'K',
      label: t('stats.tokens'),
      icon: 'Document',
      color: '#67C23A',
    },
    {
      key: 'latency',
      value: (s.avg_latency_ms || 0).toFixed(0) + 'ms',
      label: t('stats.latency'),
      icon: 'Timer',
      color: '#E6A23C',
    },
    {
      key: 'success',
      value: (s.success_rate || 0).toFixed(1) + '%',
      label: t('dashboard.successRate'),
      icon: 'CircleCheck',
      color: '#F56C6C',
    },
  ]
})

// 请求图表配置
const requestChartOption = computed(() => {
  const trend = store.trendStats || {}
  const hours = trend.hours || []
  const requests = trend.requests || []
  const hasData = requests.some(r => r > 0)

  return {
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: {
      type: 'category',
      data: hours.length > 0 ? hours : ['00:00', '06:00', '12:00', '18:00'],
      axisLine: { lineStyle: { color: '#e5e7eb' } },
      axisLabel: { color: '#6b7280', fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f3f4f6' } },
      axisLabel: { color: '#6b7280', fontSize: 11 },
    },
    series: [
      {
        name: t('stats.requests'),
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 4,
        data: hasData ? requests : [0, 0, 0, 0],
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(64, 158, 255, 0.25)' },
              { offset: 1, color: 'rgba(64, 158, 255, 0.02)' },
            ],
          },
        },
        itemStyle: { color: '#409EFF' },
        lineStyle: { width: 2 },
      },
    ],
  }
})

// Token 图表配置
const tokenChartOption = computed(() => {
  const trend = store.trendStats || {}
  const hours = trend.hours || []
  const tokens = trend.tokens || []
  const hasData = tokens.some(t => t > 0)

  return {
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: {
      type: 'category',
      data: hours.length > 0 ? hours : ['00:00', '06:00', '12:00', '18:00'],
      axisLine: { lineStyle: { color: '#e5e7eb' } },
      axisLabel: { color: '#6b7280', fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f3f4f6' } },
      axisLabel: { color: '#6b7280', fontSize: 11 },
    },
    series: [
      {
        name: t('stats.tokens'),
        type: 'bar',
        data: hasData ? tokens : [0, 0, 0, 0],
        itemStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: '#67C23A' },
              { offset: 1, color: '#95d475' },
            ],
          },
          borderRadius: [4, 4, 0, 0],
        },
      },
    ],
  }
})

// 延迟图表配置
const latencyChartOption = computed(() => {
  const trend = store.trendStats || {}
  const hours = trend.hours || []
  const avgLatency = statsData.value.avg_latency_ms || 0
  const hasData = hours.length > 0 && avgLatency > 0

  return {
    tooltip: { trigger: 'axis' },
    grid: { left: '3%', right: '4%', bottom: '3%', top: '10%', containLabel: true },
    xAxis: {
      type: 'category',
      data: hours.length > 0 ? hours : ['00:00', '06:00', '12:00', '18:00'],
      axisLine: { lineStyle: { color: '#e5e7eb' } },
      axisLabel: { color: '#6b7280', fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f3f4f6' } },
      axisLabel: { color: '#6b7280', fontSize: 11 },
    },
    series: [
      {
        name: t('stats.latency'),
        type: 'line',
        smooth: true,
        data: hasData ? new Array(hours.length).fill(avgLatency) : [0, 0, 0, 0],
        itemStyle: { color: '#E6A23C' },
        lineStyle: { width: 2 },
      },
    ],
  }
})

// 错误率图表配置
const errorRateChartOption = computed(() => {
  const s = statsData.value
  const successRate = s.success_rate || 0
  const errorRate = 100 - successRate

  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c}%' },
    legend: {
      orient: 'vertical',
      right: 0,
      top: 'center',
      itemWidth: 12,
      itemHeight: 12,
      textStyle: { fontSize: 12 }
    },
    series: [
      {
        type: 'pie',
        radius: ['55%', '75%'],
        center: ['35%', '50%'],
        data: [
          { value: successRate, name: 'Success', itemStyle: { color: '#67C23A' } },
          { value: errorRate > 0 ? errorRate : 0.1, name: 'Error', itemStyle: { color: '#F56C6C' } },
        ],
        label: { show: false },
        emphasis: { label: { show: true, fontSize: 13, fontWeight: 'bold' } },
      },
    ],
  }
})

// 供应商统计数据
const providerStats = computed(() => {
  const providerStatsData = store.providerModelStats?.providers || []
  const providersMap = new Map((store.providers || []).map(p => [p.id, p.name]))

  return providerStatsData.map(p => ({
    name: providersMap.get(p.id) || p.name,
    requests: p.requests || 0,
    tokens: p.tokens || 0,
    avg_latency: p.avg_latency || 0,
    error_rate: p.error_rate || 0,
  }))
})

// 模型统计数据
const modelStats = computed(() => {
  const modelStatsData = store.providerModelStats?.models || []

  return modelStatsData.map(m => ({
    name: m.name,
    requests: m.requests || 0,
    tokens: m.tokens || 0,
    avg_latency: m.avg_latency || 0,
    error_rate: m.error_rate || 0,
  }))
})

async function fetchData() {
  await store.fetchStats()
  await store.fetchTrendStats()
  await store.fetchProviderModelStats()
  statsData.value = store.stats || {}
}

onMounted(() => {
  fetchData()
  store.fetchProviders()
  store.fetchModels()
})
</script>

<style scoped>
.stats {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: #1f2937;
}

.stats-row {
  margin-bottom: 16px;
}

.stat-card {
  margin-bottom: 0;
}

.stat-card :deep(.el-card__body) {
  padding: 16px;
}

.stat-content {
  display: flex;
  align-items: center;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  margin-right: 12px;
  flex-shrink: 0;
}

.stat-value {
  font-size: 22px;
  font-weight: 700;
  color: #1f2937;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: #6b7280;
  margin-top: 2px;
}

.charts-row {
  margin-bottom: 16px;
}

.chart-card {
  margin-bottom: 0;
}

.chart-card :deep(.el-card__header) {
  padding: 14px 16px;
  border-bottom: 1px solid #f0f0f0;
}

.chart-card :deep(.el-card__body) {
  padding: 16px;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
}

.chart-container {
  height: 240px;
}

.pie-container {
  height: 200px;
}

.table-row {
  margin-bottom: 16px;
}

.table-row:last-child {
  margin-bottom: 0;
}

.table-card :deep(.el-card__header) {
  padding: 14px 16px;
  border-bottom: 1px solid #f0f0f0;
}

.table-card :deep(.el-card__body) {
  padding: 0;
}

.data-table {
  font-size: 13px;
}

.data-table :deep(.el-table__header th) {
  background-color: #fafafa;
  font-weight: 600;
}

.data-table :deep(.el-table__body-wrapper) {
  max-height: 300px;
  overflow-y: auto;
}
</style>
