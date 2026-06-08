<template>
  <div class="recording-player">
    <div class="player-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" @click="$emit('close')">返回</el-button>
        <div class="session-info">
          <span class="session-id">{{ recording.sessionId }}</span>
          <el-tag size="small" type="info">{{ recording.appId }}</el-tag>
        </div>
      </div>
      <div class="header-right">
        <el-button :icon="Download" @click="$emit('export')">导出</el-button>
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
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Download, VideoPause, VideoPlay, DArrowLeft, DArrowRight } from '@element-plus/icons-vue'
import rrwebPlayer from 'rrweb-player'
import type { Recording, RecordingEvent } from '../types'

interface Props {
  recording: Recording
  events: RecordingEvent[]
  loadingEvents: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
  export: []
}>()

const playerRef = ref<HTMLElement>()
const replayerRef = ref<HTMLElement>()
const isPlaying = ref(false)
const playbackSpeed = ref(1)
const progress = ref(0)
const currentTime = ref(0)
const totalDuration = ref(0)
let replayer: any = null

const currentTimeDisplay = computed(() => formatTimeMs(currentTime.value))
const totalTimeDisplay = computed(() => formatTimeMs(totalDuration.value))

onMounted(async () => {
  await nextTick()
  initPlayer()
})

watch(() => props.events, () => {
  nextTick(() => {
    initPlayer()
  })
}, { deep: true })

async function initPlayer() {
  if (!replayerRef.value || props.events.length === 0) return

  const rrwebEvents = props.events.map(e => {
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

  if (replayer) {
    try { (replayer as any).pause?.() } catch {}
    try { (replayer as any).destroy?.() } catch {}
    replayer = null
    if (replayerRef.value) replayerRef.value.innerHTML = ''
  }

  replayer = new rrwebPlayer({
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

  const meta = (replayer as any).getMetaData?.() || { totalTime: 0 }
  totalDuration.value = meta.totalTime || (rrwebEvents[rrwebEvents.length - 1].timestamp - rrwebEvents[0].timestamp)
  progress.value = 0
  currentTime.value = 0
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

function formatTimeMs(ms: number): string {
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
}
</script>

<style scoped>
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

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-right {
  display: flex;
  gap: 8px;
}

.session-info {
  display: flex;
  align-items: center;
  gap: 8px;
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
