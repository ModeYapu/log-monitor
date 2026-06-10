<template>
  <div class="issue-detail" v-if="issue">
    <div class="detail-header">
      <div class="header-info">
        <h3>{{ issue.title }}</h3>
        <div class="meta-info">
          <el-tag :type="getStatusType(issue.status)" size="small">
            {{ formatStatus(issue.status) }}
          </el-tag>
          <el-badge :value="issue.priority" :type="getPriorityType(issue.priority)" class="priority-badge">
            <span>{{ formatPriority(issue.priority) }}</span>
          </el-badge>
          <span class="meta-item">
            <strong>First Seen:</strong> {{ formatTimestamp(issue.first_seen_at) }}
          </span>
          <span class="meta-item">
            <strong>Last Seen:</strong> {{ formatTimestamp(issue.last_seen_at) }}
          </span>
        </div>
      </div>
      <div class="header-stats">
        <div class="stat-item">
          <span class="stat-value">{{ issue.event_count.toLocaleString() }}</span>
          <span class="stat-label">Events</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ issue.user_count.toLocaleString() }}</span>
          <span class="stat-label">Users</span>
        </div>
      </div>
    </div>

    <!-- Actions -->
    <div class="actions">
      <el-button-group>
        <el-button
          v-if="issue.status === 'open'"
          type="success"
          @click="$emit('resolve', issue)"
        >
          <el-icon><Select /></el-icon>
          Resolve
        </el-button>
        <el-button
          v-if="issue.status === 'open'"
          type="warning"
          @click="$emit('ignore', issue)"
        >
          <el-icon><Close /></el-icon>
          Ignore
        </el-button>
        <el-button
          v-if="issue.status === 'resolved'"
          type="primary"
          @click="$emit('reopen', issue)"
        >
          <el-icon><RefreshRight /></el-icon>
          Reopen
        </el-button>
      </el-button-group>

      <el-dropdown @command="handlePriorityChange">
        <el-button type="primary">
          Set Priority
          <el-icon class="el-icon--right"><ArrowDown /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="critical">Critical</el-dropdown-item>
            <el-dropdown-item command="high">High</el-dropdown-item>
            <el-dropdown-item command="medium">Medium</el-dropdown-item>
            <el-dropdown-item command="low">Low</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>

    <!-- Trend Chart -->
    <div class="section">
      <h4>24h Event Trend</h4>
      <div ref="trendChart" class="trend-chart"></div>
    </div>

    <!-- Affected Users -->
    <div class="section">
      <h4>Top Affected Users</h4>
      <el-table :data="topUsers" size="small" max-height="200">
        <el-table-column prop="user_id" label="User ID" width="150" />
        <el-table-column prop="count" label="Events" width="100" align="right" />
        <el-table-column prop="last_seen" label="Last Seen" />
      </el-table>
    </div>

    <!-- Recent Events -->
    <div class="section">
      <h4>Recent Events</h4>
      <el-table :data="events" size="small" max-height="300" @row-click="handleEventClick">
        <el-table-column prop="timestamp" label="Time" width="180">
          <template #default="{ row }">
            {{ formatTimestamp(row.timestamp) }}
          </template>
        </el-table-column>
        <el-table-column prop="message" label="Message" min-width="200" show-overflow-tooltip />
        <el-table-column prop="user_id" label="User" width="120" />
        <el-table-column prop="url" label="URL" min-width="150" show-overflow-tooltip />
      </el-table>
    </div>
  </div>
  <div v-else class="loading-placeholder">
    <el-skeleton :rows="5" animated />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { Select, Close, RefreshRight, ArrowDown } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import type { Issue } from '../types'
import { formatTimestamp, formatPriority, formatStatus } from '../utils/formatters'

const props = defineProps<{
  issue: Issue
  events: any[]
  loading: boolean
}>()

const emit = defineEmits<{
  resolve: [issue: Issue]
  ignore: [issue: Issue]
  reopen: [issue: Issue]
  'priority-change': [issue: Issue, priority: string]
}>()

const trendChart = ref<HTMLElement>()
let chartInstance: echarts.ECharts | null = null

// Compute top affected users from events
const topUsers = computed(() => {
  const userMap = new Map<string, { count: number; last_seen: number }>()

  props.events.forEach(event => {
    if (event.user_id) {
      const current = userMap.get(event.user_id) || { count: 0, last_seen: 0 }
      userMap.set(event.user_id, {
        count: current.count + 1,
        last_seen: Math.max(current.last_seen, event.timestamp)
      })
    }
  })

  return Array.from(userMap.entries())
    .map(([user_id, data]) => ({ user_id, ...data }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 10)
})

const handlePriorityChange = (priority: string) => {
  emit('priority-change', props.issue, priority)
}

const handleEventClick = (row: any) => {
  // Could open event detail modal here
  console.log('Event clicked:', row)
}

// Initialize trend chart
const initTrendChart = () => {
  if (!trendChart.value) return

  chartInstance = echarts.init(trendChart.value)

  // Generate sample trend data (24 hours)
  const now = Date.now()
  const trendData = []
  for (let i = 23; i >= 0; i--) {
    const timestamp = now - i * 3600 * 1000
    const count = Math.floor(Math.random() * 20) + 1
    trendData.push([timestamp, count])
  }

  const option = {
    tooltip: {
      trigger: 'axis',
      formatter: (params: any) => {
        const point = params[0]
        const time = new Date(point.value[0]).toLocaleTimeString()
        return `${time}<br/>Events: ${point.value[1]}`
      }
    },
    xAxis: {
      type: 'time',
      axisLabel: {
        formatter: (value: number) => {
          const date = new Date(value)
          return date.getHours() + ':00'
        }
      }
    },
    yAxis: {
      type: 'value',
      name: 'Events'
    },
    series: [{
      data: trendData,
      type: 'line',
      smooth: true,
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [{
            offset: 0, color: 'rgba(64, 158, 255, 0.3)'
          }, {
            offset: 1, color: 'rgba(64, 158, 255, 0.05)'
          }]
        }
      },
      lineStyle: {
        color: '#409EFF',
        width: 2
      }
    }]
  }

  chartInstance.setOption(option)
}

// Helper functions
const getStatusType = (status: string) => {
  const types: Record<string, any> = {
    open: 'danger',
    resolved: 'success',
    ignored: 'info',
    muted: 'warning'
  }
  return types[status] || 'info'
}

const getPriorityType = (priority: string) => {
  const types: Record<string, any> = {
    critical: 'danger',
    high: 'warning',
    medium: 'primary',
    low: 'info'
  }
  return types[priority] || 'info'
}

// Lifecycle
onMounted(() => {
  initTrendChart()
})

watch(() => props.issue, () => {
  // Refresh chart when issue changes
  if (chartInstance) {
    chartInstance.dispose()
    initTrendChart()
  }
})
</script>

<style scoped>
.issue-detail {
  padding: 20px;
}

.detail-header {
  margin-bottom: 20px;
  padding-bottom: 20px;
  border-bottom: 1px solid #EBEEF5;
}

.header-info h3 {
  margin: 0 0 10px 0;
  font-size: 18px;
  color: #303133;
  line-height: 1.4;
}

.meta-info {
  display: flex;
  align-items: center;
  gap: 15px;
  flex-wrap: wrap;
}

.meta-item {
  color: #606266;
  font-size: 13px;
}

.header-stats {
  display: flex;
  gap: 30px;
  margin-top: 15px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #409EFF;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-top: 5px;
}

.actions {
  display: flex;
  gap: 10px;
  margin-bottom: 30px;
}

.section {
  margin-bottom: 30px;
}

.section h4 {
  margin: 0 0 15px 0;
  font-size: 14px;
  color: #606266;
  font-weight: 600;
}

.trend-chart {
  height: 200px;
  border: 1px solid #EBEEF5;
  border-radius: 4px;
  padding: 10px;
}

.priority-badge {
  font-weight: 600;
}

.loading-placeholder {
  padding: 20px;
}

/* Row click highlight */
:deep(.el-table__row) {
  cursor: pointer;
}

:deep(.el-table__row:hover) {
  background-color: #F5F7FA;
}
</style>