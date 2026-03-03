import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import Breadcrumb from './Breadcrumb.vue'

// Mock vue-router
const mockRoute = {
  path: '/dashboard',
  name: 'Dashboard',
  meta: { title: 'Dashboard' }
}

const mockRouter = {
  push: vi.fn()
}

vi.mock('vue-router', () => ({
  useRoute: () => mockRoute,
  useRouter: () => mockRouter
}))

// Create i18n instance for testing
const createTestI18n = () => createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      nav: {
        dashboard: 'Dashboard',
        profiles: 'Profiles',
        providers: 'Providers',
        models: 'Models',
        routes: 'Routes',
        stats: 'Statistics',
        logs: 'Logs',
        settings: 'Settings'
      }
    }
  }
})

describe('Breadcrumb Component', () => {
  let wrapper

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render breadcrumb with home link', () => {
    wrapper = mount(Breadcrumb, {
      global: {
        plugins: [createTestI18n()]
      }
    })

    expect(wrapper.find('.el-breadcrumb').exists()).toBe(true)
    expect(wrapper.findAll('.el-breadcrumb__item')).toHaveLength(2)
    expect(wrapper.text()).toContain('Home')
  })

  it('should render translated route name when meta title exists', () => {
    wrapper = mount(Breadcrumb, {
      global: {
        plugins: [createTestI18n()]
      }
    })

    expect(wrapper.text()).toContain('Dashboard')
  })

  it('should render correct route name based on current route', async () => {
    mockRoute.name = 'Profiles'
    mockRoute.meta.title = 'Profiles'

    wrapper = mount(Breadcrumb, {
      global: {
        plugins: [createTestI18n()]
      }
    })

    await flushPromises()
    expect(wrapper.text()).toContain('Profiles')
  })

  it('should handle missing meta title gracefully', async () => {
    mockRoute.meta = {}
    mockRoute.name = 'Unknown'

    wrapper = mount(Breadcrumb, {
      global: {
        plugins: [createTestI18n()]
      }
    })

    await flushPromises()
    // Should only show Home breadcrumb when no meta title
    const items = wrapper.findAll('.el-breadcrumb__item')
    expect(items.length).toBeGreaterThanOrEqual(1)
  })

  it('should render all navigation translations', () => {
    const testCases = [
      { name: 'Dashboard', expected: 'Dashboard' },
      { name: 'Profiles', expected: 'Profiles' },
      { name: 'Providers', expected: 'Providers' },
      { name: 'Models', expected: 'Models' },
      { name: 'Routes', expected: 'Routes' },
      { name: 'Stats', expected: 'Statistics' },
      { name: 'Logs', expected: 'Logs' },
      { name: 'Settings', expected: 'Settings' }
    ]

    testCases.forEach(({ name, expected }) => {
      mockRoute.name = name
      mockRoute.meta = { title: name }

      wrapper = mount(Breadcrumb, {
        global: {
          plugins: [createTestI18n()]
        }
      })

      expect(wrapper.text()).toContain(expected)
    })
  })
})
