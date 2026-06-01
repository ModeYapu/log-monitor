<template>
  <div class="logs-page">
    <h1 class="sr-only">日志查询</h1>
    <el-card class="filter-card">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item label="应用">
          <el-select v-model="filters.appId" placeholder="选择应用" style="width: 180px">
            <el-option
              v-for="app in apps"
              :key="app.app_id"
              :label="app.app_id"
              :value="app.app_id"
            />
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
            <el-option label="性能" value="performance" />
            <el-option label="信息" value="info" />
            <el-option label="警告" value="warn" />
            <el-option label="追踪" value="track" />
          </el-select>
        </el-form-item>
        <el-form-item label="关键词">
          <el-input
            v-model="filters.keyword"
            placeholder="搜索消息内容"
            clearable
            style="width: 200px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch" :icon="Search">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
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
        <el-table-column prop="type" label="类型" width="100" />
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
            <div class="info-item"><span class="label">浏览器:</span> <span>{{ selectedLog.ua || '-' }}</span></div>
            <div class="info-item"><span class="label">屏幕尺寸:</span> <span>{{ selectedLog.screen || '-' }}</span></div>
            <div class="info-item"><span class="label">视口:</span> <span>{{ selectedLog.viewport || '-' }}</span></div>
          </div>
        </div>

        <div class="detail-section" v-if="selectedLog.performance && selectedLog.performance !== '{}'">
          <h4>性能数据</h4>
          <pre class="mono">{{ formatJson(selectedLog.performance) }}</pre>
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
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Search, Picture, DocumentCopy } from '@element-plus/icons-vue'
import { logApi } from '../api'
import { formatTime, truncateMessage, getLevelTag } from '../utils/formatters'
import type { Event, QueryParams } from '../types'

const route = useRoute()

const filters = ref<QueryParams>({
  appId: route.params.appId as string || '',
  level: '',
  type: '',
  keyword: '',
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
    const params = {
      ...filters.value,
      page: pagination.value.page,
      pageSize: pagination.value.pageSize
    }
    const { data } = await logApi.query(params)
    logs.value = data.data
    pagination.value.total = data.total
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

const handleReset = () => {
  filters.value.level = ''
  filters.value.type = ''
  filters.value.keyword = ''
  handleSearch()
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
  const text = `Error: ${log.message}
Type: ${log.type}
Level: ${log.level}
URL: ${log.url}
Line: ${log.line}:${log.col}
User Agent: ${log.ua}
Screen: ${log.screen}
Viewport: ${log.viewport}

Stack Trace:
${log.stack || '(none)'}

Tags:
${JSON.stringify(parsedTags(log), null, 2)}

Extra:
${log.extra || '(none)'}`

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
</style>
