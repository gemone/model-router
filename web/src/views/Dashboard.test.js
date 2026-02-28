import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from './Dashboard.vue'
import ElementPlus from 'element-plus'

// Mock vue-echarts
vi.mock('vue-echarts', () => ({
  default: {
    name: 'VChart',
    render: () => null,
  },
}))

describe('Dashboard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  const createWrapper = () => {
    const router = createRouter({
      history: createWebHistory(),
      routes: [{ path: '/', name: 'Dashboard', component: Dashboard }],
    })

    return mount(Dashboard, {
      global: {
        plugins: [router, ElementPlus],
        mocks: {
          $t: (key) => key,
        },
      },
    })
  }

  it('should render page title', () => {
    const wrapper = createWrapper()
    expect(wrapper.find('.page-title').exists()).toBe(true)
    expect(wrapper.text()).toContain('dashboard.title')
  })

  it('should render stat cards', () => {
    const wrapper = createWrapper()
    const statCards = wrapper.findAll('.stat-card')
    expect(statCards.length).toBeGreaterThan(0)
  })

  it('should render chart cards', () => {
    const wrapper = createWrapper()
    const chartCards = wrapper.findAll('.chart-card')
    expect(chartCards.length).toBeGreaterThan(0)
  })

  it('should have time range selector', () => {
    const wrapper = createWrapper()
    expect(wrapper.find('.el-radio-group').exists()).toBe(true)
  })
})
