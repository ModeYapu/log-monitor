/**
 * Co-browsing module for LogMonitor SDK
 * Provides real-time screen sharing (rrweb + WebRTC) and remote control capabilities
 */

import { initPrivacy, shouldMaskElement, maskElementValue, shouldAllowDomain } from './privacy';

interface CoBrowseConfig {
	enabled: boolean;
	wsUrl: string;
	sessionId: string;
	appId: string;
	onControlCommand?: (command: ControlCommand) => void;
	onStatusChange?: (status: CoBrowseStatus) => void;
	privacy?: {
		maskInputs?: boolean;
		maskSelectors?: string[];
		allowedDomains?: string[];
		blockDomains?: string[];
	};
}

type CoBrowseStatus = 'disconnected' | 'connecting' | 'connected' | 'controlling';

interface ControlCommand {
	action: 'click' | 'dblclick' | 'contextmenu' | 'mousemove' | 'input' | 'scroll' | 'keydown' | 'keyup' | 'navigate';
	x?: number;
	y?: number;
	selector?: string;
	value?: string;
	key?: string;
	url?: string;
	button?: number;
	deltaX?: number;
	deltaY?: number;
}

let config: CoBrowseConfig | null = null;
let ws: WebSocket | null = null;
let status: CoBrowseStatus = 'disconnected';
let recorder: (() => void) | null = null;
let eventBuffer: any[] = [];
let reconnectTimer: number | null = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;
let uiWidget: HTMLElement | null = null;
let fullSnapshotSent = false;
let flushTimer: ReturnType<typeof setInterval> | null = null;

// WebRTC state
let peerConnection: RTCPeerConnection | null = null;
let localStream: MediaStream | null = null;
let dataChannel: RTCDataChannel | null = null;
let webrtcActive = false;
let controlEnabled = false;
let adminCursor: HTMLElement | null = null;
let connectedAt = 0;

// Debug mode — set via localStorage or config
let debugMode = false;

const rtcConfig: RTCConfiguration = {
	iceServers: [
		{ urls: 'stun:stun.l.google.com:19302' },
		{ urls: 'stun:stun1.l.google.com:19302' },
		{
			urls: 'turn:14.103.85.111:3478?transport=udp',
			username: 'logmon',
			credential: 'logmon2024turn'
		},
		{
			urls: 'turn:14.103.85.111:3478?transport=tcp',
			username: 'logmon',
			credential: 'logmon2024turn'
		}
	]
};

// ==================== Logging ====================

function LOG(...args: any[]): void {
	if (debugMode) console.log('[CoBrowse]', ...args);
}

function LOG_ERR(...args: any[]): void {
	console.error('[CoBrowse]', ...args);
}

// ==================== Public API ====================

/**
 * Initialize cobrowsing module
 */
export function initCoBrowse(cfg: CoBrowseConfig, autoStart: boolean = true): void {
	if (!cfg.enabled) return;

	debugMode = !!(window as any).__LOGMON_DEBUG__;

	config = {
		onControlCommand: defaultControlHandler,
		onStatusChange: () => {},
		...cfg
	};

	if (cfg.privacy) initPrivacy(cfg.privacy);

	// Load rrweb for recording (CDN provides global window.rrweb)
	loadRRWeb().then(() => {
		LOG('Initialized');
		if (autoStart) start().catch(err => LOG_ERR('Auto-start failed:', err));
	});
}

export function start(): Promise<void> {
	if (!config) return Promise.reject(new Error('Not initialized'));
	if (status === 'connected') return Promise.resolve();

	setStatus('connecting');

	return new Promise((resolve, reject) => {
		try {
			const url = `${config!.wsUrl}/${config!.sessionId}?appId=${encodeURIComponent(config!.appId)}&ua=${encodeURIComponent(navigator.userAgent)}&url=${encodeURIComponent(window.location.href)}`;
			ws = new WebSocket(url);

			ws.onopen = () => {
				LOG('Connected');
				reconnectAttempts = 0;
				connectedAt = Date.now();
				setStatus('connected');
				startRecording();
				startFlushTimer();
				resolve();
			};

			ws.onmessage = (event) => handleMessage(event.data);
			ws.onclose = () => {
				LOG('Disconnected');
				setStatus('disconnected');
				stopRecording();
				stopFlushTimer();
				scheduleReconnect();
			};
			ws.onerror = (error) => reject(error);
		} catch (err) {
			reject(err);
		}
	});
}

export function stop(): void {
	if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
	stopWebRTC();
	stopFlushTimer();
	if (ws) { ws.close(); ws = null; }
	stopRecording();
	hideWidget();
	setStatus('disconnected');
}

export function getStatus(): CoBrowseStatus { return status; }

export function setControlMode(enabled: boolean): void {
	controlEnabled = enabled;
	if (uiWidget) updateWidgetContent();
}

// ==================== Recording ====================

function loadRRWeb(): Promise<void> {
	return new Promise((resolve) => {
		if ((window as any).rrweb?.record) { resolve(); return; }

		const script = document.createElement('script');
		script.src = 'https://cdn.jsdelivr.net/npm/rrweb@2.0.0/dist/rrweb.min.js';
		script.onload = () => {
			LOG('rrweb loaded, record:', typeof (window as any).rrweb?.record);
			resolve();
		};
		script.onerror = () => { LOG_ERR('Failed to load rrweb'); resolve(); };
		document.head.appendChild(script);
	});
}

function startRecording(): void {
	const rrweb = (window as any).rrweb;
	if (!rrweb?.record) { LOG_ERR('rrweb not loaded, cannot record'); return; }

	const maskSelectors = config?.privacy?.maskSelectors;
	recorder = rrweb.record({
		emit: emitEvent,
		recordCanvas: true,
		recordCrossOriginIframes: true,
		recordAfter: document.readyState === 'complete' ? 0 : 500,
		maskAllInputs: config?.privacy?.maskInputs ?? false,
		maskTextSelector: maskSelectors?.length ? maskSelectors.join(', ') : undefined,
		inlineStylesheet: true,
		inlineImages: false,
		useCompression: false,
		sampling: {
			mousemove: true,
			mouseInteraction: true,
			scroll: true,
			input: 'all' as const,
			media: false
		},
		dataURLOptions: { type: 'image/jpeg', quality: 0.6 },
		hooks: {
			beforeEmit: (event: any) => {
				if (event.data?.attributes?.type === 'password') {
					event.data.attributes.value = '••••••••';
				}
				return event;
			}
		}
	} as any);
	LOG('Recording started');
}

function stopRecording(): void {
	if (recorder) { try { recorder(); } catch {} recorder = null; }
	fullSnapshotSent = false;
	eventBuffer = [];
}

function emitEvent(event: any): void {
	if (!ws || ws.readyState !== WebSocket.OPEN) return;

	if (event.type === 0 || event.type === 'Meta') {
		fullSnapshotSent = true;
		sendMessage({ type: 'rrweb-full-snapshot', data: event });
	} else {
		eventBuffer.push(event);
		if (eventBuffer.length >= 10) flushEvents();
	}
}

function startFlushTimer(): void {
	stopFlushTimer();
	flushTimer = setInterval(() => {
		if (status === 'connected') flushEvents();
	}, 500);
}

function stopFlushTimer(): void {
	if (flushTimer) { clearInterval(flushTimer); flushTimer = null; }
}

function flushEvents(): void {
	if (eventBuffer.length === 0 || !ws || ws.readyState !== WebSocket.OPEN) return;
	sendMessage({ type: 'rrweb-event', data: eventBuffer });
	eventBuffer = [];
}

// ==================== WebSocket Messaging ====================

function sendMessage(msg: any): void {
	if (!ws || ws.readyState !== WebSocket.OPEN) return;
	try {
		ws.send(JSON.stringify(msg));
	} catch (err) {
		LOG_ERR('Send failed:', err);
	}
}

function handleMessage(data: string): void {
	try {
		const msg = JSON.parse(data);

		switch (msg.type) {
			case 'ping':
				sendMessage({ type: 'pong' });
				return;
			case 'webrtc-offer-request':
				handleOfferRequest();
				return;
			case 'webrtc-answer':
				if (msg.sdp) handleWebRTCAnswer(msg.sdp);
				return;
			case 'webrtc-ice':
				if (msg.candidate) handleICECandidate(msg.candidate);
				return;
			case 'webrtc-stop':
				stopWebRTC();
				return;
			case 'control':
				handleControlCommand(msg);
				return;
		}
	} catch (err) {
		LOG_ERR('Message parse error:', err);
	}
}

function handleOfferRequest(): void {
	const timeSinceConnect = Date.now() - connectedAt;
	if (timeSinceConnect < 3000) {
		LOG('Ignoring stale offer-request (connected', timeSinceConnect, 'ms ago)');
		sendMessage({ type: 'webrtc-rejected' });
		return;
	}
	LOG('Offer request received, showing dialog...');
	handleWebRTCRequest();
}

// ==================== WebRTC ====================

async function handleWebRTCRequest(): Promise<void> {
	if (!ws || ws.readyState !== WebSocket.OPEN) return;

	try {
		const accepted = await showInterventionDialog();
		if (!accepted) {
			sendMessage({ type: 'webrtc-rejected' });
			return;
		}

		// Get screen stream — prefer current tab
		const opts: any = {
			video: { cursor: 'always', displaySurface: 'browser', selfBrowserSurface: 'include' },
			audio: false,
			preferCurrentTab: true,
			systemAudio: 'exclude'
		};

		try {
			localStream = await navigator.mediaDevices.getDisplayMedia(opts);
		} catch {
			localStream = await navigator.mediaDevices.getDisplayMedia({ video: { cursor: 'always' }, audio: false });
		}

		localStream.getVideoTracks()[0].onended = () => {
			LOG('Screen sharing stopped by user');
			stopWebRTC();
			sendMessage({ type: 'webrtc-stop' });
		};

		peerConnection = new RTCPeerConnection(rtcConfig);

		localStream.getTracks().forEach(track => {
			if (peerConnection) peerConnection.addTrack(track, localStream!);
		});

		dataChannel = peerConnection.createDataChannel('control', { ordered: true, maxRetransmits: 3 });
		dataChannel.onmessage = (e) => {
			try {
				const cmd = JSON.parse(e.data);
				if (cmd.type === 'control') handleControlCommand(cmd);
			} catch {}
		};
		dataChannel.onopen = () => LOG('DataChannel open');

		peerConnection.onicecandidate = (e) => {
			if (e.candidate) sendMessage({ type: 'webrtc-ice', candidate: e.candidate.toJSON() });
		};

		peerConnection.onconnectionstatechange = () => {
			const s = peerConnection?.connectionState;
			LOG('Connection state:', s);
			if (s === 'disconnected' || s === 'failed' || s === 'closed') stopWebRTC();
		};

		peerConnection.onnegotiationneeded = () => createAndSendOffer();

		// Fallback offer if onnegotiationneeded doesn't fire
		setTimeout(() => {
			if (peerConnection && !peerConnection.localDescription) createAndSendOffer();
		}, 1000);

	} catch (err) {
		LOG_ERR('WebRTC failed:', err);
		sendMessage({ type: 'webrtc-rejected' });
		cleanupWebRTC();
	}
}

async function createAndSendOffer(): Promise<void> {
	if (!peerConnection || peerConnection.localDescription) return;
	try {
		const offer = await peerConnection.createOffer();
		await peerConnection.setLocalDescription(offer);
		sendMessage({ type: 'webrtc-offer', sdp: peerConnection.localDescription });
		webrtcActive = true;
		showWidget();
		LOG('Offer sent');
	} catch (err) {
		LOG_ERR('Offer failed:', err);
	}
}

async function handleWebRTCAnswer(sdp: RTCSessionDescriptionInit): Promise<void> {
	if (!peerConnection) return;
	try {
		await peerConnection.setRemoteDescription(new RTCSessionDescription(sdp));
		LOG('Answer set');
	} catch (err) {
		LOG_ERR('Set remote desc failed:', err);
	}
}

async function handleICECandidate(candidate: RTCIceCandidateInit): Promise<void> {
	if (!peerConnection) return;
	try { await peerConnection.addIceCandidate(new RTCIceCandidate(candidate)); } catch {}
}

function stopWebRTC(): void {
	cleanupWebRTC();
	webrtcActive = false;
	updateWidgetStatus();
}

function cleanupWebRTC(): void {
	if (dataChannel) { try { dataChannel.close(); } catch {} dataChannel = null; }
	if (peerConnection) { try { peerConnection.close(); } catch {} peerConnection = null; }
	if (localStream) { localStream.getTracks().forEach(t => t.stop()); localStream = null; }
	if (adminCursor) { adminCursor.remove(); adminCursor = null; }
	webrtcActive = false;
	hideWidget();
}

// ==================== Intervention Dialog ====================

function showInterventionDialog(): Promise<boolean> {
	return new Promise((resolve) => {
		const overlay = document.createElement('div');
		overlay.id = 'logmonitor-intervention-dialog';
		overlay.style.cssText = `position:fixed;inset:0;background:rgba(0,0,0,0.5);z-index:9999999;display:flex;align-items:center;justify-content:center;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;`;

		overlay.innerHTML = `
			<div style="background:white;border-radius:16px;padding:28px;max-width:420px;width:90%;box-shadow:0 20px 60px rgba(0,0,0,0.3);">
				<div style="display:flex;align-items:center;gap:12px;margin-bottom:16px;">
					<div style="width:48px;height:48px;border-radius:12px;background:linear-gradient(135deg,#6366f1,#8b5cf6);display:flex;align-items:center;justify-content:center;font-size:24px;">🎯</div>
					<div>
						<h3 style="margin:0;font-size:18px;color:#1a1a2e;">远程协助请求</h3>
						<p style="margin:4px 0 0;color:#888;font-size:13px;">管理员请求共享您的屏幕</p>
					</div>
				</div>
				<p style="color:#555;margin:0 0 24px;line-height:1.6;font-size:14px;">管理员希望查看您的屏幕画面并进行远程操作。您可以随时停止共享。</p>
				<div style="display:flex;gap:12px;justify-content:flex-end;">
					<button id="lm-reject" style="padding:10px 24px;border:1px solid #e0e0e0;border-radius:8px;cursor:pointer;font-size:14px;background:white;color:#666;">拒绝</button>
					<button id="lm-accept" style="padding:10px 24px;background:linear-gradient(135deg,#6366f1,#8b5cf6);color:white;border:none;border-radius:8px;cursor:pointer;font-size:14px;font-weight:500;box-shadow:0 4px 12px rgba(99,102,241,0.3);">允许共享</button>
				</div>
			</div>`;

		document.body.appendChild(overlay);
		overlay.querySelector('#lm-accept')!.addEventListener('click', () => { overlay.remove(); resolve(true); });
		overlay.querySelector('#lm-reject')!.addEventListener('click', () => { overlay.remove(); resolve(false); });
	});
}

// ==================== Control Execution ====================

function handleControlCommand(cmd: any): void {
	const command: ControlCommand = { action: cmd.action, x: cmd.x, y: cmd.y, selector: cmd.selector, value: cmd.value, key: cmd.key, url: cmd.url, button: cmd.button, deltaX: cmd.deltaX, deltaY: cmd.deltaY };

	if (cmd.action === 'mousemove' && cmd.x != null && cmd.y != null) showAdminCursor(cmd.x, cmd.y);
	else showControlFeedback(command);

	executeControlCommand(command);
	config?.onControlCommand?.(command);
}

function executeControlCommand(command: ControlCommand): void {
	const el = (command.x != null && command.y != null) ? document.elementFromPoint(command.x, command.y) : null;

	switch (command.action) {
		case 'click':
			if (el) {
				el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, clientX: command.x, clientY: command.y, button: command.button || 0 }));
				el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, clientX: command.x, clientY: command.y, button: command.button || 0 }));
				el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: command.x, clientY: command.y, button: command.button || 0 }));
			}
			break;
		case 'dblclick':
			if (el) el.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, cancelable: true, clientX: command.x, clientY: command.y }));
			break;
		case 'contextmenu':
			if (el) el.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: command.x, clientY: command.y, button: 2 }));
			break;
		case 'input':
			if (command.selector) {
				const input = document.querySelector(command.selector) as HTMLInputElement;
				if (input) {
					input.focus();
					input.value = command.value || '';
					input.dispatchEvent(new Event('input', { bubbles: true }));
					input.dispatchEvent(new Event('change', { bubbles: true }));
				}
			}
			break;
		case 'scroll':
			window.scrollBy(command.deltaX || 0, command.deltaY || 0);
			break;
		case 'keydown':
			document.dispatchEvent(new KeyboardEvent('keydown', { key: command.key, bubbles: true, cancelable: true }));
			break;
		case 'keyup':
			document.dispatchEvent(new KeyboardEvent('keyup', { key: command.key, bubbles: true, cancelable: true }));
			break;
		case 'navigate':
			if (command.url) {
				if (confirm(`技术支持请求导航到: ${command.url}\n\n是否同意？`)) window.location.href = command.url;
			}
			break;
	}
}

// ==================== Admin Cursor ====================

function showAdminCursor(x: number, y: number): void {
	if (!adminCursor) {
		adminCursor = document.createElement('div');
		adminCursor.id = 'logmonitor-admin-cursor';
		adminCursor.style.cssText = 'position:fixed;width:20px;height:20px;pointer-events:none;z-index:999998;transition:left 0.05s linear,top 0.05s linear;';
		adminCursor.innerHTML = '<svg width="20" height="20" viewBox="0 0 20 20" fill="none"><path d="M3 1L17 10L10 11L7 18L3 1Z" fill="#3b82f6" stroke="white" stroke-width="1.5"/></svg>';
		document.body.appendChild(adminCursor);
	}
	adminCursor.style.left = `${x}px`;
	adminCursor.style.top = `${y}px`;
}

function showControlFeedback(command: ControlCommand): void {
	if (command.x == null || command.y == null) return;
	if (!['click', 'dblclick', 'contextmenu'].includes(command.action)) return;

	const fb = document.createElement('div');
	fb.style.cssText = `position:fixed;left:${command.x - 15}px;top:${command.y - 15}px;width:30px;height:30px;background:rgba(99,102,241,0.2);border:2px solid rgba(99,102,241,0.6);border-radius:50%;pointer-events:none;z-index:999999;animation:cb-fb 0.4s ease-out forwards;`;

	if (!document.getElementById('cb-fb-style')) {
		const s = document.createElement('style');
		s.id = 'cb-fb-style';
		s.textContent = '@keyframes cb-fb{0%{opacity:1;transform:scale(.5)}100%{opacity:0;transform:scale(1.5)}}';
		document.head.appendChild(s);
	}
	document.body.appendChild(fb);
	setTimeout(() => fb.remove(), 400);
}

// ==================== Widget ====================

function showWidget(): void {
	if (uiWidget) return;
	uiWidget = document.createElement('div');
	uiWidget.id = 'logmonitor-cobrowse-widget';
	uiWidget.style.cssText = `position:fixed;bottom:20px;right:20px;background:white;border-radius:12px;box-shadow:0 4px 24px rgba(0,0,0,0.12);padding:14px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:13px;z-index:999998;min-width:220px;border:1px solid rgba(0,0,0,0.08);`;
	updateWidgetContent();
	makeDraggable(uiWidget);
	document.body.appendChild(uiWidget);
}

function updateWidgetStatus(): void { if (uiWidget) updateWidgetContent(); }

function updateWidgetContent(): void {
	if (!uiWidget) return;
	const modeText = webrtcActive ? '🎬 实时共享中' : (status === 'connected' ? '📡 监控中' : '⏳ 连接中...');
	const modeColor = webrtcActive ? '#6366f1' : (status === 'connected' ? '#22c55e' : '#f59e0b');

	uiWidget.innerHTML = `
		<div style="display:flex;align-items:center;gap:8px;margin-bottom:10px;">
			<div style="width:8px;height:8px;background:${modeColor};border-radius:50%;${webrtcActive ? 'animation:cb-pulse 1.5s infinite;' : ''}"></div>
			<span style="font-weight:600;font-size:13px;">${modeText}</span>
		</div>
		${webrtcActive ? '<div style="font-size:11px;color:#888;margin-bottom:10px;">管理员正在查看您的屏幕</div>' : ''}
		<div style="display:flex;gap:8px;">
			<button data-action="toggle-control" style="flex:1;padding:7px 12px;border:1px solid #e5e7eb;border-radius:6px;background:white;cursor:pointer;font-size:12px;">🖱️ ${controlEnabled ? '控制中' : '允许控制'}</button>
			<button data-action="disconnect" style="flex:1;padding:7px 12px;border:1px solid #fecaca;border-radius:6px;background:#fef2f2;color:#dc2626;cursor:pointer;font-size:12px;">⏏️ 断开</button>
		</div>`;

	uiWidget.querySelector('[data-action="toggle-control"]')?.addEventListener('click', () => { controlEnabled = !controlEnabled; updateWidgetContent(); });
	uiWidget.querySelector('[data-action="disconnect"]')?.addEventListener('click', stop);
}

function hideWidget(): void {
	if (uiWidget) { uiWidget.remove(); uiWidget = null; }
}

function makeDraggable(el: HTMLElement): void {
	let dragging = false, sx = 0, sy = 0, ix = 0, iy = 0;
	el.addEventListener('mousedown', (e) => {
		if ((e.target as HTMLElement).tagName === 'BUTTON') return;
		dragging = true; sx = e.clientX; sy = e.clientY;
		const r = el.getBoundingClientRect(); ix = r.left; iy = r.top;
	});
	const onMove = (e: MouseEvent) => {
		if (!dragging) return;
		el.style.left = `${ix + e.clientX - sx}px`;
		el.style.top = `${iy + e.clientY - sy}px`;
		el.style.right = 'auto'; el.style.bottom = 'auto';
	};
	const onUp = () => { dragging = false; };
	document.addEventListener('mousemove', onMove);
	document.addEventListener('mouseup', onUp);
}

// ==================== Reconnect ====================

function scheduleReconnect(): void {
	if (reconnectTimer) return;
	if (reconnectAttempts >= maxReconnectAttempts) { reconnectAttempts = 0; return; }

	reconnectAttempts++;
	const delay = Math.min(Math.pow(2, reconnectAttempts - 1) * 1000, 30000);

	reconnectTimer = window.setTimeout(() => {
		reconnectTimer = null;
		if (status === 'disconnected' && config?.enabled) {
			start().catch(err => LOG_ERR('Reconnect failed:', err));
		}
	}, delay);
}

function setStatus(newStatus: CoBrowseStatus): void {
	status = newStatus;
	config?.onStatusChange?.(newStatus);
}

function defaultControlHandler(command: ControlCommand): void {
	LOG('Control:', command.action);
}

// Flush on unload
window.addEventListener('beforeunload', flushEvents);
