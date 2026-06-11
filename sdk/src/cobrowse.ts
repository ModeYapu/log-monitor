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

interface RRWebEvent {
	timestamp: number;
	data: any;
}

let config: CoBrowseConfig | null = null;
let ws: WebSocket | null = null;
let status: CoBrowseStatus = 'disconnected';
let recorder: any = null;
let eventBuffer: RRWebEvent[] = [];
let reconnectTimer: number | null = null;
let reconnectAttempts = 0;
let maxReconnectAttempts = 10;
let uiWidget: HTMLElement | null = null;
let fullSnapshotSent = false;

// WebRTC state
let peerConnection: RTCPeerConnection | null = null;
let localStream: MediaStream | null = null;
let dataChannel: RTCDataChannel | null = null;
let webrtcActive = false;
let controlEnabled = false;
let adminCursor: HTMLElement | null = null;

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

let connectedAt = 0;

/**
 * Initialize cobrowsing module
 */
export function initCoBrowse(cfg: CoBrowseConfig, autoStart: boolean = true): void {
	if (!cfg.enabled) {
		return;
	}

	config = {
		onControlCommand: defaultControlHandler,
		onStatusChange: () => {},
		...cfg
	};

	// Initialize privacy settings
	if (cfg.privacy) {
		initPrivacy(cfg.privacy);
	}

	loadRRWeb().then(() => {
		console.log('[CoBrowse] Initialized');
		if (autoStart) {
			start().catch(err => console.warn('[CoBrowse] Auto-start failed:', err));
		}
	});
}

/**
 * Start cobrowsing session
 */
export function start(): Promise<void> {
	if (!config) {
		console.error('[CoBrowse] Not initialized');
		return Promise.reject(new Error('Not initialized'));
	}

	if (status === 'connected') {
		return Promise.resolve();
	}

	setStatus('connecting');

	return new Promise((resolve, reject) => {
		try {
			const url = `${config!.wsUrl}/${config!.sessionId}?appId=${encodeURIComponent(config!.appId)}&ua=${encodeURIComponent(navigator.userAgent)}&url=${encodeURIComponent(window.location.href)}`;
			ws = new WebSocket(url);

			ws.onopen = () => {
				console.log('[CoBrowse] Connected to server');
				reconnectAttempts = 0;
				connectedAt = Date.now();
				setStatus('connected');
				startRecording();
				showWidget();
				resolve();
			};

			ws.onmessage = (event) => {
				handleMessage(event.data);
			};

			ws.onclose = () => {
				console.log('[CoBrowse] Disconnected from server');
				setStatus('disconnected');
				stopRecording();
				scheduleReconnect();
			};

			ws.onerror = (error) => {
				console.error('[CoBrowse] WebSocket error:', error);
				reject(error);
			};

		} catch (err) {
			reject(err);
		}
	});
}

/**
 * Stop cobrowsing session
 */
export function stop(): void {
	if (reconnectTimer) {
		clearTimeout(reconnectTimer);
		reconnectTimer = null;
	}

	stopWebRTC();

	if (ws) {
		ws.close();
		ws = null;
	}

	stopRecording();
	hideWidget();
	setStatus('disconnected');
}

/**
 * Get current status
 */
export function getStatus(): CoBrowseStatus {
	return status;
}

/**
 * Enable/disable remote control mode
 */
export function setControlMode(enabled: boolean): void {
	controlEnabled = enabled;
	if (uiWidget) {
		const controlBtn = uiWidget.querySelector('[data-action="toggle-control"]');
		if (controlBtn) {
			controlBtn.setAttribute('aria-pressed', String(enabled));
			controlBtn.textContent = enabled ? '🖱️ 控制中' : '🖱️ 允许控制';
		}
	}
}

// ==================== WebRTC ====================

/**
 * Handle WebRTC offer request from admin
 */
async function handleWebRTCRequest(): Promise<void> {
	if (!ws || ws.readyState !== WebSocket.OPEN) return;

	try {
		// Show confirmation dialog to user
		const accepted = await showInterventionDialog();
		if (!accepted) {
			sendMessage({ type: 'webrtc-rejected' });
			console.log('[CoBrowse] User rejected screen sharing request');
			return;
		}

		// Get screen stream — prefer current tab
		const displayMediaOptions: any = {
			video: {
				cursor: 'always',
				displaySurface: 'browser' // Prefer tab capture
			},
			audio: false,
			preferCurrentTab: true // Chrome hint to show current tab first
		};
		localStream = await navigator.mediaDevices.getDisplayMedia(displayMediaOptions);

		// Handle stream ending (user stops sharing from browser UI)
		localStream.getVideoTracks()[0].onended = () => {
			console.log('[CoBrowse] Screen sharing stopped by user');
			stopWebRTC();
			sendMessage({ type: 'webrtc-stop' });
		};

		// Create PeerConnection
		peerConnection = new RTCPeerConnection(rtcConfig);

		// Add video tracks
		localStream.getTracks().forEach(track => {
			if (peerConnection) {
				peerConnection.addTrack(track, localStream!);
			}
		});

		// Create DataChannel for control commands
		dataChannel = peerConnection.createDataChannel('control', {
			ordered: true,
			maxRetransmits: 3
		});

		dataChannel.onmessage = (event) => {
			try {
				const cmd = JSON.parse(event.data);
				if (cmd.type === 'control') {
					handleControlCommand(cmd);
				}
			} catch (err) {
				console.error('[CoBrowse] Failed to parse data channel message:', err);
			}
		};

		dataChannel.onopen = () => {
			console.log('[CoBrowse] DataChannel opened — control ready');
		};

		// ICE candidates
		peerConnection.onicecandidate = (e) => {
			if (e.candidate) {
				sendMessage({
					type: 'webrtc-ice',
					candidate: e.candidate.toJSON()
				});
			}
		};

		peerConnection.onconnectionstatechange = () => {
			const state = peerConnection?.connectionState;
			console.log('[CoBrowse] WebRTC connection state:', state);
			if (state === 'disconnected' || state === 'failed' || state === 'closed') {
				stopWebRTC();
			}
		};

		peerConnection.onnegotiationneeded = async () => {
			console.log('[CoBrowse] onnegotiationneeded fired');
			await createAndSendOffer();
		};

		// Fallback: explicitly create offer after a short delay in case onnegotiationneeded doesn't fire
		setTimeout(async () => {
			if (peerConnection && !peerConnection.localDescription) {
				console.log('[CoBrowse] Fallback: creating offer manually');
				await createAndSendOffer();
			}
		}, 1000);

	} catch (err: any) {
		console.error('[CoBrowse] Failed to start WebRTC:', err);
		sendMessage({ type: 'webrtc-rejected' });
		cleanupWebRTC();
	}
}

/**
 * Handle SDP Answer from admin
 */
async function handleWebRTCAnswer(sdp: RTCSessionDescriptionInit): Promise<void> {
	if (!peerConnection) return;

	try {
		await peerConnection.setRemoteDescription(new RTCSessionDescription(sdp));
		console.log('[CoBrowse] WebRTC answer set');
	} catch (err) {
		console.error('[CoBrowse] Failed to set remote description:', err);
	}
}

/**
 * Handle ICE candidate from admin
 */
async function handleICECandidate(candidate: RTCIceCandidateInit): Promise<void> {
	if (!peerConnection) return;

	try {
		await peerConnection.addIceCandidate(new RTCIceCandidate(candidate));
	} catch (err) {
		console.error('[CoBrowse] Failed to add ICE candidate:', err);
	}
}

/**
 * Stop WebRTC and clean up
 */
function stopWebRTC(): void {
	cleanupWebRTC();
	webrtcActive = false;
	updateWidgetStatus();
}

function cleanupWebRTC(): void {
	if (dataChannel) {
		try { dataChannel.close(); } catch {}
		dataChannel = null;
	}
	if (peerConnection) {
		try { peerConnection.close(); } catch {}
		peerConnection = null;
	}
	if (localStream) {
		localStream.getTracks().forEach(track => track.stop());
		localStream = null;
	}
	if (adminCursor) {
		adminCursor.remove();
		adminCursor = null;
	}
}

// ==================== Intervention Dialog ====================

function showInterventionDialog(): Promise<boolean> {
	return new Promise((resolve) => {
		const overlay = document.createElement('div');
		overlay.id = 'logmonitor-intervention-dialog';
		overlay.style.cssText = `
			position: fixed; inset: 0; background: rgba(0,0,0,0.5); z-index: 9999999;
			display: flex; align-items: center; justify-content: center;
			animation: cobrowse-fadeIn 0.2s ease-out;
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		`;

		overlay.innerHTML = `
			<div style="background: white; border-radius: 16px; padding: 28px; max-width: 420px; width: 90%;
				box-shadow: 0 20px 60px rgba(0,0,0,0.3); transform: scale(1); animation: cobrowse-scaleIn 0.25s ease-out;">
				<div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px;">
					<div style="width: 48px; height: 48px; border-radius: 12px; background: linear-gradient(135deg, #6366f1, #8b5cf6);
						display: flex; align-items: center; justify-content: center; font-size: 24px;">🎯</div>
					<div>
						<h3 style="margin: 0; font-size: 18px; color: #1a1a2e;">远程协助请求</h3>
						<p style="margin: 4px 0 0; color: #888; font-size: 13px;">管理员请求共享您的屏幕</p>
					</div>
				</div>
				<p style="color: #555; margin: 0 0 24px; line-height: 1.6; font-size: 14px;">
					管理员希望查看您的屏幕画面并进行远程操作。您可以随时停止共享。
				</p>
				<div style="display: flex; gap: 12px; justify-content: flex-end;">
					<button id="lm-reject" style="padding: 10px 24px; border: 1px solid #e0e0e0; border-radius: 8px;
						cursor: pointer; font-size: 14px; background: white; color: #666; transition: all 0.2s;">
						拒绝
					</button>
					<button id="lm-accept" style="padding: 10px 24px; background: linear-gradient(135deg, #6366f1, #8b5cf6);
						color: white; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 500;
						transition: all 0.2s; box-shadow: 0 4px 12px rgba(99,102,241,0.3);">
						允许共享
					</button>
				</div>
			</div>
		`;

		// Add animations
		if (!document.getElementById('cobrowse-dialog-styles')) {
			const style = document.createElement('style');
			style.id = 'cobrowse-dialog-styles';
			style.textContent = `
				@keyframes cobrowse-fadeIn { from { opacity: 0; } to { opacity: 1; } }
				@keyframes cobrowse-scaleIn { from { transform: scale(0.9); opacity: 0; } to { transform: scale(1); opacity: 1; } }
			`;
			document.head.appendChild(style);
		}

		document.body.appendChild(overlay);

		const acceptBtn = overlay.querySelector('#lm-accept') as HTMLButtonElement;
		const rejectBtn = overlay.querySelector('#lm-reject') as HTMLButtonElement;

		acceptBtn.onclick = () => { overlay.remove(); resolve(true); };
		rejectBtn.onclick = () => { overlay.remove(); resolve(false); };
	});
}

// ==================== Control Execution ====================

function handleControlCommand(cmd: any): void {
	const command: ControlCommand = {
		action: cmd.action,
		x: cmd.x,
		y: cmd.y,
		selector: cmd.selector,
		value: cmd.value,
		key: cmd.key,
		url: cmd.url,
		button: cmd.button,
		deltaX: cmd.deltaX,
		deltaY: cmd.deltaY,
	};

	// Show visual feedback
	showControlFeedback(command);

	// Show admin cursor position
	if (cmd.action === 'mousemove' && cmd.x !== undefined && cmd.y !== undefined) {
		showAdminCursor(cmd.x, cmd.y);
	}

	// Execute the command
	executeControlCommand(command);

	// Notify user callback
	if (config?.onControlCommand) {
		config.onControlCommand(command);
	}
}

function executeControlCommand(command: ControlCommand): void {
	switch (command.action) {
		case 'click':
			executeClick(command.x!, command.y!, command.button);
			break;
		case 'dblclick':
			executeDblClick(command.x!, command.y!);
			break;
		case 'contextmenu':
			executeContextMenu(command.x!, command.y!);
			break;
		case 'mousemove':
			// Just show cursor, no action needed
			break;
		case 'input':
			executeInput(command.selector!, command.value!);
			break;
		case 'scroll':
			executeScroll(command.deltaX || 0, command.deltaY || 0);
			break;
		case 'keydown':
			executeKeydown(command.key!);
			break;
		case 'keyup':
			executeKeyup(command.key!);
			break;
		case 'navigate':
			executeNavigate(command.url!);
			break;
	}
}

function executeClick(x: number, y: number, button?: number): void {
	const element = document.elementFromPoint(x, y);
	if (element) {
		element.dispatchEvent(new MouseEvent('click', {
			bubbles: true, cancelable: true, clientX: x, clientY: y, button: button || 0
		}));
		element.dispatchEvent(new MouseEvent('mouseup', {
			bubbles: true, cancelable: true, clientX: x, clientY: y, button: button || 0
		}));
	}
}

function executeDblClick(x: number, y: number): void {
	const element = document.elementFromPoint(x, y);
	if (element) {
		element.dispatchEvent(new MouseEvent('dblclick', {
			bubbles: true, cancelable: true, clientX: x, clientY: y
		}));
	}
}

function executeContextMenu(x: number, y: number): void {
	const element = document.elementFromPoint(x, y);
	if (element) {
		element.dispatchEvent(new MouseEvent('contextmenu', {
			bubbles: true, cancelable: true, clientX: x, clientY: y, button: 2
		}));
	}
}

function executeInput(selector: string, value: string): void {
	const element = document.querySelector(selector) as HTMLInputElement;
	if (element) {
		element.focus();
		element.value = value;
		element.dispatchEvent(new Event('input', { bubbles: true }));
		element.dispatchEvent(new Event('change', { bubbles: true }));
	}
}

function executeScroll(deltaX: number, deltaY: number): void {
	window.scrollBy(deltaX, deltaY);
}

function executeKeydown(key: string): void {
	document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
}

function executeKeyup(key: string): void {
	document.dispatchEvent(new KeyboardEvent('keyup', { key, bubbles: true, cancelable: true }));
}

function executeNavigate(url: string): void {
	const confirmed = window.confirm(`技术支持请求导航到: ${url}\n\n是否同意？`);
	if (confirmed) {
		window.location.href = url;
	}
}

// ==================== Offer Helper ====================

async function createAndSendOffer(): Promise<void> {
	if (!peerConnection) {
		console.error('[CoBrowse] createAndSendOffer: no peerConnection');
		return;
	}
	if (peerConnection.localDescription) {
		console.log('[CoBrowse] Offer already created, skipping');
		return;
	}
	try {
		console.log('[CoBrowse] Creating offer...');
		const offer = await peerConnection.createOffer();
		console.log('[CoBrowse] Offer created, setting local description...');
		await peerConnection.setLocalDescription(offer);
		console.log('[CoBrowse] Local description set, sending offer via WS...');

		sendMessage({
			type: 'webrtc-offer',
			sdp: peerConnection.localDescription
		});

		webrtcActive = true;
		updateWidgetStatus();
		console.log('[CoBrowse] WebRTC offer sent successfully');
	} catch (err) {
		console.error('[CoBrowse] Failed to create/send offer:', err);
	}
}

// ==================== Admin Cursor ====================

function showAdminCursor(x: number, y: number): void {
	if (!adminCursor) {
		adminCursor = document.createElement('div');
		adminCursor.id = 'logmonitor-admin-cursor';
		adminCursor.style.cssText = `
			position: fixed; width: 20px; height: 20px; pointer-events: none; z-index: 999998;
			transition: left 0.05s linear, top 0.05s linear;
		`;
		adminCursor.innerHTML = `
			<svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
				<path d="M3 1L17 10L10 11L7 18L3 1Z" fill="#3b82f6" stroke="white" stroke-width="1.5"/>
			</svg>
		`;
		document.body.appendChild(adminCursor);
	}
	adminCursor.style.left = `${x}px`;
	adminCursor.style.top = `${y}px`;
}

// ==================== Visual Feedback ====================

function showControlFeedback(command: ControlCommand): void {
	const feedback = document.createElement('div');
	feedback.style.cssText = `
		position: fixed;
		background: rgba(99, 102, 241, 0.2);
		border: 2px solid rgba(99, 102, 241, 0.6);
		border-radius: 50%;
		pointer-events: none;
		z-index: 999999;
		animation: cobrowse-feedback 0.4s ease-out forwards;
	`;

	if ((command.action === 'click' || command.action === 'dblclick') && command.x !== undefined) {
		feedback.style.left = `${command.x - 15}px`;
		feedback.style.top = `${command.y - 15}px`;
		feedback.style.width = '30px';
		feedback.style.height = '30px';
	}

	if (document.body.contains(feedback)) {
		document.body.appendChild(feedback);
		setTimeout(() => feedback.remove(), 400);
	}

	if (!document.getElementById('cobrowse-feedback-style')) {
		const style = document.createElement('style');
		style.id = 'cobrowse-feedback-style';
		style.textContent = `
			@keyframes cobrowse-feedback {
				0% { opacity: 1; transform: scale(0.5); }
				100% { opacity: 0; transform: scale(1.5); }
			}
		`;
		document.head.appendChild(style);
	}
}

// ==================== Internal Functions ====================

function setStatus(newStatus: CoBrowseStatus): void {
	status = newStatus;
	if (config?.onStatusChange) {
		config.onStatusChange(newStatus);
	}
}

function loadRRWeb(): Promise<void> {
	return new Promise((resolve) => {
		if ((window as any).rrweb) {
			resolve();
			return;
		}

		const script = document.createElement('script');
		script.src = 'https://cdn.jsdelivr.net/npm/rrweb@2.0.0/dist/rrweb.min.js';
		script.onload = () => resolve();
		script.onerror = () => {
			console.error('[CoBrowse] Failed to load rrweb');
			resolve();
		};
		document.head.appendChild(script);
	});
}

function startRecording(): void {
	if (!(window as any).rrweb) {
		console.error('[CoBrowse] rrweb not loaded');
		return;
	}

	const rrweb = (window as any).rrweb;

	let maskSelector: string | null = null;
	const maskSelectors = config?.privacy?.maskSelectors;
	if (maskSelectors && maskSelectors.length > 0) {
		maskSelector = maskSelectors.join(', ');
	}

	recorder = rrweb.record({
		emit: emitEvent,
		recordCanvas: true,
		recordCrossOriginIframes: true,
		recordAfter: document.readyState === 'complete' ? 0 : 500,
		maskAllInputs: config?.privacy?.maskInputs ?? false,
		maskTextSelector: maskSelector,
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
		dataURLOptions: {
			type: 'image/jpeg',
			quality: 0.6
		},
		hooks: {
			beforeEmit: (event: any) => {
				if (event.data && event.data.attributes) {
					const attrs = event.data.attributes;
					if (attrs.type === 'password') {
						event.data.attributes.value = '••••••••';
					}
				}
				return event;
			}
		}
	} as any);

	console.log('[CoBrowse] Recording started');
}

function stopRecording(): void {
	if (recorder) {
		try {
			recorder();
			recorder = null;
		} catch (e) {
			console.warn('[CoBrowse] Error stopping recorder:', e);
		}
	}
	fullSnapshotSent = false;
	eventBuffer = [];
}

function emitEvent(event: any, isCheckout?: boolean): void {
	if (!ws || ws.readyState !== WebSocket.OPEN) {
		return;
	}

	if (event.type === 0 || event.type === 'Meta') {
		fullSnapshotSent = true;
		sendMessage({
			type: 'rrweb-full-snapshot',
			data: event
		});
	} else {
		eventBuffer.push({
			timestamp: Date.now(),
			data: event
		});

		if (eventBuffer.length >= 10) {
			flushEvents();
		}
	}
}

function flushEvents(): void {
	if (eventBuffer.length === 0 || !ws || ws.readyState !== WebSocket.OPEN) {
		return;
	}

	sendMessage({
		type: 'rrweb-event',
		data: eventBuffer
	});

	eventBuffer = [];
}

function sendMessage(msg: any): void {
	if (!ws || ws.readyState !== WebSocket.OPEN) {
		console.warn('[CoBrowse] Cannot send, WS state:', ws?.readyState);
		return;
	}

	try {
		ws.send(JSON.stringify(msg));
		console.log('[CoBrowse] Sent message type:', msg.type);
	} catch (err) {
		console.error('[CoBrowse] Failed to send message:', err);
	}
}

function handleMessage(data: string): void {
	try {
		const msg = JSON.parse(data);
		console.log('[CoBrowse] Received message type:', msg.type);

		switch (msg.type) {
			case 'ping':
				sendMessage({ type: 'pong' });
				break;

			case 'control':
				// Control command via WebSocket (rrweb mode fallback)
				handleControlCommand(msg);
				break;

			case 'webrtc-offer-request':
				// Admin requests screen sharing — ignore stale requests from reconnect
				const timeSinceConnect = Date.now() - connectedAt;
				if (timeSinceConnect < 3000) {
					console.log('[CoBrowse] Ignoring stale webrtc-offer-request (connected', timeSinceConnect, 'ms ago)');
					sendMessage({ type: 'webrtc-rejected' });
					break;
				}
				console.log('[CoBrowse] WebRTC offer request received! Showing dialog...');
				handleWebRTCRequest();
				break;

			case 'webrtc-answer':
				// Admin sent SDP answer
				console.log('[CoBrowse] WebRTC answer received, SDP type:', msg.sdp?.type);
				if (msg.sdp) {
					handleWebRTCAnswer(msg.sdp);
				}
				break;

			case 'webrtc-ice':
				// ICE candidate from admin
				console.log('[CoBrowse] ICE candidate received');
				if (msg.candidate) {
					handleICECandidate(msg.candidate);
				}
				break;

			case 'webrtc-stop':
				// Admin stopped WebRTC
				console.log('[CoBrowse] WebRTC stop received');
				stopWebRTC();
				break;

			default:
				console.log('[CoBrowse] Unknown message type:', msg.type);
				break;
		}

	} catch (err) {
		console.error('[CoBrowse] Failed to parse message:', err);
	}
}

// ==================== Widget ====================

function showWidget(): void {
	if (uiWidget) {
		uiWidget.remove();
	}

	uiWidget = document.createElement('div');
	uiWidget.id = 'logmonitor-cobrowse-widget';
	uiWidget.style.cssText = `
		position: fixed; bottom: 20px; right: 20px;
		background: white; border-radius: 12px;
		box-shadow: 0 4px 24px rgba(0, 0, 0, 0.12);
		padding: 14px; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		font-size: 13px; z-index: 999998; min-width: 220px;
		border: 1px solid rgba(0,0,0,0.08);
	`;

	updateWidgetContent();
	makeDraggable(uiWidget);
	document.body.appendChild(uiWidget);
}

function updateWidgetStatus(): void {
	if (!uiWidget) return;
	updateWidgetContent();
}

function updateWidgetContent(): void {
	if (!uiWidget) return;

	const isConnected = status === 'connected';
	const modeText = webrtcActive ? '🎬 实时共享中' : (isConnected ? '📡 监控中' : '⏳ 连接中...');
	const modeColor = webrtcActive ? '#6366f1' : (isConnected ? '#22c55e' : '#f59e0b');

	uiWidget.innerHTML = `
		<div style="display: flex; align-items: center; gap: 8px; margin-bottom: 10px;">
			<div style="width: 8px; height: 8px; background: ${modeColor}; border-radius: 50%;
				${webrtcActive ? 'animation: cobrowse-pulse 1.5s infinite;' : ''}"></div>
			<span style="font-weight: 600; font-size: 13px;">${modeText}</span>
		</div>
		${webrtcActive ? '<div style="font-size:11px;color:#888;margin-bottom:10px;">管理员正在查看您的屏幕</div>' : ''}
		<div style="display: flex; gap: 8px;">
			<button data-action="toggle-control" style="flex: 1; padding: 7px 12px; border: 1px solid #e5e7eb;
				border-radius: 6px; background: white; cursor: pointer; font-size: 12px; transition: all 0.2s;">
				🖱️ ${controlEnabled ? '控制中' : '允许控制'}
			</button>
			<button data-action="disconnect" style="flex: 1; padding: 7px 12px; border: 1px solid #fecaca;
				border-radius: 6px; background: #fef2f2; color: #dc2626; cursor: pointer; font-size: 12px;
				transition: all 0.2s;">
				⏏️ 断开
			</button>
		</div>
	`;

	uiWidget.querySelector('[data-action="toggle-control"]')?.addEventListener('click', () => {
		controlEnabled = !controlEnabled;
		updateWidgetContent();
	});

	uiWidget.querySelector('[data-action="disconnect"]')?.addEventListener('click', () => {
		stop();
	});
}

function hideWidget(): void {
	if (uiWidget) {
		uiWidget.remove();
		uiWidget = null;
	}
}

function makeDraggable(element: HTMLElement): void {
	let isDragging = false;
	let startX = 0;
	let startY = 0;
	let initialX = 0;
	let initialY = 0;

	element.addEventListener('mousedown', (e) => {
		if ((e.target as HTMLElement).tagName === 'BUTTON') return;
		isDragging = true;
		startX = e.clientX;
		startY = e.clientY;
		const rect = element.getBoundingClientRect();
		initialX = rect.left;
		initialY = rect.top;
	});

	document.addEventListener('mousemove', (e) => {
		if (!isDragging) return;
		const dx = e.clientX - startX;
		const dy = e.clientY - startY;
		element.style.left = `${initialX + dx}px`;
		element.style.top = `${initialY + dy}px`;
		element.style.right = 'auto';
		element.style.bottom = 'auto';
	});

	document.addEventListener('mouseup', () => {
		isDragging = false;
	});
}

function scheduleReconnect(): void {
	if (reconnectTimer) {
		return;
	}

	if (reconnectAttempts >= maxReconnectAttempts) {
		console.log('[CoBrowse] Max reconnect attempts reached');
		reconnectAttempts = 0;
		return;
	}

	reconnectAttempts++;
	const delay = Math.min(Math.pow(2, reconnectAttempts - 1) * 1000, 30000);

	reconnectTimer = window.setTimeout(() => {
		reconnectTimer = null;
		if (status === 'disconnected' && config?.enabled) {
			start().catch((err) => {
				console.error('[CoBrowse] Reconnect failed:', err);
			});
		}
	}, delay);
}

function defaultControlHandler(command: ControlCommand): void {
	console.log('[CoBrowse] Control command received:', command);
}

// Flush events on page unload
window.addEventListener('beforeunload', () => {
	flushEvents();
});

// Auto-flush events every 500ms (reduced frequency to avoid WS overload)
setInterval(() => {
	if (status === 'connected') {
		flushEvents();
	}
}, 500);
