<template>
  <div class="log-table">
    <el-card class="filter-card">
      <el-form :inline="true" :model="filters" @submit.prevent="$emit('search')">
        <el-form-item label="应用">
          <el-select v-model="filters.appId" placeholder="选择应用" style="width: 180px" @change="$emit('app-change')">
            <el-option
              v-for="app in apps"
              :key="app.app_id"
              :label="app.app_id"
              :value="app.app_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Release">
          <el-select v-model="filters.release" placeholder="全部" clearable style="width: 150px" @change="$emit('release-change')">
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
            @keyup.enter="$emit('search')"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="$emit('search')" :icon="Search">搜索</el-button>
          <el-button @click="$emit('reset')">重置</el-button>
          <el-button :icon="Download" @click="$emit('export')">导出</el-button>
        </el-form-item>
        <el-form-item>
          <el-radio-group v-model="viewMode" @change="$emit('view-mode-change', $event)">
            <el-radio-button value="list">列表视图</el-radio-button>
            <el-radio-button value="clusters">聚类视图</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="table-card">
      <!-- List View -->
      <el-table
        v-if="viewMode === 'list'"
        :data="logs"
        v-loading="loading"
        stripe
        @row-click="$emit('row-click', $event)"
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
              {{ row.level?.toUpperCase() || '' }}
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

      <!-- Clusters View -->
      <el-table
        v-if="viewMode === 'clusters'"
        :data="clusters"
        v-loading="loading"
        stripe
        @row-click="$emit('cluster-click', $event)"
        style="cursor: pointer"
      >
        <el-table-column prop="fingerprint" label="指纹" width="120">
          <template #default="{ row }">
            <span class="mono-inline">{{ row.fingerprint?.substring(0, 8) }}...</span>
          </template>
        </el-table-column>
        <el-table-column prop="count" label="次数" width="80" sortable />
        <el-table-column prop="users" label="用户数" width="80" sortable />
        <el-table-column prop="message" label="错误消息" min-width="300">
          <template #default="{ row }">
            <span class="log-message">{{ truncateMessage(row.message, 80) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="urls" label="URL" width="150">
          <template #default="{ row }">
            <span v-if="row.urls?.length" class="text-secondary">{{ truncateUrl(row.urls[0]) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="releases" label="Release" width="100">
          <template #default="{ row }">
            <span v-if="row.releases?.length">{{ row.releases[0] }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="firstSeen" label="首次出现" width="170">
          <template #default="{ row }">
            {{ formatTime(row.firstSeen) }}
          </template>
        </el-table-column>
        <el-table-column prop="lastSeen" label="最近出现" width="170">
          <template #default="{ row }">
            {{ formatTime(row.lastSeen) }}
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-if="viewMode === 'list'"
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100, 200]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next"
          @size-change="$emit('page-change', { page: pagination.page, size: $event })"
          @current-change="$emit('page-change', { page: $event, size: pagination.pageSize })"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Search, Download } from '@element-plus/icons-vue'
import { formatTime, truncateMessage, getLevelTag } from '../utils/formatters'
import type { Event, QueryParams } from '../types'

interface Props {
  filters: QueryParams & { dateRange?: [number, number] }
  logs: Event[]
  clusters: any[]
  apps: any[]
  releases: string[]
  loading: boolean
  pagination: { page: number; pageSize: number; total: number }
}

defineProps<Props>()

const viewMode = ref<'list' | 'clusters'>('list')

defineEmits<{
  search: []
  reset: []
  export: []
  'app-change': []
  'release-change': []
  'view-mode-change': [mode: 'list' | 'clusters']
  'row-click': [row: Event]
  'cluster-click': [row: any]
  'page-change': [params: { page: number; size: number }]
}>()

const getTypeTag = (type: string) => {
  const map: Record<string, string> = { error: 'danger', xhr: 'warning', performance: '', info: 'info', warn: 'warning', track: 'success', console: 'info' }
  return map[type] || 'info'
}

const getTypeLabel = (type: string) => {
  const map: Record<string, string> = { error: '错误', xhr: '接口', performance: '性能', info: '信息', warn: '警告', track: '追踪', console: '控制台', breadcrumb: '操作' }
  return map[type] || type
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
</script>

<style scoped>
.log-table {
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

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.mono-inline {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #a0aec0;
}

.text-secondary {
  color: #94a3b8;
}
</style>
