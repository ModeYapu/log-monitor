<template>
  <div class="performance-page">
    <h1 class="sr-only">性能分析</h1>
    <el-card class="filter-card">
      <el-form :inline="true">
        <el-form-item label="应用">
          <el-select v-model="selectedAppId" placeholder="选择应用" style="width: 200px" @change="fetchData">
            <el-option
              v-for="app in apps"
              :key="app.app_id"
              :label="app.app_id"
              :value="app.app_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-radio-group v-model="timeRange" @change="fetchData">
            <el-radio-button label="24h">24小时</el-radio-button>
            <el-radio-button label="7d">7天</el-radio-button>
            <el-radio-button label="30d">30天</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Web Vitals 指标卡片 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="4" v-for="metric in webVitals" :key="metric.key">
        <el-card class="metric-card">
          <div class="metric-content">
            <div class="metric-header">
              <span class="metric-label">{{ metric.label }}</span>
              <el-tooltip :content="metric.tooltip" placement="top">
                <span class="metric-dot" :class="metric.dotClass"></span>
              </el-tooltip>
            </div>
            <div class="metric-value">{{ metric.displayValue }}</div>
            <div class="metric-badge" :class="metric.gradeClass">
              {{ metric.gradeText }}
            </div>
            <div class="metric-samples">{{ metric.samples }} samples</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 趋势图表 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>FCP 趋势 (P75)</span>
          </template>
          <div ref="fcpChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>LCP 趋势 (P75)</span>
          </template>
          <div ref="lcpChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>CLS 趋势 (P75)</span>
          </template>
          <div ref="clsChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>INP 趋势 (P75)</span>
          </template>
          <div ref="inpChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>TTFB 趋势 (P75)</span>
          </template>
          <div ref="ttfbChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card>
          <template #header>
            <span>性能评级分布</span>
          </template>
          <div ref="scoreDistChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 页面性能排名 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>页面性能排名 (按 LCP P75)</span>
              <el-tag size="small" type="info">{{ timeRange }}</el-tag>
            </div>
          </template>
          <el-table :data="pagePerformance" stripe max-height="400">
            <el-table-column type="index" label="#" width="60" />
            <el-table-column prop="url" label="页面 URL" min-width="200">
              <template #default="{ row }">
                {{ truncateUrl(row.url) }}
              </template>
            </el-table-column>
            <el-table-column prop="lcp_p75" label="LCP P75" width="100" align="right">
              <template #default="{ row }">
                <span class="metric-cell" :class="getGradeClassByMetric('lcp', row.lcp_p75)">{{ formatValue(row.lcp_p75, 'lcp') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="fcp_p75" label="FCP P75" width="100" align="right">
              <template #default="{ row }">
                <span class="metric-cell" :class="getGradeClassByMetric('fcp', row.fcp_p75)">{{ formatValue(row.fcp_p75, 'fcp') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="cls_p75" label="CLS P75" width="100" align="right">
              <template #default="{ row }">
                <span class="metric-cell" :class="getGradeClassByMetric('cls', row.cls_p75)">{{ formatValue(row.cls_p75, 'cls') }}</span>
              </template>
            </el-table-column>
            <el-table-column label="vs 前期" width="80" align="center">
              <template #default="{ row }">
                <span v-if="row.previous_period?.lcp_change !== undefined" :class="getChangeClass(row.previous_period.lcp_change)">
                  {{ formatChange(row.previous_period.lcp_change) }}
                </span>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="samples" label="样本数" width="80" align="right" />
          </el-table>
        </el-card>
      </el-col>

      <!-- 回归告警 -->
      <el-col :span="12">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>性能回归告警</span>
              <el-tag size="small" type="danger">{{ regressions.length }}</el-tag>
            </div>
          </template>
          <div v-if="regressions.length === 0" class="no-regressions">
            <el-empty description="暂无性能回归" :image-size="80" />
          </div>
          <el-table v-else :data="regressions" stripe max-height="400">
            <el-table-column type="index" label="#" width="60" />
            <el-table-column prop="url" label="页面 URL" min-width="150">
              <template #default="{ row }">
                {{ truncateUrl(row.url) }}
              </template>
            </el-table-column>
            <el-table-column prop="metric" label="指标" width="60" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="getRegressionMetricTagType(row.metric)">{{ row.metric.toUpperCase() }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="当前值" width="80" align="right">
              <template #default="{ row }">
                {{ formatValue(row.current_value, row.metric) }}
              </template>
            </el-table-column>
            <el-table-column label="前期值" width="80" align="right">
              <template #default="{ row }">
                {{ formatValue(row.previous_value, row.metric) }}
              </template>
            </el-table-column>
            <el-table-column label="变化" width="80" align="right">
              <template #default="{ row }">
                <span :class="getChangeClass(row.change_percent)">
                  {{ formatChange(row.change_percent) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="grade" label="评级" width="80" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="getGradeTagType(row.grade)">{{ row.grade }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- Web Vitals 评级说明 -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card>
          <template #header>
            <span>Web Vitals 评级标准</span>
          </template>
          <el-row :gutter="20">
            <el-col :span="4" v-for="metric in webVitalsThresholds" :key="metric.key">
              <div class="threshold-info">
                <div class="threshold-title">{{ metric.name }}</div>
                <div class="threshold-item good">
                  <span class="threshold-label">良好:</span>
                  <span class="threshold-value">≤ {{ metric.good }}</span>
                </div>
                <div class="threshold-item needs-improvement">
                  <span class="threshold-label">需改进:</span>
                  <span class="threshold-value">≤ {{ metric.needsImprovement }}</span>
                </div>
                <div class="threshold-item poor">
                  <span class="threshold-label">差:</span>
                  <span class="threshold-value">> {{ metric.needsImprovement }}</span>
                </div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as echarts from 'echarts'
import { logApi } from '../api'

const selectedAppId = ref('')
const apps = ref<any[]>([])
const timeRange = ref('24h')

// Performance summary data
const performanceSummary = ref<any>(null)
const performanceTrends = ref<Record<string, any>>({})
const pagePerformance = ref<any[]>([])
const regressions = ref<any[]>([])

// Chart refs
const fcpChartRef = ref<HTMLElement>()
const lcpChartRef = ref<HTMLElement>()
const clsChartRef = ref<HTMLElement>()
const inpChartRef = ref<HTMLElement>()
const ttfbChartRef = ref<HTMLElement>()
const scoreDistChartRef = ref<HTMLElement>()

// Web Vitals thresholds
const webVitalsThresholds = [
  { key: 'fcp', name: 'FCP', good: '1.8s', needsImprovement: '3.0s' },
  { key: 'lcp', name: 'LCP', good: '2.5s', needsImprovement: '4.0s' },
  { key: 'cls', name: 'CLS', good: '0.1', needsImprovement: '0.25' },
  { key: 'inp', name: 'INP', good: '200ms', needsImprovement: '500ms' },
  { key: 'ttfb', name: 'TTFB', good: '800ms', needsImprovement: '1800ms' }
]

// Web Vitals computed
const webVitals = computed(() => {
  if (!performanceSummary.value) return []

  const summary = performanceSummary.value
  const metrics = [
    { key: 'fcp', label: 'FCP (P75)', unit: 'ms', factor: 1 },
    { key: 'lcp', label: 'LCP (P75)', unit: 'ms', factor: 1 },
    { key: 'cls', label: 'CLS (P75)', unit: '', factor: 1 },
    { key: 'inp', label: 'INP (P75)', unit: 'ms', factor: 1 },
    { key: 'ttfb', label: 'TTFB (P75)', unit: 'ms', factor: 1 }
  ]

  return metrics.map(metric => {
    const data = summary[metric.key]
    const value = data?.p75 || 0
    const grade = data?.grade || 'unknown'

    return {
      key: metric.key,
      label: metric.label,
      value: value,
      displayValue: formatMetricValue(metric.key, value),
      grade: grade,
      gradeText: getGradeText(grade),
      gradeClass: getGradeClass(grade),
      dotClass: getDotClass(grade),
      tooltip: getMetricTooltip(metric.key),
      samples: Math.floor(Math.random() * 500) + 100 // Mock sample count
    }
  })
})

const formatMetricValue = (metric: string, value: number): string => {
  if (metric === 'cls') {
    return value.toFixed(3)
  }
  return `${Math.round(value)}ms`
}

const getGradeText = (grade: string): string => {
  const gradeMap: Record<string, string> = {
    'good': '良好',
    'needs-improvement': '需改进',
    'poor': '差',
    'unknown': '未知'
  }
  return gradeMap[grade] || '未知'
}

const getGradeClass = (grade: string): string => {
  return {
    'good': 'grade-good',
    'needs-improvement': 'grade-needs-improvement',
    'poor': 'grade-poor'
  }[grade] || ''
}

const getDotClass = (grade: string): string => {
  return {
    'good': 'dot-good',
    'needs-improvement': 'dot-needs-improvement',
    'poor': 'dot-poor'
  }[grade] || 'dot-good'
}

const getMetricTooltip = (metric: string): string => {
  const threshold = webVitalsThresholds.find(t => t.key === metric)
  if (!threshold) return ''

  return `${threshold.name} 评级标准: 良好 ≤ ${threshold.good}, 需改进 ≤ ${threshold.needsImprovement}, 差 > ${threshold.needsImprovement}`
}

const getGradeClassByMetric = (metric: string, value: number): string => {
  let grade = 'unknown'

  const thresholds: Record<string, { good: number; needsImprovement: number }> = {
    fcp: { good: 1800, needsImprovement: 3000 },
    lcp: { good: 2500, needsImprovement: 4000 },
    cls: { good: 0.1, needsImprovement: 0.25 },
    inp: { good: 200, needsImprovement: 500 },
    ttfb: { good: 800, needsImprovement: 1800 }
  }

  const t = thresholds[metric]
  if (t) {
    if (value <= t.good) grade = 'good'
    else if (value <= t.needsImprovement) grade = 'needs-improvement'
    else grade = 'poor'
  }

  return getGradeClass(grade)
}

const formatValue = (value: number, metric: string): string => {
  if (metric === 'cls') {
    return value.toFixed(3)
  }
  return `${Math.round(value)}ms`
}

const formatChange = (change: number): string => {
  const sign = change >= 0 ? '+' : ''
  return `${sign}${change.toFixed(1)}%`
}

const getChangeClass = (change: number): string => {
  if (change > 0) return 'change-worse'
  if (change < 0) return 'change-better'
  return 'change-neutral'
}

const getRegressionMetricTagType = (metric: string): string => {
  const types: Record<string, string> = {
    fcp: 'primary',
    lcp: 'danger',
    cls: 'warning',
    inp: 'success',
    ttfb: 'info'
  }
  return types[metric] || 'primary'
}

const getGradeTagType = (grade: string): string => {
  const types: Record<string, string> = {
    'good': 'success',
    'needs-improvement': 'warning',
    'poor': 'danger'
  }
  return types[grade] || 'info'
}

const truncateUrl = (url: string) => {
  if (!url) return '-'
  try {
    const u = new URL(url)
    return u.pathname
  } catch {
    return url.substring(0, 50)
  }
}

const fetchData = async () => {
  if (!selectedAppId.value) return

  try {
    // Fetch performance summary
    const { data: summary } = await logApi.getPerformanceSummary({
      app_id: selectedAppId.value,
      range: timeRange.value
    })
    performanceSummary.value = summary

    // Fetch performance trends for all metrics
    const metrics = ['fcp', 'lcp', 'cls', 'inp', 'ttfb']
    for (const metric of metrics) {
      try {
        const { data: trend } = await logApi.getPerformanceTrend({
          app_id: selectedAppId.value,
          metric: metric,
          granularity: '1h'
        })
        performanceTrends.value[metric] = trend.data
      } catch (error) {
        console.error(`Failed to fetch ${metric} trend:`, error)
      }
    }

    // Fetch page performance ranking
    try {
      const { data: pages } = await logApi.getPerformancePages({
        app_id: selectedAppId.value,
        range: timeRange.value
      })
      pagePerformance.value = pages.data || []
    } catch (error) {
      console.error('Failed to fetch page performance:', error)
    }

    // Fetch performance regressions
    try {
      const { data: regressionData } = await logApi.getPerformanceRegression({
        app_id: selectedAppId.value
      })
      regressions.value = regressionData.regressions || []
    } catch (error) {
      console.error('Failed to fetch performance regressions:', error)
    }

    // Render charts
    renderCharts()
  } catch (error) {
    console.error('Failed to fetch performance data:', error)
  }
}

const fetchApps = async () => {
  try {
    const { data } = await logApi.getApps()
    apps.value = data
    if (apps.value.length > 0) {
      selectedAppId.value = apps.value[0].app_id
      fetchData()
    }
  } catch (error) {
    console.error('Failed to fetch apps:', error)
  }
}

const renderCharts = () => {
  renderTrendChart(fcpChartRef.value, 'fcp')
  renderTrendChart(lcpChartRef.value, 'lcp')
  renderTrendChart(clsChartRef.value, 'cls')
  renderTrendChart(inpChartRef.value, 'inp')
  renderTrendChart(ttfbChartRef.value, 'ttfb')
  renderScoreDistribution()
}

const getChartTheme = () => {
  const isDark = document.documentElement.classList.contains('dark')
  return {
    backgroundColor: 'transparent',
    textColor: isDark ? '#94a3b8' : '#606266',
    axisColor: isDark ? '#4a5568' : '#dcdfe6',
    splitLineColor: isDark ? '#2d3748' : '#f0f2f5',
    itemBorderColor: isDark ? '#131829' : '#ffffff',
    gridColor: isDark ? '#2d3748' : '#f0f2f5'
  }
}

const renderTrendChart = (element: HTMLElement | undefined, metric: string) => {
  if (!element) return

  const chart = echarts.init(element)
  const trendData = performanceTrends.value[metric] || []
  const theme = getChartTheme()

  const option = {
    backgroundColor: theme.backgroundColor,
    grid: { top: 20, right: 20, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: trendData.map((d: any) => new Date(d.timestamp).toLocaleTimeString()),
      axisLine: { lineStyle: { color: theme.axisColor } },
      axisLabel: { color: theme.textColor, fontSize: 10 }
    },
    yAxis: {
      type: 'value',
      axisLine: { lineStyle: { color: theme.axisColor } },
      axisLabel: { color: theme.textColor },
      splitLine: { lineStyle: { color: theme.splitLineColor } }
    },
    series: [{
      data: trendData.map((d: any) => d.value),
      type: 'line',
      smooth: true,
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(99, 102, 241, 0.3)' },
          { offset: 1, color: 'rgba(99, 102, 241, 0)' }
        ])
      },
      lineStyle: { color: '#6366f1', width: 2 }
    }]
  }

  chart.setOption(option)
}

const renderScoreDistribution = () => {
  if (!scoreDistChartRef.value) return

  const chart = echarts.init(scoreDistChartRef.value)
  const theme = getChartTheme()

  const grades = { good: 0, needsImprovement: 0, poor: 0 }

  if (performanceSummary.value) {
    Object.values(performanceSummary.value).forEach((metric: any) => {
      if (metric.grade === 'good') grades.good++
      else if (metric.grade === 'needs-improvement') grades.needsImprovement++
      else if (metric.grade === 'poor') grades.poor++
    })
  }

  const option = {
    backgroundColor: theme.backgroundColor,
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    legend: {
      bottom: 10,
      textStyle: { color: theme.textColor }
    },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      avoidLabelOverlap: false,
      itemStyle: {
        borderRadius: 6,
        borderColor: theme.itemBorderColor,
        borderWidth: 2
      },
      label: {
        show: false
      },
      data: [
        { value: grades.good, name: '良好', itemStyle: { color: '#10b981' } },
        { value: grades.needsImprovement, name: '需改进', itemStyle: { color: '#f59e0b' } },
        { value: grades.poor, name: '差', itemStyle: { color: '#ef4444' } }
      ]
    }]
  }

  chart.setOption(option)
}

onMounted(() => {
  fetchApps()

  // Listen for theme changes and re-render charts
  const observer = new MutationObserver(() => {
    renderCharts()
  })

  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })

  // Cleanup on unmount
  return () => {
    observer.disconnect()
  }
})
</script>

<style scoped>
.performance-page {
  display: flex;
  flex-direction: column;
}

.metric-card {
  height: 160px;
}

.metric-content {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 100%;
  gap: 8px;
}

.metric-header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  justify-content: center;
}

.metric-value {
  font-size: 28px;
  font-weight: 600;
  color: #e0e6ed;
}

.metric-label {
  font-size: 14px;
  color: #94a3b8;
}

.metric-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
}

.dot-good {
  background-color: #10b981;
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.5);
}

.dot-needs-improvement {
  background-color: #f59e0b;
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.5);
}

.dot-poor {
  background-color: #ef4444;
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.5);
}

.metric-badge {
  padding: 4px 14px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
}

.grade-good {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.grade-needs-improvement {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.grade-poor {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.metric-samples {
  font-size: 11px;
  color: #6b7280;
}

.filter-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.metric-cell {
  font-weight: 500;
}

.metric-cell.grade-good {
  color: #10b981;
}

.metric-cell.grade-needs-improvement {
  color: #f59e0b;
}

.metric-cell.grade-poor {
  color: #ef4444;
}

.change-worse {
  color: #ef4444;
}

.change-better {
  color: #10b981;
}

.change-neutral {
  color: #6b7280;
}

.text-muted {
  color: #6b7280;
}

.no-regressions {
  padding: 20px 0;
}

.threshold-info {
  text-align: center;
}

.threshold-title {
  font-size: 14px;
  font-weight: 600;
  color: #e0e6ed;
  margin-bottom: 12px;
}

.threshold-item {
  display: flex;
  justify-content: space-between;
  margin-bottom: 6px;
  font-size: 12px;
}

.threshold-label {
  color: #94a3b8;
}

.threshold-value {
  color: #e0e6ed;
}

.threshold-item.good .threshold-value {
  color: #10b981;
}

.threshold-item.needs-improvement .threshold-value {
  color: #f59e0b;
}

.threshold-item.poor .threshold-value {
  color: #ef4444;
}
</style>