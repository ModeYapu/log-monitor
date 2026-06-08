<template>
  <!-- Cluster Detail Drawer -->
  <el-drawer
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    title="错误聚类详情"
    size="70%"
    direction="rtl"
  >
    <template #extra>
      <el-button type="primary" :icon="DocumentCopy" @click="copyClusterInfo">
        复制聚类信息
      </el-button>
    </template>
    <div v-if="cluster" class="drawer-content">
      <div class="detail-section">
        <h4>聚类信息</h4>
        <div class="info-list">
          <div class="info-item"><span class="label">指纹:</span> <span class="mono-inline">{{ cluster.fingerprint }}</span></div>
          <div class="info-item"><span class="label">错误次数:</span> <span>{{ cluster.count }}</span></div>
          <div class="info-item"><span class="label">影响用户:</span> <span>{{ cluster.users }}</span></div>
          <div class="info-item"><span class="label">首次出现:</span> <span>{{ formatTime(cluster.firstSeen) }}</span></div>
          <div class="info-item"><span class="label">最近出现:</span> <span>{{ formatTime(cluster.lastSeen) }}</span></div>
          <div class="info-item"><span class="label">URL:</span> <span>{{ cluster.urls?.join(', ') || '-' }}</span></div>
          <div class="info-item"><span class="label">Release:</span> <span>{{ cluster.releases?.join(', ') || '-' }}</span></div>
        </div>
      </div>

      <div class="detail-section">
        <h4>错误消息</h4>
        <pre class="mono">{{ cluster.message }}</pre>
      </div>

      <div class="detail-section">
        <h4>事件列表</h4>
        <el-table
          :data="events"
          v-loading="loading"
          stripe
          size="small"
          @row-click="$emit('event-click', $event)"
          style="cursor: pointer"
        >
          <el-table-column prop="created_at" label="时间" width="160">
            <template #default="{ row }">
              {{ formatTime(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column prop="message" label="消息" min-width="300">
            <template #default="{ row }">
              <span class="log-message">{{ truncateMessage(row.message, 60) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="url" label="URL" width="150">
            <template #default="{ row }">
              <span class="text-secondary">{{ truncateUrl(row.url) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="user_id" label="用户" width="100" />
        </el-table>

        <div class="pagination">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :page-sizes="[20, 50, 100]"
            :total="pagination.total"
            layout="total, sizes, prev, pager, next"
            @size-change="$emit('page-change', { page: pagination.page, size: $event })"
            @current-change="$emit('page-change', { page: $event, size: pagination.pageSize })"
          />
        </div>
      </div>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { DocumentCopy } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import { truncateMessage } from '../utils/formatters'
import type { Event } from '../types'

interface Props {
  visible: boolean
  cluster: any
  events: Event[]
  loading: boolean
  pagination: { page: number; pageSize: number; total: number }
}

defineProps<Props>()

defineEmits<{
  'update:visible': [value: boolean]
  'event-click': [event: Event]
  'page-change': [params: { page: number; size: number }]
}>()

const formatTime = (timestamp: number) => {
  return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss')
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

const copyClusterInfo = () => {
  const cluster = props.cluster
  let text = `Error Cluster:\nFingerprint: ${cluster.fingerprint}\nCount: ${cluster.count}\nUsers: ${cluster.users}\nMessage: ${cluster.message}\nFirst Seen: ${new Date(cluster.firstSeen).toISOString()}\nLast Seen: ${new Date(cluster.lastSeen).toISOString()}\nURLs: ${cluster.urls?.join(', ') || 'N/A'}\nReleases: ${cluster.releases?.join(', ') || 'N/A'}\n`

  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}
</script>

<style scoped>
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
  margin: 0;
  white-space: pre-wrap;
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

.mono-inline {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #a0aec0;
  word-break: break-all;
}

.text-secondary {
  color: #94a3b8;
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
</style>
