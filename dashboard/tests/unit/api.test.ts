import { describe, it, expect, vi, beforeEach } from 'vitest'
import api, { logApi, cobrowseApi } from '../../src/api/index'
import axios from 'axios'

vi.mock('axios', () => {
  const mockAxiosInstance = {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      response: { use: vi.fn() }
    }
  }
  return {
    default: {
      create: vi.fn(() => mockAxiosInstance)
    }
  }
})

describe('logApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('query logs with params', async () => {
    const mockData = { data: { items: [], total: 0 } }
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue(mockData)

    const result = await logApi.query({ appId: 'test-app', limit: 10 })
    expect(instance.get).toHaveBeenCalledWith('/query/logs', {
      params: { appId: 'test-app', limit: 10 }
    })
  })

  it('getStats with appId', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: {} })
    await logApi.getStats('my-app')
    expect(instance.get).toHaveBeenCalledWith('/query/stats', {
      params: { appId: 'my-app' }
    })
  })

  it('getApps calls correct endpoint', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: [] })
    await logApi.getApps()
    expect(instance.get).toHaveBeenCalledWith('/query/apps')
  })

  it('getAlerts with appId', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: { rules: [], logs: [] } })
    await logApi.getAlerts('my-app')
    expect(instance.get).toHaveBeenCalledWith('/query/alerts', {
      params: { appId: 'my-app' }
    })
  })

  it('createAlert posts rule', async () => {
    const instance = axios.create()
    ;(instance.post as any).mockResolvedValue({ data: {} })
    const rule = { name: 'test', appId: 'app', condition: 'error_rate>5', enabled: true }
    await logApi.createAlert(rule as any)
    expect(instance.post).toHaveBeenCalledWith('/query/alerts', rule)
  })

  it('deleteAlert calls delete', async () => {
    const instance = axios.create()
    ;(instance.delete as any).mockResolvedValue({ data: {} })
    await logApi.deleteAlert(42)
    expect(instance.delete).toHaveBeenCalledWith('/query/alerts/42')
  })

  it('health check', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: { status: 'ok' } })
    await logApi.health()
    expect(instance.get).toHaveBeenCalledWith('/health')
  })
})

describe('cobrowseApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getLiveSessions', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: { data: [] } })
    await cobrowseApi.getLiveSessions()
    expect(instance.get).toHaveBeenCalledWith('/query/live-sessions')
  })

  it('getRecordings with params', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: { data: [] } })
    await cobrowseApi.getRecordings({ limit: 10, offset: 0 })
    expect(instance.get).toHaveBeenCalledWith('/query/recordings', {
      params: { limit: 10, offset: 0 }
    })
  })

  it('getRecording by sessionId', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: {} })
    await cobrowseApi.getRecording('sess-123')
    expect(instance.get).toHaveBeenCalledWith('/query/recordings/sess-123')
  })

  it('getRecordingEvents', async () => {
    const instance = axios.create()
    ;(instance.get as any).mockResolvedValue({ data: { sessionId: 's1', events: [] } })
    await cobrowseApi.getRecordingEvents('s1', { limit: 50 })
    expect(instance.get).toHaveBeenCalledWith('/query/recordings/s1', {
      params: { events: true, limit: 50 }
    })
  })

  it('deleteRecording', async () => {
    const instance = axios.create()
    ;(instance.delete as any).mockResolvedValue({ data: { success: true } })
    await cobrowseApi.deleteRecording('sess-456')
    expect(instance.delete).toHaveBeenCalledWith('/query/recordings/sess-456')
  })
})
