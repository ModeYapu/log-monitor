<template>
  <div class="logs-page">
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
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="log-detail">
              <el-row :gutter="20">
                <el-col :span="12">
                  <div class="detail-section">
                    <h4>错误信息</h4>
                    <pre>{{ row.message }}</pre>
                  </div>
                </el-col>
                <el-col :span="12">
                  <div class="detail-section">
                    <h4>堆栈跟踪</h4>
                    <pre v-if="row.stack">{{ row.stack }}</pre>
                    <p v-else class="text-secondary">无堆栈信息</p>
                  </div>
                </el-col>
              </el-row>
              <el-row :gutter="20" class="mt-4">
                <el-col :span="12">
                  <div class="detail-section">
                    <h4>标签</h4>
                    <div v-if="parsedTags(row).length > 0" class="tags">
                      <el-tag
                        v-for="(value, key) in parsedTags(row)"
                        :key="key"
                        size="small"
                        class="mr-1"
                      >
                        {{ key }}: {{ value }}
                      </el-tag>
                    </div>
                    <p v-else class="text-secondary">无标签</p>
                  </div>
                </el-col>
                <el-col :span="12">
                  <div class="detail-section">
                    <h4>额外数据</h4>
                    <pre v-if="row.extra">{{ formatJson(row.extra) }}</pre>
                    <p v-else class="text-secondary">无额外数据</p>
                  </div>
                </el-col>
              </el-row>
              <el-row :gutter="20" class="mt-4">
                <el-col :span="12">
                  <div class="detail-section">
                    <h4>环境信息</h4>
                    <p><strong>URL:</strong> {{ row.url }}</p>
                    <p><strong>位置:</strong> {{ row.line }}:{{ row.col }}</p>
                    <p><strong>屏幕:</strong> {{ row.screen }}</p>
                    <p><strong>视口:</strong> {{ row.viewport }}</p>
                  </div>
                </el-col>
                <el-col :span="12">
                  <div class="detail-section">
                    <h4>性能数据</h4>
                    <pre v-if="row.performance && row.performance !== '{}'">{{ formatJson(row.performance) }}</pre>
                    <p v-else class="text-secondary">无性能数据</p>
                  </div>
                </el-col>
              </el-row>
              <el-row :gutter="20" class="mt-4" v-if="row.screenshot_url">
                <el-col :span="24">
                  <div class="detail-section">
                    <h4>错误截图</h4>
                    <div class="screenshot-container">
                      <el-image
                        :src="getScreenshotUrl(row.screenshot_url)"
                        fit="contain"
                        :preview-src-list="[getScreenshotUrl(row.screenshot_url)]"
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
                </el-col>
              </el-row>
            </div>
          </template>
        </el-table-column>
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Search, Picture } from '@element-plus/icons-vue'
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
  console.log('Row clicked:', row)
}

const getScreenshotUrl = (url: string) => {
  if (!url) return ''
  // If it's a relative path, prepend the API base URL
  if (url.startsWith('/api/')) {
    return window.location.protocol + '//' + window.location.hostname + ':9200' + url
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

.log-detail {
  padding: 20px;
  background: #0a0e27;
}

.detail-section h4 {
  color: #94a3b8;
  margin-bottom: 10px;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.detail-section pre {
  background: #131829;
  padding: 12px;
  border-radius: 6px;
  font-size: 12px;
  color: #e0e6ed;
  overflow-x: auto;
  max-height: 200px;
  overflow-y: auto;
}

.detail-section p {
  color: #94a3b8;
  font-size: 13px;
  margin: 4px 0;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.mr-1 {
  margin-right: 4px;
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
