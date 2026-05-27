/**
 * LogMonitor SDK - Frontend log monitoring and error tracking
 */

// Types
export interface LogMonitorConfig {
  dsn: string;                    // Server endpoint URL, e.g., 'https://host/api/report'
  appId: string;                  // Application identifier
  release?: string;               // Release version
  sampleRate?: number;            // Sampling rate (0-1), default 1
  bufferSize?: number;            // Buffer size before auto-flush, default 10
  flushInterval?: number;         // Auto-flush interval in ms, default 5000
  captureConsole?: boolean;       // Capture console.error, default false
  captureUnhandledRejection?: boolean; // Capture unhandled promise rejections, default true
  userAttributes?: Record<string, any>; // User-specific attributes to include with all events
  cobrowse?: CoBrowseConfig;      // Co-browsing configuration
}

export interface CoBrowseConfig {
  enabled?: boolean;              // Enable co-browsing, default false
  wsUrl?: string;                 // WebSocket server URL, defaults to dsn host
  onControlCommand?: (command: CoBrowseControlCommand) => void; // Callback for control commands
  onStatusChange?: (status: CoBrowseStatus) => void; // Status change callback
}

export type CoBrowseStatus = 'disconnected' | 'connecting' | 'connected' | 'controlling';

export interface CoBrowseControlCommand {
  action: 'click' | 'input' | 'scroll' | 'keydown' | 'navigate';
  x?: number;
  y?: number;
  selector?: string;
  value?: string;
  key?: string;
  url?: string;
}

export interface LogEvent {
  type: 'error' | 'performance' | 'info' | 'warn' | 'track' | 'console';
  level: string;
  message: string;
  stack?: string;
  url?: string;
  line?: number;
  col?: number;
  tags?: Record<string, any>;
  extra?: Record<string, any>;
  performance?: Record<string, number>;
  screenshotId?: string;
}

export interface LogBatch {
  appId: string;
  release: string;
  events: LogEvent[];
}

// Internal state
let config: LogMonitorConfig | null = null;
let buffer: LogEvent[] = [];
let flushTimer: number | null = null;
let pageStartTime = Date.now();
let performanceObserver: PerformanceObserver | null = null;
let collectedPerformance: Record<string, number> = {};
let consoleBuffer: string[] = [];
let html2canvasLoaded = false;

// Default configuration
const DEFAULT_CONFIG = {
  sampleRate: 1,
  bufferSize: 10,
  flushInterval: 5000,
  captureConsole: false,
  captureUnhandledRejection: true,
};

/**
 * Initialize LogMonitor SDK
 */
export function init(cfg: LogMonitorConfig): void {
  if (config) {
    console.warn('[LogMonitor] Already initialized, ignoring duplicate init');
    return;
  }

  config = { ...DEFAULT_CONFIG, ...cfg };

  if (!config.dsn || !config.appId) {
    console.error('[LogMonitor] Missing required config: dsn and appId are required');
    return;
  }

  // Setup global error handlers
  setupErrorHandlers();

  // Setup console interception (always enabled now)
  setupConsoleInterception();

  // Initialize cobrowse if enabled
  if (config.cobrowse?.enabled) {
    import('./cobrowse').then(({ initCoBrowse }) => {
      const wsUrl = config.cobrowse?.wsUrl || config.dsn.replace('/api/report', '').replace('/api', '') + '/ws';
      initCoBrowse({
        enabled: true,
        wsUrl: wsUrl,
        sessionId: generateSessionId(),
        appId: config.appId,
        onControlCommand: config.cobrowse?.onControlCommand,
        onStatusChange: config.cobrowse?.onStatusChange,
      });
    });
  }

  // Setup performance observer
  setupPerformanceObserver();

  // Setup page visibility handler
  setupVisibilityHandlers();

  // Setup flush timer
  startFlushTimer();

  // Flush on page unload
  window.addEventListener('beforeunload', flush);
  window.addEventListener('pagehide', flush);

  console.log('[LogMonitor] Initialized with appId:', config.appId);
}

/**
 * Capture and report an error
 */
export function captureException(error: Error, tags?: Record<string, any>, extra?: Record<string, any>): void {
  if (!config) return;

  const event: LogEvent = {
    type: 'error',
    level: 'error',
    message: error.message || String(error),
    stack: error.stack || '',
    url: window.location.href,
    tags: { ...config.userAttributes, ...tags },
    extra: extra || {},
  };

  // Capture screenshot on error
  captureScreenshotForEvent(event).then(() => {
    addEvent(event);
  });
}

/**
 * Log an info message
 */
export function info(message: string, extra?: Record<string, any>): void {
  logMessage('info', message, extra);
}

/**
 * Log a warning message
 */
export function warn(message: string, extra?: Record<string, any>): void {
  logMessage('warn', message, extra);
}

/**
 * Log an error message
 */
export function error(message: string, extra?: Record<string, any>): void {
  logMessage('error', message, extra);
}

/**
 * Track a custom event
 */
export function track(event: string, data?: Record<string, any>): void {
  if (!config) return;

  const logEvent: LogEvent = {
    type: 'track',
    level: 'info',
    message: event,
    url: window.location.href,
    extra: data || {},
    tags: { ...config.userAttributes },
  };

  addEvent(logEvent);
}

/**
 * Manually flush buffered events
 */
export function flush(): void {
  if (!config || buffer.length === 0) return;

  const events = [...buffer];
  buffer = [];

  sendBatch(events);
}

/**
 * Get current SDK configuration (read-only)
 */
export function getConfig(): Readonly<LogMonitorConfig> | null {
  return config;
}

/**
 * Get current buffer size
 */
export function getBufferSize(): number {
  return buffer.length;
}

/**
 * Capture screenshot for an event
 */
async function captureScreenshotForEvent(event: LogEvent): Promise<void> {
  if (!config) return;

  // Load html2canvas from CDN if not loaded
  if (!html2canvasLoaded) {
    await loadHtml2Canvas();
  }

  const win = window as any;
  if (win.html2canvas) {
    try {
      const canvas = await win.html2canvas(document.body, {
        logging: false,
        useCORS: true,
        allowTaint: true,
        scale: window.devicePixelRatio || 1,
      });

      canvas.toBlob(async (blob: Blob | null) => {
        if (!blob) return;
        const reader = new FileReader();
        reader.onloadend = () => {
          const base64 = (reader.result as string).split(',')[1];
          uploadScreenshot(base64, event);
        };
        reader.readAsDataURL(blob);
      }, 'image/png');
    } catch (err) {
      console.warn('[LogMonitor] Failed to capture screenshot:', err);
    }
  }
}

/**
 * Load html2canvas from CDN
 */
function loadHtml2Canvas(): Promise<void> {
  return new Promise((resolve) => {
    if ((window as any).html2canvas) {
      html2canvasLoaded = true;
      resolve();
      return;
    }

    const script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/html2canvas@1.4.1/dist/html2canvas.min.js';
    script.onload = () => {
      html2canvasLoaded = true;
      resolve();
    };
    script.onerror = () => {
      console.warn('[LogMonitor] Failed to load html2canvas');
      resolve();
    };
    document.head.appendChild(script);
  });
}

/**
 * Upload screenshot to server
 */
function uploadScreenshot(base64: string, event: LogEvent): void {
  if (!config) return;

  const eventId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

  fetch(`${config.dsn}/screenshot`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      appId: config.appId,
      eventId,
      image: base64,
    }),
  }).catch((err) => {
    console.error('[LogMonitor] Failed to upload screenshot:', err);
  });

  event.screenshotId = eventId;
}

// Internal functions

function logMessage(level: string, message: string, extra?: Record<string, any>): void {
  if (!config) return;

  const event: LogEvent = {
    type: level === 'error' ? 'error' : 'info',
    level,
    message,
    url: window.location.href,
    extra: extra || {},
    tags: { ...config.userAttributes },
  };

  addEvent(event);
}

function addEvent(event: LogEvent): void {
  if (!config) return;

  // Sample rate check
  if (Math.random() > config.sampleRate!) {
    return;
  }

  // Add common attributes
  event.tags = {
    ...event.tags,
    ...getCommonTags(),
  };

  // Add performance data if available
  if (Object.keys(collectedPerformance).length > 0) {
    event.performance = { ...collectedPerformance };
  }

  buffer.push(event);

  // Auto flush if buffer is full
  if (buffer.length >= config.bufferSize!) {
    flush();
  }
}

function sendBatch(events: LogEvent[]): void {
  if (!config) return;

  // Add console logs if buffer has content
  if (consoleBuffer.length > 0) {
    const consoleEvent: LogEvent = {
      type: 'console',
      level: 'info',
      message: 'Console logs captured',
      extra: { logs: [...consoleBuffer] },
      url: window.location.href,
      tags: { ...config.userAttributes },
    };
    events.push(consoleEvent);
    consoleBuffer = []; // Clear buffer after sending
  }

  const batch: LogBatch = {
    appId: config.appId,
    release: config.release || '',
    events,
  };

  const payload = JSON.stringify(batch);

  // Try sendBeacon first (reliable on page unload)
  if (navigator.sendBeacon && navigator.sendBeacon(config.dsn, payload)) {
    return;
  }

  // Fallback to fetch
  fetch(config.dsn, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: payload,
    keepalive: true,
  }).catch((err) => {
    console.error('[LogMonitor] Failed to send batch:', err);
  });
}

function setupErrorHandlers(): void {
  if (!config) return;

  // Global error handler
  window.onerror = (message, source, lineno, colno, error) => {
    const event: LogEvent = {
      type: 'error',
      level: 'error',
      message: String(message),
      stack: error?.stack || '',
      url: source || window.location.href,
      line: lineno || 0,
      col: colno || 0,
      tags: { ...config.userAttributes, source: 'onerror' },
    };
    addEvent(event);
    return false; // Let default error handler run
  };

  // Unhandled promise rejection handler
  if (config.captureUnhandledRejection) {
    window.addEventListener('unhandledrejection', (event) => {
      const error = event.reason;
      const logEvent: LogEvent = {
        type: 'error',
        level: 'error',
        message: error?.message || String(error),
        stack: error?.stack || '',
        url: window.location.href,
        tags: { ...config.userAttributes, source: 'unhandledrejection' },
      };
      addEvent(logEvent);
    });
  }
}

/**
 * Setup console interception
 */
function setupConsoleInterception(): void {
  const intercept = (level: string, originalFn: Function) => {
    return (...args: any[]) => {
      // Call original console method
      originalFn.apply(console, args);

      // Add to console buffer
      const message = args.map((arg) => {
        if (typeof arg === 'object') {
          try {
            return JSON.stringify(arg);
          } catch {
            return String(arg);
          }
        }
        return String(arg);
      }).join(' ');

      consoleBuffer.push(`[${level.toUpperCase()}] ${message}`);

      // Keep buffer size manageable
      if (consoleBuffer.length > 50) {
        consoleBuffer.splice(0, consoleBuffer.length - 50);
      }
    };
  };

  console.log = intercept('log', console.log.bind(console));
  console.warn = intercept('warn', console.warn.bind(console));
  console.error = intercept('error', console.error.bind(console));
}

function setupPerformanceObserver(): void {
  if (!window.PerformanceObserver) return;

  try {
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        switch (entry.entryType) {
          case 'navigation':
            const navEntry = entry as PerformanceNavigationTiming;
            collectedPerformance['domContentLoaded'] = navEntry.domContentLoadedEventEnd - navEntry.domContentLoadedEventStart;
            collectedPerformance['loadComplete'] = navEntry.loadEventEnd - navEntry.loadEventStart;
            break;
          case 'paint':
            if (entry.name === 'first-contentful-paint') {
              collectedPerformance['fcp'] = entry.startTime;
            }
            break;
          case 'largest-contentful-paint':
            collectedPerformance['lcp'] = entry.startTime;
            break;
          case 'layout-shift':
            if (!(entry as any).hadInput) {
              collectedPerformance['cls'] = (collectedPerformance['cls'] || 0) + (entry as any).value;
            }
            break;
          case 'first-input':
            collectedPerformance['fid'] = (entry as any).processingStart - entry.startTime;
            break;
        }
      }
    });

    observer.observe({
      entryTypes: ['navigation', 'paint', 'largest-contentful-paint', 'layout-shift', 'first-input'],
    });

    performanceObserver = observer;
  } catch (err) {
    console.warn('[LogMonitor] PerformanceObserver not supported:', err);
  }
}

function setupVisibilityHandlers(): void {
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      // Page hidden, flush buffer
      flush();

      // Report page view duration
      if (config) {
        const duration = Date.now() - pageStartTime;
        const event: LogEvent = {
          type: 'track',
          level: 'info',
          message: 'page_view_duration',
          url: window.location.href,
          extra: { duration },
          tags: { ...config.userAttributes },
        };
        // Send immediately without buffering
        sendBatch([event]);
      }
    } else if (document.visibilityState === 'visible') {
      pageStartTime = Date.now();
    }
  });
}

function startFlushTimer(): void {
  if (flushTimer !== null) {
    clearInterval(flushTimer);
  }

  if (config && config.flushInterval && config.flushInterval > 0) {
    flushTimer = window.setInterval(() => {
      flush();
    }, config.flushInterval);
  }
}

function getCommonTags(): Record<string, any> {
  return {
    ua: navigator.userAgent,
    screen: `${screen.width}x${screen.height}`,
    viewport: `${window.innerWidth}x${window.innerHeight}`,
    lang: navigator.language,
  };
}

function generateSessionId(): string {
  return `session_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

// Export as UMD global
if (typeof window !== 'undefined') {
  (window as any).LogMonitor = {
    init,
    captureException,
    info,
    warn,
    error,
    track,
    flush,
    getConfig,
    getBufferSize,
    captureScreenshot: captureException, // Alias
    // Co-browsing API
    cobrowse: {
      start: () => import('./cobrowse').then(m => m.start()),
      stop: () => import('./cobrowse').then(m => m.stop()),
      getStatus: () => import('./cobrowse').then(m => m.getStatus()),
      setControlMode: (enabled: boolean) => import('./cobrowse').then(m => m.setControlMode(enabled)),
    }
  };
}

// Re-export cobrowse module
export * from './cobrowse';

// Export privacy utilities
export * from './privacy';
