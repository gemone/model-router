import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from './app'
import axios from 'axios'

// Mock axios
vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('App Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('State', () => {
    it('should have initial state', () => {
      const store = useAppStore()

      expect(store.profiles).toEqual([])
      expect(store.providers).toEqual([])
      expect(store.models).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.sidebarCollapsed).toBe(false)
    })
  })

  describe('Getters', () => {
    it('should return profile options', () => {
      const store = useAppStore()
      store.profiles = [
        { id: '1', name: 'Default' },
        { id: '2', name: 'Claude' },
      ]

      expect(store.profileOptions).toEqual([
        { label: 'Default', value: '1' },
        { label: 'Claude', value: '2' },
      ])
    })

    it('should return provider options', () => {
      const store = useAppStore()
      store.providers = [
        { id: 'p1', name: 'OpenAI' },
        { id: 'p2', name: 'Anthropic' },
      ]

      expect(store.providerOptions).toEqual([
        { label: 'OpenAI', value: 'p1' },
        { label: 'Anthropic', value: 'p2' },
      ])
    })
  })

  describe('Actions - Profiles', () => {
    it('should fetch profiles', async () => {
      const mockProfiles = [
        { id: '1', name: 'Default', path: 'default' },
      ]
      axios.get.mockResolvedValue({ data: mockProfiles })

      const store = useAppStore()
      await store.fetchProfiles()

      expect(axios.get).toHaveBeenCalledWith('/api/admin/profiles')
      expect(store.profiles).toEqual(mockProfiles)
      expect(store.loading).toBe(false)
    })

    it('should handle fetch profiles error', async () => {
      axios.get.mockRejectedValue(new Error('Network error'))

      const store = useAppStore()
      await expect(store.fetchProfiles()).rejects.toThrow('Network error')
      expect(store.loading).toBe(false)
    })

    it('should create profile', async () => {
      const newProfile = { name: 'Test', path: 'test' }
      const createdProfile = { id: '123', ...newProfile }
      axios.post.mockResolvedValue({ data: createdProfile })
      axios.get.mockResolvedValue({ data: [createdProfile] })

      const store = useAppStore()
      const result = await store.createProfile(newProfile)

      expect(axios.post).toHaveBeenCalledWith('/api/admin/profiles', newProfile)
      expect(result).toEqual(createdProfile)
    })

    it('should update profile', async () => {
      const profile = { id: '1', name: 'Updated', path: 'updated' }
      axios.put.mockResolvedValue({ data: profile })
      axios.get.mockResolvedValue({ data: [profile] })

      const store = useAppStore()
      const result = await store.updateProfile('1', profile)

      expect(axios.put).toHaveBeenCalledWith('/api/admin/profiles/1', profile)
      expect(result).toEqual(profile)
    })

    it('should delete profile', async () => {
      axios.delete.mockResolvedValue({})
      axios.get.mockResolvedValue({ data: [] })

      const store = useAppStore()
      await store.deleteProfile('1')

      expect(axios.delete).toHaveBeenCalledWith('/api/admin/profiles/1')
    })
  })

  describe('Actions - Providers', () => {
    it('should fetch providers', async () => {
      const mockProviders = [{ id: '1', name: 'OpenAI', type: 'openai' }]
      axios.get.mockResolvedValue({ data: mockProviders })

      const store = useAppStore()
      await store.fetchProviders()

      expect(axios.get).toHaveBeenCalledWith('/api/admin/providers')
      expect(store.providers).toEqual(mockProviders)
    })

    it('should create provider', async () => {
      const newProvider = { name: 'Test', type: 'openai', base_url: 'https://api.test.com' }
      const createdProvider = { id: '1', ...newProvider }
      axios.post.mockResolvedValue({ data: createdProvider })
      axios.get.mockResolvedValue({ data: [createdProvider] })

      const store = useAppStore()
      const result = await store.createProvider(newProvider)

      expect(axios.post).toHaveBeenCalledWith('/api/admin/providers', newProvider)
      expect(result).toEqual(createdProvider)
    })
  })

  describe('Actions - Models', () => {
    it('should fetch models', async () => {
      const mockModels = [{ id: '1', name: 'gpt-4', provider_id: '1' }]
      axios.get.mockResolvedValue({ data: mockModels })

      const store = useAppStore()
      await store.fetchModels()

      expect(axios.get).toHaveBeenCalledWith('/api/admin/models')
      expect(store.models).toEqual(mockModels)
    })

    it('should test model', async () => {
      const testResult = { success: true, latency: 150 }
      axios.post.mockResolvedValue({ data: testResult })

      const store = useAppStore()
      const result = await store.testModel('provider1', 'gpt-4')

      expect(axios.post).toHaveBeenCalledWith('/api/admin/test', {
        provider_id: 'provider1',
        model: 'gpt-4',
      })
      expect(result).toEqual(testResult)
    })
  })

  describe('Actions - UI', () => {
    it('should toggle sidebar', () => {
      const store = useAppStore()

      expect(store.sidebarCollapsed).toBe(false)
      store.toggleSidebar()
      expect(store.sidebarCollapsed).toBe(true)
      store.toggleSidebar()
      expect(store.sidebarCollapsed).toBe(false)
    })
  })
})
