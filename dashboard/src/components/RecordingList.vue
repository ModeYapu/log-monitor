<template>
  <div class="recording-list">
    <div class="list-header">
      <div class="header-left">
        <el-input
          v-model="filterSearch"
          placeholder="搜索会话ID或URL..."
          :prefix-icon="Search"
          clearable
          style="width: 200px"
        />
        <el-select
          v-model="filterAppId"
          placeholder="应用筛选"
          clearable
          style="width: 150px"
        >
          <el-option
            v-for="app in apps"
            :key="app.app_id"
            :label="app.app_id"
            :value="app.app_id"
          />
        </el-select>
        <el-date-picker
          v-model="filterDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          clearable
          style="width: 240px"
          :default-time="defaultTime"
        />
        <el-select
          v-model="filterStatus"
          placeholder="状态筛选"
          clearable
          style="width: 120px"
        >
          <el-option label="全部" value="" />
          <el-option label="录制中" value="recording" />
          <el-option label="已完成" value="completed" />
          <el-option label="错误" value="error" />
        </el-select>
        <el-button @click="resetFilters" :icon="Delete" circle title="重置筛选" />
      </div>
      <div class="header-right">
        <el-button :icon="Refresh" @click="$emit('refresh')" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-table :data="paginatedRecordings" v-loading="loading" stripe>
      <el-table-column prop="sessionId" label="会话ID" width="200">
        <template #default="{ row }">
          <span class="session-id">{{ row.sessionId }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="appId" label="应用" width="150" />
      <el-table-column prop="url" label="页面URL" min-width="200">
        <template #default="{ row }">
          {{ formatUrl(row.url) }}
        </template>
      </el-table-column>
      <el-table-column prop="durationMs" label="时长" width="100">
        <template #default="{ row }">
          {{ formatDuration(row.durationMs) }}
        </template>
      </el-table-column>
      <el-table-column prop="eventCount" label="事件数" width="80" />
      <el-table-column prop="startTime" label="开始时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.startTime) }}
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" :icon="VideoPlay" @click="$emit('play', row)" size="small">
            回放
          </el-button>
          <el-popconfirm title="确认删除此录制？" @confirm="$emit('delete', row)">
            <template #reference>
              <el-button type="danger" :icon="Delete" size="small" />
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-if="total > 0"
      v-model:current-page="page"
      v-model:page-size="pageSize"
      :total="total"
      :page-sizes="[10, 20, 50]"
      layout="total, sizes, prev, pager, next"
      @size-change="$emit('refresh')"
      @current-change="$emit('refresh')"
      class="pagination"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Search, Refresh, VideoPlay, Delete } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import type { Recording } from '../types'

interface Props {
  recordings: Recording[]
  apps: any[]
  loading: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  refresh: []
  play: [recording: Recording]
  delete: [recording: Recording]
}>()

// Filter states
const filterSearch = ref('')
const filterAppId = ref('')
const filterStatus = ref('')
const filterDateRange = ref<[Date, Date] | null>(null)
const page = ref(1)
const pageSize = ref(20)
const defaultTime: [Date, Date] = [
  new Date(2000, 1, 1, 0, 0, 0),
  new Date(2000, 1, 1, 23, 59, 59)
]

const filteredRecordings = computed(() => {
  let result = props.recordings

  if (filterSearch.value) {
    const search = filterSearch.value.toLowerCase()
    result = result.filter(r =>
      r.sessionId.toLowerCase().includes(search) ||
      (r.url && r.url.toLowerCase().includes(search))
    )
  }

  if (filterAppId.value) {
    result = result.filter(r => r.appId === filterAppId.value)
  }

  if (filterStatus.value) {
    result = result.filter(r => r.status === filterStatus.value)
  }

  if (filterDateRange.value && filterDateRange.value.length === 2) {
    const [start, end] = filterDateRange.value
    const startDate = dayjs(start).startOf('day').valueOf()
    const endDate = dayjs(end).endOf('day').valueOf()
    result = result.filter(r => {
      const startTime = r.startTime || 0
      return startTime >= startDate && startTime <= endDate
    })
  }

  return result
})

const paginatedRecordings = computed(() => {
  const start = (page.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredRecordings.value.slice(start, end)
})

const total = computed(() => filteredRecordings.value.length)

watch([filterSearch, filterAppId, filterStatus, filterDateRange], () => {
  page.value = 1
})

function formatUrl(url: string): string {
  try {
    const u = new URL(url)
    return u.pathname + u.search
  } catch {
    return url
  }
}

function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
}

function formatTime(timestamp: number): string {
  return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss')
}

function getStatusType(status: string): string {
  switch (status) {
    case 'recording': return 'warning'
    case 'completed': return 'success'
    case 'error': return 'danger'
    default: return 'info'
  }
}

function getStatusText(status: string): string {
  switch (status) {
    case 'recording': return '录制中'
    case 'completed': return '已完成'
    case 'error': return '错误'
    default: return status
  }
}

function resetFilters() {
  filterSearch.value = ''
  filterAppId.value = ''
  filterStatus.value = ''
  filterDateRange.value = null
  page.value = 1
}
</script>

<style scoped>
.recording-list {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.list-header {
  padding: 16px;
  border: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-left {
  display: flex;
  gap: 12px;
  align-items: center;
}

.header-right {
  display: flex;
  gap: 8px;
}

.session-id {
  font-family: monospace;
  font-size: 14px;
  color: var(--color-text-secondary);
}

.pagination {
  padding: 16px;
  border: 1px solid var(--color-border);
  display: flex;
  justify-content: center;
}
</style>
