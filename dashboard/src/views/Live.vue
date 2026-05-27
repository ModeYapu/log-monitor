<template>
  <div class="live-view">
    <el-page-header @back="$router.back()" title="返回">
      <template #content>
        <div class="flex items-center gap-2">
          <el-icon><VideoCamera /></el-icon>
          <span style="font-weight:600">实时会话</span>
          <el-tag v-if="liveSessions.length > 0" type="success" size="small" effect="dark">
            {{ liveSessions.length }} 个在线
          </el-tag>
        </div>
      </template>
    </el-page-header>

    <div class="live-layout">
      <div class="session-list">
        <div class="session-list-header">
          <h3>在线用户</h3>
          <el-button :icon="Refresh" @click="refreshSessions" circle size="small" />
        </div>
        <div class="session-list-content">
          <div
            v-for="session in liveSessions"
            :key="session.sessionId"
            class="session-item"
            :class="{ active: selectedSessionId === session.sessionId }"
            @click="selectSession(session)"
          >
            <div class="session-header">
              <div class="flex items-center gap-2">
                <span class="status-dot online"></span>
                <span class="app-name">{{ session.appId }}</span>
              </div>
              <span class="viewer-count" v-if="session.viewerCount > 0">
                <el-icon><View /></el-icon> {{ session.viewerCount }}
              </span>
            </div>
            <div class="session-info">
              <div class="url">{{ formatUrl(session.url) }}</div>
              <div class="meta">
                <span>{{ formatUA(session.ua) }}</span>
                <span>{{ formatDuration(session.connectedAt) }}</span>
              </div>
            </div>
          </div>
          <el-empty v-if="liveSessions.length === 0" description="暂无在线会话" :image-size="80" />
        </div>
      </div>

      <div class="live-viewer">
        <div v-if="selectedSession" class="viewer-container">
          <div class="viewer-toolbar">
            <div class="toolbar-left">
              <el-tag :type="wsConnected ? 'success' : (reconnecting ? 'warning' : 'danger')" size="small" effect="dark">
                <span class="flex items-center gap-1">
                  <span class="live-dot" v-if="wsConnected"></span>
                  {{ wsConnected ? '实时' : (connecting ? '连接中' : (reconnecting ? '重连中...' : '断开')) }}
                </span>
              </el-tag>
              <span class="event-count" v-if="eventCount > 0">{{ eventCount }} events</span>
              <span class="reconnect-info" v-if="reconnecting">重连次数: {{ reconnectAttempts }}/{{ maxReconnectAttempts }}</span>
            </div>
            <div class="toolbar-right">
              <el-button :icon="Link" @click="openOriginalPage" size="small">原页面</el-button>
              <el-button :icon="CloseBold" @click="disconnect" type="danger" size="small">断开</el-button>
            </div>
          </div>

          <div class="viewer-content">
            <div ref="replayContainerRef" class="replay-container"></div>
            <div v-if="connecting" class="viewer-overlay">
              <el-icon class="is-loading" :size="32"><Loading /></el-icon>
              <span>连接中...</span>
            </div>
            <div v-else-if="!wsConnected && eventCount === 0" class="viewer-overlay">
              <el-icon :size="32"><Connection /></el-icon>
              <span>等待数据...</span>
            </div>
          </div>
        </div>

        <div v-else class="viewer-empty">
          <el-empty description="请选择一个会话开始观看" :image-size="120">
            <template #image>
              <el-icon :size="80" color="var(--color-text-secondary)"><VideoCamera /></el-icon>
            </template>
          </el-empty>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { cobrowseApi } from '../api'
import type { LiveSession } from '../types'
import {
  VideoCamera, Refresh, View, Link, CloseBold, Loading, Connection
} from '@element-plus/icons-vue'

const router = useRouter()

const liveSessions = ref<LiveSession[]>([])
const selectedSession = ref<LiveSession | null>(null)
const selectedSessionId = ref('')
const wsConnected = ref(false)
const connecting = ref(false)
const eventCount = ref(0)
const reconnecting = ref(false)
const reconnectAttempts = ref(0)
const maxReconnectAttempts = 10

const replayContainerRef = ref<HTMLElement>()
let ws: WebSocket | null = null
let replayer: any = null
let allEvents: any[] = []
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let refreshTimer: ReturnType<typeof setInterval> | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let rebuildTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  refreshSessions()
  refreshTimer = setInterval(refreshSessions, 5000)
})

onUnmounted(() => {
  cleanup()
  if (refreshTimer) clearInterval(refreshTimer)
})

function cleanup() {
  destroyReplayer()
  if (ws) { ws.close(); ws = null }
  wsConnected.value = false
  connecting.value = false
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  if (rebuildTimer) { clearTimeout(rebuildTimer); rebuildTimer = null }
}

function destroyReplayer() {
  if (replayer) {
    try { replayer.pause() } catch {}
    replayer = null
  }
  if (replayContainerRef.value) {
    replayContainerRef.value.innerHTML = ''
  }
}

async function refreshSessions() {
  try {
    const { data } = await cobrowseApi.getLiveSessions()
    liveSessions.value = data.data || []
  } catch {}
}

function selectSession(session: LiveSession) {
  if (selectedSessionId.value === session.sessionId) return
  selectedSessionId.value = session.sessionId
  selectedSession.value = session
  allEvents = []
  eventCount.value = 0
  destroyReplayer()
  connectToSession(session.sessionId)
}

function connectToSession(sessionId: string) {
  cleanup()
  connecting.value = true

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  ws = new WebSocket(`${proto}//${location.host}/ws/cobrowse/${sessionId}/view`)

  ws.onopen = () => {
    wsConnected.value = true
    connecting.value = false
    reconnecting.value = false
    reconnectAttempts.value = 0
    heartbeatTimer = setInterval(() => {
      if (ws?.readyState === WebSocket.OPEN) ws.send('{"type":"pong"}')
    }, 25000)
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'ping') { ws?.send('{"type":"pong"}'); return }

      if (msg.type === 'rrweb-full-snapshot') {
        allEvents = [msg.data]
        eventCount.value = 1
        // Schedule rebuild — rrweb Replayer needs >= 2 events
        if (rebuildTimer) clearTimeout(rebuildTimer)
        rebuildTimer = setTimeout(() => { rebuildTimer = null; rebuildReplayer() }, 500)
      } else if (msg.type === 'rrweb-event') {
        allEvents.push(msg.data)
        eventCount.value = allEvents.length
        // Throttled rebuild — 每 500ms 重建一次
        if (!rebuildTimer) {
          rebuildTimer = setTimeout(() => {
            rebuildTimer = null
            rebuildReplayer()
          }, 500)
        }
      }
    } catch (err) {
      // Ignore malformed messages
    }
  }

  ws.onclose = () => {
    wsConnected.value = false
    connecting.value = false
    if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
    scheduleReconnect()
  }

  ws.onerror = () => {
    connecting.value = false
  }
}

  ws.onerror = () => {
    connecting.value = false
  }
}

function rebuildReplayer() {
  const container = replayContainerRef.value
  if (!container || allEvents.length < 2) return  // rrweb needs >= 2 events

  const rrwebLib = (window as any).rrweb
  if (!rrwebLib?.Replayer) {
    return
  }

  // Destroy old replayer
  if (replayer) {
    try { replayer.pause() } catch {}
    replayer = null
  }
  container.innerHTML = ''

  try {
    // Replayer(events, options) — events is array of {type, data, timestamp}
    replayer = new rrwebLib.Replayer(allEvents, {
      root: container,
      UNSAFE_replayCanvas: true,
      mouseTail: false  // 关掉 mouse tail 避免布局问题
    })
    // Play from end — 最后一个事件的时间
    const lastTime = allEvents[allEvents.length - 1]?.timestamp || 0
    replayer.play(lastTime)

    // Force iframe visible
    const iframe = container.querySelector('iframe')
    if (iframe) {
      ;(iframe as HTMLIFrameElement).style.display='block'
      ;(iframe as HTMLIFrameElement).style.visibility='visible'
      ;(iframe as HTMLIFrameElement).style.opacity='1'
    }
  } catch (err) {
    replayer = null
    // Show fallback: raw event count
    container.innerHTML = `
      <div style="padding:40px;text-align:center;color:#94a3b8">
        <p style="font-size:16px;margin-bottom:8px">🎬 已接收 ${allEvents.length} 个实时事件</p>
        <p style="font-size:13px">Replayer: ${(err as Error).message}</p>
      </div>`
  }
}

function disconnect() {
  cleanup()
  selectedSessionId.value = ''
  selectedSession.value = null
}

function scheduleReconnect() {
  if (reconnectTimer || !selectedSession.value) return
  if (reconnectAttempts.value >= maxReconnectAttempts) {
    reconnecting.value = false
    reconnectAttempts.value = 0
    ElMessage.error('重连失败次数过多，请手动刷新页面')
    return
  }

  reconnectAttempts.value++
  reconnecting.value = true

  // Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s
  const delay = Math.min(Math.pow(2, reconnectAttempts.value - 1) * 1000, 30000)

  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    if (selectedSession.value) {
      connectToSession(selectedSession.value.sessionId)
    }
  }, delay)
}

function openOriginalPage() {
  if (selectedSession.value?.url) window.open(selectedSession.value.url, '_blank')
}

function formatUrl(url: string): string {
  try { return new URL(url).pathname + new URL(url).search } catch { return url }
}

function formatUA(ua: string): string {
  if (ua.includes('Edg')) return 'Edge'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Firefox')) return 'Firefox'
  return 'Other'
}

function formatDuration(startMs: number): string {
  const d = Date.now() - startMs
  const m = Math.floor(d / 60000)
  const s = Math.floor((d % 60000) / 1000)
  return `${m}m ${s}s`
}
</script>

<style scoped>
.live-view { padding: 20px; height: calc(100vh - 40px); display: flex; flex-direction: column; }
.live-layout { display: flex; gap: 20px; flex: 1; min-height: 0; margin-top: 16px; }

.session-list {
  width: 320px; background: var(--color-bg-secondary); border-radius: 8px;
  border: 1px solid var(--color-border); display: flex; flex-direction: column;
}
.session-list-header {
  padding: 16px; border-bottom: 1px solid var(--color-border);
  display: flex; align-items: center; justify-content: space-between;
}
.session-list-header h3 { margin: 0; font-size: 16px; font-weight: 500; color: var(--color-text); }
.session-list-content { flex: 1; overflow-y: auto; padding: 8px; }
.session-item {
  padding: 12px; border-radius: 6px; cursor: pointer; transition: all 0.2s;
  border: 1px solid transparent; margin-bottom: 4px;
}
.session-item:hover { background: var(--color-bg-tertiary); }
.session-item.active { background: rgba(99,102,241,0.15); border-color: var(--color-primary); }
.session-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.status-dot.online { background: var(--color-success); animation: pulse 2s infinite; }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }
.app-name { font-weight: 500; font-size: 14px; color: var(--color-text); }
.viewer-count { font-size: 12px; color: var(--color-text-secondary); display: flex; align-items: center; gap: 2px; }
.session-info { font-size: 12px; color: var(--color-text-secondary); }
.url { margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.meta { display: flex; gap: 8px; }

.live-viewer {
  flex: 1; background: var(--color-bg-secondary); border-radius: 8px;
  border: 1px solid var(--color-border); display: flex; flex-direction: column; overflow: hidden;
}
.viewer-container { display: flex; flex-direction: column; height: 100%; }
.viewer-toolbar {
  padding: 12px 16px; border-bottom: 1px solid var(--color-border);
  display: flex; align-items: center; justify-content: space-between;
}
.toolbar-left { display: flex; align-items: center; gap: 12px; }
.live-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--color-success); display: inline-block; animation: pulse 1.5s infinite; }
.event-count { font-size: 11px; color: var(--color-text-secondary); background: var(--color-bg-tertiary); padding: 2px 8px; border-radius: 10px; }
.toolbar-right { display: flex; align-items: center; }

.viewer-content { flex: 1; position: relative; background: #1a1a1a; overflow: hidden; min-height: 0; }
.replay-container { width: 100%; height: 100%; overflow: hidden; position: relative; }
.replay-container :deep(.replayer-wrapper) { width: 100% !important; height: 100% !important; }
.replay-container :deep(.replayer-wrapper iframe) { width: 100% !important; height: 100% !important; display: block !important; visibility: visible !important; opacity: 1 !important; }
.viewer-overlay {
  position: absolute; inset: 0; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 12px;
  background: rgba(0,0,0,0.6); font-size: 14px; color: #94a3b8; z-index: 10;
}
.viewer-empty { flex: 1; display: flex; align-items: center; justify-content: center; }
</style>
