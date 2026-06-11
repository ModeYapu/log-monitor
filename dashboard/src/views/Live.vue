<template>
  <div class="live-view" ref="liveViewRef">
    <div class="page-header">
      <button class="back-btn" @click="$router.back()">← 返回</button>
      <h2><span class="icon">📹</span> 实时会话</h2>
      <span v-if="liveSessions.length > 0" class="online-badge">{{ liveSessions.length }} 个在线</span>
    </div>

    <div class="live-layout">
      <div class="session-list" v-show="!isFullscreen">
        <div class="session-list-header">
          <h3>在线用户</h3>
          <button class="refresh-btn" @click="refreshSessions">🔄</button>
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
            </div>
            <div class="session-info">
              <div class="url">{{ formatUrl(session.url) }}</div>
              <div class="meta">
                <span>{{ formatUA(session.ua) }}</span>
                <span>{{ formatDuration(session.connectedAt) }}</span>
              </div>
            </div>
          </div>
          <div v-if="liveSessions.length === 0" class="empty-state">暂无在线会话</div>
        </div>
      </div>

      <div class="live-viewer" ref="liveViewerRef">
        <div v-if="selectedSession" class="viewer-container">
          <div class="viewer-toolbar">
            <div class="toolbar-left">
              <span :class="['ws-status', wsConnected ? 'connected' : (reconnecting ? 'reconnecting' : 'disconnected')]">
                <span v-if="wsConnected" class="live-dot"></span>
                {{ wsConnected ? (webrtcActive ? '🔴 实时共享' : '🟢 观看中') : (connecting ? '⏳ 连接中' : (reconnecting ? '🔄 重连中' : '⚫ 断开')) }}
              </span>
              <span v-if="!webrtcActive && eventCount > 0" class="event-badge">{{ eventCount }} events</span>
              <span v-if="webrtcState === 'requesting'" class="status-warn">⏳ 等待用户确认...</span>
              <span v-if="webrtcState === 'connecting'" class="status-warn">🔗 建立 WebRTC...</span>
            </div>

            <div class="toolbar-right">
              <button v-if="wsConnected && !webrtcActive" class="btn btn-primary" @click="requestIntervene" :disabled="webrtcState === 'requesting'">
                {{ webrtcState === 'requesting' ? '⏳ 等待确认...' : '🎯 介入' }}
              </button>
              <button v-if="webrtcActive" class="btn btn-danger" @click="stopIntervene">✕ 退出介入</button>
              <button v-if="webrtcActive" :class="['btn', controlMode ? 'btn-warning' : 'btn-default']" @click="toggleControlMode">
                {{ controlMode ? '🖱️ 控制中(点击关闭)' : '🖱️ 开始控制' }}
              </button>
              <div v-if="webrtcActive" class="zoom-controls">
                <button class="btn btn-sm" @click="zoomOut" :disabled="zoomLevel <= 50">−</button>
                <span class="zoom-label">{{ zoomLevel }}%</span>
                <button class="btn btn-sm" @click="zoomIn" :disabled="zoomLevel >= 300">+</button>
                <button class="btn btn-sm" @click="zoomReset">1:1</button>
              </div>
              <button v-if="selectedSession?.url" class="btn btn-default" @click="openOriginalPage">🔗 原页面</button>
              <button class="btn btn-default" @click="toggleFullscreen" :title="isFullscreen ? '退出全屏' : '全屏'">
                {{ isFullscreen ? '⬜' : '⛶' }}
              </button>
              <button class="btn btn-danger" @click="disconnect">✕ 断开</button>
            </div>
          </div>

          <div class="viewer-content" ref="viewerContentRef" @dblclick="toggleFullscreen">
            <div v-show="!webrtcActive" ref="replayContainerRef" class="replay-container"></div>
            <div v-show="webrtcActive" class="webrtc-wrapper" ref="webrtcWrapperRef">
              <div class="webrtc-scale-container" :style="{ transform: `scale(${zoomLevel / 100})`, transformOrigin: 'top left' }">
                <video ref="webrtcVideoRef" autoplay playsinline muted class="webrtc-video" />
                <canvas v-if="controlMode" ref="controlCanvasRef" class="control-overlay"
                  @mousedown="onControlMouseDown" @dblclick="onControlDblClick"
                  @contextmenu.prevent="onControlContextMenu" @wheel="onControlWheel" />
              </div>
            </div>
            <div v-if="connecting" class="viewer-overlay"><span class="spinner"></span><span>连接中...</span></div>
            <div v-else-if="!wsConnected && eventCount === 0 && !webrtcActive" class="viewer-overlay"><span class="big-icon">📹</span><span>等待数据...</span></div>
          </div>
        </div>
        <div v-else class="viewer-empty"><span class="big-icon">📹</span><p>请选择一个会话开始观看</p></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { cobrowseApi } from '../api'
import type { LiveSession } from '../types'

const router = useRouter()

// ==================== State ====================
const liveSessions = ref<LiveSession[]>([])
const selectedSession = ref<LiveSession | null>(null)
const selectedSessionId = ref('')
const wsConnected = ref(false)
const connecting = ref(false)
const eventCount = ref(0)
const reconnecting = ref(false)
const reconnectAttempts = ref(0)
const maxReconnectAttempts = 10

const webrtcActive = ref(false)
const webrtcState = ref<'idle' | 'requesting' | 'connecting' | 'connected'>('idle')
const controlMode = ref(false)
const zoomLevel = ref(100)
const isFullscreen = ref(false)

// Template refs
const replayContainerRef = ref<HTMLElement>()
const webrtcVideoRef = ref<HTMLVideoElement>()
const webrtcWrapperRef = ref<HTMLElement>()
const controlCanvasRef = ref<HTMLCanvasElement>()
const viewerContentRef = ref<HTMLElement>()
const liveViewerRef = ref<HTMLElement>()
const liveViewRef = ref<HTMLElement>()

// Internal state
let ws: WebSocket | null = null
let replayer: any = null
let allEvents: any[] = []
let heartbeatTimer: ReturnType<typeof setInterval> | null = null
let refreshTimer: ReturnType<typeof setInterval> | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let rebuildTimer: ReturnType<typeof setTimeout> | null = null
let peerConnection: RTCPeerConnection | null = null
let dataChannel: RTCDataChannel | null = null
let keydownHandler: ((e: KeyboardEvent) => void) | null = null
let keyupHandler: ((e: KeyboardEvent) => void) | null = null

const debugMode = ref(false)
const LOG = (...args: any[]) => { if (debugMode.value) console.log('[Live]', ...args) }
const LOG_ERR = (...args: any[]) => console.error('[Live]', ...args)

const rtcConfig: RTCConfiguration = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    { urls: 'turn:14.103.85.111:3478?transport=udp', username: 'logmon', credential: 'logmon2024turn' },
    { urls: 'turn:14.103.85.111:3478?transport=tcp', username: 'logmon', credential: 'logmon2024turn' }
  ]
}

// ==================== Lifecycle ====================

onMounted(() => {
  debugMode.value = !!(window as any).__LOGMON_DEBUG__
  refreshSessions()
  refreshTimer = setInterval(refreshSessions, 5000)
  document.addEventListener('fullscreenchange', onFullscreenChange)
  loadRRWebReplayer()
})

onUnmounted(() => {
  cleanup()
  if (refreshTimer) clearInterval(refreshTimer)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  removeKeyboardListeners()
})

// ==================== rrweb Replayer Loading ====================

function loadRRWebReplayer() {
  if ((window as any).rrweb?.Replayer) return
  LOG('Loading rrweb Replayer...')
  import('rrweb/lib/replay/rrweb-replay.js').then(mod => {
    LOG('rrweb Replayer loaded')
    ;(window as any).rrweb = { Replayer: mod.Replayer }
    // If we have events waiting, rebuild
    if (allEvents.length >= 2 && !webrtcActive.value) rebuildReplayer()
  }).catch(err => LOG_ERR('rrweb load failed:', err))
}

function rebuildReplayer() {
  const container = replayContainerRef.value
  if (!container || allEvents.length < 2) return

  const ReplayerClass = (window as any).rrweb?.Replayer
  if (!ReplayerClass) {
    LOG('Replayer not available, retrying in 2s...')
    if (rebuildTimer) clearTimeout(rebuildTimer)
    rebuildTimer = setTimeout(() => { rebuildTimer = null; rebuildReplayer() }, 2000)
    return
  }

  // Destroy old replayer
  if (replayer) { try { replayer.pause() } catch {} replayer = null }
  container.innerHTML = ''

  try {
    replayer = new ReplayerClass(allEvents, {
      root: container,
      UNSAFE_replayCanvas: true,
      mouseTail: false
    })
    const lastTime = allEvents[allEvents.length - 1]?.timestamp || 0
    replayer.play(lastTime)
  } catch (err) {
    LOG_ERR('Replayer error:', err)
    replayer = null
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
  cleanupWebRTC()
  selectedSessionId.value = session.sessionId
  selectedSession.value = session
  allEvents = []
  eventCount.value = 0
  if (replayer) { try { replayer.pause() } catch {} replayer = null }
  connectToSession(session.sessionId)
}

function connectToSession(sessionId: string) {
  cleanup()
  connecting.value = true

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('logmon_token')
  const wsUrl = new URL(`${proto}//${location.host}/ws/cobrowse/${sessionId}/view`)
  if (token) wsUrl.searchParams.set('token', token)

  LOG('Connecting:', wsUrl.toString())
  ws = new WebSocket(wsUrl.toString())

  ws.onopen = () => {
    LOG('WS open')
    wsConnected.value = true
    connecting.value = false
    reconnecting.value = false
    reconnectAttempts.value = 0
    if (webrtcState.value !== 'connected') webrtcState.value = 'idle'
    heartbeatTimer = setInterval(() => {
      if (ws?.readyState === WebSocket.OPEN) ws.send('{"type":"pong"}')
    }, 25000)
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'ping') { ws?.send('{"type":"pong"}'); return }
      if (msg.type === 'session-removed') {
        LOG('Session removed by server')
        cleanup()
        selectedSessionId.value = ''
        selectedSession.value = null
        return
      }
      handleWSMessage(msg)
    } catch {}
  }

  ws.onclose = (e) => {
    LOG('WS closed, code:', e.code)
    wsConnected.value = false
    connecting.value = false
    if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
    if (e.code !== 1000) scheduleReconnect()
  }

  ws.onerror = () => { connecting.value = false }
}

function handleWSMessage(msg: any) {
  // WebRTC signaling
  if (msg.type === 'webrtc-offer') { handleWebRTCOffer(msg.sdp); return }
  if (msg.type === 'webrtc-ice') { handleICECandidate(msg.candidate); return }
  if (msg.type === 'webrtc-rejected') { webrtcState.value = 'idle'; return }
  if (msg.type === 'webrtc-stop') { cleanupWebRTC(); return }

  // rrweb events
  if (msg.type === 'rrweb-full-snapshot') {
    allEvents = [msg.data]
    eventCount.value = 1
    if (rebuildTimer) clearTimeout(rebuildTimer)
    if (!webrtcActive.value) {
      rebuildTimer = setTimeout(() => { rebuildTimer = null; rebuildReplayer() }, 500)
    }
  } else if (msg.type === 'rrweb-event') {
    const newEvents = Array.isArray(msg.data) ? msg.data : [msg.data]
    allEvents.push(...newEvents)
    eventCount.value = allEvents.length
    if (!rebuildTimer && !webrtcActive.value) {
      rebuildTimer = setTimeout(() => { rebuildTimer = null; rebuildReplayer() }, 500)
    }
  }
}

// ==================== WebRTC ====================

function requestIntervene() {
  if (!ws || ws.readyState !== WebSocket.OPEN) return
  LOG('Requesting intervene')
  webrtcState.value = 'requesting'
  ws.send(JSON.stringify({ type: 'webrtc-offer-request' }))
}

async function handleWebRTCOffer(sdp: RTCSessionDescriptionInit) {
  LOG('Handling offer')
  webrtcState.value = 'connecting'

  try {
    peerConnection = new RTCPeerConnection(rtcConfig)

    peerConnection.ontrack = (event) => {
      const video = webrtcVideoRef.value
      if (!video) return
      const stream = event.streams[0] || new MediaStream([event.track])
      video.srcObject = stream
      webrtcActive.value = true
      webrtcState.value = 'connected'
      video.play().catch(() => {})
      nextTick(() => resizeControlCanvas())
    }

    peerConnection.ondatachannel = (event) => { dataChannel = event.channel }

    peerConnection.onicecandidate = (e) => {
      if (e.candidate) {
        LOG('Local ICE:', e.candidate.type, e.candidate.address || 'hidden')
        ws?.send(JSON.stringify({ type: 'webrtc-ice', candidate: e.candidate.toJSON() }))
      }
    }

    peerConnection.onconnectionstatechange = () => {
      const s = peerConnection?.connectionState
      LOG('PC state:', s)
      if (s === 'disconnected' || s === 'failed' || s === 'closed') cleanupWebRTC()
    }

    peerConnection.onicecandidate = (e) => {
      if (e.candidate) {
        LOG('Local ICE:', e.candidate.type, e.candidate.address || 'hidden')
        ws?.send(JSON.stringify({ type: 'webrtc-ice', candidate: e.candidate.toJSON() }))
      }
    }

    peerConnection.oniceconnectionstatechange = () => {
      LOG('ICE state:', peerConnection?.iceConnectionState)
    }

    // Log selected candidate pair to see if direct or relay
    peerConnection.getStats().then(() => {
      setInterval(async () => {
        if (!peerConnection) return
        const stats = await peerConnection.getStats()
        stats.forEach((report) => {
          if (report.type === 'candidate-pair' && report.state === 'succeeded') {
            LOG('ICE pair:', report.localCandidateId, '→', report.remoteCandidateId,
              'type:', report.candidateType || 'unknown')
          }
        })
      }, 5000)
    })

    await peerConnection.setRemoteDescription(new RTCSessionDescription(sdp))
    const answer = await peerConnection.createAnswer()
    await peerConnection.setLocalDescription(answer)
    ws?.send(JSON.stringify({ type: 'webrtc-answer', sdp: peerConnection.localDescription }))
  } catch (err) {
    LOG_ERR('WebRTC setup failed:', err)
    cleanupWebRTC()
  }
}

async function handleICECandidate(candidate: RTCIceCandidateInit) {
  if (!peerConnection) return
  try { await peerConnection.addIceCandidate(new RTCIceCandidate(candidate)) } catch {}
}

function stopIntervene() {
  if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'webrtc-stop' }))
  cleanupWebRTC()
}

function cleanupWebRTC() {
  if (dataChannel) { try { dataChannel.close() } catch {} dataChannel = null }
  if (peerConnection) { try { peerConnection.close() } catch {} peerConnection = null }
  if (webrtcVideoRef.value) webrtcVideoRef.value.srcObject = null
  webrtcActive.value = false
  webrtcState.value = 'idle'
  controlMode.value = false
  removeKeyboardListeners()
}

// ==================== Control ====================

function toggleControlMode() {
  controlMode.value = !controlMode.value
  if (controlMode.value) {
    setupKeyboardListeners()
    nextTick(() => resizeControlCanvas())
  } else {
    removeKeyboardListeners()
  }
}

function resizeControlCanvas() {
  const canvas = controlCanvasRef.value
  const video = webrtcVideoRef.value
  if (!canvas || !video) return
  const rect = video.getBoundingClientRect()
  canvas.width = rect.width
  canvas.height = rect.height
  canvas.style.width = rect.width + 'px'
  canvas.style.height = rect.height + 'px'
}

function getVideoCoords(e: MouseEvent): { x: number; y: number } | null {
  const video = webrtcVideoRef.value
  if (!video || !video.videoWidth) return null
  const r = video.getBoundingClientRect()
  return {
    x: Math.round((e.clientX - r.left) * video.videoWidth / r.width),
    y: Math.round((e.clientY - r.top) * video.videoHeight / r.height)
  }
}

function sendControl(cmd: any) {
  if (dataChannel?.readyState === 'open') dataChannel.send(JSON.stringify({ type: 'control', ...cmd }))
  else if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'control', ...cmd }))
}

function onControlMouseDown(e: MouseEvent) { const c = getVideoCoords(e); if (c) sendControl({ action: 'click', x: c.x, y: c.y, button: e.button }) }
function onControlDblClick(e: MouseEvent) { const c = getVideoCoords(e); if (c) sendControl({ action: 'dblclick', x: c.x, y: c.y }) }
function onControlContextMenu(e: MouseEvent) { const c = getVideoCoords(e); if (c) sendControl({ action: 'contextmenu', x: c.x, y: c.y }) }
function onControlWheel(e: WheelEvent) { sendControl({ action: 'scroll', deltaX: Math.round(e.deltaX), deltaY: Math.round(e.deltaY) }) }

function setupKeyboardListeners() {
  keydownHandler = (e) => {
    if (!controlMode.value || !webrtcActive.value || e.ctrlKey || e.metaKey || e.altKey) return
    e.preventDefault()
    sendControl({ action: 'keydown', key: e.key })
  }
  keyupHandler = (e) => {
    if (!controlMode.value || !webrtcActive.value || e.ctrlKey || e.metaKey || e.altKey) return
    sendControl({ action: 'keyup', key: e.key })
  }
  document.addEventListener('keydown', keydownHandler)
  document.addEventListener('keyup', keyupHandler)
}

function removeKeyboardListeners() {
  if (keydownHandler) { document.removeEventListener('keydown', keydownHandler); keydownHandler = null }
  if (keyupHandler) { document.removeEventListener('keyup', keyupHandler); keyupHandler = null }
}

// ==================== Zoom & Fullscreen ====================

function zoomIn() { zoomLevel.value = Math.min(zoomLevel.value + 25, 300) }
function zoomOut() { zoomLevel.value = Math.max(zoomLevel.value - 25, 50) }
function zoomReset() { zoomLevel.value = 100 }
function toggleFullscreen() {
  if (isFullscreen.value) document.exitFullscreen()
  else liveViewerRef.value?.requestFullscreen()
}
function onFullscreenChange() { isFullscreen.value = !!document.fullscreenElement }

// ==================== Cleanup & Reconnect ====================

function cleanup() {
  if (replayer) { try { replayer.pause() } catch {} replayer = null }
  if (ws) { ws.close(); ws = null }
  wsConnected.value = false
  connecting.value = false
  if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  if (rebuildTimer) { clearTimeout(rebuildTimer); rebuildTimer = null }
  removeKeyboardListeners()
}

function disconnect() {
  cleanupWebRTC()
  cleanup()
  selectedSessionId.value = ''
  selectedSession.value = null
}

function scheduleReconnect() {
  if (reconnectTimer || !selectedSession.value) return
  if (reconnectAttempts.value >= maxReconnectAttempts) {
    reconnecting.value = false
    reconnectAttempts.value = 0
    return
  }
  reconnectAttempts.value++
  reconnecting.value = true
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.value - 1), 30000)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    if (!selectedSession.value) return
    const alive = liveSessions.value.some(s => s.sessionId === selectedSession.value!.sessionId)
    if (alive) connectToSession(selectedSession.value.sessionId)
    else { reconnecting.value = false; cleanup(); selectedSessionId.value = ''; selectedSession.value = null }
  }, delay)
}

// ==================== Helpers ====================

function openOriginalPage() { if (selectedSession.value?.url) window.open(selectedSession.value.url, '_blank') }
function formatUrl(url: string): string { try { return new URL(url).pathname + new URL(url).search } catch { return url } }
function formatUA(ua: string): string {
  if (ua.includes('Edg')) return 'Edge'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Firefox')) return 'Firefox'
  return 'Other'
}
function formatDuration(startMs: number): string {
  const d = Date.now() - startMs
  return `${Math.floor(d / 60000)}m ${Math.floor((d % 60000) / 1000)}s`
}
</script>

<style scoped>
.live-view { padding: 20px; height: calc(100vh - 40px); display: flex; flex-direction: column; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; color: var(--color-text); }
.live-layout { display: flex; gap: 20px; flex: 1; min-height: 0; margin-top: 16px; }

.page-header { display: flex; align-items: center; gap: 12px; }
.page-header h2 { margin: 0; font-size: 18px; font-weight: 600; display: flex; align-items: center; gap: 8px; }
.back-btn { background: none; border: 1px solid var(--color-border); border-radius: 6px; padding: 6px 14px; cursor: pointer; font-size: 13px; color: var(--color-text-secondary); }
.back-btn:hover { background: var(--color-bg-secondary); }
.online-badge { background: #22c55e; color: white; font-size: 12px; padding: 2px 10px; border-radius: 10px; font-weight: 500; }

.session-list { width: 320px; background: var(--color-bg-secondary); border-radius: 8px; border: 1px solid var(--color-border); display: flex; flex-direction: column; }
.session-list-header { padding: 14px 16px; border-bottom: 1px solid var(--color-border); display: flex; align-items: center; justify-content: space-between; }
.session-list-header h3 { margin: 0; font-size: 15px; }
.refresh-btn { background: none; border: none; cursor: pointer; font-size: 16px; padding: 4px 8px; border-radius: 4px; }
.refresh-btn:hover { background: var(--color-bg-tertiary); }
.session-list-content { flex: 1; overflow-y: auto; padding: 8px; }
.session-item { padding: 12px; border-radius: 6px; cursor: pointer; border: 1px solid transparent; margin-bottom: 4px; transition: all 0.15s; }
.session-item:hover { background: var(--color-bg-tertiary); }
.session-item.active { background: rgba(99,102,241,0.15); border-color: #6366f1; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.status-dot.online { background: #22c55e; animation: pulse 2s infinite; }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }
.app-name { font-weight: 500; font-size: 14px; }
.session-info { font-size: 12px; color: var(--color-text-secondary); margin-top: 6px; }
.url { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.meta { display: flex; gap: 8px; margin-top: 4px; }
.empty-state { text-align: center; padding: 40px; color: var(--color-text-secondary); }

.live-viewer { flex: 1; background: var(--color-bg-secondary); border-radius: 8px; border: 1px solid var(--color-border); display: flex; flex-direction: column; overflow: hidden; }
.live-viewer:fullscreen { background: #0a0e27; border-radius: 0; }
.live-viewer:fullscreen .viewer-content { border-radius: 0; }
.live-viewer:fullscreen .viewer-toolbar { background: rgba(10,14,39,0.95); border-color: rgba(255,255,255,0.1); color: #e2e8f0; }

.viewer-container { display: flex; flex-direction: column; height: 100%; }
.viewer-toolbar { padding: 10px 16px; border-bottom: 1px solid var(--color-border); display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
.toolbar-left { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.toolbar-right { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

.ws-status { font-size: 13px; display: flex; align-items: center; gap: 6px; font-weight: 500; }
.ws-status.connected { color: #22c55e; }
.ws-status.reconnecting { color: #f59e0b; }
.ws-status.disconnected { color: #ef4444; }
.live-dot { width: 6px; height: 6px; border-radius: 50%; background: #22c55e; display: inline-block; animation: pulse 1.5s infinite; }
.event-badge { font-size: 11px; color: var(--color-text-secondary); background: var(--color-bg-tertiary); padding: 2px 8px; border-radius: 10px; }
.status-warn { font-size: 13px; color: #f59e0b; }

.btn { padding: 6px 14px; border: 1px solid var(--color-border); border-radius: 6px; cursor: pointer; font-size: 13px; background: var(--color-bg); color: var(--color-text); transition: all 0.15s; }
.btn:hover { filter: brightness(1.1); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.btn-primary { background: #6366f1; color: white; border-color: #6366f1; }
.btn-danger { background: #ef4444; color: white; border-color: #ef4444; }
.btn-warning { background: #f59e0b; color: white; border-border: #f59e0b; }
.btn-default { background: var(--color-bg); color: var(--color-text); }

.zoom-controls { display: flex; align-items: center; gap: 4px; }
.zoom-label { font-size: 12px; color: var(--color-text-secondary); min-width: 40px; text-align: center; }

.viewer-content { flex: 1; position: relative; background: #1a1a1a; overflow: hidden; min-height: 0; }
.replay-container { width: 100%; height: 100%; overflow: hidden; position: relative; }
.replay-container :deep(.replayer-wrapper) { width: 100% !important; height: 100% !important; }
.replay-container :deep(.replayer-wrapper iframe) { width: 100% !important; height: 100% !important; display: block !important; visibility: visible !important; opacity: 1 !important; }

.webrtc-wrapper { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; overflow: auto; }
.webrtc-scale-container { position: relative; transition: transform 0.2s ease; width: 100%; }
.webrtc-video { display: block; width: 100%; min-height: 300px; max-height: 80vh; height: auto; background: #000; object-fit: contain; }
.control-overlay { position: absolute; top: 0; left: 0; width: 100%; height: 100%; cursor: crosshair; z-index: 10; background: transparent; }

.viewer-overlay { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; background: rgba(0,0,0,0.6); font-size: 14px; color: #94a3b8; z-index: 10; }
.viewer-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; color: var(--color-text-secondary); }
.big-icon { font-size: 60px; }
.spinner { width: 32px; height: 32px; border: 3px solid rgba(255,255,255,0.2); border-top: 3px solid #6366f1; border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
