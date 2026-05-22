<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()

if (!auth.user) {
  router.replace('/merchant/login')
}

function handleLogout() {
  auth.logout()
  router.push('/merchant/login')
}
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
        <h1 class="text-lg font-semibold text-gray-800">商户经营后台</h1>
        <div class="flex items-center gap-4">
          <span class="text-sm text-gray-600">
            {{ auth.user?.merchant_name || '我的店铺' }}
          </span>
          <span class="text-sm text-gray-600">
            {{ auth.user?.display_name || auth.user?.username }}
          </span>
          <button
            @click="handleLogout"
            class="text-sm text-red-600 hover:text-red-800"
          >
            退出登录
          </button>
        </div>
      </div>
    </header>

    <!-- Content -->
    <main class="max-w-7xl mx-auto px-4 py-8">
      <div class="bg-white rounded-lg shadow-sm p-8">
        <h2 class="text-xl font-semibold mb-4">
          欢迎回来，{{ auth.user?.display_name || auth.user?.username }}
        </h2>
        <p class="text-gray-500">
          店铺：{{ auth.user?.merchant_name || '未设置' }}
        </p>
      </div>
    </main>
  </div>
</template>
