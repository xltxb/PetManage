<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const auth = useAuthStore()
const router = useRouter()

interface Metric {
  value: number
  label: string
}

const metrics = ref<Metric[]>([])
const loading = ref(true)
const noData = ref(false)
const period = ref('all')
const lastUpdated = ref('')

const periodLabels: Record<string, string> = {
  all: '全部',
  today: '今日',
  week: '本周',
  month: '本月',
  year: '本年',
}

const metricIcons: Record<string, string> = {
  '商户总数': '🏪',
  '活跃商户': '✅',
  '新增商户': '🆕',
  '累计交易额(元)': '💰',
  '订单总量': '📋',
  '新增会员': '👤',
  '服务完成量': '🔧',
}

async function fetchOverview() {
  loading.value = true
  try {
    const data = await api.getDashboardOverview(period.value)
    metrics.value = data.metrics

    const total = data.metrics.reduce((sum, m) => sum + m.value, 0)
    noData.value = total === 0

    lastUpdated.value = new Date().toLocaleTimeString('zh-CN')
  } catch {
    metrics.value = []
    noData.value = true
  } finally {
    loading.value = false
  }
}

function switchPeriod(p: string) {
  period.value = p
  fetchOverview()
}

function handleLogout() {
  auth.logout()
  router.push('/login')
}

onMounted(() => {
  const token = api.getToken()
  if (!token) {
    router.push('/login')
    return
  }
  fetchOverview()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm">
      <div class="max-w-7xl mx-auto px-4 py-4 flex justify-between items-center">
        <h1 class="text-xl font-bold text-gray-900">宠物店管理系统</h1>
        <div class="flex items-center gap-4">
          <span class="text-sm text-gray-600">{{ auth.user?.username }}</span>
          <button
            @click="handleLogout"
            class="text-sm text-red-600 hover:text-red-800"
          >
            退出登录
          </button>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-8">
      <!-- Title and time switcher -->
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-6">
        <div>
          <h2 class="text-lg font-semibold text-gray-900">平台经营大盘</h2>
          <p v-if="lastUpdated" class="text-xs text-gray-400 mt-1">
            更新时间: {{ lastUpdated }}
          </p>
        </div>
        <div class="flex gap-1 mt-3 sm:mt-0 bg-gray-100 rounded-lg p-1">
          <button
            v-for="(label, key) in periodLabels"
            :key="key"
            @click="switchPeriod(key)"
            :class="[
              'px-3 py-1.5 text-sm rounded-md transition-colors',
              period === key
                ? 'bg-white text-blue-600 shadow-sm font-medium'
                : 'text-gray-500 hover:text-gray-700',
            ]"
          >
            {{ label }}
          </button>
        </div>
      </div>

      <!-- Loading state -->
      <div v-if="loading" class="flex justify-center py-20">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full" />
      </div>

      <!-- No data placeholder -->
      <div v-else-if="noData" class="text-center py-20">
        <div class="text-6xl mb-4">📊</div>
        <p class="text-gray-400 text-lg">暂无数据</p>
        <p class="text-gray-300 text-sm mt-1">平台经营数据将在业务开展后展示</p>
      </div>

      <!-- Metric cards -->
      <div v-else class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
        <div
          v-for="m in metrics"
          :key="m.label"
          class="bg-white rounded-lg shadow-sm p-5 hover:shadow-md transition-shadow"
        >
          <div class="flex items-center justify-between mb-3">
            <span class="text-sm text-gray-500">{{ m.label }}</span>
            <span class="text-xl">{{ metricIcons[m.label] || '📌' }}</span>
          </div>
          <p class="text-3xl font-bold text-gray-900">
            {{ m.value.toLocaleString() }}
          </p>
          <p class="text-xs text-gray-400 mt-2">
            {{ periodLabels[period] }}统计
          </p>
        </div>
      </div>
    </main>
  </div>
</template>
