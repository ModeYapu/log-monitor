/**
 * Co-browsing module for LogMonitor SDK
 * Provides real-time screen sharing and remote control capabilities
 */

import { initPrivacy, shouldMaskElement, maskElementValue, shouldAllowDomain } from './privacy';

interface CoBrowseConfig {
	enabled: boolean;
	wsUrl: string; // WebSocket server URL
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
	action: 'click' | 'input' | 'scroll' | 'keydown' | 'navigate';
	x?: number;
	y?: number;
	selector?: string;
	value?: string;
	key?: string;
	url?: string;
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
			const url = `${config!.wsUrl}/${config!.sessionId}`;
			ws = new WebSocket(url);

			ws.onopen = () => {
				console.log('[CoBrowse] Connected to server');
				reconnectAttempts = 0; // Reset reconnect counter on successful connection
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
	if (uiWidget) {
		const controlBtn = uiWidget.querySelector('[data-action="toggle-control"]');
		if (controlBtn) {
			controlBtn.setAttribute('aria-pressed', String(enabled));
			controlBtn.textContent = enabled ? '🖱️ 控制中' : '🖱️ 允许控制';
		}
	}
}

// Internal functions

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

		// Load rrweb record from CDN
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

	// Build privacy mask selector
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
		maskAllInputs: config?.privacy?.maskInputs ?? false, // Only mask password fields by default
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
		// Custom privacy hook
		hooks: {
			beforeEmit: (event: any) => {
				// Apply additional privacy filtering
				if (event.data && event.data.attributes) {
					// Check for sensitive attributes
					const attrs = event.data.attributes;
					if (attrs.type === 'password') {
						// Always mask password fields
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

	// Check if this is a full snapshot
	if (event.type === 0 || event.type === 'Meta') { // Full snapshot
		fullSnapshotSent = true;
		sendMessage({
			type: 'rrweb-full-snapshot',
			data: event
		});
	} else {
		// Buffer incremental events
		eventBuffer.push({
			timestamp: Date.now(),
			data: event
		});

		// Send buffered events periodically
		if (eventBuffer.length >= 5) {
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
		return;
	}

	try {
		ws.send(JSON.stringify(msg));
	} catch (err) {
		console.error('[CoBrowse] Failed to send message:', err);
	}
}

function handleMessage(data: string): void {
	try {
		const msg = JSON.parse(data);

		// Handle ping/pong
		if (msg.type === 'ping') {
			sendMessage({ type: 'pong' });
			return;
		}

		// Handle control commands
		if (msg.type === 'control') {
			handleControlCommand(msg);
		}

	} catch (err) {
		console.error('[CoBrowse] Failed to parse message:', err);
	}
}

function handleControlCommand(msg: any): void {
	const command: ControlCommand = {
		action: msg.action,
		x: msg.x,
		y: msg.y,
		selector: msg.selector,
		value: msg.value,
		key: msg.key,
		url: msg.url
	};

	// Show visual feedback
	showControlFeedback(command);

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
			executeClick(command.x!, command.y!);
			break;
		case 'input':
			executeInput(command.selector!, command.value!);
			break;
		case 'scroll':
			executeScroll(command.x!, command.y!);
			break;
		case 'keydown':
			executeKeydown(command.key!);
			break;
		case 'navigate':
			executeNavigate(command.url!);
			break;
	}
}

function executeClick(x: number, y: number): void {
	const element = document.elementFromPoint(x, y);
	if (element) {
		element.dispatchEvent(new MouseEvent('click', {
			bubbles: true,
			cancelable: true,
			clientX: x,
			clientY: y
		}));
	}
}

function executeInput(selector: string, value: string): void {
	const element = document.querySelector(selector) as HTMLInputElement;
	if (element) {
		// Focus the element first
		element.focus();

		// Set value
		element.value = value;

		// Trigger input and change events
		element.dispatchEvent(new Event('input', { bubbles: true }));
		element.dispatchEvent(new Event('change', { bubbles: true }));
	}
}

function executeScroll(x: number, y: number): void {
	window.scrollTo(x, y);
}

function executeKeydown(key: string): void {
	const event = new KeyboardEvent('keydown', {
		key: key,
		bubbles: true,
		cancelable: true
	});
	document.dispatchEvent(event);
}

function executeNavigate(url: string): void {
	// Ask user for confirmation before navigating
	const confirmed = window.confirm(`技术支持请求导航到: ${url}\n\n是否同意？`);
	if (confirmed) {
		window.location.href = url;
	}
}

function showControlFeedback(command: ControlCommand): void {
	// Create a temporary highlight element
	const feedback = document.createElement('div');
	feedback.style.cssText = `
		position: fixed;
		background: rgba(59, 130, 246, 0.3);
		border: 2px solid #3b82f6;
		border-radius: 4px;
		pointer-events: none;
		z-index: 999999;
		animation: cobrowse-pulse 0.5s ease-out;
	`;

	if (command.action === 'click' && command.x !== undefined) {
		feedback.style.left = `${command.x - 10}px`;
		feedback.style.top = `${command.y - 10}px`;
		feedback.style.width = '20px';
		feedback.style.height = '20px';
	} else if (command.action === 'input' && command.selector) {
		const element = document.querySelector(command.selector);
		if (element) {
			const rect = (element as HTMLElement).getBoundingClientRect();
			feedback.style.left = `${rect.left}px`;
			feedback.style.top = `${rect.top}px`;
			feedback.style.width = `${rect.width}px`;
			feedback.style.height = `${rect.height}px`;
		}
	}

	if (feedback.hasChildNodes()) {
		document.body.appendChild(feedback);
		setTimeout(() => feedback.remove(), 500);
	}

	// Add animation keyframes if not exists
	if (!document.getElementById('cobrowse-styles')) {
		const style = document.createElement('style');
		style.id = 'cobrowse-styles';
		style.textContent = `
			@keyframes cobrowse-pulse {
				0% { opacity: 0.8; transform: scale(1); }
				100% { opacity: 0; transform: scale(1.5); }
			}
		`;
		document.head.appendChild(style);
	}
}

function showWidget(): void {
	if (uiWidget) {
		uiWidget.remove();
	}

	uiWidget = document.createElement('div');
	uiWidget.id = 'logmonitor-cobrowse-widget';
	uiWidget.style.cssText = `
		position: fixed;
		bottom: 20px;
		right: 20px;
		background: white;
		border-radius: 8px;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
		padding: 12px;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		font-size: 14px;
		z-index: 999998;
		min-width: 200px;
	`;

	const isConnected = status === 'connected';
	const viewerCount = 0; // TODO: Get from server

	uiWidget.innerHTML = `
		<div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px;">
			<div style="width: 8px; height: 8px; background: ${isConnected ? '#22c55e' : '#ef4444'}; border-radius: 50%;"></div>
			<span style="font-weight: 500;">${isConnected ? '技术支持已连接' : '正在连接...'}</span>
		</div>
		<div style="display: flex; gap: 8px;">
			<button data-action="toggle-control" style="flex: 1; padding: 6px 12px; border: 1px solid #e5e7eb; border-radius: 4px; background: white; cursor: pointer; font-size: 12px;">
				🖱️ 允许控制
			</button>
			<button data-action="disconnect" style="flex: 1; padding: 6px 12px; border: 1px solid #e5e7eb; border-radius: 4px; background: #fee2e2; color: #dc2626; cursor: pointer; font-size: 12px;">
				⏏️ 断开
			</button>
		</div>
	`;

	// Add event listeners
	uiWidget.querySelector('[data-action="toggle-control"]')?.addEventListener('click', () => {
		const btn = uiWidget!.querySelector('[data-action="toggle-control"]') as HTMLButtonElement;
		const isPressed = btn.getAttribute('aria-pressed') === 'true';
		setControlMode(!isPressed);
	});

	uiWidget.querySelector('[data-action="disconnect"]')?.addEventListener('click', () => {
		stop();
	});

	// Make widget draggable
	makeDraggable(uiWidget);

	document.body.appendChild(uiWidget);
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

	const header = element;
	header.style.cursor = 'move';

	header.addEventListener('mousedown', (e) => {
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
		console.log('[CoBrowse] Max reconnect attempts reached, giving up');
		reconnectAttempts = 0;
		return;
	}

	reconnectAttempts++;

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s
	const delay = Math.min(Math.pow(2, reconnectAttempts - 1) * 1000, 30000);

	console.log(`[CoBrowse] Scheduling reconnect in ${delay}ms (attempt ${reconnectAttempts}/${maxReconnectAttempts})`);

	reconnectTimer = window.setTimeout(() => {
		reconnectTimer = null;
		if (status === 'disconnected' && config?.enabled) {
			console.log('[CoBrowse] Attempting to reconnect...');
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

// Auto-flush events every 100ms
setInterval(() => {
	if (status === 'connected') {
		flushEvents();
	}
}, 100);
