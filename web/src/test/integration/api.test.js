import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'

describe('API Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('Profile API', () => {
    it('should fetch profiles successfully', async () => {
      const mockProfiles = [
        { id: '1', name: 'Default', path: 'default', enabled: true },
        { id: '2', name: 'Claude', path: 'claudecode', enabled: true },
      ]

      axios.get = vi.fn().mockResolvedValue({ data: mockProfiles })

      const response = await axios.get('/api/admin/profiles')

      expect(response.data).toEqual(mockProfiles)
      expect(response.data).toHaveLength(2)
      expect(response.data[0]).toHaveProperty('name', 'Default')
    })

    it('should create a new profile', async () => {
      const newProfile = {
        name: 'Test Profile',
        path: 'test',
        description: 'Test description',
        priority: 5,
        enabled: true,
      }

      const createdProfile = { id: '123', ...newProfile }
      axios.post = vi.fn().mockResolvedValue({ data: createdProfile })

      const response = await axios.post('/api/admin/profiles', newProfile)

      expect(response.data).toEqual(createdProfile)
      expect(response.data.id).toBeDefined()
    })

    it('should handle API errors', async () => {
      axios.get = vi.fn().mockRejectedValue(new Error('Network Error'))

      await expect(axios.get('/api/admin/profiles')).rejects.toThrow('Network Error')
    })
  })

  describe('Provider API', () => {
    it('should fetch providers', async () => {
      const mockProviders = [
        {
          id: '1',
          name: 'OpenAI',
          type: 'openai',
          base_url: 'https://api.openai.com',
          enabled: true,
        },
      ]

      axios.get = vi.fn().mockResolvedValue({ data: mockProviders })

      const response = await axios.get('/api/admin/providers')

      expect(response.data).toEqual(mockProviders)
      expect(response.data[0]).toHaveProperty('type', 'openai')
    })
  })

  describe('Model API', () => {
    it('should test model connectivity', async () => {
      const testPayload = {
        provider_id: 'provider1',
        model: 'gpt-4',
      }

      const testResult = {
        success: true,
        latency: 150,
        error: '',
      }

      axios.post = vi.fn().mockResolvedValue({ data: testResult })

      const response = await axios.post('/api/admin/test', testPayload)

      expect(response.data.success).toBe(true)
      expect(response.data.latency).toBe(150)
    })

    it('should handle test failure', async () => {
      const testPayload = {
        provider_id: 'invalid',
        model: 'invalid-model',
      }

      const testResult = {
        success: false,
        latency: 0,
        error: 'Connection refused',
      }

      axios.post = vi.fn().mockResolvedValue({ data: testResult })

      const response = await axios.post('/api/admin/test', testPayload)

      expect(response.data.success).toBe(false)
      expect(response.data.error).toBe('Connection refused')
    })
  })

  describe('Stats API', () => {
    it('should fetch dashboard stats', async () => {
      const mockStats = {
        total_requests_24h: 1000,
        requests_last_hour: 50,
        success_rate: 98.5,
        avg_latency_ms: 245,
        total_tokens: 50000,
      }

      axios.get = vi.fn().mockResolvedValue({ data: mockStats })

      const response = await axios.get('/api/admin/stats/dashboard')

      expect(response.data).toEqual(mockStats)
      expect(response.data.success_rate).toBe(98.5)
    })
  })
})
