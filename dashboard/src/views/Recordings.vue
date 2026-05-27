<template>
  <div class="recordings-view">
    <el-page-header @back="$router.back()" title="返回">
      <template #content>
        <div class="flex items-center">
          <el-icon class="mr-2"><Film /></el-icon>
          <span class="text-lg font-medium">录制回放</span>
        </div>
      </template>
    </el-page-header>

    <!-- List view -->
    <div v-if="!selectedRecording" class="recordings-list">
      <div class="list-header">
        <div class="header-left">
          <el-input
            v-model="filterSearch"
            placeholder="搜索会话ID或URL..."
            :prefix-icon="Search"
            clearable
            style="width: 200px"
            @change="loadRecordings"
          />
          <el-select
            v-model="filterAppId"
            placeholder="应用筛选"
            clearable
            style="width: 150px"
            @change="loadRecordings"
          >
            <el-option label="应用1" value="app1" />
            <el-option label="应用2" value="app2" />
          </el-select>
          <el-select
            v-model="filterStatus"
            placeholder="状态筛选"
            clearable
            style="width: 120px"
            @change="loadRecordings"
          >
            <el-option label="录制中" value="recording" />
            <el-option label="已完成" value="completed" />
            <el-option label="错误" value="error" />
          </el-select>
        </div>
        <div class="header-right">
          <el-button :icon="Refresh" @click="loadRecordings" :loading="loading">刷新</el-button>
        </div>
      </div>

      <el-table :data="filteredRecordings" v-loading="loading" stripe>
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
            <el-button type="primary" :icon="VideoPlay" @click="playRecording(row)" size="small">
              回放
            </el-button>
            <el-popconfirm title="确认删除此录制？" @confirm="deleteRecording(row)">
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
        @size-change="loadRecordings"
        @current-change="loadRecordings"
        class="pagination"
      />
    </div>

    <!-- Player view -->
    <div v-else class="recording-player">
      <div class="player-header">
        <div class="header-left">
          <el-button :icon="ArrowLeft" @click="closePlayer">返回</el-button>
          <div class="session-info">
            <span class="session-id">{{ selectedRecording.sessionId }}</span>
            <el-tag size="small" type="info">{{ selectedRecording.appId }}</el-tag>
          </div>
        </div>
        <div class="header-right">
          <el-button :icon="Download" @click="exportRecording">导出</el-button>
        </div>
      </div>

      <div class="player-content">
        <div class="player-toolbar">
          <el-button-group>
            <el-tooltip content="后退 5 秒">
              <el-button :icon="DArrowLeft" @click="seekBackward" />
            </el-tooltip>
            <el-tooltip :content="isPlaying ? '暂停' : '播放'">
              <el-button :type="isPlaying ? 'primary' : 'default'" :icon="isPlaying ? VideoPause : VideoPlay" @click="togglePlay" />
            </el-tooltip>
            <el-tooltip content="前进 5 秒">
              <el-button :icon="DArrowRight" @click="seekForward" />
            </el-tooltip>
          </el-button-group>

          <el-select v-model="playbackSpeed" style="width: 80px" size="small">
            <el-option label="0.5x" :value="0.5" />
            <el-option label="1x" :value="1" />
            <el-option label="2x" :value="2" />
            <el-option label="4x" :value="4" />
          </el-select>

          <span class="time-display">{{ currentTimeDisplay }} / {{ totalTimeDisplay }}</span>

          <el-slider
            v-model="progress"
            :max="totalDuration"
            :format-tooltip="formatProgress"
            @change="onSeek"
            class="progress-slider"
            style="flex: 1; max-width: 300px"
          />
        </div>

        <div class="player-viewport" ref="playerRef">
          <div v-loading="loadingEvents" class="replayer-container" ref="replayerRef"></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { ElMessage } from 'element-plus'

function useSnackbar() {
  return {
    success: (msg: string) => ElMessage.success(msg),
    error: (msg: string) => ElMessage.error(msg),
    warning: (msg: string) => ElMessage.warning(msg),
    info: (msg: string) => ElMessage.info(msg)
  }
}
import { cobrowseApi } from '../api'
import type { Recording, RecordingEvent } from '../types'
import {
  Film,
  Search,
  Refresh,
  VideoPlay,
  Delete,
  ArrowLeft,
  Download,
  VideoPause,
  DArrowLeft,
  DArrowRight
} from '@element-plus/icons-vue'
import dayjs from 'dayjs'

const { showSuccess, showError } = useSnackbar()

const loading = ref(false)
const recordings = ref<Recording[]>([])
const selectedRecording = ref<Recording | null>(null)
const searchAppId = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// Filter states
const filterAppId = ref('')
const filterStatus = ref('')
const filterStartTimeRange = ref<[Date, Date] | null>(null)
const filterMinDuration = ref<number>()
const filterMaxDuration = ref<number>()
const filterSearch = ref('')

// Player state
const playerRef = ref<HTMLElement>()
const replayerRef = ref<HTMLElement>()
const isPlaying = ref(false)
const playbackSpeed = ref(1)
const progress = ref(0)
const currentTime = ref(0)
const totalDuration = ref(0)
const loadingEvents = ref(false)
let replayer: any = null
let events: RecordingEvent[] = []

onMounted(() => {
  loadRecordings()
})

const filteredRecordings = computed(() => {
  if (!searchAppId.value) return recordings.value
  return recordings.value.filter(r =>
    r.appId.toLowerCase().includes(searchAppId.value.toLowerCase()) ||
    r.sessionId.toLowerCase().includes(searchAppId.value.toLowerCase())
  )
})

const currentTimeDisplay = computed(() => formatTimeMs(currentTime.value))
const totalTimeDisplay = computed(() => formatTimeMs(totalDuration.value))

async function loadRecordings() {
  loading.value = true
  try {
    const offset = (page.value - 1) * pageSize.value
    const params: any = { limit: pageSize.value, offset }

    // Add filter parameters
    if (filterAppId.value) params.app_id = filterAppId.value
    if (filterStatus.value) params.status = filterStatus.value
    if (filterStartTimeRange.value && filterStartTimeRange.value.length === 2) {
      params.start_from = filterStartTimeRange.value[0].getTime()
      params.start_to = filterStartTimeRange.value[1].getTime()
    }
    if (filterMinDuration.value) params.min_duration = filterMinDuration.value
    if (filterMaxDuration.value) params.max_duration = filterMaxDuration.value
    if (filterSearch.value) params.search = filterSearch.value

    const { data } = await cobrowseApi.getRecordings(params)
    recordings.value = (data.data || [])
    // For demo, assume we have up to 100 recordings
    total.value = Math.min(pageSize.value * page.value, 100)
  } catch (err) {
    showError('加载录制列表失败')
  } finally {
    loading.value = false
  }
}

async function playRecording(recording: Recording) {
  selectedRecording.value = recording
  await loadRecordingEvents(recording.sessionId)
  await nextTick()
  initPlayer()
}

async function loadRecordingEvents(sessionId: string) {
  loadingEvents.value = true
  try {
    // First load stats to get total event count
    const stats = await loadRecordingStats(sessionId)
    const totalEventsCount = stats?.totalEvents || 0

    // Load initial batch of events (limit to 50 for performance)
    const { data } = await cobrowseApi.getRecordingEvents(sessionId, { limit: 50 })
    events = (data.events || [])

    // Calculate total duration
    if (events.length > 0) {
      const firstEvent = events[0]
      const lastEvent = events[events.length - 1]
      totalDuration.value = lastEvent.timestamp - firstEvent.timestamp
    }

    // If there are more events to load, show loading indicator
    if (totalEventsCount > 50) {
      ElMessage.info(`已加载 ${events.length}/${totalEventsCount} 个事件，其他事件已分页加载`)
    }
  } catch (err) {
    showError('加载录制事件失败')
  } finally {
    loadingEvents.value = false
  }
}

async function initPlayer() {
  if (!replayerRef.value || events.length === 0) return

  // Use rrweb-player from CDN global
  const rrwebPlayerLib = (window as any).rrwebPlayer
  if (!rrwebPlayerLib) {
    return
  }

  // Convert events to rrweb format
  const rrwebEvents = events.map(e => {
    try {
      const parsed = JSON.parse(e.eventData)
      return {
        type: parsed.type,
        timestamp: e.timestamp,
        data: parsed.data || parsed
      }
    } catch {
      return null
    }
  }).filter(Boolean).sort((a: any, b: any) => a.timestamp - b.timestamp)

  if (rrwebEvents.length < 2) {
    return
  }

  // Clean up previous player
  if (replayer) {
    try { (replayer as any).pause?.() } catch {}
    try { (replayer as any).destroy?.() } catch {}
    replayer = null
    if (replayerRef.value) replayerRef.value.innerHTML = ''
  }

  // Create new player
  replayer = new rrwebPlayerLib({
    target: replayerRef.value,
    props: {
      events: rrwebEvents,
      autoPlay: false,
      skipInactive: false,
      showController: true,
      width: replayerRef.value.clientWidth || 1024,
      height: replayerRef.value.clientHeight || 576,
      mouseTail: false
    }
  }) as any

  // Get metadata
  const meta = (replayer as any).getMetaData?.() || { totalTime: 0 }
  totalDuration.value = meta.totalTime || (rrwebEvents[rrwebEvents.length - 1].timestamp - rrwebEvents[0].timestamp)
  progress.value = 0
  currentTime.value = 0
}

// Add new function to load recording stats
async function loadRecordingStats(sessionId: string) {
  try {
    const { data } = await cobrowseApi.getRecordingStats(sessionId)
    return data
  } catch (err) {
    showError('加载录制统计失败')
    return null
  }
}

function togglePlay() {
  if (!replayer) return

  if (isPlaying.value) {
    replayer.pause()
  } else {
    if (currentTime.value === 0) {
      replayer.play()
    } else {
      replayer.play(currentTime.value)
    }
  }
  isPlaying.value = !isPlaying.value
}

function seekForward() {
  const newTime = Math.min(currentTime.value + 5000, totalDuration.value)
  replayer?.goto(newTime)
}

function seekBackward() {
  const newTime = Math.max(currentTime.value - 5000, 0)
  replayer?.goto(newTime)
}

function onSeek(value: number) {
  replayer?.goto(value)
}

function formatProgress(value: number): string {
  return formatTimeMs(value)
}

watch(playbackSpeed, (speed) => {
  replayer?.setSpeed(speed)
})

function closePlayer() {
  if (replayer) {
    replayer.pause()
    replayer = null
  }
  selectedRecording.value = null
  isPlaying.value = false
  progress.value = 0
  currentTime.value = 0
}

async function deleteRecording(recording: Recording) {
  try {
    await cobrowseApi.deleteRecording(recording.sessionId)
    showSuccess('录制已删除')
    loadRecordings()
  } catch (err) {
    showError('删除录制失败')
  }
}

function exportRecording() {
  if (!selectedRecording.value) return

  const data = {
    sessionId: selectedRecording.value.sessionId,
    events: events
  }

  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `recording-${selectedRecording.value.sessionId}.json`
  a.click()
  URL.revokeObjectURL(url)

  showSuccess('录制已导出')
}

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

function formatTimeMs(ms: number): string {
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
</script>

<style scoped>
.recordings-view {
  padding: 20px;
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
}

.recordings-list {
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

.pagination {
  padding: 16px;
  border: 1px solid var(--color-border);
  display: flex;
  justify-content: center;
}

.recording-player {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.player-header {
  padding: 12px 16px;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.session-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 16px;
}

.session-id {
  font-family: monospace;
  font-size: 14px;
  color: var(--color-text-secondary);
}

.player-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

.player-toolbar {
  padding: 12px 16px;
  border: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  gap: 12px;
}

.time-display {
  font-family: monospace;
  font-size: 14px;
  color: var(--color-text-secondary);
  min-width: 100px;
}

.progress-slider {
  margin: 0 16px;
}

.player-viewport {
  flex: 1;
  position: relative;
  background: var(--color-bg-tertiary);
}

.replayer-container {
  width: 100%;
  height: 100%;
  background: var(--color-bg-secondary);
}

:deep(.rrweb-player) {
  width: 100% !important;
  height: 100% !important;
}

:deep(.rrweb-player-container) {
  width: 100% !important;
  height: 100% !important;
}
</style>
