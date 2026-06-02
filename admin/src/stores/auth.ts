import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import client from '../api/client'

export interface StoreInfo {
  id: number
  name: string
  role: string
}

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(localStorage.getItem('access') || '')
  const refreshToken = ref(localStorage.getItem('refresh') || '')
  const stores = ref<StoreInfo[]>([])
  const currentStoreId = ref<number>(Number(localStorage.getItem('storeId')) || 0)
  const currentRole = ref(localStorage.getItem('role') || '')
  const userDisplayName = ref('')

  const isLoggedIn = computed(() => !!accessToken.value)

  async function login(username: string, password: string) {
    const { data } = await client.post('/auth/login', { username, password })
    accessToken.value = data.data.access
    refreshToken.value = data.data.refresh
    stores.value = data.data.stores
    if (stores.value.length > 0) {
      currentStoreId.value = stores.value[0].id
      currentRole.value = stores.value[0].role
      localStorage.setItem('storeId', String(currentStoreId.value))
      localStorage.setItem('role', currentRole.value)
    }
    localStorage.setItem('access', accessToken.value)
    localStorage.setItem('refresh', refreshToken.value)
  }

  async function tryRefresh(): Promise<boolean> {
    try {
      const { data } = await client.post('/auth/refresh', {
        refresh_token: refreshToken.value,
      })
      accessToken.value = data.data.access
      refreshToken.value = data.data.refresh
      localStorage.setItem('access', accessToken.value)
      localStorage.setItem('refresh', refreshToken.value)
      return true
    } catch {
      return false
    }
  }

  function switchStore(storeId: number) {
    const store = stores.value.find((s) => s.id === storeId)
    if (store) {
      currentStoreId.value = store.id
      currentRole.value = store.role
      localStorage.setItem('storeId', String(store.id))
      localStorage.setItem('role', store.role)
    }
  }

  function logout() {
    accessToken.value = ''
    refreshToken.value = ''
    stores.value = []
    localStorage.clear()
    window.location.href = '/login'
  }

  return { accessToken, refreshToken, stores, currentStoreId, currentRole, userDisplayName, isLoggedIn, login, tryRefresh, switchStore, logout }
})
