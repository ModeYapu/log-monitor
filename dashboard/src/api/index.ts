import axios from 'axios'
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

// Response interceptor: handle 401 and 500 errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('logmon_token')
      localStorage.removeItem('logmon_user')
      // Only redirect if not already on login page
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login')
      }
    } else if (error.response?.status === 500) {
      if (typeof window !== 'undefined' && (window as any).ElMessage) {
        ;(window as any).ElMessage.error('服务器错误，请稍后重试')
      }
    }
    // Only log non-401 errors to reduce noise
    if (error.response?.status !== 401) {
      console.error('API Error:', error.message)
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

  health: () =>
    api.get<{ status: string; time: number }>('/health')
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
    api.delete<{ success: boolean }>(`/query/recordings/${sessionId}`)
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
      dbSize: number
      totalEvents: number
      totalRecordings: number
      retentionDays: number
      uptime: number
      serverTime: number
    }>('/system/info'),

  triggerCleanup: () =>
    api.post<{
      eventsDeleted: number
      recordingEventsDeleted: number
    }>('/system/cleanup')
}

export default api
