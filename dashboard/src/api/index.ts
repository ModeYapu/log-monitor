import axios from 'axios'
import { ElMessage } from 'element-plus'
import type { Event, QueryParams, QueryResult, Stats, App, AlertRule, AlertLog, LiveSession, Recording, RecordingEvent, UserInfo, User, LoginRequest, LoginResponse, CreateUserRequest, UpdateUserRequest, ChangePasswordRequest } from '../types'
import router from '../router'

const api = axios.create({
  baseURL: '/logmon-api',
  timeout: 30000
})

// Request interceptor: add token to headers
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('logmon_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor: unified error handling
api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    const path = router.currentRoute.value.path

    if (status === 401) {
      localStorage.removeItem('logmon_token')
      localStorage.removeItem('logmon_user')
      if (path !== '/login') {
        ElMessage.error('登录已过期，请重新登录')
        router.push('/login')
      }
    } else if (status === 403) {
      ElMessage.error('无权限访问该资源')
    } else if (status === 500) {
      ElMessage.error('服务器错误，请稍后重试')
    } else if (status === 404) {
      ElMessage.error('请求的资源不存在')
    } else if (status === 429) {
      ElMessage.warning('请求过于频繁，请稍后再试')
    } else if (!error.response) {
      // Network error
      ElMessage.error('网络连接失败，请检查网络')
    }

    return Promise.reject(error)
  }
)

export const logApi = {
  query: (params: QueryParams) =>
    api.get<QueryResult>('/query/logs', { params }),

  getStats: (appId: string) =>
    api.get<Stats>('/query/stats', { params: { appId } }),

  getApps: () =>
    api.get<App[]>('/query/apps'),

  getAlerts: (appId: string) =>
    api.get<{ rules: AlertRule[], logs: AlertLog[] }>('/query/alerts', { params: { appId } }),

  createAlert: (rule: Omit<AlertRule, 'id'>) =>
    api.post('/query/alerts', rule),

  deleteAlert: (id: number) =>
    api.delete(`/query/alerts/${id}`),

  testAlert: (data: { notify_type: string; notify_config: string; message: string }) =>
    api.post('/alerts/test', data),

  silenceAlert: (data: { id: number; durationMinutes: number }) =>
    api.post('/alerts/silence', data),

  unsilenceAlert: (data: { id: number }) =>
    api.post('/alerts/unsilence', data),

  health: () =>
    api.get<{ status: string; time: number }>('/health'),

  getTop: (params: { appId: string; type?: string; orderBy?: string; level?: string; limit?: number; startTime?: number; endTime?: number }) =>
    api.get<{ type: string; data: Array<{ key: string; count: number; users: number; lastSeen: number; firstSeen: number; isNew: boolean; impactScore: number }> }>('/query/top', { params }),

  getSimilar: (params: { appId: string; message: string; threshold?: number; limit?: number }) =>
    api.get<{ query: string; clusters: any[] }>('/query/similar', { params }),

  exportData: (params: { appId: string; type?: string; level?: string; release?: string; env?: string; keyword?: string; format?: string }) =>
    api.get('/query/export', { params, responseType: params.format === 'csv' ? 'blob' : 'json' }),

  getClusters: (params: { appId: string; startTime?: number; endTime?: number; limit?: number }) =>
    api.get<{ total: number; data: Array<{ fingerprint: string; message: string; count: number; users: number; firstSeen: number; lastSeen: number; urls: string[]; releases: string[] }> }>('/query/clusters', { params }),

  getClusterEvents: (params: { appId: string; fingerprint: string; page?: number; pageSize?: number }) =>
    api.get<{ fingerprint: string; total: number; page: number; pageSize: number; data: Event[] }>(`/query/clusters/${params.fingerprint}/events`, { params: { appId: params.appId, page: params.page, pageSize: params.pageSize } }),

  getReleaseHealth: (params: { appId: string; startTime?: number; endTime?: number }) =>
    api.get<{ releases: Array<{ release: string; env: string; totalSessions: number; crashSessions: number; crashFreeRate: number; errorCount: number; firstSeen: number; lastSeen: number; adoptionRate: number }>; totalSessions: number }>('/query/release-health', { params }),

  getSessionStats: (params: { appId: string; startTime?: number; endTime?: number }) =>
    api.get<{ totalSessions: number; crashSessions: number; crashFreeRate: number; errorCount: number; avgSessionDuration: number; startTime: number; endTime: number }>('/query/session-stats', { params }),

  // Performance API
  getPerformanceSummary: (params: { app_id: string; range?: string }) =>
    api.get<{
      fcp: { p75: number; grade: string };
      lcp: { p75: number; grade: string };
      cls: { p75: number; grade: string };
      inp: { p75: number; grade: string };
      ttfb: { p75: number; grade: string };
    }>('/query/performance/summary', { params }),

  getPerformanceTrend: (params: { app_id: string; metric: string; granularity?: string }) =>
    api.get<{ metric: string; granularity: string; data: Array<{ timestamp: number; value: number; count: number }> }>('/query/performance/trend', { params }),

  getPerformancePages: (params: { app_id: string; range?: string }) =>
    api.get<{ time_range: string; data: Array<{ url: string; fcp_p75: number; lcp_p75: number; cls_p75: number; inp_p75: number; ttfb_p75: number; samples: number; previous_period?: { lcp_change?: number } }> }>('/query/performance/pages', { params }),

  getPerformanceRegression: (params: { app_id: string }) =>
    api.get<{ regressions: Array<{ url: string; metric: string; current_value: number; previous_value: number; change_percent: number; grade: string }>; count: number }>('/query/performance/regression', { params })
}

export const cobrowseApi = {
  getLiveSessions: () =>
    api.get<{ data: LiveSession[] }>('/query/live-sessions'),

  getRecordings: (params?: { limit?: number; offset?: number }) =>
    api.get<{ data: Recording[] }>('/query/recordings', { params }),

  getRecording: (sessionId: string) =>
    api.get<Recording>(`/query/recordings/${sessionId}`),

  getRecordingEvents: (sessionId: string, params?: { limit?: number; offset?: number }) =>
    api.get<{ sessionId: string; events: RecordingEvent[] }>(`/query/recordings/${sessionId}`, {
      params: { events: true, ...params }
    }),

  getRecordingStats: (sessionId: string) =>
    api.get<any>(`/query/recordings/${sessionId}/stats`),

  deleteRecording: (sessionId: string) =>
    api.delete<{ success: boolean }>(`/query/recordings/${sessionId}`),

  getSessionEvents: (sessionId: string, params?: { limit?: number }) =>
    api.get<{ sessionId: string; events: Event[]; errorCount: number; totalEvents: number }>(`/query/sessions/${sessionId}`, { params })
}

export const authApi = {
  // Login - no token required
  login: (data: LoginRequest) => {
    // Create a separate instance without auth interceptor for login
    const authApi = axios.create({
      baseURL: '/logmon-api',
      timeout: 30000
    })
    return authApi.post<LoginResponse>('/auth/login', data)
  },

  // Get current user info
  me: () =>
    api.get<UserInfo>('/auth/me'),

  // Change password
  changePassword: (data: ChangePasswordRequest) =>
    api.put<{ message: string }>('/auth/password', data),

  // List all users (admin only)
  listUsers: () =>
    api.get<{ data: User[] }>('/users'),

  // Create user (admin only)
  createUser: (data: CreateUserRequest) =>
    api.post<{ message: string; user: { id: number; username: string; role: string } }>('/users', data),

  // Update user (admin only)
  updateUser: (id: number, data: UpdateUserRequest) =>
    api.put<{ message: string }>(`/users/${id}`, data),

  // Delete user (admin only)
  deleteUser: (id: number) =>
    api.delete<{ message: string }>(`/users/${id}`),

  // Reset password (admin only)
  resetPassword: (id: number, data: { new_password: string }) =>
    api.put<{ message: string }>(`/users/${id}`, data)
}

export const systemApi = {
  getSystemInfo: () =>
    api.get<{
      status: string
      version: string
      dbSize: number
      totalEvents: number
      totalRecordings: number
      retentionDays: number
      uptime: number
      serverTime: number
      lastCleanupTime: number
    }>('/system/info'),

  triggerCleanup: (days?: number) =>
    api.post<{
      eventsDeleted: number
      recordingEventsDeleted: number
      alertLogsDeleted: number
      lastCleanupTime: number
    }>('/system/cleanup', undefined, { params: days ? { days } : undefined })
}

export default api
