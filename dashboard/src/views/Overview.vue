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
            </div>
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

const statsCards = computed(() => [
  {
    key: 'total',
    label: '总事件数',
    value: formatNumber(stats.value?.totalEvents || 0),
    icon: InfoFilled,
    color: 'linear-gradient(135deg, #3b82f6, #1d4ed8)'
  },
  {
    key: 'errors',
    label: '错误数',
    value: formatNumber(stats.value?.errorCount || 0),
    icon: WarningFilled,
    color: 'linear-gradient(135deg, #ef4444, #dc2626)'
  },
  {
    key: 'warnings',
    label: '警告数',
    value: formatNumber(stats.value?.warnCount || 0),
    icon: Warning,
    color: 'linear-gradient(135deg, #f59e0b, #d97706)'
  },
  {
    key: 'info',
    label: '信息数',
    value: formatNumber(stats.value?.infoCount || 0),
    icon: CircleCheck,
    color: 'linear-gradient(135deg, #10b981, #059669)'
  }
])

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

onMounted(() => {
  fetchData()
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
</style>
