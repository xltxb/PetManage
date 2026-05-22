import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<{ user_id: number; username: string } | null>(null)
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

  function logout() {
    api.logout()
    user.value = null
    localStorage.removeItem('auth_user')
  }

  return { user, loading, login, logout, restoreUser }
})
