<template>
  <div class="logs-page">
    <h1 class="sr-only">日志查询</h1>

    <LogTable
      :filters="filters"
      :logs="logs"
      :clusters="clusters"
      :apps="apps"
      :releases="releases"
      :loading="loading"
      :pagination="pagination"
      @search="handleSearch"
      @reset="handleReset"
      @export="handleExport"
      @app-change="handleAppChange"
      @release-change="handleReleaseChange"
      @view-mode-change="handleViewModeChange"
      @row-click="handleRowClick"
      @cluster-click="handleClusterClick"
      @page-change="handlePageChange"
    />

    <LogDetail v-model:visible="drawerVisible" :log="selectedLog" />

    <ClusterDetail
      v-model:visible="clusterDrawerVisible"
      :cluster="selectedCluster"
      :events="clusterEvents"
      :loading="loading"
      :pagination="clusterPagination"
      @event-click="handleEventClick"
      @page-change="handleClusterPageChange"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { logApi } from '../api'
import type { Event, QueryParams } from '../types'
import LogTable from '../components/LogTable.vue'
import LogDetail from '../components/LogDetail.vue'
import ClusterDetail from '../components/ClusterDetail.vue'

const route = useRoute()

const filters = ref<QueryParams & { dateRange?: [number, number] }>({
  appId: route.params.appId as string || '',
  release: '',
  env: '',
  level: '',
  type: '',
  keyword: '',
  dateRange: undefined,
  page: 1,
  pageSize: 50
})

const pagination = ref({
  page: 1,
  pageSize: 50,
  total: 0
})

const logs = ref<Event[]>([])
const apps = ref<any[]>([])
const releases = ref<string[]>([])
const loading = ref(false)
const drawerVisible = ref(false)
const selectedLog = ref<Event | null>(null)
const clusters = ref<any[]>([])
const clusterDrawerVisible = ref(false)
const selectedCluster = ref<any | null>(null)
const clusterEvents = ref<Event[]>([])
const clusterPagination = ref({ page: 1, pageSize: 50, total: 0 })

const fetchLogs = async () => {
  if (!filters.value.appId) {
    ElMessage.warning('请选择应用')
    return
  }

  loading.value = true
  try {
    const params: any = {
      appId: filters.value.appId,
      release: filters.value.release || undefined,
      env: filters.value.env || undefined,
      level: filters.value.level || undefined,
      type: filters.value.type || undefined,
      keyword: filters.value.keyword || undefined,
      page: pagination.value.page,
      pageSize: pagination.value.pageSize,
    }
    if (filters.value.dateRange && filters.value.dateRange.length === 2) {
      params.startTime = filters.value.dateRange[0]
      params.endTime = filters.value.dateRange[1]
    }
    const { data } = await logApi.query(params)
    logs.value = data.data
    pagination.value.total = data.total

    const uniqueReleases = new Set<string>()
    logs.value.forEach((log: Event) => {
      if (log.release) uniqueReleases.add(log.release)
    })
    releases.value = Array.from(uniqueReleases).sort().reverse()
  } catch (error) {
    ElMessage.error('获取日志失败')
  } finally {
    loading.value = false
  }
}

const fetchApps = async () => {
  try {
    const { data } = await logApi.getApps()
    apps.value = data
    if (!filters.value.appId && apps.value.length > 0) {
      filters.value.appId = apps.value[0].app_id
    }
  } catch (error) {
    console.error('Failed to fetch apps:', error)
  }
}

const handleSearch = () => {
  pagination.value.page = 1
  fetchLogs()
}

const handleAppChange = () => {
  filters.value.release = ''
  filters.value.env = ''
  handleSearch()
}

const handleReleaseChange = () => {
  pagination.value.page = 1
  fetchLogs()
}

const handleReset = () => {
  filters.value.release = ''
  filters.value.env = ''
  filters.value.level = ''
  filters.value.type = ''
  filters.value.keyword = ''
  filters.value.dateRange = undefined
  handleSearch()
}

const handleExport = () => {
  if (!logs.value.length) {
    ElMessage.warning('没有可导出的数据')
    return
  }

  const headers = ['时间', '级别', '类型', '消息', 'URL', '浏览器', '屏幕']
  const rows = logs.value.map(log => [
    formatTime(log.created_at),
    log.level?.toUpperCase() || '',
    log.type || '',
    `"${(log.message || '').replace(/"/g, '""')}"`,
    log.url || '',
    log.ua || '',
    log.screen || ''
  ])

  const csv = [headers.join(','), ...rows.map(r => r.join(','))].join('\n')
  const BOM = '﻿'
  const blob = new Blob([BOM + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logmonitor-${filters.value.appId}-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success(`已导出 ${logs.value.length} 条日志`)
}

const handlePageChange = ({ page, size }: { page: number; size: number }) => {
  pagination.value.page = page
  pagination.value.pageSize = size
  fetchLogs()
}

const handleRowClick = (row: Event) => {
  selectedLog.value = row
  drawerVisible.value = true
}

const handleEventClick = (row: Event) => {
  selectedLog.value = row
  drawerVisible.value = true
}

const handleViewModeChange = (mode: 'list' | 'clusters') => {
  if (mode === 'clusters') {
    fetchClusters()
  } else {
    fetchLogs()
  }
}

const fetchClusters = async () => {
  if (!filters.value.appId) {
    ElMessage.warning('请选择应用')
    return
  }

  loading.value = true
  try {
    const params: any = {
      appId: filters.value.appId,
      limit: 50
    }
    if (filters.value.dateRange && filters.value.dateRange.length === 2) {
      params.startTime = filters.value.dateRange[0]
      params.endTime = filters.value.dateRange[1]
    }
    const { data } = await logApi.getClusters(params)
    clusters.value = data.data
  } catch (error) {
    ElMessage.error('获取错误聚类失败')
  } finally {
    loading.value = false
  }
}

const handleClusterClick = async (row: any) => {
  selectedCluster.value = row
  clusterDrawerVisible.value = true
  await fetchClusterEvents(row.fingerprint)
}

const fetchClusterEvents = async (fingerprint: string) => {
  if (!filters.value.appId) return

  try {
    const params: any = {
      appId: filters.value.appId,
      fingerprint: fingerprint,
      page: clusterPagination.value.page,
      pageSize: clusterPagination.value.pageSize
    }
    const { data } = await logApi.getClusterEvents(params)
    clusterEvents.value = data.data
    clusterPagination.value.total = data.total
  } catch (error) {
    ElMessage.error('获取聚类事件失败')
  }
}

const handleClusterPageChange = ({ page, size }: { page: number; size: number }) => {
  clusterPagination.value.page = page
  clusterPagination.value.pageSize = size
  if (selectedCluster.value) {
    fetchClusterEvents(selectedCluster.value.fingerprint)
  }
}

const formatTime = (timestamp: number) => {
  return new Date(timestamp).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

onMounted(() => {
  fetchApps().then(() => {
    fetchLogs()
  })
})
</script>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
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
  border-width: 0;
}
</style>
