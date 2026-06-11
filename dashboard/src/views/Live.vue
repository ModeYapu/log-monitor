<template>
  <div class="live-view" ref="liveViewRef">
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
      <div class="session-list" v-show="!isFullscreen">
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

      <div class="live-viewer" ref="liveViewerRef">
        <div v-if="selectedSession" class="viewer-container">
          <div class="viewer-toolbar">
            <div class="toolbar-left">
              <el-tag :type="wsConnected ? 'success' : (reconnecting ? 'warning' : 'danger')" size="small" effect="dark">
                <span class="flex items-center gap-1">
                  <span class="live-dot" v-if="wsConnected"></span>
                  {{ wsConnected ? (webrtcActive ? '实时' : '观看') : (connecting ? '连接中' : (reconnecting ? '重连中...' : '断开')) }}
                </span>
              </el-tag>
              <span class="event-count" v-if="!webrtcActive && eventCount > 0">{{ eventCount }} events</span>

              <!-- WebRTC status -->
              <el-tag v-if="webrtcState === 'requesting'" type="warning" size="small">
                <el-icon class="is-loading" style="margin-right:4px"><Loading /></el-icon>
                等待用户确认...
              </el-tag>
              <el-tag v-if="webrtcState === 'connecting'" type="warning" size="small">
                <el-icon class="is-loading" style="margin-right:4px"><Loading /></el-icon>
                建立 WebRTC...
              </el-tag>
            </div>

            <div class="toolbar-right">
              <!-- Intervene button -->
              <el-button
                v-if="wsConnected && !webrtcActive"
                type="primary"
                size="small"
                @click="requestIntervene"
                :loading="webrtcState === 'requesting'"
              >
                <el-icon style="margin-right:4px"><Aim /></el-icon>
                介入
              </el-button>

              <!-- Exit intervene -->
              <el-button
                v-if="webrtcActive"
                type="danger"
                size="small"
                @click="stopIntervene"
              >
                退出介入
              </el-button>

              <!-- Control mode toggle -->
              <el-button
                v-if="webrtcActive"
                :type="controlMode ? 'warning' : 'default'"
                size="small"
                @click="toggleControlMode"
              >
                {{ controlMode ? '🖱️ 控制中' : '🖱️ 开始控制' }}
              </el-button>

              <!-- Zoom controls -->
              <div class="zoom-controls" v-if="webrtcActive">
                <el-button size="small" @click="zoomOut" :disabled="zoomLevel <= 50">-</el-button>
                <span class="zoom-label">{{ zoomLevel }}%</span>
                <el-button size="small" @click="zoomIn" :disabled="zoomLevel >= 300">+</el-button>
                <el-button size="small" @click="zoomReset">重置</el-button>
              </div>

              <el-button v-if="selectedSession?.url" :icon="Link" @click="openOriginalPage" size="small">原页面</el-button>

              <!-- Fullscreen -->
              <el-button size="small" @click="toggleFullscreen" :title="isFullscreen ? '退出全屏' : '全屏'">
                {{ isFullscreen ? '⬜' : '⛶' }}
              </el-button>

              <el-button :icon="CloseBold" @click="disconnect" type="danger" size="small">断开</el-button>
            </div>
          </div>

          <div class="viewer-content" ref="viewerContentRef" @dblclick="toggleFullscreen">
            <!-- rrweb mode -->
            <div v-show="!webrtcActive" ref="replayContainerRef" class="replay-container"></div>

            <!-- WebRTC mode -->
            <div v-show="webrtcActive" class="webrtc-wrapper" ref="webrtcWrapperRef">
              <div class="webrtc-scale-container" :style="{ transform: `scale(${zoomLevel / 100})`, transformOrigin: 'top left' }">
                <video ref="webrtcVideoRef" autoplay playsinline class="webrtc-video" />
                <!-- Control overlay canvas -->
                <canvas
                  v-if="controlMode"
                  ref="controlCanvasRef"
                  class="control-overlay"
                  @mousedown="onControlMouseDown"
                  @mouseup="onControlMouseUp"
                  @mousemove="onControlMouseMove"
                  @dblclick="onControlDblClick"
                  @contextmenu.prevent="onControlContextMenu"
                  @wheel="onControlWheel"
                />
              </div>
            </div>

            <div v-if="connecting" class="viewer-overlay">
              <el-icon class="is-loading" :size="32"><Loading /></el-icon>
              <span>连接中...</span>
            </div>
            <div v-else-if="!wsConnected && eventCount === 0 && !webrtcActive" class="viewer-overlay">
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
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { cobrowseApi } from '../api'
import type { LiveSession } from '../types'
import {
  VideoCamera, Refresh, View, Link, CloseBold, Loading, Connection, Aim
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

// WebRTC state
const webrtcActive = ref(false)
const webrtcState = ref<'idle' | 'requesting' | 'connecting' | 'connected'>('idle')
const controlMode = ref(false)
const zoomLevel = ref(100)
const isFullscreen = ref(false)

// Refs
const replayContainerRef = ref<HTMLElement>()
const webrtcVideoRef = ref<HTMLVideoElement>()
const webrtcWrapperRef = ref<HTMLElement>()
const controlCanvasRef = ref<HTMLCanvasElement>()
const viewerContentRef = ref<HTMLElement>()
const liveViewerRef = ref<HTMLElement>()
const liveViewRef = ref<HTMLElement>()

let ws: WebSocket | null = null
let replayer: any = null
let allEvents: any[] = []
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let refreshTimer: ReturnType<typeof setInterval> | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let rebuildTimer: ReturnType<typeof setTimeout> | null = null

// WebRTC
let peerConnection: RTCPeerConnection | null = null
let dataChannel: RTCDataChannel | null = null

const rtcConfig: RTCConfiguration = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
  ]
}

// Keyboard listener for control mode
let keydownHandler: ((e: KeyboardEvent) => void) | null = null
let keyupHandler: ((e: KeyboardEvent) => void) | null = null

onMounted(() => {
  refreshSessions()
  refreshTimer = setInterval(refreshSessions, 5000)
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

onUnmounted(() => {
  cleanup()
  if (refreshTimer) clearInterval(refreshTimer)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  removeKeyboardListeners()
})

function removeKeyboardListeners() {
  if (keydownHandler) { document.removeEventListener('keydown', keydownHandler); keydownHandler = null }
  if (keyupHandler) { document.removeEventListener('keyup', keyupHandler); keyupHandler = null }
}

function cleanup() {
  destroyReplayer()
  cleanupWebRTC()
  if (ws) { ws.close(); ws = null }
  wsConnected.value = false
  connecting.value = false
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  if (rebuildTimer) { clearTimeout(rebuildTimer); rebuildTimer = null }
  removeKeyboardListeners()
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

// ==================== Session Management ====================

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
  cleanupWebRTC()
  connectToSession(session.sessionId)
}

function connectToSession(sessionId: string) {
  cleanup()
  connecting.value = true

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('logmon_token')
  const wsUrl = new URL(`${proto}//${location.host}/ws/cobrowse/${sessionId}/view`)
  if (token) {
    wsUrl.searchParams.set('token', token)
  }
  ws = new WebSocket(wsUrl.toString())

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

      // WebRTC signaling
      if (msg.type === 'webrtc-offer') {
        handleWebRTCoffer(msg.sdp)
        return
      }
      if (msg.type === 'webrtc-ice') {
        handleICECandidate(msg.candidate)
        return
      }
      if (msg.type === 'webrtc-rejected') {
        webrtcState.value = 'idle'
        ElMessage.warning('用户拒绝了屏幕共享请求')
        return
      }
      if (msg.type === 'webrtc-stop') {
        cleanupWebRTC()
        ElMessage.info('用户停止了屏幕共享')
        return
      }

      // rrweb events
      if (msg.type === 'rrweb-full-snapshot') {
        allEvents = [msg.data]
        eventCount.value = 1
        if (rebuildTimer) clearTimeout(rebuildTimer)
        rebuildTimer = setTimeout(() => { rebuildTimer = null; rebuildReplayer() }, 500)
      } else if (msg.type === 'rrweb-event') {
        allEvents.push(msg.data)
        eventCount.value = allEvents.length
        if (!rebuildTimer) {
          rebuildTimer = setTimeout(() => {
            rebuildTimer = null
            rebuildReplayer()
          }, 500)
        }
      }
    } catch (err) {
      // Ignore
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

function rebuildReplayer() {
  const container = replayContainerRef.value
  if (!container || allEvents.length < 2) return

  const rrwebLib = (window as any).rrweb
  if (!rrwebLib?.Replayer) return

  if (replayer) {
    try { replayer.pause() } catch {}
    replayer = null
  }
  container.innerHTML = ''

  try {
    replayer = new rrwebLib.Replayer(allEvents, {
      root: container,
      UNSAFE_replayCanvas: true,
      mouseTail: false
    })
    const lastTime = allEvents[allEvents.length - 1]?.timestamp || 0
    replayer.play(lastTime)

    const iframe = container.querySelector('iframe')
    if (iframe) {
      (iframe as HTMLIFrameElement).style.display = 'block'
      ;(iframe as HTMLIFrameElement).style.visibility = 'visible'
      ;(iframe as HTMLIFrameElement).style.opacity = '1'
    }
  } catch (err) {
    replayer = null
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
  const delay = Math.min(Math.pow(2, reconnectAttempts.value - 1) * 1000, 30000)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    if (selectedSession.value) {
      connectToSession(selectedSession.value.sessionId)
    }
  }, delay)
}

// ==================== WebRTC ====================

function requestIntervene() {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    ElMessage.error('WebSocket 未连接')
    return
  }
  webrtcState.value = 'requesting'
  ws.send(JSON.stringify({ type: 'webrtc-offer-request' }))
  ElMessage.info('已发送介入请求，等待用户确认...')
}

async function handleWebRTCoffer(sdp: RTCSessionDescriptionInit) {
  webrtcState.value = 'connecting'

  try {
    peerConnection = new RTCPeerConnection(rtcConfig)

    // Receive remote video track
    peerConnection.ontrack = (event) => {
      const video = webrtcVideoRef.value
      if (video && event.streams[0]) {
        video.srcObject = event.streams[0]
        webrtcActive.value = true
        webrtcState.value = 'connected'

        nextTick(() => {
          resizeControlCanvas()
        })
      }
    }

    // Receive DataChannel from user
    peerConnection.ondatachannel = (event) => {
      dataChannel = event.channel
      dataChannel.onopen = () => {
        console.log('[Live] DataChannel opened')
      }
    }

    // ICE candidates — forward to user via WS
    peerConnection.onicecandidate = (e) => {
      if (e.candidate) {
        ws?.send(JSON.stringify({
          type: 'webrtc-ice',
          candidate: e.candidate.toJSON()
        }))
      }
    }

    peerConnection.onconnectionstatechange = () => {
      const state = peerConnection?.connectionState
      if (state === 'disconnected' || state === 'failed' || state === 'closed') {
        cleanupWebRTC()
        ElMessage.warning('WebRTC 连接断开')
      }
    }

    // Set remote offer and create answer
    await peerConnection.setRemoteDescription(new RTCSessionDescription(sdp))
    const answer = await peerConnection.createAnswer()
    await peerConnection.setLocalDescription(answer)

    ws?.send(JSON.stringify({
      type: 'webrtc-answer',
      sdp: peerConnection.localDescription
    }))

  } catch (err) {
    console.error('[Live] WebRTC setup failed:', err)
    cleanupWebRTC()
    ElMessage.error('WebRTC 连接失败: ' + (err as Error).message)
  }
}

async function handleICECandidate(candidate: RTCIceCandidateInit) {
  if (!peerConnection) return
  try {
    await peerConnection.addIceCandidate(new RTCIceCandidate(candidate))
  } catch (err) {
    console.error('[Live] ICE candidate failed:', err)
  }
}

function stopIntervene() {
  if (ws?.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'webrtc-stop' }))
  }
  cleanupWebRTC()
}

function cleanupWebRTC() {
  if (dataChannel) { try { dataChannel.close() } catch {}; dataChannel = null }
  if (peerConnection) { try { peerConnection.close() } catch {}; peerConnection = null }
  if (webrtcVideoRef.value) {
    webrtcVideoRef.value.srcObject = null
  }
  webrtcActive.value = false
  webrtcState.value = 'idle'
  controlMode.value = false
  removeKeyboardListeners()
}

function toggleControlMode() {
  controlMode.value = !controlMode.value
  if (controlMode.value) {
    ElMessage.success('已开启控制模式，您可以直接操作用户页面')
    setupKeyboardListeners()
    nextTick(() => resizeControlCanvas())
  } else {
    ElMessage.info('已关闭控制模式')
    removeKeyboardListeners()
  }
}

// ==================== Control Overlay ====================

function resizeControlCanvas() {
  const canvas = controlCanvasRef.value
  const video = webrtcVideoRef.value
  const wrapper = webrtcWrapperRef.value
  if (!canvas || !video || !wrapper) return

  // Match canvas to actual video dimensions
  const rect = wrapper.getBoundingClientRect()
  canvas.width = rect.width
  canvas.height = rect.height
  canvas.style.width = rect.width + 'px'
  canvas.style.height = rect.height + 'px'
}

function getVideoCoords(event: MouseEvent): { x: number; y: number } | null {
  const video = webrtcVideoRef.value
  if (!video || !video.videoWidth) return null

  const videoRect = video.getBoundingClientRect()
  const scaleX = video.videoWidth / videoRect.width
  const scaleY = video.videoHeight / videoRect.height

  return {
    x: Math.round((event.clientX - videoRect.left) * scaleX),
    y: Math.round((event.clientY - videoRect.top) * scaleY)
  }
}

function sendControlCommand(cmd: any) {
  if (!dataChannel || dataChannel.readyState !== 'open') {
    // Fallback to WebSocket
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'control', ...cmd }))
    }
    return
  }
  dataChannel.send(JSON.stringify({ type: 'control', ...cmd }))
}

function onControlMouseDown(e: MouseEvent) {
  const coords = getVideoCoords(e)
  if (!coords) return
  sendControlCommand({ action: 'click', x: coords.x, y: coords.y, button: e.button })
}

function onControlMouseUp(e: MouseEvent) {
  // Handled in mousedown for simplicity
}

let lastMouseMoveTime = 0
function onControlMouseMove(e: MouseEvent) {
  const now = Date.now()
  if (now - lastMouseMoveTime < 50) return // Throttle to 20fps
  lastMouseMoveTime = now

  const coords = getVideoCoords(e)
  if (!coords) return
  sendControlCommand({ action: 'mousemove', x: coords.x, y: coords.y })
}

function onControlDblClick(e: MouseEvent) {
  const coords = getVideoCoords(e)
  if (!coords) return
  sendControlCommand({ action: 'dblclick', x: coords.x, y: coords.y })
}

function onControlContextMenu(e: MouseEvent) {
  const coords = getVideoCoords(e)
  if (!coords) return
  sendControlCommand({ action: 'contextmenu', x: coords.x, y: coords.y })
}

function onControlWheel(e: WheelEvent) {
  sendControlCommand({ action: 'scroll', deltaX: Math.round(e.deltaX), deltaY: Math.round(e.deltaY) })
}

function setupKeyboardListeners() {
  keydownHandler = (e: KeyboardEvent) => {
    if (!controlMode.value || !webrtcActive.value) return
    // Don't intercept browser shortcuts
    if (e.ctrlKey || e.metaKey || e.altKey) return
    sendControlCommand({ action: 'keydown', key: e.key })
  }
  keyupHandler = (e: KeyboardEvent) => {
    if (!controlMode.value || !webrtcActive.value) return
    if (e.ctrlKey || e.metaKey || e.altKey) return
    sendControlCommand({ action: 'keyup', key: e.key })
  }
  document.addEventListener('keydown', keydownHandler)
  document.addEventListener('keyup', keyupHandler)
}

// ==================== Zoom & Fullscreen ====================

function zoomIn() { zoomLevel.value = Math.min(zoomLevel.value + 25, 300) }
function zoomOut() { zoomLevel.value = Math.max(zoomLevel.value - 25, 50) }
function zoomReset() { zoomLevel.value = 100 }

function toggleFullscreen() {
  if (isFullscreen.value) {
    document.exitFullscreen()
  } else {
    const el = liveViewerRef.value
    if (el?.requestFullscreen) {
      el.requestFullscreen()
    }
  }
}

function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement
}

// ==================== Helpers ====================

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
.live-viewer:fullscreen { background: #0a0e27; border-radius: 0; }
.live-viewer:fullscreen .viewer-content { border-radius: 0; }
.live-viewer:fullscreen .viewer-toolbar { background: rgba(10,14,39,0.95); border-color: rgba(255,255,255,0.1); }

.viewer-container { display: flex; flex-direction: column; height: 100%; }
.viewer-toolbar {
  padding: 10px 16px; border-bottom: 1px solid var(--color-border);
  display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px;
}
.toolbar-left { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.live-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--color-success); display: inline-block; animation: pulse 1.5s infinite; }
.event-count { font-size: 11px; color: var(--color-text-secondary); background: var(--color-bg-tertiary); padding: 2px 8px; border-radius: 10px; }
.toolbar-right { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

.zoom-controls { display: flex; align-items: center; gap: 4px; }
.zoom-label { font-size: 12px; color: var(--color-text-secondary); min-width: 40px; text-align: center; }

.viewer-content { flex: 1; position: relative; background: #1a1a1a; overflow: hidden; min-height: 0; }
.replay-container { width: 100%; height: 100%; overflow: hidden; position: relative; }
.replay-container :deep(.replayer-wrapper) { width: 100% !important; height: 100% !important; }
.replay-container :deep(.replayer-wrapper iframe) { width: 100% !important; height: 100% !important; display: block !important; visibility: visible !important; opacity: 1 !important; }

.webrtc-wrapper {
  width: 100%; height: 100%; overflow: auto; display: flex; align-items: flex-start; justify-content: center;
}
.webrtc-scale-container { position: relative; transition: transform 0.2s ease; }
.webrtc-video { display: block; max-width: 100%; height: auto; background: #000; }

.control-overlay {
  position: absolute; top: 0; left: 0; width: 100%; height: 100%;
  cursor: crosshair; z-index: 10; background: transparent;
}

.viewer-overlay {
  position: absolute; inset: 0; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 12px;
  background: rgba(0,0,0,0.6); font-size: 14px; color: #94a3b8; z-index: 10;
}
.viewer-empty { flex: 1; display: flex; align-items: center; justify-content: center; }
</style>
