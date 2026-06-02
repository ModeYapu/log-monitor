export interface Event {
  id: number
  app_id: string
  release: string
  env: string
  build_id: string
  user_id: string
  session_id: string
  type: string
  level: string
  message: string
  stack: string
  url: string
  line: number
  col: number
  tags: string
  extra: string
  ua: string
  screen: string
  viewport: string
  performance: string
  ip: string
  created_at: number
  screenshot_url?: string
}

export interface QueryParams {
  appId: string
  release?: string
  env?: string
  type?: string
  level?: string
  keyword?: string
  startTime?: number
  endTime?: number
  page: number
  pageSize: number
}

export interface QueryResult {
  total: number
  page: number
  size: number
  data: Event[]
}

export interface Stats {
  totalEvents: number
  errorCount: number
  warnCount: number
  infoCount: number
  topErrors: Array<{
    message: string
    count: number
  }>
  errorTrend: Array<{
    timestamp: number
    count: number
  }>
}

export interface App {
  app_id: string
  release: string
  event_count: number
  error_count: number
  last_seen: number
}

export interface AlertRule {
  id?: number
  app_id: string
  name: string
  condition_type: 'threshold' | 'rate' | 'new_error'
  condition_config: Record<string, any>
  notify_type: 'webhook' | 'feishu' | 'email' | 'wecom' | 'dingtalk' | 'telegram'
  notify_config: Record<string, any>
  enabled: boolean
  cooldown_minutes: number
  last_triggered_at?: number
  silenced_until?: number
  created_at?: number
  _toggling?: boolean
}

export interface AlertLog {
  id: number
  rule_id: number
  app_id: string
  message: string
  created_at: number
}

// Co-browsing types
export interface LiveSession {
  sessionId: string
  appId: string
  url: string
  ua: string
  connectedAt: number
  viewerCount: number
  isControlled: boolean
}

export interface Recording {
  id: number
  sessionId: string
  appId: string
  startTime: number
  endTime: number
  durationMs: number
  eventCount: number
  fullSnapshot: string
  url: string
  ua: string
  status: 'recording' | 'completed' | 'error'
  createdAt: number
}

export interface RecordingEvent {
  id: number
  sessionId: string
  seq: number
  timestamp: number
  eventData: string
  createdAt: number
}

export interface ControlCommand {
  type: 'control'
  action: 'click' | 'input' | 'scroll' | 'keydown' | 'navigate'
  x?: number
  y?: number
  selector?: string
  value?: string
  key?: string
  url?: string
}

// Auth types
export interface User {
  id: number
  username: string
  display_name: string
  role: 'admin' | 'user'
  enabled: boolean
  last_login_at: number
  created_at: number
  updated_at: number
}

export interface UserInfo {
  id: number
  username: string
  display_name: string
  role: 'admin' | 'user'
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: UserInfo
}

export interface CreateUserRequest {
  username: string
  password: string
  display_name: string
  role: 'admin' | 'user'
}

export interface UpdateUserRequest {
  display_name: string
  role: 'admin' | 'user'
  enabled: boolean
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface ResetPasswordRequest {
  new_password: string
}
