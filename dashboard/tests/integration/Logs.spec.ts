import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import ElementPlus from 'element-plus'
import Logs from '../../src/views/Logs.vue'

const mockLogs = [
  {
    timestamp: Date.now() - 1000,
    type: 'error',
    level: 'error',
    message: 'TypeError: Cannot read property "x" of undefined',
    stack: 'at Component.vue:42:15',
    url: 'https://example.com/page',
    ua: 'Mozilla/5.0 Chrome/120',
    extra: { tag1: 'value1' }
  },
  {
    timestamp: Date.now() - 5000,
    type: 'info',
    level: 'info',
    message: 'Page loaded successfully',
    stack: '',
    url: 'https://example.com/',
    ua: 'Mozilla/5.0 Firefox/121',
    extra: {}
  }
]

vi.mock('../../src/api', () => ({
  logApi: {
    query: vi.fn(() => Promise.resolve({
      data: { items: mockLogs, total: 2 }
    })),
    getApps: vi.fn(() => Promise.resolve({
      data: [
        { app_id: 'webgpu-3d-studio', release: '1.0.0', event_count: 10, error_count: 1, last_seen: Date.now() }
      ]
    }))
  }
}))

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/logs', component: Logs },
    { path: '/logs/:appId?', component: Logs }
  ]
})

describe('Logs.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    router.push('/logs')
  })

  it('renders filter form with app select, level, type, keyword', async () => {
    const wrapper = mount(Logs, {
      global: { plugins: [ElementPlus, router] }
    })
    await flushPromises()

    // Check form items exist
    const selects = wrapper.findAllComponents({ name: 'ElSelect' })
    expect(selects.length).toBeGreaterThanOrEqual(2)
    
    const inputs = wrapper.findAllComponents({ name: 'ElInput' })
    expect(inputs.length).toBeGreaterThanOrEqual(1)
  })

  it('renders logs table component', async () => {
    const wrapper = mount(Logs, {
      global: { plugins: [ElementPlus, router] }
    })
    await flushPromises()

    // Table component exists
    const table = wrapper.findComponent({ name: 'ElTable' })
    expect(table.exists()).toBe(true)
  })

  it('loads data from API on mount', async () => {
    const { logApi } = await import('../../src/api')
    const wrapper = mount(Logs, {
      global: { plugins: [ElementPlus, router] }
    })
    await flushPromises()

    // Verify API was called
    expect(logApi.query).toHaveBeenCalled()
  })

  it('has search and reset buttons', async () => {
    const wrapper = mount(Logs, {
      global: { plugins: [ElementPlus, router] }
    })
    await flushPromises()

    const buttons = wrapper.findAllComponents({ name: 'ElButton' })
    const texts = buttons.map(b => b.text())
    expect(texts.some(t => t.includes('搜索'))).toBe(true)
    expect(texts.some(t => t.includes('重置'))).toBe(true)
  })
})
