import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'

interface UserInfo {
  user_id: number
  username: string
  merchant_id?: number | null
  merchant_name?: string
  display_name?: string
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<UserInfo | null>(null)
  const loading = ref(false)

  function restoreUser() {
    const saved = localStorage.getItem('auth_user')
    if (saved) {
      try { user.value = JSON.parse(saved) } catch { /* ignore */ }
    }
  }

  async function login(username: string, password: string) {
    loading.value = true
    try {
      const data = await api.login(username, password)
      user.value = { user_id: data.user_id, username: data.username }
      localStorage.setItem('auth_user', JSON.stringify(user.value))
      return data
    } finally {
      loading.value = false
    }
  }

  async function merchantLogin(username: string, password: string) {
    loading.value = true
    try {
      const data = await api.merchantLogin(username, password)
      user.value = {
        user_id: data.user_id,
        username: data.username,
        merchant_id: data.merchant_id,
        merchant_name: data.merchant_name,
        display_name: data.display_name,
      }
      localStorage.setItem('auth_user', JSON.stringify(user.value))
      return data
    } finally {
      loading.value = false
    }
  }

  function logout() {
    api.logout()
    user.value = null
    localStorage.removeItem('auth_user')
  }

  return { user, loading, login, merchantLogin, logout, restoreUser }
})
