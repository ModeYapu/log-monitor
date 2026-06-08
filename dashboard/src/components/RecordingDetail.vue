<template>
  <div class="recording-detail">
    <div class="detail-header">
      <h3>录制详情</h3>
      <el-button :icon="Close" @click="$emit('close')" />
    </div>

    <div class="detail-content">
      <div class="info-grid">
        <div class="info-item">
          <span class="label">会话ID:</span>
          <span class="value mono">{{ recording.sessionId }}</span>
        </div>
        <div class="info-item">
          <span class="label">应用:</span>
          <span class="value">{{ recording.appId }}</span>
        </div>
        <div class="info-item">
          <span class="label">页面URL:</span>
          <span class="value">{{ recording.url }}</span>
        </div>
        <div class="info-item">
          <span class="label">状态:</span>
          <el-tag :type="getStatusType(recording.status)" size="small">
            {{ getStatusText(recording.status) }}
          </el-tag>
        </div>
        <div class="info-item">
          <span class="label">开始时间:</span>
          <span class="value">{{ formatTime(recording.startTime) }}</span>
        </div>
        <div class="info-item">
          <span class="label">结束时间:</span>
          <span class="value">{{ formatTime(recording.endTime) }}</span>
        </div>
        <div class="info-item">
          <span class="label">时长:</span>
          <span class="value">{{ formatDuration(recording.durationMs) }}</span>
        </div>
        <div class="info-item">
          <span class="label">事件数:</span>
          <span class="value">{{ recording.eventCount }}</span>
        </div>
        <div class="info-item">
          <span class="label">用户代理:</span>
          <span class="value">{{ truncateUA(recording.ua) }}</span>
        </div>
      </div>

      <div class="action-buttons">
        <el-button type="primary" :icon="VideoPlay" @click="$emit('play')">回放</el-button>
        <el-button :icon="Download" @click="$emit('export')">导出</el-button>
        <el-popconfirm title="确认删除此录制？" @confirm="$emit('delete')">
          <template #reference>
            <el-button type="danger" :icon="Delete">删除</el-button>
          </template>
        </el-popconfirm>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Close, VideoPlay, Download, Delete } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import type { Recording } from '../types'

interface Props {
  recording: Recording
}

defineProps<Props>()

defineEmits<{
  close: []
  play: []
  export: []
  delete: []
}>()

function formatTime(timestamp: number): string {
  return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss')
}

function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
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

function truncateUA(ua: string): string {
  if (!ua) return '-'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Firefox')) return 'Firefox'
  if (ua.includes('Safari')) return 'Safari'
  if (ua.includes('Edge')) return 'Edge'
  return ua.substring(0, 50)
}
</script>

<style scoped>
.recording-detail {
  background: var(--color-bg-secondary);
  border-radius: 8px;
  padding: 20px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--color-border);
}

.detail-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.detail-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.info-item {
  display: flex;
  gap: 8px;
  font-size: 14px;
}

.info-item .label {
  color: var(--color-text-secondary);
  min-width: 80px;
  font-weight: 500;
}

.info-item .value {
  color: var(--color-text-primary);
  word-break: break-all;
}

.info-item .value.mono {
  font-family: monospace;
  font-size: 13px;
}

.action-buttons {
  display: flex;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
}
</style>
