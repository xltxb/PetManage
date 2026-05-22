import { defineStore } from 'pinia'
import { ref } from 'vue'
import { supabase } from '@/api/supabase'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<any>(null)
  const session = ref<any>(null)

  async function login(email: string, password: string) {
    const { data, error } = await supabase.auth.signInWithPassword({ email, password })
    if (error) throw error
    user.value = data.user
    session.value = data.session
    return data
  }

  async function logout() {
    await supabase.auth.signOut()
    user.value = null
    session.value = null
  }

  async function getSession() {
    const { data } = await supabase.auth.getSession()
    session.value = data.session
    user.value = data.session?.user ?? null
  }

  return { user, session, login, logout, getSession }
})
