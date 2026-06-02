<template>
  <div class="min-h-screen flex items-center justify-center" style="background: var(--color-canvas)">
    <div class="kpi-card w-full max-w-sm">
      <h1 class="text-2xl font-bold text-center mb-6" style="color: var(--color-ink)">🐾 爪迹 PawPrint</h1>
      <form @submit.prevent="handleLogin" class="space-y-4">
        <input v-model="username" type="text" placeholder="用户名"
          class="w-full px-4 py-2.5 rounded-lg border border-black/10 text-base" autocomplete="username" />
        <input v-model="password" type="password" placeholder="密码"
          class="w-full px-4 py-2.5 rounded-lg border border-black/10 text-base" autocomplete="current-password" />
        <p v-if="error" class="text-sm text-center" style="color: var(--color-berry)">{{ error }}</p>
        <button type="submit" :disabled="loading"
          class="w-full py-2.5 rounded-lg text-white font-medium transition-opacity"
          style="background: var(--color-coral)">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    router.push('/')
  } catch (e: any) {
    error.value = e.response?.data?.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>
