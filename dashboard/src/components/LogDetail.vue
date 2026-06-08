<template>
  <!-- Detail Drawer -->
  <el-drawer
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    title="日志详情"
    size="700px"
    direction="rtl"
  >
    <template #extra>
      <el-button type="primary" :icon="DocumentCopy" @click="copyErrorInfo">
        复制全部
      </el-button>
    </template>
    <div v-if="log" class="drawer-content">
      <div class="detail-section">
        <h4>错误信息</h4>
        <pre class="mono">{{ log.message }}</pre>
      </div>

      <div class="detail-section" v-if="log.fingerprint">
        <h4>错误指纹</h4>
        <div class="fingerprint-container">
          <span class="mono-inline fingerprint-text">{{ log.fingerprint }}</span>
          <el-button size="small" :icon="DocumentCopy" @click="copyText(log.fingerprint, '指纹')">复制</el-button>
        </div>
      </div>

      <div class="detail-section" v-if="log.stack">
        <h4>堆栈跟踪</h4>
        <div class="stack-container">
          <pre class="mono stack-trace">{{ log.stack }}</pre>
          <el-button size="small" :icon="DocumentCopy" @click="copyText(log.stack, '堆栈跟踪')">复制堆栈</el-button>
        </div>
      </div>

      <div class="detail-section">
        <h4>标签</h4>
        <div v-if="Object.keys(parsedTags).length > 0" class="key-value-list">
          <div v-for="(value, key) in parsedTags" :key="key" class="key-value-item">
            <span class="key">{{ key }}:</span>
            <span class="value">{{ value }}</span>
          </div>
        </div>
        <p v-else class="empty">无标签</p>
      </div>

      <div class="detail-section">
        <h4>额外数据</h4>
        <div v-if="log.extra && log.extra !== '{}'" class="extra-container">
          <pre class="mono">{{ formatJson(log.extra) }}</pre>
          <el-button size="small" :icon="DocumentCopy" @click="copyText(formatJson(log.extra), '额外数据')">复制</el-button>
        </div>
        <p v-else class="empty">无额外数据</p>
      </div>

      <div class="detail-section">
        <h4>环境信息</h4>
        <div class="info-list">
          <div class="info-item"><span class="label">URL:</span> <span>{{ log.url || '-' }}</span></div>
          <div class="info-item"><span class="label">位置:</span> <span>{{ log.line }}:{{ log.col }}</span></div>
          <div class="info-item"><span class="label">Release:</span> <span>{{ log.release || '-' }}</span></div>
          <div class="info-item"><span class="label">环境:</span> <span>{{ log.env || '-' }}</span></div>
          <div class="info-item"><span class="label">用户ID:</span> <span>{{ log.user_id || '-' }}</span></div>
          <div class="info-item"><span class="label">会话ID:</span> <span class="mono-inline">{{ log.session_id || '-' }}</span></div>
          <div class="info-item"><span class="label">浏览器:</span> <span>{{ log.ua || '-' }}</span></div>
          <div class="info-item"><span class="label">屏幕尺寸:</span> <span>{{ log.screen || '-' }}</span></div>
          <div class="info-item"><span class="label">视口:</span> <span>{{ log.viewport || '-' }}</span></div>
        </div>
      </div>

      <div class="detail-section" v-if="log.performance && log.performance !== '{}'">
        <h4>性能数据</h4>
        <pre class="mono">{{ formatJson(log.performance) }}</pre>
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

      <div class="detail-section" v-if="log.screenshot_url">
        <h4>错误截图</h4>
        <div class="screenshot-container">
          <el-image
            :src="getScreenshotUrl(log.screenshot_url)"
            fit="contain"
            :preview-src-list="[getScreenshotUrl(log.screenshot_url)]"
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
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Picture, DocumentCopy } from '@element-plus/icons-vue'
import type { Event } from '../types'

interface Props {
  visible: boolean
  log: Event | null
}

const props = defineProps<Props>()

defineEmits<{
  'update:visible': [value: boolean]
}>()

const parsedTags = computed(() => {
  if (!props.log) return {}
  try {
    return JSON.parse(props.log.tags || '{}')
  } catch {
    return {}
  }
})

const xhrData = computed(() => {
  if (!props.log) return null
  try {
    const extra = JSON.parse(props.log.extra || '{}')
    if (extra.xhr) return extra.xhr
    return null
  } catch {
    return null
  }
})

const breadcrumbs = computed(() => {
  if (!props.log) return []
  try {
    const extra = JSON.parse(props.log.extra || '{}')
    if (extra.breadcrumbs && Array.isArray(extra.breadcrumbs)) {
      return extra.breadcrumbs
    }
    return []
  } catch {
    return []
  }
})

const formatJson = (jsonStr: string) => {
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2)
  } catch {
    return jsonStr
  }
}

const copyText = async (text: string, label: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`${label}已复制到剪贴板`)
  } catch {
    ElMessage.error('复制失败')
  }
}

const copyErrorInfo = () => {
  if (!props.log) return

  const log = props.log
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

  text += `\nStack Trace:\n${log.stack || '(none)'}\n\nTags:\n${JSON.stringify(parsedTags.value, null, 2)}\n\nExtra:\n${log.extra || '(none)'}`

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

const getBreadcrumbIcon = (type: string) => {
  const map: Record<string, string> = { click: '👆', navigation: '🔗', xhr: '🌐', console: '🖥️', custom: '⭐', error: '❌' }
  return map[type] || '📌'
}

const formatBreadcrumbTime = (ts: number) => {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString()
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

.fingerprint-container {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px;
  background: #131829;
  border-radius: 6px;
}

.fingerprint-text {
  flex: 1;
  color: #a0aec0;
  font-size: 13px;
  word-break: break-all;
}

.stack-container,
.extra-container {
  position: relative;
}

.stack-container .el-button,
.extra-container .el-button {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 1;
}

.stack-trace {
  position: relative;
  line-height: 1.6;
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
