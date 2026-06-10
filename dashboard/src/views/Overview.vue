<template>
  <div class="overview">
    <h1 class="sr-only">系统概览</h1>
    <el-row :gutter="20">
      <el-col :span="6" v-for="stat in statsCards" :key="stat.key">
        <el-card class="stat-card" shadow="hover">
          <div class="stat-content">
            <div class="stat-icon" :style="{ background: stat.color }">
              <component :is="stat.icon" />
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ stat.value }}</div>
              <div class="stat-label">{{ stat.label }}</div>
              <div v-if="stat.comparison" class="stat-comparison" :class="getTrendClass(stat.trend)">
                {{ stat.comparison }}
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Anomaly Workstation Area -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="8">
        <el-card class="anomaly-card">
          <template #header>
            <div class="card-header">
              <span>需要关注</span>
              <el-tag size="small" type="danger">NEW</el-tag>
            </div>
          </template>
          <div v-loading="loadingNewErrors" class="anomaly-content">
            <div v-if="newErrors.length === 0" class="empty-state">
              <el-empty description="暂无新错误" :image-size="40" />
            </div>
            <div v-else class="error-list">
              <div
                v-for="(error, index) in newErrors"
                :key="index"
                class="error-item-new"
                @click="goToLogsWithError(error.message)"
              >
                <div class="error-info">
                  <div class="error-message">{{ truncateMessage(error.message, 50) }}</div>
                  <div class="error-meta">
                    <span>{{ error.count }} 次</span>
                    <span>{{ error.affected_users }} 用户</span>
                    <span class="time-ago">{{ formatRelativeTime(error.first_seen) }}</span>
                  </div>
                </div>
                <el-tag size="small" type="danger">NEW</el-tag>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="8">
        <el-card class="anomaly-card">
          <template #header>
            <div class="card-header">
              <span>最近告警</span>
              <el-button size="small" link @click="goToAlerts">查看全部</el-button>
            </div>
          </template>
          <div v-loading="loadingAlerts" class="anomaly-content">
            <div v-if="alertTriggers.length === 0" class="empty-state">
              <el-empty description="暂无告警触发" :image-size="40" />
            </div>
            <div v-else class="alert-list">
              <div
                v-for="(alert, index) in alertTriggers"
                :key="index"
                class="alert-item"
                :class="getAlertClass(alert.severity)"
                @click="goToAlerts"
              >
                <div class="alert-info">
                  <div class="alert-name">{{ alert.alert_name || `告警 #${alert.alert_id}` }}</div>
                  <div class="alert-meta">
                    <span>{{ formatRelativeTime(alert.triggered_at) }}</span>
                  </div>
                </div>
                <el-tag :type="getAlertTagType(alert.severity)" size="small">
                  {{ alert.severity }}
                </el-tag>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="8">
        <el-card class="anomaly-card">
          <template #header>
            <div class="card-header">
              <span>活跃会话</span>
              <el-button size="small" link @click="refreshSessions">刷新</el-button>
            </div>
          </template>
          <div v-loading="loadingSessions" class="anomaly-content">
            <div v-if="activeSessions.length === 0" class="empty-state">
              <el-empty description="暂无活跃会话" :image-size="40" />
            </div>
            <div v-else class="session-list">
              <div
                v-for="(session, index) in activeSessions"
                :key="index"
                class="session-item"
              >
                <div class="session-info">
                  <div class="session-url">{{ truncateMessage(session.url, 40) }}</div>
                  <div class="session-meta">
                    <span>{{ session.event_count }} 事件</span>
                    <span class="time-ago">{{ formatRelativeTime(session.last_activity) }}</span>
                  </div>
                </div>
                <el-button size="small" type="primary" link @click="goToSession(session.session_id)">
                  回放
                </el-button>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Quick Actions Area -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card class="quick-actions-card">
          <template #header>
            <span>快捷操作</span>
          </template>
          <div class="quick-actions">
            <el-button @click="goToTodayErrors" type="danger" plain>
              今天错误
            </el-button>
            <el-button @click="goToYesterdayCompare" type="info" plain>
              昨天对比
            </el-button>
            <el-button @click="goToThisWeekTop" type="warning" plain>
              本周 Top
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="16">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>24h 错误趋势</span>
              <el-button size="small" @click="refreshStats">刷新</el-button>
            </div>
          </template>
          <div ref="trendChartRef" style="height: 300px"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="chart-card">
          <template #header>
            <span>Top 统计</span>
          </template>
          <el-tabs v-model="topTab" @tab-change="handleTopTabChange">
            <el-tab-pane label="错误" name="errors"></el-tab-pane>
            <el-tab-pane label="页面" name="pages"></el-tab-pane>
            <el-tab-pane label="版本" name="releases"></el-tab-pane>
            <el-tab-pane label="浏览器" name="browsers"></el-tab-pane>
          </el-tabs>
          <div class="top-content">
            <div class="top-controls">
              <el-select v-model="topOrderBy" size="small" @change="fetchTopData" style="width: 120px">
                <el-option label="按频次" value="count" />
                <el-option label="按影响面" value="users" />
                <el-option label="按影响值" value="impact" />
                <el-option label="按最近" value="recent" />
                <el-option label="按回归" value="regression" />
              </el-select>
            </div>
            <div v-loading="loadingTop" class="top-list">
              <div
                v-for="(item, index) in topData"
                :key="index"
                class="top-item"
                :class="{ 'top-item-new': item.isNew }"
              >
              <span class="top-rank" :class="`rank-${index + 1}`">{{ index + 1 }}</span>
              <div class="top-info">
                <div class="top-key">{{ truncateTopKey(item.key, topTab) }}</div>
                <div class="top-meta">
                  <span>{{ item.count }} 次</span>
                  <span>{{ item.users }} 用户</span>
                  <span v-if="item.isNew" class="new-badge">新</span>
                </div>
              </div>
              <span class="top-score">{{ item.impactScore }}</span>
            </div>
            <el-empty v-if="topData.length === 0" description="暂无数据" :image-size="60" />
          </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card>
          <template #header>
            <span>应用列表</span>
          </template>
          <el-table :data="apps" stripe>
            <el-table-column prop="app_id" label="应用 ID" width="200" />
            <el-table-column prop="release" label="版本" width="120" />
            <el-table-column prop="event_count" label="总事件数" width="120" align="right">
              <template #default="{ row }">
                {{ formatNumber(row.event_count) }}
              </template>
            </el-table-column>
            <el-table-column prop="error_count" label="错误数" width="120" align="right">
              <template #default="{ row }">
                <span :class="{ 'text-error': row.error_count > 0 }">{{ row.error_count }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="last_seen" label="最后活跃" width="180">
              <template #default="{ row }">
                {{ formatRelativeTime(row.last_seen) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" align="center">
              <template #default="{ row }">
                <el-button type="primary" size="small" link @click="goToLogs(row.app_id)">
                  查看日志
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { logApi } from '../api'
import { formatNumber, formatRelativeTime, truncateMessage } from '../utils/formatters'
import { Warning, InfoFilled, WarningFilled, CircleCheck } from '@element-plus/icons-vue'

const router = useRouter()
const trendChartRef = ref<HTMLElement>()
const apps = ref<any[]>([])
const stats = ref<any>(null)
const topTab = ref('errors')
const topOrderBy = ref('count')
const topData = ref<any[]>([])
const loadingTop = ref(false)
let trendChart: echarts.ECharts | null = null

// Anomaly workstation data
const newErrors = ref<any[]>([])
const alertTriggers = ref<any[]>([])
const activeSessions = ref<any[]>([])
const statsComparison = ref<any>(null)
const loadingNewErrors = ref(false)
const loadingAlerts = ref(false)
const loadingSessions = ref(false)

const statsCards = computed(() => [
  {
    key: 'total',
    label: '总事件数',
    value: formatNumber(stats.value?.totalEvents || 0),
    icon: InfoFilled,
    color: 'linear-gradient(135deg, #3b82f6, #1d4ed8)',
    comparison: getComparisonDisplay('events'),
    trend: getTrendDirection('events')
  },
  {
    key: 'errors',
    label: '错误数',
    value: formatNumber(stats.value?.errorCount || 0),
    icon: WarningFilled,
    color: 'linear-gradient(135deg, #ef4444, #dc2626)',
    comparison: getComparisonDisplay('errors'),
    trend: getTrendDirection('errors')
  },
  {
    key: 'warnings',
    label: '警告数',
    value: formatNumber(stats.value?.warnCount || 0),
    icon: Warning,
    color: 'linear-gradient(135deg, #f59e0b, #d97706)',
    comparison: null, // No comparison for warnings
    trend: null
  },
  {
    key: 'info',
    label: '信息数',
    value: formatNumber(stats.value?.infoCount || 0),
    icon: CircleCheck,
    color: 'linear-gradient(135deg, #10b981, #059669)',
    comparison: null, // No comparison for info
    trend: null
  },
  {
    key: 'affectedUsers',
    label: '影响用户数',
    value: formatNumber(statsComparison.value?.today_affected_users || 0),
    icon: InfoFilled,
    color: 'linear-gradient(135deg, #8b5cf6, #7c3aed)',
    comparison: getComparisonDisplay('affected_users'),
    trend: getTrendDirection('affected_users')
  }
])

// Helper functions for stat card enhancements
const getComparisonDisplay = (metric: string) => {
  if (!statsComparison.value) return null

  const changeKey = `${metric}_change` as keyof typeof statsComparison.value
  const change = statsComparison.value[changeKey] as number

  if (change === undefined || change === 0) return null

  const direction = change > 0 ? '↑' : '↓'
  const percentage = Math.abs(change).toFixed(1)
  return `${direction} ${percentage}%`
}

const getTrendDirection = (metric: string) => {
  if (!statsComparison.value) return null

  const changeKey = `${metric}_change` as keyof typeof statsComparison.value
  const change = statsComparison.value[changeKey] as number

  if (change === undefined || change === 0) return null

  // For errors, positive change is bad (red), negative is good (green)
  // For events and affected users, positive change is usually neutral
  if (metric === 'errors') {
    return change > 0 ? 'bad' : 'good'
  }
  return change > 0 ? 'neutral' : 'neutral'
}

const getTrendClass = (trend: string | null) => {
  if (!trend) return ''
  const classes: Record<string, string> = {
    good: 'trend-good',
    bad: 'trend-bad',
    neutral: 'trend-neutral'
  }
  return classes[trend] || ''
}

const topErrors = computed(() => stats.value?.topErrors || [])

const currentAppId = computed(() => apps.value.length > 0 ? apps.value[0].app_id : '')

// Fetch Top N data
const fetchTopData = async () => {
	if (!currentAppId.value) return

	loadingTop.value = true
	try {
		const { data } = await logApi.getTop({
			appId: currentAppId.value,
			type: topTab.value,
			orderBy: topOrderBy.value,
			limit: 10
		})
		topData.value = data.data || []
	} catch (error) {
		console.error('Failed to fetch top data:', error)
		topData.value = []
	} finally {
		loadingTop.value = false
	}
}

const handleTopTabChange = () => {
	fetchTopData()
}

const truncateTopKey = (key: string, type: string) => {
	if (type === 'pages') {
		// For URLs, show the path part
		try {
			const url = new URL(key)
			return url.pathname + url.search
		} catch {
			return key
		}
	}
	return truncateMessage(key, 40)
}

const fetchData = async () => {
  try {
    const appsRes = await logApi.getApps()
    apps.value = appsRes.data

    // Use first app for stats, or 'all' if no apps
    const appId = apps.value.length > 0 ? apps.value[0].app_id : 'all'
    try {
      const statsRes = await logApi.getStats(appId)
      stats.value = statsRes.data
      renderTrendChart()
    } catch (statsErr) {
      // Stats may fail if no appId matches, use defaults
      stats.value = { totalEvents: 0, errorCount: 0, warnCount: 0, infoCount: 0, topErrors: [], errorTrend: [] }
      renderTrendChart()
    }
  } catch (error) {
    console.error('Failed to fetch overview data:', error)
  }
}

const refreshStats = () => {
  fetchData()
}

const renderTrendChart = () => {
  if (!trendChartRef.value) return
  const trend = stats.value?.errorTrend || []
  if (trend.length === 0) return

  // Dispose previous chart instance to prevent memory leak
  if (trendChart) {
    trendChart.dispose()
    trendChart = null
  }

  trendChart = echarts.init(trendChartRef.value)
  const option = {
    backgroundColor: 'transparent',
    grid: { top: 20, right: 20, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: trend.map((t: any) => {
        const date = new Date(t.timestamp)
        return `${date.getHours().toString().padStart(2, '0')}:00`
      }),
      axisLine: { lineStyle: { color: 'var(--color-border)' } },
      axisLabel: { color: '#94a3b8' }
    },
    yAxis: {
      type: 'value',
      axisLine: { lineStyle: { color: 'var(--color-border)' } },
      axisLabel: { color: '#94a3b8' },
      splitLine: { lineStyle: { color: 'var(--color-border)' } }
    },
    series: [{
      data: trend.map((t: any) => t.count),
      type: 'line',
      smooth: true,
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(99, 102, 241, 0.3)' },
          { offset: 1, color: 'rgba(99, 102, 241, 0)' }
        ])
      },
      lineStyle: { color: '#6366f1', width: 2 },
      itemStyle: { color: '#6366f1' }
    }]
  }
  trendChart.setOption(option)
}

const goToLogs = (appId: string) => {
  router.push(`/logs/${appId}`)
}

// Anomaly workstation methods
const fetchAnomalyData = async () => {
  if (!currentAppId.value) return

  // Fetch new errors from last hour
  loadingNewErrors.value = true
  try {
    const { data } = await logApi.getNewErrors({ app_id: currentAppId.value, since: 60 })
    newErrors.value = data.data || []
  } catch (error) {
    console.error('Failed to fetch new errors:', error)
    newErrors.value = []
  } finally {
    loadingNewErrors.value = false
  }

  // Fetch recent alert triggers
  loadingAlerts.value = true
  try {
    const { data } = await logApi.getAlertTriggers({ limit: 5 })
    alertTriggers.value = data.data || []
  } catch (error) {
    console.error('Failed to fetch alert triggers:', error)
    alertTriggers.value = []
  } finally {
    loadingAlerts.value = false
  }

  // Fetch active sessions
  loadingSessions.value = true
  try {
    const { data } = await logApi.getActiveSessions({ app_id: currentAppId.value, limit: 5 })
    activeSessions.value = data.data || []
  } catch (error) {
    console.error('Failed to fetch active sessions:', error)
    activeSessions.value = []
  } finally {
    loadingSessions.value = false
  }

  // Fetch stats comparison
  try {
    const { data } = await logApi.getStatsComparison({ app_id: currentAppId.value })
    statsComparison.value = data
  } catch (error) {
    console.error('Failed to fetch stats comparison:', error)
    statsComparison.value = null
  }
}

const refreshSessions = () => {
  fetchAnomalyData()
}

const goToLogsWithError = (errorMessage: string) => {
  router.push({
    path: `/logs/${currentAppId.value}`,
    query: { keyword: errorMessage, level: 'error' }
  })
}

const goToAlerts = () => {
  router.push(`/alerts/${currentAppId.value}`)
}

const goToSession = (sessionId: string) => {
  router.push(`/sessions/${currentAppId.value}/${sessionId}`)
}

// Quick action methods
const goToTodayErrors = () => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const startTime = today.getTime()

  router.push({
    path: `/logs/${currentAppId.value}`,
    query: { level: 'error', startTime: startTime.toString() }
  })
}

const goToYesterdayCompare = () => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const yesterdayStart = new Date(today)
  yesterdayStart.setDate(yesterdayStart.getDate() - 1)
  const todayEnd = today.getTime()

  router.push({
    path: `/logs/${currentAppId.value}`,
    query: { startTime: yesterdayStart.getTime().toString(), endTime: todayEnd.toString() }
  })
}

const goToThisWeekTop = () => {
  router.push({
    path: `/logs/${currentAppId.value}`,
    query: { type: 'top', orderBy: 'count' }
  })
}

// Alert helper methods
const getAlertClass = (severity: string) => {
  const classes: Record<string, string> = {
    critical: 'alert-critical',
    high: 'alert-high',
    medium: 'alert-medium',
    low: 'alert-low'
  }
  return classes[severity] || ''
}

const getAlertTagType = (severity: string) => {
  const types: Record<string, string> = {
    critical: 'danger',
    high: 'warning',
    medium: 'info',
    low: 'info'
  }
  return types[severity] || 'info'
}

onMounted(() => {
  fetchData()
  fetchAnomalyData()
})

onUnmounted(() => {
  // Dispose of chart instance to prevent memory leak
  if (trendChart) {
    trendChart.dispose()
    trendChart = null
  }
})
</script>

<style scoped>
.overview {
  padding: 0;
}

.stat-card {
  height: 100px;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 50px;
  height: 50px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 24px;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text);
}

.stat-label {
  font-size: 14px;
  color: var(--color-text-secondary);
}

.chart-card {
  height: 400px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.top-errors {
  max-height: 280px;
  overflow-y: auto;
}

.error-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border);
}

.error-item:last-child {
  border-bottom: none;
}

.error-rank {
  width: 28px;
  height: 28px;
  background: var(--color-bg-tertiary);
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-right: 12px;
}

.error-rank:nth-child(1) {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: #fff;
}

.error-message {
  flex: 1;
  color: var(--color-text);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.error-count {
  padding: 4px 10px;
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  font-size: 12px;
  color: #ef4444;
  font-weight: 600;
}

/* Anomaly Workstation Styles */
.anomaly-card {
  height: 320px;
}

.anomaly-content {
  min-height: 200px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 160px;
}

/* New Errors List */
.error-list {
  max-height: 240px;
  overflow-y: auto;
}

.error-item-new {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.error-item-new:hover {
  background: var(--color-bg-tertiary);
}

.error-item-new:last-child {
  margin-bottom: 0;
}

.error-info {
  flex: 1;
  min-width: 0;
}

.error-message {
  font-size: 13px;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 4px;
}

.error-meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.time-ago {
  margin-left: auto;
}

/* Alert Triggers List */
.alert-list {
  max-height: 240px;
  overflow-y: auto;
}

.alert-item {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.alert-item:hover {
  background: var(--color-bg-tertiary);
}

.alert-item:last-child {
  margin-bottom: 0;
}

.alert-item.alert-critical {
  border-left: 3px solid #ef4444;
}

.alert-item.alert-high {
  border-left: 3px solid #f59e0b;
}

.alert-item.alert-medium {
  border-left: 3px solid #3b82f6;
}

.alert-item.alert-low {
  border-left: 3px solid #6b7280;
}

.alert-info {
  flex: 1;
  min-width: 0;
}

.alert-name {
  font-size: 13px;
  color: var(--color-text);
  font-weight: 500;
  margin-bottom: 4px;
}

.alert-meta {
  font-size: 12px;
  color: var(--color-text-secondary);
}

/* Active Sessions List */
.session-list {
  max-height: 240px;
  overflow-y: auto;
}

.session-item {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
}

.session-item:last-child {
  margin-bottom: 0;
}

.session-info {
  flex: 1;
  min-width: 0;
}

.session-url {
  font-size: 13px;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 4px;
}

.session-meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

/* Quick Actions Card */
.quick-actions-card {
  background: linear-gradient(135deg, #f8fafc, #e2e8f0);
}

.quick-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

/* Stat Card Enhancements */
.stat-comparison {
  font-size: 12px;
  margin-top: 4px;
  font-weight: 500;
}

.trend-good {
  color: #10b981;
}

.trend-bad {
  color: #ef4444;
}

.trend-neutral {
  color: var(--color-text-secondary);
}

/* Responsive adjustments */
@media (max-width: 1200px) {
  .anomaly-card {
    height: 280px;
  }

  .error-list,
  .alert-list,
  .session-list {
    max-height: 200px;
  }
}
</style>
