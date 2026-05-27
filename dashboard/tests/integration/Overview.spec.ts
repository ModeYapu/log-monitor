import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import ElementPlus from 'element-plus'
import Overview from '../../src/views/Overview.vue'

// Mock echarts
vi.mock('echarts', () => ({
  default: {
    init: vi.fn(() => ({
      setOption: vi.fn(),
      resize: vi.fn(),
      dispose: vi.fn()
    })),
    graphic: {
      LinearGradient: vi.fn()
    }
  },
  init: vi.fn(() => ({
    setOption: vi.fn(),
    resize: vi.fn(),
    dispose: vi.fn()
  }))
}))

// Mock API
vi.mock('../../src/api', () => ({
  logApi: {
    getApps: vi.fn(() => Promise.resolve({
      data: [
        { app_id: 'webgpu-3d-studio', release: '1.0.0', event_count: 120, error_count: 5, last_seen: Date.now() },
        { app_id: 'test-app', release: '2.0.0', event_count: 50, error_count: 0, last_seen: Date.now() - 60000 }
      ]
    })),
    getStats: vi.fn(() => Promise.resolve({
      data: {
        totalEvents: 170,
        errorCount: 5,
        warnCount: 3,
        infoCount: 162,
        topErrors: [
          { message: 'TypeError: Cannot read property', count: 3 },
          { message: 'NetworkError: Failed to fetch', count: 2 }
        ],
        errorTrend: Array.from({ length: 24 }, (_, i) => ({
          timestamp: Date.now() - (23 - i) * 3600000,
          count: Math.floor(Math.random() * 10)
        }))
      }
    }))
  }
}))

const router = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', component: Overview },
    { path: '/logs/:appId?', component: { template: '<div>Logs</div>' } }
  ]
})

describe('Overview.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    router.push('/')
  })

  it('renders stat cards with correct labels', async () => {
    const wrapper = mount(Overview, {
      global: {
        plugins: [ElementPlus, router]
      }
    })

    await flushPromises()

    const labels = wrapper.findAll('.stat-label')
    expect(labels.length).toBe(4)
    expect(labels[0].text()).toBe('总事件数')
    expect(labels[1].text()).toBe('错误数')
    expect(labels[2].text()).toBe('警告数')
    expect(labels[3].text()).toBe('信息数')
  })

  it('displays stat values from API', async () => {
    const wrapper = mount(Overview, {
      global: {
        plugins: [ElementPlus, router]
      }
    })

    await flushPromises()

    const values = wrapper.findAll('.stat-value')
    expect(values[0].text()).toBe('170')
    expect(values[1].text()).toBe('5')
    expect(values[2].text()).toBe('3')
  })

  it('renders app table with data', async () => {
    const wrapper = mount(Overview, {
      global: {
        plugins: [ElementPlus, router]
      }
    })

    await flushPromises()

    const rows = wrapper.findAll('.el-table__body-wrapper .el-table__row')
    expect(rows.length).toBe(2)
  })

  it('shows top errors from stats', async () => {
    const wrapper = mount(Overview, {
      global: {
        plugins: [ElementPlus, router]
      }
    })

    await flushPromises()

    const errors = wrapper.findAll('.error-item')
    expect(errors.length).toBe(2)
    expect(errors[0].text()).toContain('TypeError')
  })

  it('navigates to logs on click', async () => {
    const pushSpy = vi.spyOn(router, 'push')
    const wrapper = mount(Overview, {
      global: {
        plugins: [ElementPlus, router]
      }
    })

    await flushPromises()

    const buttons = wrapper.findAllComponents({ name: 'ElButton' })
    const viewLogBtn = buttons.find(b => b.text().includes('查看日志'))
    if (viewLogBtn) {
      await viewLogBtn.trigger('click')
      expect(pushSpy).toHaveBeenCalled()
    }
  })
})
