<template>
  <div class="logs-page">
    <h1 class="sr-only">日志查询</h1>
    <el-card class="filter-card">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="应用">
          <el-select v-model="filters.appId" placeholder="选择应用" style="width: 180px" @change="handleAppChange">
            <el-option
              v-for="app in apps"
              :key="app.app_id"
              :label="app.app_id"
              :value="app.app_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Release">
          <el-select v-model="filters.release" placeholder="全部" clearable style="width: 150px" @change="handleReleaseChange">
            <el-option
              v-for="rel in releases"
              :key="rel"
              :label="rel"
              :value="rel"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="环境">
          <el-select v-model="filters.env" placeholder="全部" clearable style="width: 120px">
            <el-option label="生产" value="production" />
            <el-option label="预发" value="staging" />
            <el-option label="开发" value="development" />
            <el-option label="测试" value="test" />
          </el-select>
        </el-form-item>
        <el-form-item label="级别">
          <el-select v-model="filters.level" placeholder="全部" clearable style="width: 120px">
            <el-option label="Error" value="error" />
            <el-option label="Warn" value="warn" />
            <el-option label="Info" value="info" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="filters.type" placeholder="全部" clearable style="width: 140px">
            <el-option label="错误" value="error" />
            <el-option label="接口请求" value="xhr" />
            <el-option label="性能" value="performance" />
            <el-option label="信息" value="info" />
            <el-option label="警告" value="warn" />
            <el-option label="追踪" value="track" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-date-picker
            v-model="filters.dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            format="YYYY-MM-DD HH:mm"
            value-format="x"
            style="width: 360px"
          />
        </el-form-item>
        <el-form-item label="关键词">
          <el-input
            v-model="filters.keyword"
            placeholder="搜索消息内容"
            clearable
            style="width: 200px"
            @keyup.enter="handleSearch"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch" :icon="Search">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
          <el-button :icon="Download" @click="handleExport">导出</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="table-card">
      <el-table
        :data="logs"
        v-loading="loading"
        stripe
        @row-click="handleRowClick"
        style="cursor: pointer"
      >
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="level" label="级别" width="80">
          <template #default="{ row }">
            <el-tag :type="getLevelTag(row.level)" size="small">
              {{ row.level.toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="getTypeTag(row.type)" size="small">
              {{ getTypeLabel(row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="message" label="消息" min-width="300">
          <template #default="{ row }">
            <span class="log-message">{{ truncateMessage(row.message, 100) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="url" label="来源" width="150">
          <template #default="{ row }">
            <span class="text-secondary">{{ truncateUrl(row.url) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="ua" label="浏览器" width="120">
          <template #default="{ row }">
            {{ parseUA(row.ua) }}
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100, 200]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- Detail Drawer -->
    <el-drawer
      v-model="drawerVisible"
      title="日志详情"
      size="600px"
      direction="rtl"
    >
      <template #extra>
        <el-button type="primary" :icon="DocumentCopy" @click="copyErrorInfo">
          复制
        </el-button>
      </template>
      <div v-if="selectedLog" class="drawer-content">
        <div class="detail-section">
          <h4>错误信息</h4>
          <pre class="mono">{{ selectedLog.message }}</pre>
        </div>

        <div class="detail-section" v-if="selectedLog.stack">
          <h4>堆栈跟踪</h4>
          <pre class="mono">{{ selectedLog.stack }}</pre>
        </div>

        <div class="detail-section">
          <h4>标签</h4>
          <div v-if="Object.keys(parsedTags(selectedLog)).length > 0" class="key-value-list">
            <div v-for="(value, key) in parsedTags(selectedLog)" :key="key" class="key-value-item">
              <span class="key">{{ key }}:</span>
              <span class="value">{{ value }}</span>
            </div>
          </div>
          <p v-else class="empty">无标签</p>
        </div>

        <div class="detail-section">
          <h4>额外数据</h4>
          <pre v-if="selectedLog.extra && selectedLog.extra !== '{}'" class="mono">{{ formatJson(selectedLog.extra) }}</pre>
          <p v-else class="empty">无额外数据</p>
        </div>

        <div class="detail-section">
          <h4>环境信息</h4>
          <div class="info-list">
            <div class="info-item"><span class="label">URL:</span> <span>{{ selectedLog.url || '-' }}</span></div>
            <div class="info-item"><span class="label">位置:</span> <span>{{ selectedLog.line }}:{{ selectedLog.col }}</span></div>
            <div class="info-item"><span class="label">Release:</span> <span>{{ selectedLog.release || '-' }}</span></div>
            <div class="info-item"><span class="label">环境:</span> <span>{{ selectedLog.env || '-' }}</span></div>
            <div class="info-item"><span class="label">用户ID:</span> <span>{{ selectedLog.user_id || '-' }}</span></div>
            <div class="info-item"><span class="label">会话ID:</span> <span class="mono-inline">{{ selectedLog.session_id || '-' }}</span></div>
            <div class="info-item"><span class="label">浏览器:</span> <span>{{ selectedLog.ua || '-' }}</span></div>
            <div class="info-item"><span class="label">屏幕尺寸:</span> <span>{{ selectedLog.screen || '-' }}</span></div>
            <div class="info-item"><span class="label">视口:</span> <span>{{ selectedLog.viewport || '-' }}</span></div>
          </div>
        </div>

        <div class="detail-section" v-if="selectedLog.performance && selectedLog.performance !== '{}'">
          <h4>性能数据</h4>
          <pre class="mono">{{ formatJson(selectedLog.performance) }}</pre>
        </div>

        <div class="detail-section" v-if="xhrData">
          <h4>接口请求详情</h4>
          <div class="info-list">
            <div class="info-item"><span class="label">方法:</span> <span class="badge">{{ xhrData.method }}</span></div>
            <div class="info-item"><span class="label">地址:</span> <span class="mono-inline">{{ xhrData.url }}</span></div>
            <div class="info-item"><span class="label">状态:</span> <span :class="xhrData.status >= 400 ? 'text-error' : 'text-success'">{{ xhrData.status }} {{ xhrData.statusText }}</span></div>
            <div class="info-item"><span class="label">耗时:</span> <span>{{ xhrData.duration }}ms</span></div>
          </div>
          <div v-if="xhrData.requestBody" class="xhr-body">
            <h5>请求体</h5>
            <pre class="mono">{{ formatJson(xhrData.requestBody) }}</pre>
          </div>
          <div v-if="xhrData.responseBody" class="xhr-body">
            <h5>响应体</h5>
            <pre class="mono">{{ formatJson(xhrData.responseBody) }}</pre>
          </div>
        </div>

        <div class="detail-section" v-if="breadcrumbs.length > 0">
          <h4>用户操作轨迹（面包屑）</h4>
          <div class="breadcrumb-timeline">
            <div v-for="(crumb, idx) in breadcrumbs" :key="idx" class="breadcrumb-item" :class="'crumb-' + crumb.type">
              <span class="crumb-icon">{{ getBreadcrumbIcon(crumb.type) }}</span>
              <span class="crumb-time">{{ formatBreadcrumbTime(crumb.timestamp) }}</span>
              <span class="crumb-text">{{ crumb.message }}</span>
            </div>
          </div>
        </div>

        <div class="detail-section" v-if="selectedLog.screenshot_url">
          <h4>错误截图</h4>
          <div class="screenshot-container">
            <el-image
              :src="getScreenshotUrl(selectedLog.screenshot_url)"
              fit="contain"
              :preview-src-list="[getScreenshotUrl(selectedLog.screenshot_url)]"
              preview-teleported
            >
              <template #error>
                <div class="image-error">
                  <el-icon><Picture /></el-icon>
                  <span>截图加载失败</span>
                </div>
              </template>
            </el-image>
          </div>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Search, Picture, DocumentCopy, Download } from '@element-plus/icons-vue'
import { logApi } from '../api'
import { formatTime, truncateMessage, getLevelTag } from '../utils/formatters'
import type { Event, QueryParams } from '../types'

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

const parsedTags = (row: Event) => {
  try {
    return JSON.parse(row.tags || '{}')
  } catch {
    return {}
  }
}

const xhrData = computed(() => {
  if (!selectedLog.value) return null
  try {
    const extra = JSON.parse(selectedLog.value.extra || '{}')
    if (extra.xhr) return extra.xhr
    return null
  } catch {
    return null
  }
})

const breadcrumbs = computed(() => {
  if (!selectedLog.value) return []
  try {
    const extra = JSON.parse(selectedLog.value.extra || '{}')
    if (extra.breadcrumbs && Array.isArray(extra.breadcrumbs)) {
      return extra.breadcrumbs
    }
    return []
  } catch {
    return []
  }
})

const getTypeTag = (type: string) => {
  const map: Record<string, string> = { error: 'danger', xhr: 'warning', performance: '', info: 'info', warn: 'warning', track: 'success', console: 'info' }
  return map[type] || 'info'
}

const getTypeLabel = (type: string) => {
  const map: Record<string, string> = { error: '错误', xhr: '接口', performance: '性能', info: '信息', warn: '警告', track: '追踪', console: '控制台', breadcrumb: '操作' }
  return map[type] || type
}

const getBreadcrumbIcon = (type: string) => {
  const map: Record<string, string> = { click: '👆', navigation: '🔗', xhr: '🌐', console: '🖥️', custom: '⭐', error: '❌' }
  return map[type] || '📌'
}

const formatBreadcrumbTime = (ts: number) => {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString()
}

const formatJson = (jsonStr: string) => {
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2)
  } catch {
    return jsonStr
  }
}

const truncateUrl = (url: string) => {
  if (!url) return '-'
  try {
    const u = new URL(url)
    return u.pathname + u.search
  } catch {
    return url.substring(0, 30)
  }
}

const parseUA = (ua: string) => {
  if (!ua) return '-'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Firefox')) return 'Firefox'
  if (ua.includes('Safari')) return 'Safari'
  if (ua.includes('Edge')) return 'Edge'
  return 'Other'
}

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

    // Extract unique releases from current result
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
  // Reset release and env when app changes
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
  const BOM = '\uFEFF'
  const blob = new Blob([BOM + csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logmonitor-${filters.value.appId}-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  URL.revokeObjectURL(url)
  ElMessage.success(`已导出 ${logs.value.length} 条日志`)
}

const handlePageChange = (page: number) => {
  pagination.value.page = page
  fetchLogs()
}

const handleSizeChange = (size: number) => {
  pagination.value.pageSize = size
  pagination.value.page = 1
  fetchLogs()
}

const handleRowClick = (row: Event) => {
  selectedLog.value = row
  drawerVisible.value = true
}

const copyErrorInfo = () => {
  if (!selectedLog.value) return

  const log = selectedLog.value
  let text = `Error: ${log.message}\nType: ${log.type}\nLevel: ${log.level}\nURL: ${log.url}\nLine: ${log.line}:${log.col}\nUser Agent: ${log.ua}\nScreen: ${log.screen}\nViewport: ${log.viewport}\n`

  if (xhrData.value) {
    text += `\nXHR Request:\n  ${xhrData.value.method} ${xhrData.value.url}\n  Status: ${xhrData.value.status} ${xhrData.value.statusText}\n  Duration: ${xhrData.value.duration}ms\n`
    if (xhrData.value.requestBody) text += `  Request: ${xhrData.value.requestBody}\n`
    if (xhrData.value.responseBody) text += `  Response: ${xhrData.value.responseBody}\n`
  }

  if (breadcrumbs.value.length > 0) {
    text += `\nBreadcrumbs:\n`
    for (const b of breadcrumbs.value) {
      text += `  [${new Date(b.timestamp).toLocaleTimeString()}] ${b.type}: ${b.message}\n`
    }
  }

  text += `\nStack Trace:\n${log.stack || '(none)'}\n\nTags:\n${JSON.stringify(parsedTags(log), null, 2)}\n\nExtra:\n${log.extra || '(none)'}`

  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

const getScreenshotUrl = (url: string) => {
  if (!url) return ''
  const token = localStorage.getItem('logmon_token')
  if (url.startsWith('/api/')) {
    const screenshotUrl = new URL(window.location.protocol + '//' + window.location.hostname + ':9200' + url)
    if (token) {
      screenshotUrl.searchParams.set('token', token)
    }
    return screenshotUrl.toString()
  }
  return url
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

.filter-card {
  margin-bottom: 20px;
}

.table-card {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.table-card :deep(.el-card__body) {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.el-table {
  flex: 1;
}

.log-message {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #e0e6ed;
}

.drawer-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-section h4 {
  color: #94a3b8;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin: 0;
  font-weight: 600;
}

.detail-section pre.mono {
  background: #131829;
  padding: 12px;
  border-radius: 6px;
  font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
  font-size: 12px;
  color: #e0e6ed;
  overflow-x: auto;
  max-height: 300px;
  overflow-y: auto;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.key-value-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.key-value-item {
  display: flex;
  gap: 8px;
  font-size: 13px;
}

.key-value-item .key {
  color: #60a5fa;
  font-weight: 500;
  min-width: 100px;
}

.key-value-item .value {
  color: #e0e6ed;
  word-break: break-word;
}

.info-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-item {
  display: flex;
  gap: 8px;
  font-size: 13px;
}

.info-item .label {
  color: #94a3b8;
  min-width: 80px;
}

.info-item span:not(.label) {
  color: #e0e6ed;
  word-break: break-all;
}

.empty {
  color: #64748b;
  font-size: 13px;
  margin: 0;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.screenshot-container {
  background: #131829;
  border-radius: 8px;
  overflow: hidden;
  max-width: 100%;
}

.screenshot-container :deep(.el-image) {
  width: 100%;
  max-height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.screenshot-container :deep(.el-image__inner) {
  max-width: 100%;
  max-height: 400px;
}

.image-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: #94a3b8;
  gap: 8px;
}

.image-error .el-icon {
  font-size: 32px;
}

.xhr-body {
  margin-top: 12px;
}

.xhr-body h5 {
  color: #94a3b8;
  font-size: 12px;
  margin: 0 0 8px 0;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  background: #2d3748;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  color: #a0aec0;
}

.mono-inline {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #a0aec0;
  word-break: break-all;
}

.text-success {
  color: #10b981;
}

.text-error {
  color: #ef4444;
}

.breadcrumb-timeline {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 300px;
  overflow-y: auto;
}

.breadcrumb-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 13px;
  padding: 6px 8px;
  background: #131829;
  border-radius: 6px;
  border-left: 3px solid #4a5568;
}

.breadcrumb-item.crumb-click { border-left-color: #6366f1; }
.breadcrumb-item.crumb-navigation { border-left-color: #10b981; }
.breadcrumb-item.crumb-xhr { border-left-color: #f59e0b; }
.breadcrumb-item.crumb-error { border-left-color: #ef4444; }

.crumb-icon {
  flex-shrink: 0;
  font-size: 14px;
}

.crumb-time {
  color: #64748b;
  font-size: 11px;
  flex-shrink: 0;
  min-width: 70px;
}

.crumb-text {
  color: #e0e6ed;
  word-break: break-all;
}
</style>
