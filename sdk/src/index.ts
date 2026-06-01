/**
 * LogMonitor SDK - Frontend log monitoring and error tracking
 */

// Types
export interface LogMonitorConfig {
  dsn: string;
  appId: string;
  release?: string;
  sampleRate?: number;
  bufferSize?: number;
  flushInterval?: number;
  captureConsole?: boolean;
  captureUnhandledRejection?: boolean;
  userAttributes?: Record<string, any>;
  cobrowse?: CoBrowseConfig;
  // New features
  captureBreadcrumbs?: boolean;       // Auto-capture user actions, default true
  captureXhr?: boolean;               // Auto-capture XHR/fetch requests, default true
  maxBreadcrumbs?: number;            // Max breadcrumbs per session, default 30
  privacy?: PrivacyConfig;            // Data masking configuration
}

export interface PrivacyConfig {
  enabled?: boolean;                   // Enable data masking, default true
  maskPatterns?: MaskPattern[];        // Custom mask patterns
  maskUrlParams?: string[];            // URL params to mask (e.g., ['token', 'key'])
  maskFields?: string[];              // Form field names to mask
  maskHeaders?: string[];             // HTTP headers to mask
  blockUrls?: (string | RegExp)[];    // URLs to not capture at all
  blockXhrUrls?: (string | RegExp)[]; // XHR URLs to not capture
}

export interface MaskPattern {
  name: string;
  pattern: RegExp;
  replacement?: string;               // Default: '***'
}

export interface CoBrowseConfig {
  enabled?: boolean;
  wsUrl?: string;
  onControlCommand?: (command: CoBrowseControlCommand) => void;
  onStatusChange?: (status: CoBrowseStatus) => void;
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

export interface Breadcrumb {
  type: 'click' | 'navigation' | 'console' | 'xhr' | 'custom' | 'error';
  category: string;
  message: string;
  data?: Record<string, any>;
  timestamp: number;
  level: 'info' | 'warn' | 'error';
}

export interface XhrLog {
  method: string;
  url: string;
  status: number;
  statusText: string;
  requestHeaders?: Record<string, string>;
  requestBody?: string;
  responseHeaders?: Record<string, string>;
  responseBody?: string;
  duration: number;
  timestamp: number;
  error?: string;
}

export interface LogEvent {
  type: 'error' | 'performance' | 'info' | 'warn' | 'track' | 'console' | 'xhr' | 'breadcrumb';
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
  breadcrumbs?: Breadcrumb[];
  xhr?: XhrLog;
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

// New state
let breadcrumbs: Breadcrumb[] = [];
let privacyEngine: PrivacyEngine | null = null;

// Default configuration
const DEFAULT_CONFIG = {
  sampleRate: 1,
  bufferSize: 10,
  flushInterval: 5000,
  captureConsole: false,
  captureUnhandledRejection: true,
  captureBreadcrumbs: true,
  captureXhr: true,
  maxBreadcrumbs: 30,
};

// ==================== Privacy Engine ====================

class PrivacyEngine {
  private config: PrivacyConfig;
  private defaultPatterns: MaskPattern[] = [
    { name: 'credit_card', pattern: /\b(?:\d[ -]*?){13,16}\b/g, replacement: '[CARD]' },
    { name: 'email', pattern: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b/g, replacement: '[EMAIL]' },
    { name: 'phone_cn', pattern: /\b1[3-9]\d{9}\b/g, replacement: '[PHONE]' },
    { name: 'id_card_cn', pattern: /\b\d{17}[\dXx]\b/g, replacement: '[ID]' },
    { name: 'ip_address', pattern: /\b(?:\d{1,3}\.){3}\d{1,3}\b/g, replacement: '[IP]' },
    { name: 'jwt_token', pattern: /\beyJ[A-Za-z0-9-_]+\.eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\b/g, replacement: '[TOKEN]' },
  ];

  constructor(config: PrivacyConfig) {
    this.config = {
      enabled: true,
      maskPatterns: [],
      maskUrlParams: ['token', 'key', 'secret', 'password', 'access_token', 'auth', 'session', 'jwt', 'api_key', 'ak', 'sk'],
      maskFields: ['password', 'passwd', 'pwd', 'secret', 'token', 'credit_card', 'card_number', 'cvv', 'ssn'],
      maskHeaders: ['authorization', 'cookie', 'set-cookie', 'x-api-key', 'x-auth-token'],
      blockUrls: [],
      blockXhrUrls: [],
      ...config,
    };
  }

  maskText(text: string): string {
    if (!this.config.enabled || !text) return text;

    let masked = text;
    const patterns = [...this.defaultPatterns, ...(this.config.maskPatterns || [])];
    for (const { pattern, replacement = '***' } of patterns) {
      masked = masked.replace(pattern, replacement);
    }
    return masked;
  }

  maskUrl(url: string): string {
    if (!this.config.enabled || !url) return url;
    try {
      const u = new URL(url);
      const paramsToMask = this.config.maskUrlParams || [];
      for (const key of paramsToMask) {
        if (u.searchParams.has(key)) {
          u.searchParams.set(key, '***');
        }
      }
      return u.toString();
    } catch {
      return url;
    }
  }

  maskObject(obj: Record<string, any>, depth = 0): Record<string, any> {
    if (!this.config.enabled || depth > 5) return obj;
    const result: Record<string, any> = {};
    const fieldsToMask = this.config.maskFields || [];

    for (const [key, value] of Object.entries(obj)) {
      const keyLower = key.toLowerCase();
      if (fieldsToMask.some(f => keyLower.includes(f.toLowerCase()))) {
        result[key] = '***';
      } else if (typeof value === 'string') {
        result[key] = this.maskText(value);
      } else if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
        result[key] = this.maskObject(value, depth + 1);
      } else {
        result[key] = value;
      }
    }
    return result;
  }

  maskHeaders(headers: Record<string, string>): Record<string, string> {
    if (!this.config.enabled) return headers;
    const result: Record<string, string> = {};
    const headersToMask = this.config.maskHeaders || [];

    for (const [key, value] of Object.entries(headers)) {
      if (headersToMask.some(h => key.toLowerCase() === h.toLowerCase())) {
        result[key] = '***';
      } else {
        result[key] = value;
      }
    }
    return result;
  }

  shouldBlockUrl(url: string, blockList?: (string | RegExp)[]): boolean {
    if (!url) return false;
    const list = blockList || this.config.blockUrls || [];
    for (const pattern of list) {
      if (typeof pattern === 'string') {
        if (url.includes(pattern)) return true;
      } else {
        if (pattern.test(url)) return true;
      }
    }
    return false;
  }

  shouldBlockXhr(url: string): boolean {
    return this.shouldBlockUrl(url, this.config.blockXhrUrls);
  }
}

// ==================== Breadcrumb System ====================

function addBreadcrumb(crumb: Breadcrumb): void {
  if (!config?.captureBreadcrumbs) return;
  breadcrumbs.push(crumb);
  if (breadcrumbs.length > (config.maxBreadcrumbs || 30)) {
    breadcrumbs.shift();
  }
}

function getBreadcrumbs(): Breadcrumb[] {
  return [...breadcrumbs];
}

function clearBreadcrumbs(): void {
  breadcrumbs = [];
}

function setupBreadcrumbCapture(): void {
  if (!config?.captureBreadcrumbs) return;

  // Click tracking
  document.addEventListener('click', (e) => {
    const target = e.target as HTMLElement;
    if (!target) return;

    const selector = getElementSelector(target);
    const text = target.textContent?.trim().slice(0, 50) || '';
    const tag = target.tagName.toLowerCase();

    addBreadcrumb({
      type: 'click',
      category: 'ui',
      message: `Click <${tag}> ${text}`,
      data: {
        selector,
        tag,
        text: text.slice(0, 100),
        x: e.clientX,
        y: e.clientY,
      },
      timestamp: Date.now(),
      level: 'info',
    });
  }, true);

  // Navigation tracking (SPA)
  const originalPushState = history.pushState;
  const originalReplaceState = history.replaceState;

  history.pushState = function (...args) {
    addBreadcrumb({
      type: 'navigation',
      category: 'navigation',
      message: `Navigate: ${args[2] || window.location.href}`,
      data: { from: window.location.href, to: args[2]?.toString() || '' },
      timestamp: Date.now(),
      level: 'info',
    });
    return originalPushState.apply(this, args);
  };

  history.replaceState = function (...args) {
    addBreadcrumb({
      type: 'navigation',
      category: 'navigation',
      message: `Replace: ${args[2] || window.location.href}`,
      data: { from: window.location.href, to: args[2]?.toString() || '' },
      timestamp: Date.now(),
      level: 'info',
    });
    return originalReplaceState.apply(this, args);
  };

  window.addEventListener('popstate', () => {
    addBreadcrumb({
      type: 'navigation',
      category: 'navigation',
      message: `Popstate: ${window.location.href}`,
      data: { url: window.location.href },
      timestamp: Date.now(),
      level: 'info',
    });
  });

  // Input change tracking (no values, just the fact of interaction)
  document.addEventListener('change', (e) => {
    const target = e.target as HTMLInputElement;
    if (!target) return;

    const selector = getElementSelector(target);
    const isSensitive = target.type === 'password' || privacyEngine?.config.maskFields?.some(
      f => (target.name || target.id || '').toLowerCase().includes(f)
    );

    addBreadcrumb({
      type: 'custom',
      category: 'input',
      message: `Input changed: ${target.name || target.id || selector}`,
      data: {
        selector,
        field: target.name || target.id,
        type: target.type,
        value: isSensitive ? '***' : undefined, // Never capture sensitive values
      },
      timestamp: Date.now(),
      level: 'info',
    });
  }, true);
}

function getElementSelector(el: HTMLElement): string {
  if (el.id) return `#${el.id}`;
  if (el.className && typeof el.className === 'string') {
    const classes = el.className.trim().split(/\s+/).slice(0, 2).join('.');
    return `${el.tagName.toLowerCase()}.${classes}`;
  }
  return el.tagName.toLowerCase();
}

// ==================== XHR/Fetch Interception ====================

function setupXhrCapture(): void {
  if (!config?.captureXhr) return;

  // Intercept XMLHttpRequest
  const originalXhrOpen = XMLHttpRequest.prototype.open;
  const originalXhrSend = XMLHttpRequest.prototype.send;
  const originalXhrSetHeader = XMLHttpRequest.prototype.setRequestHeader;

  XMLHttpRequest.prototype.open = function (method: string, url: string | URL, ...rest: any[]) {
    this._logMonitorData = {
      method: method.toUpperCase(),
      url: typeof url === 'string' ? url : url.toString(),
      requestHeaders: {} as Record<string, string>,
      startTime: 0,
    };
    return originalXhrOpen.call(this, method, url, ...rest);
  };

  XMLHttpRequest.prototype.setRequestHeader = function (name: string, value: string) {
    if (this._logMonitorData?.requestHeaders) {
      this._logMonitorData.requestHeaders[name] = value;
    }
    return originalXhrSetHeader.call(this, name, value);
  };

  XMLHttpRequest.prototype.send = function (body?: Document | XMLHttpRequestBodyInit | null) {
    if (this._logMonitorData) {
      const xhrData = this._logMonitorData;
      xhrData.startTime = Date.now();

      // Check if URL should be blocked
      if (privacyEngine?.shouldBlockXhr(xhrData.url)) {
        return originalXhrSend.call(this, body);
      }

      // Mask the URL
      const maskedUrl = privacyEngine?.maskUrl(xhrData.url) || xhrData.url;

      // Capture request body (masked)
      let maskedBody: string | undefined;
      if (body) {
        if (typeof body === 'string') {
          try {
            const parsed = JSON.parse(body);
            maskedBody = JSON.stringify(privacyEngine?.maskObject(parsed) || parsed);
          } catch {
            maskedBody = body.slice(0, 1000);
          }
        }
      }

      const onLoadEnd = () => {
        const duration = Date.now() - xhrData.startTime;
        const xhr = this as XMLHttpRequest;

        // Capture response (limited size, masked)
        let maskedResponse: string | undefined;
        try {
          const raw = xhr.responseText;
          if (raw && raw.length < 10000) {
            try {
              const parsed = JSON.parse(raw);
              maskedResponse = JSON.stringify(privacyEngine?.maskObject(parsed) || parsed);
            } catch {
              maskedResponse = raw.slice(0, 2000);
            }
          }
        } catch {}

        // Mask headers
        const maskedReqHeaders = privacyEngine?.maskHeaders(xhrData.requestHeaders) || xhrData.requestHeaders;

        const xhrLog: XhrLog = {
          method: xhrData.method,
          url: maskedUrl,
          status: xhr.status,
          statusText: xhr.statusText,
          requestHeaders: maskedReqHeaders,
          requestBody: maskedBody?.slice(0, 5000),
          responseHeaders: undefined, // Can't easily get response headers from XHR
          responseBody: maskedResponse?.slice(0, 5000),
          duration,
          timestamp: xhrData.startTime,
          error: xhr.status >= 400 ? `HTTP ${xhr.status}` : undefined,
        };

        // Add as breadcrumb
        addBreadcrumb({
          type: 'xhr',
          category: 'network',
          message: `${xhrData.method} ${maskedUrl} → ${xhr.status} (${duration}ms)`,
          data: { status: xhr.status, duration },
          timestamp: xhrData.startTime,
          level: xhr.status >= 400 ? 'error' : xhr.status >= 300 ? 'warn' : 'info',
        });

        // Add as event for failed requests
        if (xhr.status >= 400) {
          addEvent({
            type: 'xhr',
            level: xhr.status >= 500 ? 'error' : 'warn',
            message: `${xhrData.method} ${maskedUrl} failed with ${xhr.status}`,
            url: window.location.href,
            xhr: xhrLog,
            tags: { ...config?.userAttributes },
          });
        } else {
          // Track all successful XHRs as track events for the dashboard
          addEvent({
            type: 'xhr',
            level: 'info',
            message: `${xhrData.method} ${maskedUrl} → ${xhr.status}`,
            url: window.location.href,
            xhr: xhrLog,
            tags: { ...config?.userAttributes },
          });
        }
      };

      this.addEventListener('loadend', onLoadEnd);
      this.addEventListener('error', () => {
        const duration = Date.now() - xhrData.startTime;
        addBreadcrumb({
          type: 'xhr',
          category: 'network',
          message: `${xhrData.method} ${maskedUrl} → Network Error (${duration}ms)`,
          data: { error: true, duration },
          timestamp: xhrData.startTime,
          level: 'error',
        });
      });
    }

    return originalXhrSend.call(this, body);
  };

  // Intercept fetch
  const originalFetch = window.fetch;
  window.fetch = async function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method = init?.method?.toUpperCase() || 'GET';
    const startTime = Date.now();

    if (privacyEngine?.shouldBlockXhr(url)) {
      return originalFetch.call(this, input, init);
    }

    const maskedUrl = privacyEngine?.maskUrl(url) || url;

    // Mask request body
    let maskedBody: string | undefined;
    if (init?.body) {
      if (typeof init.body === 'string') {
        try {
          const parsed = JSON.parse(init.body);
          maskedBody = JSON.stringify(privacyEngine?.maskObject(parsed) || parsed);
        } catch {
          maskedBody = (init.body as string).slice(0, 1000);
        }
      }
    }

    // Mask request headers
    let maskedHeaders: Record<string, string> | undefined;
    if (init?.headers) {
      const headerObj: Record<string, string> = {};
      if (init.headers instanceof Headers) {
        init.headers.forEach((v, k) => { headerObj[k] = v; });
      } else if (typeof init.headers === 'object') {
        Object.entries(init.headers).forEach(([k, v]) => { headerObj[k] = v as string; });
      }
      maskedHeaders = privacyEngine?.maskHeaders(headerObj) || headerObj;
    }

    try {
      const response = await originalFetch.call(this, input, init);
      const duration = Date.now() - startTime;

      // Capture response body for errors
      let maskedResponse: string | undefined;
      if (!response.ok && response.status >= 400) {
        try {
          const cloned = response.clone();
          const text = await cloned.text();
          if (text.length < 10000) {
            try {
              const parsed = JSON.parse(text);
              maskedResponse = JSON.stringify(privacyEngine?.maskObject(parsed) || parsed);
            } catch {
              maskedResponse = text.slice(0, 2000);
            }
          }
        } catch {}
      }

      const xhrLog: XhrLog = {
        method,
        url: maskedUrl,
        status: response.status,
        statusText: response.statusText,
        requestHeaders: maskedHeaders,
        requestBody: maskedBody?.slice(0, 5000),
        responseBody: maskedResponse?.slice(0, 5000),
        duration,
        timestamp: startTime,
        error: response.status >= 400 ? `HTTP ${response.status}` : undefined,
      };

      addBreadcrumb({
        type: 'xhr',
        category: 'network',
        message: `${method} ${maskedUrl} → ${response.status} (${duration}ms)`,
        data: { status: response.status, duration },
        timestamp: startTime,
        level: response.status >= 400 ? 'error' : response.status >= 300 ? 'warn' : 'info',
      });

      if (response.status >= 400 || duration > 3000) {
        addEvent({
          type: 'xhr',
          level: response.status >= 500 ? 'error' : 'warn',
          message: `${method} ${maskedUrl} → ${response.status} (${duration}ms)`,
          url: window.location.href,
          xhr: xhrLog,
          tags: { ...config?.userAttributes },
        });
      }

      return response;
    } catch (err: any) {
      const duration = Date.now() - startTime;
      addBreadcrumb({
        type: 'xhr',
        category: 'network',
        message: `${method} ${maskedUrl} → Network Error (${duration}ms)`,
        data: { error: true, duration },
        timestamp: startTime,
        level: 'error',
      });

      addEvent({
        type: 'xhr',
        level: 'error',
        message: `${method} ${maskedUrl} failed: ${err?.message || 'Network error'}`,
        url: window.location.href,
        xhr: {
          method,
          url: maskedUrl,
          status: 0,
          statusText: 'Network Error',
          requestHeaders: maskedHeaders,
          requestBody: maskedBody?.slice(0, 5000),
          duration,
          timestamp: startTime,
          error: err?.message || 'Network error',
        },
        tags: { ...config?.userAttributes },
      });

      throw err;
    }
  };
}

// ==================== Enhanced Performance ====================

function setupEnhancedPerformance(): void {
  if (!window.PerformanceObserver) return;

  try {
    // Long task detection
    const longTaskObserver = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.duration > 50) {
          collectedPerformance['longTaskCount'] = (collectedPerformance['longTaskCount'] || 0) + 1;
          collectedPerformance['longestTask'] = Math.max(
            collectedPerformance['longestTask'] || 0,
            entry.duration
          );
        }
      }
    });
    longTaskObserver.observe({ entryTypes: ['longtask'] });

    // Interaction to Next Paint (INP)
    const inpObserver = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.entryType === 'event') {
          const duration = (entry as any).duration;
          if (!collectedPerformance['inp'] || duration > collectedPerformance['inp']) {
            collectedPerformance['inp'] = duration;
          }
        }
      }
    });
    inpObserver.observe({ entryTypes: ['event'] });

  } catch (err) {
    // Long task / INP not supported, that's fine
  }

  // TTFB from navigation timing
  const navEntries = performance.getEntriesByType('navigation') as PerformanceNavigationTiming[];
  if (navEntries.length > 0) {
    const nav = navEntries[0];
    collectedPerformance['ttfb'] = nav.responseStart - nav.requestStart;
    collectedPerformance['dnsTime'] = nav.domainLookupEnd - nav.domainLookupStart;
    collectedPerformance['tcpTime'] = nav.connectEnd - nav.connectStart;
    collectedPerformance['domInteractive'] = nav.domInteractive;
    collectedPerformance['fullLoadTime'] = nav.loadEventEnd - nav.startTime;
  }
}

// ==================== Core Functions ====================

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

  // Initialize privacy engine
  if (config.privacy) {
    privacyEngine = new PrivacyEngine(config.privacy);
  } else {
    privacyEngine = new PrivacyEngine({ enabled: true }); // Default: mask common patterns
  }

  // Setup global error handlers
  setupErrorHandlers();

  // Setup console interception
  setupConsoleInterception();

  // Setup breadcrumb capture (user actions)
  setupBreadcrumbCapture();

  // Setup XHR/Fetch interception
  setupXhrCapture();

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
  setupEnhancedPerformance();

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
 * Add custom breadcrumb
 */
export function addCustomBreadcrumb(category: string, message: string, data?: Record<string, any>): void {
  addBreadcrumb({
    type: 'custom',
    category,
    message: privacyEngine?.maskText(message) || message,
    data: data ? (privacyEngine?.maskObject(data) || data) : undefined,
    timestamp: Date.now(),
    level: 'info',
  });
}

/**
 * Capture and report an error
 */
export function captureException(error: Error, tags?: Record<string, any>, extra?: Record<string, any>): void {
  if (!config) return;

  const maskedMessage = privacyEngine?.maskText(error.message || String(error)) || error.message || String(error);

  const event: LogEvent = {
    type: 'error',
    level: 'error',
    message: maskedMessage,
    stack: error.stack || '',
    url: privacyEngine?.maskUrl(window.location.href) || window.location.href,
    tags: { ...config.userAttributes, ...tags },
    extra: extra ? (privacyEngine?.maskObject(extra) || extra) : {},
    breadcrumbs: getBreadcrumbs(), // Attach breadcrumbs to errors
  };

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
    url: privacyEngine?.maskUrl(window.location.href) || window.location.href,
    extra: data ? (privacyEngine?.maskObject(data) || data) : {},
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

function uploadScreenshot(base64: string, event: LogEvent): void {
  if (!config) return;

  const eventId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

  fetch(`${config.dsn}/screenshot`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
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

// ==================== Internal Functions ====================

function logMessage(level: string, message: string, extra?: Record<string, any>): void {
  if (!config) return;

  const event: LogEvent = {
    type: level === 'error' ? 'error' : 'info',
    level,
    message: privacyEngine?.maskText(message) || message,
    url: privacyEngine?.maskUrl(window.location.href) || window.location.href,
    extra: extra ? (privacyEngine?.maskObject(extra) || extra) : {},
    tags: { ...config.userAttributes },
  };

  addEvent(event);
}

function addEvent(event: LogEvent): void {
  if (!config) return;

  if (Math.random() > config.sampleRate!) return;

  event.tags = {
    ...event.tags,
    ...getCommonTags(),
  };

  if (Object.keys(collectedPerformance).length > 0) {
    event.performance = { ...collectedPerformance };
  }

  // Attach breadcrumbs to error events
  if (event.type === 'error' && !event.breadcrumbs) {
    event.breadcrumbs = getBreadcrumbs();
  }

  buffer.push(event);

  if (buffer.length >= config.bufferSize!) {
    flush();
  }
}

function sendBatch(events: LogEvent[]): void {
  if (!config) return;

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
    consoleBuffer = [];
  }

  const batch: LogBatch = {
    appId: config.appId,
    release: config.release || '',
    events,
  };

  const payload = JSON.stringify(batch);

  if (navigator.sendBeacon && navigator.sendBeacon(config.dsn, payload)) {
    return;
  }

  fetch(config.dsn, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: payload,
    keepalive: true,
  }).catch((err) => {
    console.error('[LogMonitor] Failed to send batch:', err);
  });
}

function setupErrorHandlers(): void {
  if (!config) return;

  window.onerror = (message, source, lineno, colno, error) => {
    const event: LogEvent = {
      type: 'error',
      level: 'error',
      message: privacyEngine?.maskText(String(message)) || String(message),
      stack: error?.stack || '',
      url: source ? (privacyEngine?.maskUrl(source) || source) : window.location.href,
      line: lineno || 0,
      col: colno || 0,
      tags: { ...config.userAttributes, source: 'onerror' },
      breadcrumbs: getBreadcrumbs(),
    };
    addEvent(event);
    return false;
  };

  if (config.captureUnhandledRejection) {
    window.addEventListener('unhandledrejection', (event) => {
      const err = event.reason;
      const logEvent: LogEvent = {
        type: 'error',
        level: 'error',
        message: privacyEngine?.maskText(err?.message || String(err)) || err?.message || String(err),
        stack: err?.stack || '',
        url: privacyEngine?.maskUrl(window.location.href) || window.location.href,
        tags: { ...config.userAttributes, source: 'unhandledrejection' },
        breadcrumbs: getBreadcrumbs(),
      };
      addEvent(logEvent);
    });
  }
}

function setupConsoleInterception(): void {
  const intercept = (level: string, originalFn: Function) => {
    return (...args: any[]) => {
      originalFn.apply(console, args);

      const message = args.map((arg) => {
        if (typeof arg === 'object') {
          try {
            return JSON.stringify(privacyEngine?.maskObject(arg) || arg);
          } catch {
            return String(arg);
          }
        }
        return privacyEngine?.maskText(String(arg)) || String(arg);
      }).join(' ');

      consoleBuffer.push(`[${level.toUpperCase()}] ${message}`);

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
      flush();

      if (config) {
        const duration = Date.now() - pageStartTime;
        const event: LogEvent = {
          type: 'track',
          level: 'info',
          message: 'page_view_duration',
          url: privacyEngine?.maskUrl(window.location.href) || window.location.href,
          extra: { duration },
          tags: { ...config.userAttributes },
        };
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
    addBreadcrumb: addCustomBreadcrumb,
    captureScreenshot: captureException,
    cobrowse: {
      start: () => import('./cobrowse').then(m => m.start()),
      stop: () => import('./cobrowse').then(m => m.stop()),
      getStatus: () => import('./cobrowse').then(m => m.getStatus()),
      setControlMode: (enabled: boolean) => import('./cobrowse').then(m => m.setControlMode(enabled)),
    }
  };
}

export * from './cobrowse';
