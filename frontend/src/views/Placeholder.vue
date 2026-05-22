<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()

const pageNames: Record<string, string> = {
  '/merchant/products': '商品管理',
  '/merchant/orders': '订单管理',
  '/merchant/appointments': '预约管理',
  '/merchant/members': '会员管理',
  '/merchant/inventory': '库存管理',
  '/merchant/services': '服务记录',
}

const title = pageNames[route.path] || '功能页面'
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center">
        <button
          @click="router.push('/merchant')"
          class="text-sm text-blue-600 hover:text-blue-800 cursor-pointer mr-4"
        >
          &larr; 返回首页
        </button>
        <h1 class="text-lg font-semibold text-gray-800">{{ title }}</h1>
      </div>
    </header>
    <main class="max-w-7xl mx-auto px-4 py-12">
      <div class="bg-white rounded-lg shadow-sm p-12 text-center">
        <p class="text-4xl mb-4">🚧</p>
        <h2 class="text-xl font-semibold text-gray-700 mb-2">{{ title }}</h2>
        <p class="text-gray-500">该功能正在建设中...</p>
      </div>
    </main>
  </div>
</template>
