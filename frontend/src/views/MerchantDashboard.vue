<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const router = useRouter()
const auth = useAuthStore()

interface DashboardData {
  today_revenue: number
  today_orders: number
  today_new_members: number
  today_appointments: number
  today_service_complete: number
  stock_warnings: number
  pending_appointments: number
  birthday_reminders: number
  revenue_trend: number[]
  merchant_id: number
}

const data = ref<DashboardData | null>(null)
const loading = ref(true)
const error = ref('')
const shopLogo = ref('')
const shopNotice = ref('')

if (!auth.user) {
  router.replace('/merchant/login')
}

async function loadDashboard() {
  loading.value = true
  error.value = ''
  try {
    const [d, s] = await Promise.all([
      api.getMerchantDashboard(),
      api.getShopSettings().catch(() => null),
    ])
    data.value = d
    if (s) {
      shopLogo.value = s.logo_url || ''
      shopNotice.value = s.notice || ''
    }
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function formatCents(cents: number): string {
  if (cents === 0) return '0.00'
  return (cents / 100).toFixed(2)
}

const trendPoints = computed(() => {
  if (!data.value) return ''
  const values = data.value.revenue_trend
  if (values.length === 0) return ''
  const max = Math.max(...values, 1)
  const w = 560
  const h = 160
  const pad = 10
  const pts = values.map((v, i) => {
    const x = pad + (i / (values.length - 1)) * (w - pad * 2)
    const y = h - pad - (v / max) * (h - pad * 2)
    return `${x},${y}`
  })
  return pts.join(' ')
})

const trendPolyline = computed(() => {
  if (!data.value) return ''
  const values = data.value.revenue_trend
  const allZero = values.every(v => v === 0)
  if (allZero) return ''
  return trendPoints.value
})

const weekDays = ['日', '一', '二', '三', '四', '五', '六']

const trendLabels = computed(() => {
  const labels: string[] = []
  const now = new Date()
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now)
    d.setDate(d.getDate() - i)
    labels.push(`${d.getMonth() + 1}/${d.getDate()}`)
  }
  return labels
})

function handleLogout() {
  auth.logout()
  router.push('/merchant/login')
}

const quickLinks = [
  { label: 'POS收银', path: '/merchant/pos', icon: '💰' },
  { label: '商品管理', path: '/merchant/products', icon: '📦' },
  { label: '订单管理', path: '/merchant/orders', icon: '📋' },
  { label: '预约管理', path: '/merchant/appointments', icon: '📅' },
  { label: '会员管理', path: '/merchant/members', icon: '👤' },
  { label: '库存管理', path: '/merchant/inventory', icon: '🏪' },
  { label: '服务记录', path: '/merchant/services', icon: '📝' },
]

onMounted(loadDashboard)
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-3">
          <img v-if="shopLogo" :src="shopLogo" alt="Logo" class="h-8 w-8 rounded object-contain" />
          <h1 class="text-lg font-semibold text-gray-800">商户经营后台</h1>
        </div>
        <div class="flex items-center gap-4">
          <span class="text-sm text-gray-600">
            {{ auth.user?.merchant_name || '我的店铺' }}
          </span>
          <button
            @click="router.push('/merchant/settings')"
            class="text-sm text-blue-600 hover:text-blue-800 cursor-pointer"
          >
            店铺设置
          </button>
          <span class="text-sm text-gray-500">
            {{ auth.user?.display_name || auth.user?.username }}
          </span>
          <button
            @click="handleLogout"
            class="text-sm text-red-600 hover:text-red-800 cursor-pointer"
          >
            退出登录
          </button>
        </div>
      </div>
    </header>

    <!-- Content -->
    <main class="max-w-7xl mx-auto px-4 py-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full"></div>
        <span class="ml-3 text-gray-500">加载经营数据...</span>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
        <p class="text-red-600">{{ error }}</p>
        <button @click="loadDashboard" class="mt-3 text-sm text-blue-600 hover:text-blue-800 cursor-pointer">
          重新加载
        </button>
      </div>

      <!-- Dashboard -->
      <template v-else-if="data">
        <!-- Shop Notice -->
        <div v-if="shopNotice" class="bg-blue-50 border border-blue-200 rounded-lg px-4 py-3 mb-4">
          <p class="text-sm text-blue-700">{{ shopNotice }}</p>
        </div>

        <!-- Metric Cards -->
        <div class="grid grid-cols-5 gap-4 mb-6">
          <div class="bg-white rounded-lg shadow-sm p-5 border-l-4 border-blue-500">
            <p class="text-sm text-gray-500 mb-1">今日营收</p>
            <p class="text-2xl font-bold text-gray-800">¥{{ formatCents(data.today_revenue) }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5 border-l-4 border-green-500">
            <p class="text-sm text-gray-500 mb-1">订单数</p>
            <p class="text-2xl font-bold text-gray-800">{{ data.today_orders }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5 border-l-4 border-purple-500">
            <p class="text-sm text-gray-500 mb-1">新增会员</p>
            <p class="text-2xl font-bold text-gray-800">{{ data.today_new_members }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5 border-l-4 border-orange-500">
            <p class="text-sm text-gray-500 mb-1">预约数</p>
            <p class="text-2xl font-bold text-gray-800">{{ data.today_appointments }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5 border-l-4 border-teal-500">
            <p class="text-sm text-gray-500 mb-1">服务完成量</p>
            <p class="text-2xl font-bold text-gray-800">{{ data.today_service_complete }}</p>
          </div>
        </div>

        <!-- Alert Area -->
        <div class="grid grid-cols-3 gap-4 mb-6">
          <div class="bg-white rounded-lg shadow-sm p-4 flex items-center gap-3">
            <div class="w-10 h-10 rounded-full flex items-center justify-center text-lg"
              :class="data.stock_warnings > 0 ? 'bg-red-100' : 'bg-gray-100'">
              ⚠️
            </div>
            <div>
              <p class="text-xs text-gray-500">库存预警</p>
              <p class="text-lg font-semibold" :class="data.stock_warnings > 0 ? 'text-red-600' : 'text-gray-400'">
                {{ data.stock_warnings }} 项
              </p>
            </div>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-4 flex items-center gap-3">
            <div class="w-10 h-10 rounded-full flex items-center justify-center text-lg"
              :class="data.pending_appointments > 0 ? 'bg-yellow-100' : 'bg-gray-100'">
              📅
            </div>
            <div>
              <p class="text-xs text-gray-500">待确认预约</p>
              <p class="text-lg font-semibold" :class="data.pending_appointments > 0 ? 'text-yellow-600' : 'text-gray-400'">
                {{ data.pending_appointments }} 条
              </p>
            </div>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-4 flex items-center gap-3">
            <div class="w-10 h-10 rounded-full flex items-center justify-center text-lg"
              :class="data.birthday_reminders > 0 ? 'bg-pink-100' : 'bg-gray-100'">
              🎂
            </div>
            <div>
              <p class="text-xs text-gray-500">会员生日提醒</p>
              <p class="text-lg font-semibold" :class="data.birthday_reminders > 0 ? 'text-pink-600' : 'text-gray-400'">
                {{ data.birthday_reminders }} 人
              </p>
            </div>
          </div>
        </div>

        <!-- 7-Day Revenue Trend -->
        <div class="bg-white rounded-lg shadow-sm p-6 mb-6">
          <h3 class="text-base font-semibold text-gray-800 mb-4">近7日营收趋势</h3>
          <div class="relative">
            <svg viewBox="0 0 560 160" class="w-full h-40">
              <!-- Grid lines -->
              <line v-for="i in 4" :key="'grid'+i"
                :x1="10" :y1="i * 35" :x2="550" :y2="i * 35"
                stroke="#f0f0f0" stroke-width="1" />
              <!-- Axes -->
              <line x1="10" y1="150" x2="550" y2="150" stroke="#e0e0e0" stroke-width="1" />
              <line x1="10" y1="10" x2="10" y2="150" stroke="#e0e0e0" stroke-width="1" />
              <!-- Data line -->
              <polyline
                v-if="trendPolyline"
                :points="trendPolyline"
                fill="none" stroke="#3b82f6" stroke-width="2"
                stroke-linejoin="round" />
              <!-- Data dots -->
              <g v-if="trendPoints">
                <circle
                  v-for="(pt, idx) in trendPoints.split(' ')"
                  :key="'dot'+idx"
                  :cx="pt.split(',')[0]"
                  :cy="pt.split(',')[1]"
                  r="3" fill="#3b82f6" />
              </g>
              <!-- Zero state -->
              <text
                v-if="!trendPolyline"
                x="280" y="80" text-anchor="middle"
                fill="#9ca3af" font-size="14">
                暂无营收数据
              </text>
            </svg>
            <!-- X-axis labels -->
            <div class="flex justify-between px-3 mt-1">
              <span v-for="(label, idx) in trendLabels" :key="'lbl'+idx"
                class="text-xs text-gray-400">
                {{ label }}
              </span>
            </div>
          </div>
        </div>

        <!-- Quick Actions -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-base font-semibold text-gray-800 mb-4">快捷入口</h3>
          <div class="grid grid-cols-6 gap-3">
            <button
              v-for="link in quickLinks"
              :key="link.path"
              class="flex flex-col items-center gap-2 p-4 rounded-lg border border-gray-200 hover:border-blue-300 hover:bg-blue-50 transition-colors cursor-pointer"
            >
              <span class="text-2xl">{{ link.icon }}</span>
              <span class="text-sm text-gray-600">{{ link.label }}</span>
            </button>
          </div>
        </div>
      </template>
    </main>
  </div>
</template>
