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
    <RecordingList
      v-if="!selectedRecording"
      :recordings="recordings"
      :apps="apps"
      :loading="loading"
      @refresh="loadRecordings"
      @play="playRecording"
      @delete="deleteRecording"
    />

    <!-- Player view -->
    <RecordingPlayer
      v-else
      :recording="selectedRecording"
      :events="events"
      :loading-events="loadingEvents"
      @close="closePlayer"
      @export="exportRecording"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Film } from '@element-plus/icons-vue'
import { cobrowseApi, logApi } from '../api'
import type { Recording, RecordingEvent } from '../types'
import RecordingList from '../components/RecordingList.vue'
import RecordingPlayer from '../components/RecordingPlayer.vue'

function useSnackbar() {
  return {
    success: (msg: string) => ElMessage.success(msg),
    error: (msg: string) => ElMessage.error(msg),
    warning: (msg: string) => ElMessage.warning(msg),
    info: (msg: string) => ElMessage.info(msg)
  }
}

const { success, error } = useSnackbar()

const loading = ref(false)
const recordings = ref<Recording[]>([])
const selectedRecording = ref<Recording | null>(null)
const apps = ref<any[]>([])
const events = ref<RecordingEvent[]>([])
const loadingEvents = ref(false)

onMounted(() => {
  loadApps()
  loadRecordings()
})

async function loadApps() {
  try {
    const { data } = await logApi.getApps()
    apps.value = data
  } catch (err) {
    console.error('Failed to load apps:', err)
  }
}

async function loadRecordings() {
  loading.value = true
  try {
    const params: any = { limit: 1000, offset: 0 }
    const { data } = await cobrowseApi.getRecordings(params)
    recordings.value = (data.data || [])
  } catch (err) {
    error('加载录制列表失败')
  } finally {
    loading.value = false
  }
}

async function playRecording(recording: Recording) {
  selectedRecording.value = recording
  await loadRecordingEvents(recording.sessionId)
}

async function loadRecordingEvents(sessionId: string) {
  loadingEvents.value = true
  try {
    const stats = await loadRecordingStats(sessionId)
    const totalEventsCount = stats?.totalEvents || 0

    const { data } = await cobrowseApi.getRecordingEvents(sessionId, { limit: 50 })
    events.value = (data.events || [])

    if (totalEventsCount > 50) {
      ElMessage.info(`已加载 ${events.value.length}/${totalEventsCount} 个事件，其他事件已分页加载`)
    }
  } catch (err) {
    error('加载录制事件失败')
  } finally {
    loadingEvents.value = false
  }
}

async function loadRecordingStats(sessionId: string) {
  try {
    const { data } = await cobrowseApi.getRecordingStats(sessionId)
    return data
  } catch (err) {
    error('加载录制统计失败')
    return null
  }
}

function closePlayer() {
  selectedRecording.value = null
  events.value = []
  loadingEvents.value = false
}

async function deleteRecording(recording: Recording) {
  try {
    await cobrowseApi.deleteRecording(recording.sessionId)
    success('录制已删除')
    loadRecordings()
  } catch (err) {
    error('删除录制失败')
  }
}

function exportRecording() {
  if (!selectedRecording.value) return

  const data = {
    sessionId: selectedRecording.value.sessionId,
    events: events.value
  }

  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `recording-${selectedRecording.value.sessionId}.json`
  a.click()
  URL.revokeObjectURL(url)

  success('录制已导出')
}
</script>

<style scoped>
.recordings-view {
  padding: 20px;
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
}
</style>
