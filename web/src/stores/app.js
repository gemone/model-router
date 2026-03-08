import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

export const useAppStore = defineStore('app', () => {
  // State
  const profiles = ref([])
  const providers = ref([])
  const models = ref([])
  const routeRules = ref([])
  const stats = ref({})
  const trendStats = ref({
    hours: [],
    requests: [],
    tokens: []
  })
  const providerModelStats = ref({
    providers: [],
    models: []
  })
  const logs = ref([])
  const loading = ref(false)
  const sidebarCollapsed = ref(false)

  // Authentication state
  const token = ref(localStorage.getItem('admin_token') || '')
  const isAuthenticated = computed(() => !!token.value)

  // Store interceptor IDs to prevent memory leaks
  let requestInterceptorId = null
  let responseInterceptorId = null

  // Authentication methods
  function setToken(newToken) {
    token.value = newToken
    if (newToken) {
      localStorage.setItem('admin_token', newToken)
    } else {
      localStorage.removeItem('admin_token')
    }
    // Update interceptor without re-registering
    // The interceptor uses the reactive token.value, so it will automatically pick up changes
  }

  function clearToken() {
    token.value = ''
    localStorage.removeItem('admin_token')
  }

  async function login(password) {
    try {
      const { data } = await axios.post('/api/admin/login', { password })
      if (data.success) {
        // The backend doesn't return the token in the response for security
        // Use the password directly as the token (since ADMIN_TOKEN IS the password in this design)
        setToken(password)
        return true
      }
      return false
    } catch (error) {
      console.error('Login failed:', error)
      throw error
    }
  }

  async function logout() {
    try {
      await axios.post('/api/admin/logout')
    } finally {
      clearToken()
    }
  }

  async function checkAuth() {
    try {
      const { data } = await axios.get('/api/admin/auth/status')
      return data.enabled
    } catch {
      return false
    }
  }

  // Setup axios interceptor for token authentication (only once)
  function setupAxiosInterceptor() {
    // Remove existing interceptors if they exist
    if (requestInterceptorId !== null) {
      axios.interceptors.request.eject(requestInterceptorId)
    }
    if (responseInterceptorId !== null) {
      axios.interceptors.response.eject(responseInterceptorId)
    }

    // Register new interceptors
    requestInterceptorId = axios.interceptors.request.use(
      (config) => {
        if (token.value) {
          config.headers.Authorization = `Bearer ${token.value}`
        }
        return config
      },
      (error) => {
        return Promise.reject(error)
      }
    )

    responseInterceptorId = axios.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401) {
          clearToken()
          window.location.hash = '/login'
        }
        return Promise.reject(error)
      }
    )
  }

  // Initialize interceptor on store creation
  setupAxiosInterceptor()

  // Getters
  const profileOptions = computed(() => {
    return profiles.value.map(p => ({ label: p.name, value: p.id }))
  })

  const providerOptions = computed(() => {
    return providers.value.map(p => ({ label: p.name, value: p.id }))
  })

  // Actions
  async function fetchProfiles() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/admin/profiles')
      profiles.value = data
    } finally {
      loading.value = false
    }
  }

  async function fetchProviders() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/admin/providers')
      providers.value = data
    } finally {
      loading.value = false
    }
  }

  async function fetchModels() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/admin/models')
      models.value = data
    } finally {
      loading.value = false
    }
  }

  async function fetchStats() {
    try {
      const { data } = await axios.get('/api/admin/stats/dashboard')
      stats.value = data
    } catch (e) {
      console.error('Failed to fetch stats:', e)
    }
  }

  async function fetchTrendStats() {
    try {
      const { data } = await axios.get('/api/admin/stats/trend')
      trendStats.value = data
    } catch (e) {
      console.error('Failed to fetch trend stats:', e)
    }
  }

  async function fetchProviderModelStats() {
    try {
      const { data } = await axios.get('/api/admin/stats/all')
      providerModelStats.value = data
    } catch (e) {
      console.error('Failed to fetch provider/model stats:', e)
    }
  }

  async function fetchLogs(page = 1, pageSize = 50) {
    loading.value = true
    try {
      const { data } = await axios.get('/api/admin/logs', {
        params: { page, pageSize }
      })
      logs.value = data.logs || []
    } finally {
      loading.value = false
    }
  }

  async function clearLogs() {
    await axios.delete('/api/admin/logs')
    logs.value = []
  }

  async function createProfile(profile) {
    const { data } = await axios.post('/api/admin/profiles', profile)
    await fetchProfiles()
    return data
  }

  async function updateProfile(id, profile) {
    const { data } = await axios.put(`/api/admin/profiles/${id}`, profile)
    await fetchProfiles()
    return data
  }

  async function deleteProfile(id) {
    await axios.delete(`/api/admin/profiles/${id}`)
    await fetchProfiles()
  }

  async function createProvider(provider) {
    const { data } = await axios.post('/api/admin/providers', provider)
    await fetchProviders()
    return data
  }

  async function updateProvider(id, provider) {
    const { data } = await axios.put(`/api/admin/providers/${id}`, provider)
    await fetchProviders()
    return data
  }

  async function deleteProvider(id) {
    await axios.delete(`/api/admin/providers/${id}`)
    await fetchProviders()
  }

  async function createModel(model) {
    const { data } = await axios.post('/api/admin/models', model)
    await fetchModels()
    return data
  }

  async function updateModel(id, model) {
    const { data } = await axios.put(`/api/admin/models/${id}`, model)
    await fetchModels()
    return data
  }

  async function deleteModel(id) {
    await axios.delete(`/api/admin/models/${id}`)
    await fetchModels()
  }

  async function testModel(providerId, modelName) {
    const { data } = await axios.post('/api/admin/test', {
      provider_id: providerId,
      model: modelName
    })
    return data
  }

  async function testProvider(providerId) {
    try {
      // 确保模型数据已加载
      await fetchModels()
      // 获取该 provider 下的第一个可用模型
      const providerModels = models.value.filter(m => m.provider_id === providerId && m.enabled)
      if (providerModels.length === 0) {
        throw new Error('No enabled models found for this provider')
      }
      const modelName = providerModels[0].name
      return await testModel(providerId, modelName)
    } catch (e) {
      throw e
    }
  }

  async function fetchSettings() {
    try {
      const { data } = await axios.get('/api/admin/settings')
      return data
    } catch (e) {
      console.error('Failed to fetch settings:', e)
      throw e
    }
  }

  async function updateSettings(settings) {
    try {
      const { data } = await axios.put('/api/admin/settings', settings)
      return data
    } catch (e) {
      console.error('Failed to update settings:', e)
      throw e
    }
  }

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  async function fetchRouteRules() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/admin/routes')
      routeRules.value = data
    } finally {
      loading.value = false
    }
  }

  async function createRouteRule(rule) {
    const { data } = await axios.post('/api/admin/routes', rule)
    await fetchRouteRules()
    return data
  }

  async function updateRouteRule(id, rule) {
    const { data } = await axios.put(`/api/admin/routes/${id}`, rule)
    await fetchRouteRules()
    return data
  }

  async function deleteRouteRule(id) {
    await axios.delete(`/api/admin/routes/${id}`)
    await fetchRouteRules()
  }

  // Rule management
  async function fetchRules() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/admin/rules')
      return data
    } finally {
      loading.value = false
    }
  }

  async function fetchRulesByProfile(profileId) {
    const { data } = await axios.get(`/api/admin/profiles/${profileId}/rules`)
    return data
  }

  async function createRule(rule) {
    const { data } = await axios.post('/api/admin/rules', rule)
    return data
  }

  async function updateRule(id, rule) {
    const { data } = await axios.put(`/api/admin/rules/${id}`, rule)
    return data
  }

  async function deleteRule(id) {
    await axios.delete(`/api/admin/rules/${id}`)
  }

  async function enableRule(id) {
    const { data } = await axios.put(`/api/admin/rules/${id}/enable`)
    return data
  }

  async function disableRule(id) {
    const { data } = await axios.put(`/api/admin/rules/${id}/disable`)
    return data
  }

  return {
    // State
    profiles,
    providers,
    models,
    routeRules,
    stats,
    trendStats,
    providerModelStats,
    logs,
    loading,
    sidebarCollapsed,
    token,
    isAuthenticated,
    // Getters
    profileOptions,
    providerOptions,
    // Actions
    fetchProfiles,
    fetchProviders,
    fetchModels,
    fetchRouteRules,
    fetchStats,
    fetchTrendStats,
    fetchProviderModelStats,
    fetchLogs,
    clearLogs,
    createProfile,
    updateProfile,
    deleteProfile,
    createProvider,
    updateProvider,
    deleteProvider,
    createModel,
    updateModel,
    deleteModel,
    testModel,
    testProvider,
    fetchSettings,
    updateSettings,
    toggleSidebar,
    createRouteRule,
    updateRouteRule,
    deleteRouteRule,
    // Rule management
    fetchRules,
    fetchRulesByProfile,
    createRule,
    updateRule,
    deleteRule,
    enableRule,
    disableRule,
    // Authentication
    setToken,
    clearToken,
    login,
    logout,
    checkAuth,
    setupAxiosInterceptor,
  }
})
