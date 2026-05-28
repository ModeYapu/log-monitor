import axios from 'axios'
import type { Event, QueryParams, QueryResult, Stats, App, AlertRule, AlertLog, LiveSession, Recording, RecordingEvent } from '../types'

const api = axios.create({
  baseURL: '/logmon-api',
  timeout: 30000
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('API Error:', error)
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

export default api
