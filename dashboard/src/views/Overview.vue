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

    <!-- 7-Day Error Trend Chart (CSS Bar Chart) -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
        <el-card class="chart-card trend-card">
          <template #header>
            <div class="card-header">
              <span>近7天错误趋势</span>
              <el-button size="small" @click="refreshStats">刷新</el-button>
            </div>
          </template>
          <div v-loading="loadingTrend" class="trend-chart-container">
            <div v-if="dailyTrend.length === 0" class="empty-state">
              <el-empty description="暂无趋势数据" :image-size="60" />
            </div>
            <div v-else class="trend-chart">
              <div class="trend-bars">
                <div
                  v-for="(day, index) in dailyTrend"
                  :key="index"
                  class="trend-bar-wrapper"
                >
                  <div
                    class="trend-bar"
                    :style="{
                      height: getBarHeight(day.count, maxDailyCount) + '%',
                      background: getBarColor(day.count)
                    }"
                    @mouseenter="hoveredBar = index"
                    @mouseleave="hoveredBar = -1"
                  >
                    <div v-if="hoveredBar === index" class="bar-tooltip">
                      <div class="tooltip-date">{{ formatDate(day.timestamp) }}</div>
                      <div class="tooltip-count">{{ day.count }} 次错误</div>
                    </div>
                  </div>
                  <div class="trend-label">{{ formatDayLabel(day.timestamp) }}</div>
                  <div class="trend-count">{{ day.count }}</div>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Top 5 Errors & Recent Alerts -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="12">
        <el-card class="ranking-card">
          <template #header>
            <div class="card-header">
              <span>Top 5 错误</span>
              <el-button size="small" link @click="goToIssues">查看全部</el-button>
            </div>
          </template>
          <div v-loading="loadingTopErrors" class="ranking-content">
            <div v-if="topErrors.length === 0" class="empty-state">
              <el-empty description="暂无错误数据" :image-size="40" />
            </div>
            <div v-else class="top-errors-list">
              <div
                v-for="(error, index) in topErrors"
                :key="index"
                class="top-error-item"
                @click="goToIssue(error.id)"
              >
                <div class="error-rank" :class="`rank-${index + 1}`">{{ index + 1 }}</div>
                <div class="error-content">
                  <div class="error-title" :title="error.title">{{ truncateMessage(error.title, 80) }}</div>
                  <div class="error-meta">
                    <span class="error-count">{{ error.event_count }} 次</span>
                    <span class="error-users">{{ error.user_count }} 用户</span>
                    <span class="error-time">{{ formatRelativeTime(error.last_seen_at) }}</span>
                  </div>
                </div>
                <el-icon class="arrow-icon"><ArrowRight /></el-icon>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="ranking-card">
          <template #header>
            <div class="card-header">
              <span>最近告警</span>
              <el-button size="small" link @click="goToAlerts">查看全部</el-button>
            </div>
          </template>
          <div v-loading="loadingAlerts" class="ranking-content">
            <div v-if="recentAlerts.length === 0" class="empty-state">
              <el-empty description="暂无告警记录" :image-size="40" />
            </div>
            <div v-else class="alerts-list">
              <div
                v-for="(alert, index) in recentAlerts"
                :key="index"
                class="alert-item"
                @click="goToAlerts"
              >
                <div class="alert-icon" :class="getAlertIconClass(alert)">
                  <el-icon><WarningFilled v-if="alert.severity === 'critical'" /><Warning v-else /></el-icon>
                </div>
                <div class="alert-content">
                  <div class="alert-name">{{ alert.alert_name || `告警 #${alert.alert_id}` }}</div>
                  <div class="alert-meta">
                    <span class="alert-time">{{ formatRelativeTime(alert.triggered_at) }}</span>
                    <el-tag :type="getAlertTagType(alert.severity)" size="small">
                      {{ alert.severity }}
                    </el-tag>
                  </div>
                </div>
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
              <span>待处理问题</span>
              <el-button size="small" link @click="goToIssues">查看全部</el-button>
            </div>
          </template>
          <div v-loading="loadingIssues" class="anomaly-content">
            <div v-if="issueStats.total_count === 0" class="empty-state">
              <el-empty description="暂无待处理问题" :image-size="40" />
            </div>
            <div v-else class="issue-stats">
              <div class="issue-count" @click="goToIssues">
                <div class="count-number">{{ issueStats.open_count }}</div>
                <div class="count-label">待处理</div>
              </div>
              <div class="issue-meta">
                <div class="meta-item">
                  <el-tag size="small" type="danger">{{ issueStats.critical_priority }} 严重</el-tag>
                </div>
                <div class="meta-item">
                  <el-tag size="small" type="warning">{{ issueStats.high_priority }} 高优</el-tag>
                </div>
              </div>
              <div class="recent-issues">
                <div
                  v-for="(issue, index) in recentIssues"
                  :key="index"
                  class="issue-item"
                  @click="goToIssue(issue.id)"
                >
                  <div class="issue-info">
                    <div class="issue-title">{{ truncateMessage(issue.title, 40) }}</div>
                    <div class="issue-meta">
                      <span>{{ issue.event_count }} 事件</span>
                      <span class="time-ago">{{ formatRelativeTime(issue.last_seen_at) }}</span>
                    </div>
                  </div>
                  <el-tag :type="getPriorityTagType(issue.priority)" size="small">
                    {{ formatPriority(issue.priority) }}
                  </el-tag>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <el-col :span="8">
        <el-card class="anomaly-card">
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

    <!-- Top Statistics -->
    <el-row :gutter="20" class="mt-4">
      <el-col :span="24">
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
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { logApi } from '../api'
import { formatNumber, formatRelativeTime, truncateMessage, formatPriority } from '../utils/formatters'
import { Warning, InfoFilled, WarningFilled, CircleCheck, ArrowRight } from '@element-plus/icons-vue'

const router = useRouter()
const apps = ref<any[]>([])
const stats = ref<any>(null)
const topTab = ref('errors')
const topOrderBy = ref('count')
const topData = ref<any[]>([])
const loadingTop = ref(false)

// Trend chart data
const dailyTrend = ref<Array<{ timestamp: number; count: number }>>([])
const hoveredBar = ref(-1)
const loadingTrend = ref(false)

// Anomaly workstation data
const newErrors = ref<any[]>([])
const recentAlerts = ref<any[]>([])
const activeSessions = ref<any[]>([])
const statsComparison = ref<any>(null)
const loadingNewErrors = ref(false)
const loadingAlerts = ref(false)
const loadingSessions = ref(false)

// Top Errors data
const topErrors = ref<any[]>([])
const loadingTopErrors = ref(false)

// Issues data
const issueStats = ref<any>({
  open_count: 0,
  resolved_count: 0,
  ignored_count: 0,
  muted_count: 0,
  total_count: 0,
  high_priority: 0,
  critical_priority: 0,
  by_status: {},
  by_priority: {},
  trend_data: []
})
const recentIssues = ref<any[]>([])
const loadingIssues = ref(false)

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
    comparison: null,
    trend: null
  },
  {
    key: 'info',
    label: '信息数',
    value: formatNumber(stats.value?.infoCount || 0),
    icon: CircleCheck,
    color: 'linear-gradient(135deg, #10b981, #059669)',
    comparison: null,
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

const maxDailyCount = computed(() => {
  const counts = dailyTrend.value.map(d => d.count)
  return counts.length > 0 ? Math.max(...counts) : 1
})

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

const currentAppId = computed(() => apps.value.length > 0 ? apps.value[0].app_id : '')

// Bar chart helpers
const getBarHeight = (count: number, max: number): number => {
  if (max === 0) return 0
  return Math.max((count / max) * 100, 5) // Minimum 5% height for visibility
}

const getBarColor = (count: number): string => {
  if (count === 0) return '#e5e7eb'
  const ratio = count / maxDailyCount.value
  if (ratio > 0.7) return 'linear-gradient(180deg, #ef4444, #dc2626)'
  if (ratio > 0.4) return 'linear-gradient(180deg, #f59e0b, #d97706)'
  return 'linear-gradient(180deg, #10b981, #059669)'
}

const formatDate = (timestamp: number): string => {
  const date = new Date(timestamp)
  return `${date.getMonth() + 1}/${date.getDate()}`
}

const formatDayLabel = (timestamp: number): string => {
  const date = new Date(timestamp)
  const days = ['日', '一', '二', '三', '四', '五', '六']
  return days[date.getDay()]
}

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

    const appId = apps.value.length > 0 ? apps.value[0].app_id : 'all'
    try {
      const statsRes = await logApi.getStats(appId)
      stats.value = statsRes.data
    } catch (statsErr) {
      stats.value = { totalEvents: 0, errorCount: 0, warnCount: 0, infoCount: 0, topErrors: [], errorTrend: [] }
    }
  } catch (error) {
    console.error('Failed to fetch overview data:', error)
  }
}

// Fetch 7-day trend data
const fetchTrendData = async () => {
  if (!currentAppId.value) return

  loadingTrend.value = true
  try {
    // Calculate 7 days ago timestamp
    const now = Date.now()
    const sevenDaysAgo = now - 7 * 24 * 60 * 60 * 1000

    // Use stats API which returns errorTrend
    const { data } = await logApi.getStats(currentAppId.value)

    // Filter error trend for last 7 days
    const trendData = data.errorTrend || []

    // Group by day
    const dailyData = new Map<string, number>()
    for (let i = 0; i < 7; i++) {
      const date = new Date(now - i * 24 * 60 * 60 * 1000)
      const dateKey = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
      dailyData.set(dateKey, 0)
    }

    // Aggregate trend data by day
    trendData.forEach((t: any) => {
      const date = new Date(t.timestamp)
      const dateKey = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
      if (dailyData.has(dateKey)) {
        dailyData.set(dateKey, (dailyData.get(dateKey) || 0) + t.count)
      }
    })

    // Convert to array and sort by date
    dailyTrend.value = Array.from(dailyData.entries())
      .map(([key, count]) => {
        const [year, month, day] = key.split('-').map(Number)
        const timestamp = new Date(year, month, day).getTime()
        return { timestamp, count }
      })
      .sort((a, b) => a.timestamp - b.timestamp)
      .slice(-7)
  } catch (error) {
    console.error('Failed to fetch trend data:', error)
    dailyTrend.value = []
  } finally {
    loadingTrend.value = false
  }
}

// Fetch Top 5 Errors
const fetchTopErrors = async () => {
  if (!currentAppId.value) return

  loadingTopErrors.value = true
  try {
    const { data } = await logApi.getIssues({
      app_id: currentAppId.value,
      sort: 'count',
      page: 1,
      page_size: 5
    })
    topErrors.value = data.data || []
  } catch (error) {
    console.error('Failed to fetch top errors:', error)
    topErrors.value = []
  } finally {
    loadingTopErrors.value = false
  }
}

// Fetch recent alerts
const fetchRecentAlerts = async () => {
  loadingAlerts.value = true
  try {
    const { data } = await logApi.getAlertTriggers({ limit: 5 })
    recentAlerts.value = data.data || []
  } catch (error) {
    console.error('Failed to fetch alert triggers:', error)
    recentAlerts.value = []
  } finally {
    loadingAlerts.value = false
  }
}

const refreshStats = () => {
  fetchData()
  fetchTrendData()
  fetchTopErrors()
  fetchRecentAlerts()
}

const goToLogs = (appId: string) => {
  router.push(`/logs/${appId}`)
}

// Anomaly workstation methods
const fetchAnomalyData = async () => {
  if (!currentAppId.value) return

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

  try {
    const { data } = await logApi.getStatsComparison({ app_id: currentAppId.value })
    statsComparison.value = data
  } catch (error) {
    console.error('Failed to fetch stats comparison:', error)
    statsComparison.value = null
  }

  fetchIssuesData()
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
const getAlertTagType = (severity: string) => {
  const types: Record<string, string> = {
    critical: 'danger',
    high: 'warning',
    medium: 'info',
    low: 'info'
  }
  return types[severity] || 'info'
}

const getAlertIconClass = (alert: any) => {
  return alert.severity === 'critical' ? 'alert-icon-critical' : 'alert-icon-warning'
}

// Issues methods
const fetchIssuesData = async () => {
  if (!currentAppId.value) return

  loadingIssues.value = true
  try {
    const { data: statsData } = await logApi.getIssueStats({ app_id: currentAppId.value })
    issueStats.value = statsData

    const { data: issuesData } = await logApi.getIssues({
      app_id: currentAppId.value,
      status: 'open',
      sort: 'last_seen',
      page: 1,
      page_size: 5
    })
    recentIssues.value = issuesData.data
  } catch (error) {
    console.error('Failed to fetch issues data:', error)
    issueStats.value = {
      open_count: 0,
      resolved_count: 0,
      ignored_count: 0,
      muted_count: 0,
      total_count: 0,
      high_priority: 0,
      critical_priority: 0,
      by_status: {},
      by_priority: {},
      trend_data: []
    }
    recentIssues.value = []
  } finally {
    loadingIssues.value = false
  }
}

const goToIssues = () => {
  router.push(`/issues/${currentAppId.value}`)
}

const goToIssue = (issueId: number) => {
  router.push(`/issues/${currentAppId.value}`)
}

const getPriorityTagType = (priority: string) => {
  const types: Record<string, string> = {
    critical: 'danger',
    high: 'warning',
    medium: 'primary',
    low: 'info'
  }
  return types[priority] || 'info'
}

onMounted(() => {
  fetchData()
  fetchTrendData()
  fetchTopErrors()
  fetchRecentAlerts()
  fetchAnomalyData()
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
  min-height: 200px;
}

.trend-card {
  min-height: 280px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.mt-4 {
  margin-top: 20px;
}

/* 7-Day Trend Chart Styles */
.trend-chart-container {
  min-height: 200px;
  padding: 20px 0;
}

.trend-chart {
  width: 100%;
  height: 200px;
}

.trend-bars {
  display: flex;
  justify-content: space-around;
  align-items: flex-end;
  height: 100%;
  padding: 0 20px;
}

.trend-bar-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  max-width: 80px;
  position: relative;
}

.trend-bar {
  width: 40px;
  border-radius: 4px 4px 0 0;
  position: relative;
  transition: height 0.3s ease, transform 0.2s ease;
  cursor: pointer;
}

.trend-bar:hover {
  transform: scaleX(1.1);
  box-shadow: 0 0 12px rgba(0, 0, 0, 0.15);
}

.bar-tooltip {
  position: absolute;
  top: -50px;
  left: 50%;
  transform: translateX(-50%);
  background: #1f2937;
  color: #fff;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
  white-space: nowrap;
  z-index: 10;
  pointer-events: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.bar-tooltip::after {
  content: '';
  position: absolute;
  bottom: -6px;
  left: 50%;
  transform: translateX(-50%);
  border-left: 6px solid transparent;
  border-right: 6px solid transparent;
  border-top: 6px solid #1f2937;
}

.tooltip-date {
  font-size: 11px;
  color: #9ca3af;
  margin-bottom: 2px;
}

.tooltip-count {
  font-weight: 600;
}

.trend-label {
  margin-top: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.trend-count {
  margin-top: 4px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

/* Ranking Card Styles */
.ranking-card {
  min-height: 320px;
}

.ranking-content {
  min-height: 240px;
}

/* Top Errors List */
.top-errors-list {
  max-height: 260px;
  overflow-y: auto;
}

.top-error-item {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.top-error-item:hover {
  background: var(--color-bg-tertiary);
}

.top-error-item:last-child {
  margin-bottom: 0;
}

.error-rank {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  background: var(--color-bg-tertiary);
  margin-right: 12px;
  flex-shrink: 0;
}

.error-rank.rank-1 {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: #fff;
}

.error-rank.rank-2 {
  background: linear-gradient(135deg, #94a3b8, #64748b);
  color: #fff;
}

.error-rank.rank-3 {
  background: linear-gradient(135deg, #b45309, #92400e);
  color: #fff;
}

.error-content {
  flex: 1;
  min-width: 0;
}

.error-title {
  font-size: 13px;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 4px;
}

.error-meta {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.error-count {
  color: #ef4444;
  font-weight: 600;
}

.error-users {
  color: #f59e0b;
}

.error-time {
  margin-left: auto;
}

.arrow-icon {
  color: var(--color-text-tertiary);
  margin-left: 8px;
}

/* Recent Alerts List */
.alerts-list {
  max-height: 260px;
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

.alert-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 12px;
  flex-shrink: 0;
}

.alert-icon-critical {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.alert-icon-warning {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.alert-content {
  flex: 1;
  min-width: 0;
}

.alert-name {
  font-size: 13px;
  color: var(--color-text);
  font-weight: 500;
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alert-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

/* Anomaly Workstation Styles */
.anomaly-card {
  height: 320px;
}

.anomaly-content {
  min-height: 200px;
}

.empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 160px;
}

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

/* Issues Styles */
.issue-stats {
  padding: 16px 0;
}

.issue-count {
  text-align: center;
  cursor: pointer;
  padding: 16px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  margin-bottom: 12px;
  transition: background 0.2s;
}

.issue-count:hover {
  background: var(--color-bg-tertiary);
}

.count-number {
  font-size: 32px;
  font-weight: 600;
  color: var(--color-danger);
  line-height: 1;
}

.count-label {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-top: 4px;
}

.issue-meta {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  padding: 0 16px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.recent-issues {
  max-height: 200px;
  overflow-y: auto;
}

.issue-item {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s;
}

.issue-item:hover {
  background: var(--color-bg-tertiary);
}

.issue-item:last-child {
  margin-bottom: 0;
}

/* Quick Actions */
.quick-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
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

/* Top Content Styles */
.top-content {
  padding: 16px 0;
}

.top-controls {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.top-list {
  max-height: 280px;
  overflow-y: auto;
}

.top-item {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
}

.top-item:last-child {
  margin-bottom: 0;
}

.top-rank {
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

.top-rank.rank-1 {
  background: linear-gradient(135deg, #f59e0b, #d97706);
  color: #fff;
}

.top-info {
  flex: 1;
  min-width: 0;
}

.top-key {
  font-size: 13px;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 4px;
}

.top-meta {
  display: flex;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.new-badge {
  padding: 2px 6px;
  background: #10b981;
  color: #fff;
  border-radius: 4px;
  font-size: 10px;
}

.top-score {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.top-item-new {
  border-left: 3px solid #10b981;
}

.text-error {
  color: #ef4444;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

/* Responsive adjustments */
@media (max-width: 1200px) {
  .anomaly-card {
    height: 280px;
  }

  .error-list,
  .alerts-list,
  .top-errors-list {
    max-height: 200px;
  }
}
</style>
