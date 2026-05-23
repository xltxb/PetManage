<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const auth = useAuthStore()
const router = useRouter()

interface EndpointMetric {
  endpoint: string
  method: string
  call_count: number
  success_rate: number
  error_rate: number
  p95_latency_ms: number
  avg_latency_ms: number
}

interface DeveloperMetric {
  developer_id: number
  company_name: string
  call_count: number
  success_rate: number
  error_rate: number
  p95_latency_ms: number
  avg_latency_ms: number
}

const period = ref('24h')
const keyword = ref('')
const sortBy = ref('call_count')
const sortDir = ref('desc')
const endpoints = ref<EndpointMetric[]>([])
const developers = ref<DeveloperMetric[]>([])
const anomalies = ref<EndpointMetric[]>([])
const loading = ref(true)
const lastUpdated = ref('')

const ANOMALY_THRESHOLD = 10

const periodLabels: Record<string, string> = {
  '1h': '1小时',
  '24h': '24小时',
  '7d': '7天',
}

const totalCalls = computed(() => endpoints.value.reduce((s, e) => s + e.call_count, 0))
const avgSuccessRate = computed(() => {
  if (endpoints.value.length === 0) return 100
  const total = endpoints.value.reduce((s, e) => s + e.call_count, 0)
  if (total === 0) return 100
  const success = endpoints.value.reduce((s, e) => s + Math.round(e.call_count * e.success_rate / 100), 0)
  return Math.round(success / total * 10) / 10
})
const avgP95 = computed(() => {
  if (endpoints.value.length === 0) return 0
  const withP95 = endpoints.value.filter(e => e.p95_latency_ms > 0)
  if (withP95.length === 0) return 0
  return Math.round(withP95.reduce((s, e) => s + e.p95_latency_ms, 0) / withP95.length)
})
const anomalyCount = computed(() => anomalies.value.length)

function formatTime() {
  const now = new Date()
  return now.toLocaleTimeString('zh-CN', { hour12: false })
}

async function fetchData() {
  loading.value = true
  try {
    const [eps, devs, anoms] = await Promise.all([
      api.getMonitorEndpoints({ period: period.value, keyword: keyword.value, sort_by: sortBy.value, sort_dir: sortDir.value }),
      api.getMonitorDevelopers(period.value),
      api.getMonitorAnomalies(period.value),
    ])
    endpoints.value = eps || []
    developers.value = devs || []
    anomalies.value = anoms || []
    lastUpdated.value = formatTime()
  } catch (e) {
    console.error('Failed to fetch monitor data:', e)
  } finally {
    loading.value = false
  }
}

function switchPeriod(p: string) {
  period.value = p
}

function toggleSort(col: string) {
  if (sortBy.value === col) {
    sortDir.value = sortDir.value === 'desc' ? 'asc' : 'desc'
  } else {
    sortBy.value = col
    sortDir.value = 'desc'
  }
}

function sortIndicator(col: string): string {
  if (sortBy.value !== col) return ''
  return sortDir.value === 'desc' ? ' ↓' : ' ↑'
}

function errorColor(rate: number): string {
  if (rate > ANOMALY_THRESHOLD) return 'text-red-600 font-bold'
  if (rate > 5) return 'text-orange-500'
  return 'text-green-600'
}

function handleLogout() {
  auth.logout()
  router.push('/login')
}

onMounted(fetchData)
watch([period, keyword, sortBy, sortDir], fetchData)
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm">
      <div class="max-w-7xl mx-auto px-4 py-4 flex justify-between items-center">
        <div class="flex items-center gap-6">
          <h1 class="text-xl font-bold text-gray-900">宠物店管理系统</h1>
          <nav class="flex gap-4 text-sm">
            <router-link to="/" class="text-gray-500 hover:text-blue-600">经营大盘</router-link>
            <router-link to="/monitor" class="text-blue-600 font-medium">API监控</router-link>
          </nav>
        </div>
        <div class="flex items-center gap-4">
          <span class="text-sm text-gray-600">{{ auth.user?.username }}</span>
          <button @click="handleLogout" class="text-sm text-red-600 hover:text-red-800">退出登录</button>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-8">
      <!-- Title and time switcher -->
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-6">
        <div>
          <h2 class="text-lg font-semibold text-gray-900">API监控面板</h2>
          <p v-if="lastUpdated" class="text-xs text-gray-400 mt-1">更新时间: {{ lastUpdated }}</p>
        </div>
        <div class="flex gap-1 mt-3 sm:mt-0 bg-gray-100 rounded-lg p-1">
          <button
            v-for="(label, key) in periodLabels"
            :key="key"
            @click="switchPeriod(key)"
            :class="[
              'px-3 py-1.5 text-sm rounded-md transition-colors',
              period === key ? 'bg-white text-blue-600 shadow-sm font-medium' : 'text-gray-500 hover:text-gray-700',
            ]"
          >
            {{ label }}
          </button>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex justify-center py-20">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full" />
      </div>

      <template v-else>
        <!-- Summary cards -->
        <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
          <div class="bg-white rounded-lg shadow-sm p-4">
            <div class="text-xs text-gray-500 mb-1">总调用量</div>
            <div class="text-2xl font-bold text-gray-900">{{ totalCalls.toLocaleString() }}</div>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-4">
            <div class="text-xs text-gray-500 mb-1">成功率</div>
            <div class="text-2xl font-bold" :class="avgSuccessRate >= 95 ? 'text-green-600' : 'text-orange-500'">
              {{ avgSuccessRate }}%
            </div>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-4">
            <div class="text-xs text-gray-500 mb-1">P95响应时间</div>
            <div class="text-2xl font-bold" :class="avgP95 < 500 ? 'text-green-600' : avgP95 < 1000 ? 'text-orange-500' : 'text-red-600'">
              {{ avgP95 }}ms
            </div>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-4"
               :class="anomalyCount > 0 ? 'ring-2 ring-red-400' : ''">
            <div class="text-xs text-gray-500 mb-1">异常预警</div>
            <div class="text-2xl font-bold" :class="anomalyCount > 0 ? 'text-red-600' : 'text-gray-900'">
              {{ anomalyCount }}
            </div>
            <div v-if="anomalyCount > 0" class="text-xs text-red-500 mt-1">错误率超过 {{ ANOMALY_THRESHOLD }}%</div>
          </div>
        </div>

        <!-- Anomaly alerts -->
        <div v-if="anomalies.length > 0" class="bg-red-50 border border-red-200 rounded-lg p-4 mb-6">
          <h3 class="text-sm font-semibold text-red-700 mb-2">异常预警</h3>
          <div class="space-y-2">
            <div
              v-for="a in anomalies"
              :key="a.endpoint + a.method"
              class="flex items-center justify-between text-sm"
            >
              <div>
                <span class="font-mono text-red-800">{{ a.method }}</span>
                <span class="text-red-700 ml-2">{{ a.endpoint }}</span>
              </div>
              <div class="flex items-center gap-4">
                <span class="text-red-600">调用 {{ a.call_count }} 次</span>
                <span class="text-red-600 font-bold">错误率 {{ a.error_rate }}%</span>
                <span class="text-red-500">P95 {{ a.p95_latency_ms }}ms</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Search -->
        <div class="mb-4">
          <input
            v-model="keyword"
            type="text"
            placeholder="按接口路径搜索..."
            class="w-full max-w-md px-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <!-- Endpoint metrics table -->
        <div class="bg-white rounded-lg shadow-sm overflow-hidden mb-8">
          <div class="px-4 py-3 border-b border-gray-200">
            <h3 class="text-sm font-semibold text-gray-900">接口调用统计</h3>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="bg-gray-50 text-left text-xs text-gray-500 uppercase">
                  <th class="px-4 py-3">接口</th>
                  <th class="px-4 py-3">方法</th>
                  <th class="px-4 py-3 cursor-pointer hover:text-gray-700" @click="toggleSort('call_count')">
                    调用量{{ sortIndicator('call_count') }}
                  </th>
                  <th class="px-4 py-3 cursor-pointer hover:text-gray-700" @click="toggleSort('success_rate')">
                    成功率{{ sortIndicator('success_rate') }}
                  </th>
                  <th class="px-4 py-3 cursor-pointer hover:text-gray-700" @click="toggleSort('error_rate')">
                    错误率{{ sortIndicator('error_rate') }}
                  </th>
                  <th class="px-4 py-3 cursor-pointer hover:text-gray-700" @click="toggleSort('p95_latency_ms')">
                    P95响应{{ sortIndicator('p95_latency_ms') }}
                  </th>
                  <th class="px-4 py-3 cursor-pointer hover:text-gray-700" @click="toggleSort('avg_latency_ms')">
                    平均响应{{ sortIndicator('avg_latency_ms') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100">
                <tr
                  v-for="ep in endpoints"
                  :key="ep.endpoint + ep.method"
                  :class="ep.error_rate > ANOMALY_THRESHOLD ? 'bg-red-50' : ''"
                >
                  <td class="px-4 py-3 font-mono text-gray-800">{{ ep.endpoint }}</td>
                  <td class="px-4 py-3">
                    <span class="px-1.5 py-0.5 rounded text-xs font-medium"
                          :class="ep.method === 'GET' ? 'bg-blue-100 text-blue-700' :
                                   ep.method === 'POST' ? 'bg-green-100 text-green-700' :
                                   ep.method === 'PUT' ? 'bg-yellow-100 text-yellow-700' :
                                   'bg-red-100 text-red-700'">
                      {{ ep.method }}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-gray-900">{{ ep.call_count }}</td>
                  <td class="px-4 py-3 font-medium" :class="ep.success_rate >= 95 ? 'text-green-600' : 'text-orange-500'">
                    {{ ep.success_rate }}%
                  </td>
                  <td class="px-4 py-3 font-medium" :class="errorColor(ep.error_rate)">
                    {{ ep.error_rate }}%
                    <span v-if="ep.error_rate > ANOMALY_THRESHOLD" class="ml-1 text-red-500 text-xs">⚠</span>
                  </td>
                  <td class="px-4 py-3 text-gray-600">{{ ep.p95_latency_ms }}ms</td>
                  <td class="px-4 py-3 text-gray-500">{{ ep.avg_latency_ms }}ms</td>
                </tr>
                <tr v-if="endpoints.length === 0">
                  <td colspan="7" class="px-4 py-8 text-center text-gray-400">暂无数据</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Developer metrics -->
        <div class="bg-white rounded-lg shadow-sm overflow-hidden">
          <div class="px-4 py-3 border-b border-gray-200">
            <h3 class="text-sm font-semibold text-gray-900">开发者调用统计</h3>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="bg-gray-50 text-left text-xs text-gray-500 uppercase">
                  <th class="px-4 py-3">开发者</th>
                  <th class="px-4 py-3">调用量</th>
                  <th class="px-4 py-3">成功率</th>
                  <th class="px-4 py-3">错误率</th>
                  <th class="px-4 py-3">P95响应</th>
                  <th class="px-4 py-3">平均响应</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100">
                <tr v-for="dev in developers" :key="dev.developer_id">
                  <td class="px-4 py-3 font-medium text-gray-900">{{ dev.company_name }}</td>
                  <td class="px-4 py-3 text-gray-900">{{ dev.call_count }}</td>
                  <td class="px-4 py-3 font-medium" :class="dev.success_rate >= 95 ? 'text-green-600' : 'text-orange-500'">
                    {{ dev.success_rate }}%
                  </td>
                  <td class="px-4 py-3 font-medium" :class="errorColor(dev.error_rate)">
                    {{ dev.error_rate }}%
                    <span v-if="dev.error_rate > ANOMALY_THRESHOLD" class="ml-1 text-red-500 text-xs">⚠</span>
                  </td>
                  <td class="px-4 py-3 text-gray-600">{{ dev.p95_latency_ms }}ms</td>
                  <td class="px-4 py-3 text-gray-500">{{ dev.avg_latency_ms }}ms</td>
                </tr>
                <tr v-if="developers.length === 0">
                  <td colspan="6" class="px-4 py-8 text-center text-gray-400">暂无数据</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </main>
  </div>
</template>
