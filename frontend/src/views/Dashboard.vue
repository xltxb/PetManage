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

interface MerchantItem {
  id: number
  name: string
  license_number: string
  status: string
  contract_status?: string
  created_at: string
}

const metrics = ref<Metric[]>([])
const loading = ref(true)
const noData = ref(false)
const period = ref('all')
const lastUpdated = ref('')
const merchants = ref<MerchantItem[]>([])
const merchantsLoading = ref(false)
const exportStartDate = ref('')
const exportEndDate = ref('')
const exporting = ref<{ operating: boolean; transactions: boolean }>({ operating: false, transactions: false })
const exportError = ref('')
const exportSuccess = ref('')

const statusLabels: Record<string, string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已驳回',
  frozen: '已冻结',
  closed: '已关停',
}

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-700',
  approved: 'bg-green-100 text-green-700',
  rejected: 'bg-red-100 text-red-700',
  frozen: 'bg-blue-100 text-blue-700',
  closed: 'bg-gray-100 text-gray-700',
}

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

async function fetchMerchants() {
  merchantsLoading.value = true
  try {
    const data = await api.getMerchantList({ page_size: 100 })
    merchants.value = data.merchants
  } catch {
    merchants.value = []
  } finally {
    merchantsLoading.value = false
  }
}

function triggerDownload(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

async function exportOperating() {
  if (!exportStartDate.value || !exportEndDate.value) {
    exportError.value = '请选择起止时间'
    return
  }
  if (exportStartDate.value > exportEndDate.value) {
    exportError.value = '开始时间不能晚于结束时间'
    return
  }
  exportError.value = ''
  exportSuccess.value = ''
  exporting.value.operating = true
  try {
    const { blob, filename } = await api.downloadFile(
      `/api/v1/reports/operating?start_time=${exportStartDate.value}&end_time=${exportEndDate.value}`
    )
    triggerDownload(blob, filename)
    exportSuccess.value = '经营报表已下载'
  } catch (e: any) {
    exportError.value = e.message || '导出失败'
  } finally {
    exporting.value.operating = false
  }
}

async function exportTransactions() {
  if (!exportStartDate.value || !exportEndDate.value) {
    exportError.value = '请选择起止时间'
    return
  }
  if (exportStartDate.value > exportEndDate.value) {
    exportError.value = '开始时间不能晚于结束时间'
    return
  }
  exportError.value = ''
  exportSuccess.value = ''
  exporting.value.transactions = true
  try {
    const { blob, filename } = await api.downloadFile(
      `/api/v1/reports/transactions?start_time=${exportStartDate.value}&end_time=${exportEndDate.value}`
    )
    triggerDownload(blob, filename)
    exportSuccess.value = '交易报表已下载'
  } catch (e: any) {
    exportError.value = e.message || '导出失败'
  } finally {
    exporting.value.transactions = false
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
  fetchMerchants()
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

      <!-- Report export -->
      <div class="mt-8 bg-white rounded-lg shadow-sm p-6">
        <h2 class="text-lg font-semibold text-gray-900 mb-4">数据报表导出</h2>
        <div class="flex flex-wrap items-end gap-4">
          <div>
            <label class="block text-sm text-gray-500 mb-1">开始时间</label>
            <input
              v-model="exportStartDate"
              type="date"
              class="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div>
            <label class="block text-sm text-gray-500 mb-1">结束时间</label>
            <input
              v-model="exportEndDate"
              type="date"
              class="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <button
            @click="exportOperating"
            :disabled="exporting.operating"
            class="px-4 py-2 bg-blue-500 text-white rounded-lg text-sm hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <span v-if="exporting.operating" class="inline-block animate-spin mr-1">&#9696;</span>
            {{ exporting.operating ? '导出中...' : '导出经营报表' }}
          </button>
          <button
            @click="exportTransactions"
            :disabled="exporting.transactions"
            class="px-4 py-2 bg-green-500 text-white rounded-lg text-sm hover:bg-green-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <span v-if="exporting.transactions" class="inline-block animate-spin mr-1">&#9696;</span>
            {{ exporting.transactions ? '导出中...' : '导出交易报表' }}
          </button>
        </div>
        <p v-if="exportError" class="mt-3 text-sm text-red-500">{{ exportError }}</p>
        <p v-if="exportSuccess" class="mt-3 text-sm text-green-500">{{ exportSuccess }}</p>
      </div>

      <!-- Merchant list -->
      <div class="mt-8">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-lg font-semibold text-gray-900">商户列表</h2>
          <router-link to="/merchants/ranking" class="text-sm text-blue-500 hover:underline">
            营收排行
          </router-link>
        </div>

        <div v-if="merchantsLoading" class="flex justify-center py-8">
          <div class="animate-spin h-6 w-6 border-3 border-blue-500 border-t-transparent rounded-full" />
        </div>

        <div v-else-if="merchants.length === 0" class="text-center py-8 text-gray-400">
          暂无商户数据
        </div>

        <div v-else class="bg-white rounded-lg shadow-sm overflow-hidden">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-gray-500">
              <tr>
                <th class="text-left px-4 py-3">商户名称</th>
                <th class="text-left px-4 py-3">营业执照号</th>
                <th class="text-left px-4 py-3">状态</th>
                <th class="text-left px-4 py-3">合同状态</th>
                <th class="text-left px-4 py-3">入驻时间</th>
                <th class="text-right px-4 py-3">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="m in merchants"
                :key="m.id"
                class="border-t border-gray-100 hover:bg-gray-50 transition-colors"
              >
                <td class="px-4 py-3 font-medium text-gray-900">{{ m.name }}</td>
                <td class="px-4 py-3 text-gray-500">{{ m.license_number }}</td>
                <td class="px-4 py-3">
                  <span :class="['inline-block px-2 py-0.5 rounded text-xs font-medium', statusColors[m.status] || 'bg-gray-100 text-gray-600']">
                    {{ statusLabels[m.status] || m.status }}
                  </span>
                </td>
                <td class="px-4 py-3">
                  <span v-if="m.contract_status" :class="['inline-block px-2 py-0.5 rounded text-xs font-medium', m.contract_status === 'active' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700']">
                    {{ m.contract_status === 'active' ? '有效' : '已过期' }}
                  </span>
                  <span v-else class="text-gray-300 text-xs">-</span>
                </td>
                <td class="px-4 py-3 text-gray-500">{{ m.created_at?.slice(0, 10) }}</td>
                <td class="px-4 py-3 text-right">
                  <router-link
                    :to="`/merchants/${m.id}/analysis`"
                    class="text-blue-500 hover:underline text-xs"
                  >
                    经营分析
                  </router-link>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </main>
  </div>
</template>
