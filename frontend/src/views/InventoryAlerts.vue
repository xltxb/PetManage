<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

interface AlertItem {
  id: number
  merchant_id: number
  product_id: number
  name: string
  barcode: string
  stock: number
  alert_stock: number
  expiry_date: string | null
  alert_type: string
  days_left: number | null
  status: string
}

const alerts = ref<AlertItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const alertType = ref(route.query.alert_type as string || '')
const loading = ref(true)
const error = ref('')

const tabs = [
  { key: '', label: '全部预警' },
  { key: 'low_stock', label: '低库存' },
  { key: 'near_expiry', label: '临期' },
  { key: 'expired', label: '已过期' },
]

const alertLabels: Record<string, string> = {
  low_stock: '低库存',
  near_expiry: '临期',
  expired: '已过期',
}

const alertColors: Record<string, string> = {
  low_stock: 'bg-red-100 text-red-700',
  near_expiry: 'bg-orange-100 text-orange-700',
  expired: 'bg-gray-200 text-gray-700',
}

if (!auth.user) {
  router.replace('/merchant/login')
}

async function loadAlerts() {
  loading.value = true
  error.value = ''
  try {
    const result = await api.getInventoryAlerts({
      alert_type: alertType.value || undefined,
      page: page.value,
      page_size: pageSize,
    })
    alerts.value = result.alerts
    total.value = result.total
    page.value = result.page
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function setAlertType(type: string) {
  alertType.value = type
  page.value = 1
  router.replace({ query: type ? { alert_type: type } : {} })
}

function goToProduct(productId: number) {
  router.push('/merchant/products')
}

const totalPages = (): number => Math.max(1, Math.ceil(total.value / pageSize))

watch(alertType, loadAlerts)
watch(page, loadAlerts)

onMounted(loadAlerts)
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-4">
          <button
            @click="router.push('/merchant')"
            class="text-gray-500 hover:text-gray-700 cursor-pointer"
          >
            &larr; 返回
          </button>
          <h1 class="text-lg font-semibold text-gray-800">库存预警</h1>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-6">
      <!-- Filter Tabs -->
      <div class="flex gap-2 mb-6">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          @click="setAlertType(tab.key)"
          class="px-4 py-2 rounded-lg text-sm font-medium border transition-colors cursor-pointer"
          :class="alertType === tab.key
            ? 'bg-blue-600 text-white border-blue-600'
            : 'bg-white text-gray-600 border-gray-300 hover:border-blue-400'"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full"></div>
        <span class="ml-3 text-gray-500">加载预警数据...</span>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
        <p class="text-red-600">{{ error }}</p>
        <button @click="loadAlerts" class="mt-3 text-sm text-blue-600 hover:text-blue-800 cursor-pointer">
          重新加载
        </button>
      </div>

      <!-- Empty -->
      <div v-else-if="alerts.length === 0" class="bg-white rounded-lg shadow-sm p-12 text-center">
        <p class="text-gray-400 text-lg">暂无预警商品</p>
        <p class="text-gray-400 text-sm mt-2">当前没有需要关注的库存预警</p>
      </div>

      <!-- Alert List -->
      <template v-else>
        <div class="bg-white rounded-lg shadow-sm overflow-hidden">
          <table class="w-full">
            <thead class="bg-gray-50 border-b">
              <tr>
                <th class="text-left px-4 py-3 text-xs font-medium text-gray-500">商品名称</th>
                <th class="text-left px-4 py-3 text-xs font-medium text-gray-500">条码</th>
                <th class="text-center px-4 py-3 text-xs font-medium text-gray-500">当前库存</th>
                <th class="text-center px-4 py-3 text-xs font-medium text-gray-500">预警阈值</th>
                <th class="text-center px-4 py-3 text-xs font-medium text-gray-500">有效期</th>
                <th class="text-center px-4 py-3 text-xs font-medium text-gray-500">剩余天数</th>
                <th class="text-center px-4 py-3 text-xs font-medium text-gray-500">预警类型</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100">
              <tr
                v-for="item in alerts"
                :key="item.id"
                class="hover:bg-gray-50 cursor-pointer"
                @click="goToProduct(item.product_id)"
              >
                <td class="px-4 py-3">
                  <span class="text-sm font-medium text-gray-800">{{ item.name }}</span>
                </td>
                <td class="px-4 py-3">
                  <span class="text-sm text-gray-500 font-mono">{{ item.barcode || '—' }}</span>
                </td>
                <td class="px-4 py-3 text-center">
                  <span
                    class="text-sm font-semibold"
                    :class="item.stock <= 0 ? 'text-red-600' : item.stock < item.alert_stock ? 'text-red-600' : 'text-gray-800'"
                  >
                    {{ item.stock }}
                  </span>
                </td>
                <td class="px-4 py-3 text-center">
                  <span class="text-sm text-gray-500">{{ item.alert_stock || '—' }}</span>
                </td>
                <td class="px-4 py-3 text-center">
                  <span
                    class="text-sm"
                    :class="item.alert_type === 'expired' ? 'text-red-600 font-semibold' : 'text-gray-600'"
                  >
                    {{ item.expiry_date || '—' }}
                  </span>
                </td>
                <td class="px-4 py-3 text-center">
                  <span
                    v-if="item.days_left !== null && item.days_left < 0"
                    class="text-sm text-red-600 font-semibold"
                  >
                    已过期{{ Math.abs(item.days_left) }}天
                  </span>
                  <span
                    v-else-if="item.days_left !== null && item.days_left <= 30"
                    class="text-sm text-orange-600"
                  >
                    {{ item.days_left }}天
                  </span>
                  <span v-else class="text-sm text-gray-400">—</span>
                </td>
                <td class="px-4 py-3 text-center">
                  <span
                    class="inline-block px-2 py-1 rounded text-xs font-medium"
                    :class="alertColors[item.alert_type] || 'bg-gray-100 text-gray-600'"
                  >
                    {{ alertLabels[item.alert_type] || item.alert_type }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div class="flex items-center justify-between mt-4 px-2">
          <span class="text-sm text-gray-500">共 {{ total }} 条记录</span>
          <div class="flex items-center gap-2">
            <button
              @click="page--"
              :disabled="page <= 1"
              class="px-3 py-1.5 text-sm border rounded-lg cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              上一页
            </button>
            <span class="text-sm text-gray-600">第 {{ page }} / {{ totalPages() }} 页</span>
            <button
              @click="page++"
              :disabled="page >= totalPages()"
              class="px-3 py-1.5 text-sm border rounded-lg cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed hover:bg-gray-50"
            >
              下一页
            </button>
          </div>
        </div>
      </template>
    </main>
  </div>
</template>
