import axios from 'axios'
import { useAuthStore } from '../stores/auth'

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  headers: { 'Content-Type': 'application/json' },
})

// Request interceptor — inject JWT + store ID
client.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.accessToken) {
    config.headers.Authorization = `Bearer ${auth.accessToken}`
  }
  if (auth.currentStoreId) {
    config.headers['X-Store-Id'] = String(auth.currentStoreId)
  }
  return config
})

// Response interceptor — handle 401 auto-refresh
client.interceptors.response.use(
  (res) => res,
  async (error) => {
    if (error.response?.status === 401) {
      const auth = useAuthStore()
      const refreshed = await auth.tryRefresh()
      if (refreshed) {
        error.config.headers.Authorization = `Bearer ${auth.accessToken}`
        return client.request(error.config)
      }
      auth.logout()
    }
    return Promise.reject(error)
  }
)

export default client
