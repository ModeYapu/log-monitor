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

    <el-row :gutter="20" class="mt-4">
      <el-col :span="6" v-for="metric in performanceMetrics" :key="metric.key">
        <el-card class="metric-card">
          <div class="metric-content">
            <div class="metric-header">
              <span class="metric-label">{{ metric.label }}</span>
              <span class="metric-dot" :class="metric.dotClass"></span>
            </div>
            <div class="metric-value">{{ metric.value }}</div>
            <div class="metric-badge" :class="metric.gradeClass">
              {{ metric.grade }}
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>FCP 趋势</span>
          </template>
          <div ref="fcpChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>LCP 趋势</span>
          </template>
          <div ref="lcpChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>Cls 趋势</span>
          </template>
          <div ref="clsChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>性能评分分布</span>
          </template>
          <div ref="scoreDistChartRef" style="height: 280px"></div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card>
          <template #header>
            <span>慢页面 Top 10</span>
          </template>
          <el-table :data="slowPages" stripe>
            <el-table-column type="index" label="#" width="60" />
            <el-table-column prop="url" label="页面 URL" min-width="300">
              <template #default="{ row }">
                {{ truncateUrl(row.url) }}
              </template>
            </el-table-column>
            <el-table-column prop="fcp" label="FCP" width="120" align="right">
              <template #default="{ row }">
                <span class="metric-cell" :class="getPerformanceGradeClass('fcp', row.fcp)">{{ row.fcp }}ms</span>
              </template>
            </el-table-column>
            <el-table-column prop="lcp" label="LCP" width="120" align="right">
              <template #default="{ row }">
                <span class="metric-cell" :class="getPerformanceGradeClass('lcp', row.lcp)">{{ row.lcp }}ms</span>
              </template>
            </el-table-column>
            <el-table-column prop="cls" label="CLS" width="120" align="right">
              <template #default="{ row }">
                <span class="metric-cell" :class="getPerformanceGradeClass('cls', row.cls)">{{ row.cls }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="samples" label="样本数" width="100" align="right" />
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as echarts from 'echarts'
import { logApi } from '../api'
import type { Event } from '../types'

const selectedAppId = ref('')
const apps = ref<any[]>([])
const timeRange = ref('24h')
const performanceEvents = ref<Event[]>([])

const fcpChartRef = ref<HTMLElement>()
const lcpChartRef = ref<HTMLElement>()
const clsChartRef = ref<HTMLElement>()
const scoreDistChartRef = ref<HTMLElement>()

const performanceMetrics = computed(() => {
  const metrics = calculateMetrics()
  return [
    {
      key: 'fcp',
      label: 'FCP (P95)',
      value: `${metrics.fcp}ms`,
      grade: getPerformanceGrade('fcp', metrics.fcp),
      gradeClass: getPerformanceGradeClass('fcp', metrics.fcp),
      dotClass: getPerformanceDotClass('fcp', metrics.fcp)
    },
    {
      key: 'lcp',
      label: 'LCP (P95)',
      value: `${metrics.lcp}ms`,
      grade: getPerformanceGrade('lcp', metrics.lcp),
      gradeClass: getPerformanceGradeClass('lcp', metrics.lcp),
      dotClass: getPerformanceDotClass('lcp', metrics.lcp)
    },
    {
      key: 'cls',
      label: 'CLS (P95)',
      value: metrics.cls.toFixed(3),
      grade: getPerformanceGrade('cls', metrics.cls),
      gradeClass: getPerformanceGradeClass('cls', metrics.cls),
      dotClass: getPerformanceDotClass('cls', metrics.cls)
    },
    {
      key: 'fid',
      label: 'FID (P95)',
      value: `${metrics.fid}ms`,
      grade: getPerformanceGrade('fid', metrics.fid),
      gradeClass: getPerformanceGradeClass('fid', metrics.fid),
      dotClass: getPerformanceDotClass('fid', metrics.fid)
    }
  ]
})

const slowPages = computed(() => {
  const pageMap = new Map<string, any>()

  performanceEvents.value.forEach(event => {
    try {
      const perf = JSON.parse(event.performance || '{}')
      if (!perf.fcp && !perf.lcp) return

      const url = event.url || 'unknown'
      if (!pageMap.has(url)) {
        pageMap.set(url, {
          url,
          fcp: [],
          lcp: [],
          cls: [],
          samples: 0
        })
      }

      const page = pageMap.get(url)
      if (perf.fcp) page.fcp.push(perf.fcp)
      if (perf.lcp) page.lcp.push(perf.lcp)
      if (perf.cls) page.cls.push(perf.cls)
      page.samples++
    } catch (e) {}
  })

  return Array.from(pageMap.values())
    .map(page => ({
      ...page,
      fcp: Math.round(average(page.fcp) || 0),
      lcp: Math.round(average(page.lcp) || 0),
      cls: parseFloat((average(page.cls) || 0).toFixed(3))
    }))
    .sort((a, b) => b.lcp - a.lcp)
    .slice(0, 10)
})

const average = (arr: number[]) => {
  if (!arr.length) return 0
  return arr.reduce((a, b) => a + b, 0) / arr.length
}

const percentile = (arr: number[], p: number) => {
  if (!arr.length) return 0
  const sorted = [...arr].sort((a, b) => a - b)
  const idx = Math.ceil((p / 100) * sorted.length) - 1
  return sorted[Math.max(0, idx)]
}

const calculateMetrics = () => {
  const fcpValues: number[] = []
  const lcpValues: number[] = []
  const clsValues: number[] = []
  const fidValues: number[] = []

  performanceEvents.value.forEach(event => {
    try {
      const perf = JSON.parse(event.performance || '{}')
      if (perf.fcp) fcpValues.push(perf.fcp)
      if (perf.lcp) lcpValues.push(perf.lcp)
      if (perf.cls) clsValues.push(perf.cls)
      if (perf.fid) fidValues.push(perf.fid)
    } catch (e) {}
  })

  return {
    fcp: Math.round(percentile(fcpValues, 95)),
    lcp: Math.round(percentile(lcpValues, 95)),
    cls: parseFloat((percentile(clsValues, 95) || 0).toFixed(3)),
    fid: Math.round(percentile(fidValues, 95) || 0)
  }
}

const getPerformanceGrade = (metric: string, value: number): string => {
  const thresholds: Record<string, { good: number, needsImprovement: number }> = {
    fcp: { good: 1800, needsImprovement: 3000 },
    lcp: { good: 2500, needsImprovement: 4000 },
    cls: { good: 0.1, needsImprovement: 0.25 },
    fid: { good: 100, needsImprovement: 300 }
  }

  const t = thresholds[metric]
  if (!t) return '-'

  if (value <= t.good) return '好'
  if (value <= t.needsImprovement) return '需改进'
  return '差'
}

const getPerformanceGradeClass = (metric: string, value: number): string => {
  const grade = getPerformanceGrade(metric, value)
  return {
    '好': 'grade-good',
    '需改进': 'grade-needs-improvement',
    '差': 'grade-poor'
  }[grade] || ''
}

const getPerformanceDotClass = (metric: string, value: number): string => {
  const grade = getPerformanceGrade(metric, value)
  return {
    '好': 'dot-good',
    '需改进': 'dot-needs-improvement',
    '差': 'dot-poor'
  }[grade] || 'dot-good'
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
    const endTime = Date.now()
    const startTimeMap: Record<string, number> = {
      '24h': endTime - 24 * 60 * 60 * 1000,
      '7d': endTime - 7 * 24 * 60 * 60 * 1000,
      '30d': endTime - 30 * 24 * 60 * 60 * 1000
    }

    const { data } = await logApi.query({
      appId: selectedAppId.value,
      type: 'performance',
      startTime: startTimeMap[timeRange.value] || startTimeMap['24h'],
      endTime,
      page: 1,
      pageSize: 5000
    })

    performanceEvents.value = data.data
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
  renderScoreDistribution()
}

const renderTrendChart = (element: HTMLElement | undefined, metric: string) => {
  if (!element) return

  const chart = echarts.init(element)

  const data = performanceEvents.value
    .map(e => {
      try {
        return {
          time: e.created_at,
          value: (JSON.parse(e.performance || '{}') as any)[metric]
        }
      } catch {
        return null
      }
    })
    .filter(d => d?.value)

  const option = {
    backgroundColor: 'transparent',
    grid: { top: 20, right: 20, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: data.map(d => new Date(d.time).toLocaleTimeString()),
      axisLine: { lineStyle: { color: '#4a5568' } },
      axisLabel: { color: '#94a3b8', fontSize: 10 }
    },
    yAxis: {
      type: 'value',
      axisLine: { lineStyle: { color: '#4a5568' } },
      axisLabel: { color: '#94a3b8' },
      splitLine: { lineStyle: { color: '#2d3748' } }
    },
    series: [{
      data: data.map(d => d.value),
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

  const grades = { good: 0, needsImprovement: 0, poor: 0 }

  performanceEvents.value.forEach(event => {
    try {
      const perf = JSON.parse(event.performance || '{}')
      const fcp = perf.fcp || 0
      const lcp = perf.lcp || 0
      const cls = perf.cls || 0

      let score = 0
      if (fcp <= 1800) score++
      if (lcp <= 2500) score++
      if (cls <= 0.1) score++

      if (score === 3) grades.good++
      else if (score >= 1) grades.needsImprovement++
      else grades.poor++
    } catch (e) {}
  })

  const option = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    legend: {
      bottom: 10,
      textStyle: { color: '#94a3b8' }
    },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      avoidLabelOverlap: false,
      itemStyle: {
        borderRadius: 6,
        borderColor: '#131829',
        borderWidth: 2
      },
      label: {
        show: false
      },
      data: [
        { value: grades.good, name: '优秀', itemStyle: { color: '#10b981' } },
        { value: grades.needsImprovement, name: '需改进', itemStyle: { color: '#f59e0b' } },
        { value: grades.poor, name: '差', itemStyle: { color: '#ef4444' } }
      ]
    }]
  }

  chart.setOption(option)
}

onMounted(() => {
  fetchApps()
})
</script>

<style scoped>
.performance-page {
  display: flex;
  flex-direction: column;
}

.metric-card {
  height: 140px;
}

.metric-content {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 100%;
  gap: 6px;
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

.filter-card {
  margin-bottom: 20px;
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
</style>
