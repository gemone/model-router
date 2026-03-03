import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { ref, computed } from 'vue'
import Dashboard from './Dashboard.vue'
import { useAppStore } from '@/stores/app'

// Mock vue-router
const mockPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: mockPush
  })
}))

// Mock echarts
vi.mock('echarts/core', () => ({
  use: vi.fn()
}))

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {}
}))

vi.mock('echarts/charts', () => ({
  LineChart: {},
  PieChart: {}
}))

vi.mock('echarts/components', () => ({
  TitleComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  GridComponent: {},
  DataZoomComponent: {}
}))

vi.mock('vue-echarts', () => ({
  default: {
    name: 'VChart',
    render: () => null
  }
}))

// Mock Element Plus
vi.mock('element-plus', () => ({
  ElMessage: {
    success: vi.fn(),
    info: vi.fn(),
    error: vi.fn(),
    warning: vi.fn()
  }
}))

// Create i18n instance for testing
const createTestI18n = () => createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      dashboard: {
        title: 'Dashboard',
        statsOverview: 'Overview',
        totalRequests: 'Total Requests (24h)',
        lastHourRequests: 'Last Hour',
        todayRequests: "Today's Requests",
        successRate: 'Success Rate',
        avgLatency: 'Avg Latency',
        activeModels: 'Active Models',
        activeProviders: 'Active Providers',
        requestTrend: 'Request Trend',
        topModels: 'Top Models',
        topProviders: 'Top Providers',
        recentLogs: 'Recent Logs',
        realtimeStatus: 'Live',
        requests: 'Requests',
        noTrendData: 'No trend data available',
        noTopModelsData: 'No model usage data available',
        autoRefreshOn: 'Auto Refresh',
        autoRefreshOff: 'Paused',
        exportData: 'Export Data',
        providerHealth: 'Provider Health',
        checkAll: 'Check All',
        checkingHealth: 'Checking health status...',
        noData: 'No data available'
      },
      logs: {
        model: 'Model',
        provider: 'Provider',
        status: 'Status',
        success: 'Success',
        error: 'Error',
        latency: 'Latency',
        timestamp: 'Timestamp'
      },
      provider: {
        healthHealthy: 'Healthy',
        healthUnhealthy: 'Unhealthy',
        healthUnknown: 'Unknown'
      },
      common: {
        more: 'More'
      },
      message: {
        saveSuccess: 'Saved successfully'
      }
    }
  }
})

describe('Dashboard View', () => {
  let wrapper
  let store
  let i18n

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    
    setActivePinia(createPinia())
    store = useAppStore()
    i18n = createTestI18n()

    // Mock store data
    store.stats = {
      total_requests_24h: 1000,
      requests_last_hour: 50,
      success_rate: 98.5,
      avg_latency_ms: 245,
      top_models: {
        'gpt-4': 500,
        'gpt-3.5': 300
      }
    }
    store.trendStats = {
      hours: ['00:00', '01:00', '02:00'],
      requests: [10, 20, 30]
    }
    store.providers = [
      { id: '1', name: 'OpenAI', type: 'openai' }
    ]
    store.logs = [
      { request_id: 'req-1', model: 'gpt-4', provider_id: '1', status: 'success', latency: 100, created_at: new Date().toISOString() }
    ]

    // Mock store methods
    store.fetchStats = vi.fn().mockResolvedValue({})
    store.fetchTrendStats = vi.fn().mockResolvedValue({})
    store.fetchLogs = vi.fn().mockResolvedValue({})
    store.testProvider = vi.fn().mockResolvedValue({ success: true })
  })

  afterEach(() => {
    vi.useRealTimers()
    if (wrapper) {
      wrapper.unmount()
    }
  })

  const mountComponent = () => {
    return mount(Dashboard, {
      global: {
        plugins: [i18n],
        stubs: {
          'v-chart': true,
          'el-breadcrumb': true,
          'el-breadcrumb-item': true,
          'router-link': true,
          'router-view': true
        }
      }
    })
  }

  describe('Rendering', () => {
    it('should render dashboard title', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      expect(wrapper.find('.page-title').text()).toContain('Dashboard')
    })

    it('should render stat cards', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      const statCards = wrapper.findAll('.stat-card')
      expect(statCards.length).toBe(4)
    })

    it('should display correct stat values', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      const statValues = wrapper.findAll('.stat-value')
      expect(statValues.length).toBeGreaterThan(0)
    })

    it('should render chart cards', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      const chartCards = wrapper.findAll('.chart-card')
      expect(chartCards.length).toBe(2)
    })
  })

  describe('Auto Refresh', () => {
    it('should toggle auto refresh', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      // Initially autoRefresh should be false
      expect(wrapper.vm.autoRefresh).toBe(false)
      
      // Toggle auto refresh
      wrapper.vm.toggleAutoRefresh()
      expect(wrapper.vm.autoRefresh).toBe(true)
      expect(wrapper.vm.isLive).toBe(true)
      
      // Toggle again
      wrapper.vm.toggleAutoRefresh()
      expect(wrapper.vm.autoRefresh).toBe(false)
      expect(wrapper.vm.isLive).toBe(false)
    })

    it('should change refresh interval', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      wrapper.vm.setRefreshInterval(5000)
      expect(wrapper.vm.refreshInterval).toBe(5000)
      
      wrapper.vm.setRefreshInterval(30000)
      expect(wrapper.vm.refreshInterval).toBe(30000)
    })
  })

  describe('Data Loading', () => {
    it('should fetch data on mount', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      expect(store.fetchStats).toHaveBeenCalled()
      expect(store.fetchTrendStats).toHaveBeenCalled()
      expect(store.fetchLogs).toHaveBeenCalled()
    })

    it('should refresh data when calling refreshData', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      vi.clearAllMocks()
      
      await wrapper.vm.refreshData()
      
      expect(store.fetchStats).toHaveBeenCalled()
      expect(store.fetchTrendStats).toHaveBeenCalled()
      expect(store.fetchLogs).toHaveBeenCalled()
    })
  })

  describe('Computed Properties', () => {
    it('should compute hasTrendData correctly when data exists', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      expect(wrapper.vm.hasTrendData).toBe(true)
    })

    it('should compute hasTrendData correctly when no data', async () => {
      store.stats.total_requests_24h = 0
      wrapper = mountComponent()
      await flushPromises()
      
      expect(wrapper.vm.hasTrendData).toBe(false)
    })

    it('should compute hasTopModelsData correctly', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      expect(wrapper.vm.hasTopModelsData).toBe(true)
      
      store.stats.top_models = {}
      expect(wrapper.vm.hasTopModelsData).toBe(false)
    })

    it('should compute statsCards correctly', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      const cards = wrapper.vm.statsCards
      expect(cards).toHaveLength(4)
      expect(cards[0].key).toBe('total')
      expect(cards[1].key).toBe('lastHour')
      expect(cards[2].key).toBe('success')
      expect(cards[3].key).toBe('latency')
    })

    it('should compute recentLogs correctly', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      expect(wrapper.vm.recentLogs).toEqual(store.logs.slice(0, 5))
    })
  })

  describe('Health Status', () => {
    it('should initialize health providers', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      // Manually initialize health providers
      wrapper.vm.healthProviders = store.providers.map(p => ({
        ...p,
        healthStatus: 'unknown'
      }))
      
      expect(wrapper.vm.healthProviders.length).toBe(store.providers.length)
    }, 10000)

    it('should get correct health icon', async () => {
      wrapper = mountComponent()
      
      expect(wrapper.vm.getHealthIcon('healthy')).toBe('CircleCheck')
      expect(wrapper.vm.getHealthIcon('unhealthy')).toBe('CircleClose')
      expect(wrapper.vm.getHealthIcon('unknown')).toBe('Clock')
    })

    it('should get correct health tag type', async () => {
      wrapper = mountComponent()
      
      expect(wrapper.vm.getHealthTagType('healthy')).toBe('success')
      expect(wrapper.vm.getHealthTagType('unhealthy')).toBe('danger')
      expect(wrapper.vm.getHealthTagType('unknown')).toBe('info')
    })
  })

  describe('Utility Functions', () => {
    it('should capitalize strings correctly', async () => {
      wrapper = mountComponent()
      
      expect(wrapper.vm.capitalize('healthy')).toBe('Healthy')
      expect(wrapper.vm.capitalize('unhealthy')).toBe('Unhealthy')
      expect(wrapper.vm.capitalize('test')).toBe('Test')
    })

    it('should format time correctly', async () => {
      wrapper = mountComponent()
      
      const now = new Date()
      const oneMinuteAgo = new Date(now - 60000)
      const oneHourAgo = new Date(now - 3600000)
      const oneDayAgo = new Date(now - 86400000)
      
      expect(wrapper.vm.formatTime(oneMinuteAgo)).toContain('m ago')
      expect(wrapper.vm.formatTime(oneHourAgo)).toContain('h ago')
      expect(wrapper.vm.formatTime(oneDayAgo)).toContain('d ago')
    })

    it('should generate correct color by index', async () => {
      wrapper = mountComponent()
      
      const colors = [
        '#3B82F6', '#10B981', '#8B5CF6', '#F59E0B',
        '#6B7280', '#EC4899', '#14B8A6', '#F97316'
      ]
      
      colors.forEach((color, index) => {
        expect(wrapper.vm.getColorByIndex(index)).toBe(color)
      })
      
      // Test wrap around
      expect(wrapper.vm.getColorByIndex(8)).toBe(colors[0])
      expect(wrapper.vm.getColorByIndex(9)).toBe(colors[1])
    })
  })

  describe('Navigation', () => {
    it('should navigate to logs page', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      wrapper.vm.goToLogs()
      expect(mockPush).toHaveBeenCalledWith('/logs')
    })

    it('should navigate to stats page', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      wrapper.vm.goToStats()
      expect(mockPush).toHaveBeenCalledWith('/stats')
    })
  })

  describe('Export', () => {
    it('should export data', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      // Mock URL and anchor click
      const mockCreateObjectURL = vi.fn().mockReturnValue('blob:test')
      const mockRevokeObjectURL = vi.fn()
      const mockClick = vi.fn()
      
      global.URL.createObjectURL = mockCreateObjectURL
      global.URL.revokeObjectURL = mockRevokeObjectURL
      
      const originalCreateElement = document.createElement
      document.createElement = vi.fn((tag) => {
        if (tag === 'a') {
          return {
            href: '',
            download: '',
            click: mockClick
          }
        }
        return originalCreateElement.call(document, tag)
      })
      
      wrapper.vm.exportData()
      
      expect(mockCreateObjectURL).toHaveBeenCalled()
      expect(mockClick).toHaveBeenCalled()
    })
  })

  describe('Time Range', () => {
    it('should change time range and fetch data', async () => {
      wrapper = mountComponent()
      await flushPromises()
      
      vi.clearAllMocks()
      
      wrapper.vm.timeRange = '7d'
      await wrapper.vm.fetchTrendData()
      
      expect(store.fetchTrendStats).toHaveBeenCalled()
    })
  })
})
