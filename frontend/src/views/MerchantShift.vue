<template>
  <div class="min-h-screen bg-gray-50">
    <div class="max-w-4xl mx-auto p-6">
      <!-- Header -->
      <div class="flex items-center justify-between mb-6">
        <div>
          <h1 class="text-2xl font-bold text-gray-800">日结对账</h1>
          <p class="text-sm text-gray-500 mt-1">收银交班报表与历史记录</p>
        </div>
        <router-link to="/merchant" class="text-blue-600 hover:text-blue-800 text-sm">
          &larr; 返回首页
        </router-link>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex justify-center py-12">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>

      <!-- Today status -->
      <div v-if="!loading" class="bg-white rounded-lg shadow p-6 mb-6">
        <h2 class="text-lg font-semibold text-gray-700 mb-4">今日交班状态</h2>
        <div v-if="todayShift && todayShift.id" class="space-y-3">
          <div class="flex items-center gap-2">
            <span class="px-3 py-1 text-sm rounded-full"
              :class="todayShift.status === 'confirmed' ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'">
              {{ todayShift.status === 'confirmed' ? '已确认' : '待审核' }}
            </span>
            <span class="text-sm text-gray-500">{{ formatTime(todayShift.created_at) }}</span>
          </div>
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
            <div class="p-3 bg-gray-50 rounded">
              <div class="text-xs text-gray-500">应收总额</div>
              <div class="text-lg font-bold text-gray-800">¥{{ formatCents(todayShift.expected_total_cents) }}</div>
            </div>
            <div class="p-3 bg-gray-50 rounded">
              <div class="text-xs text-gray-500">实收总额</div>
              <div class="text-lg font-bold text-gray-800">¥{{ formatCents(todayShift.actual_total_cents) }}</div>
            </div>
            <div class="p-3 bg-gray-50 rounded">
              <div class="text-xs text-gray-500">差异金额</div>
              <div class="text-lg font-bold" :class="todayShift.difference_cents !== 0 ? 'text-red-600' : 'text-gray-800'">
                ¥{{ formatCents(todayShift.difference_cents) }}
              </div>
            </div>
            <div class="p-3 bg-gray-50 rounded">
              <div class="text-xs text-gray-500">订单数</div>
              <div class="text-lg font-bold text-gray-800">{{ todayShift.order_count }}</div>
            </div>
          </div>
          <!-- Payment breakdown -->
          <div class="mt-4">
            <div class="text-xs text-gray-500 mb-2">实收明细（按支付方式）</div>
            <div class="flex flex-wrap gap-2">
              <span v-for="(val, key) in parseBreakdown(todayShift.actual_breakdown)" :key="key"
                class="px-3 py-1 bg-blue-50 text-blue-700 rounded text-sm">
                {{ methodLabel(key) }}: ¥{{ formatCents(val) }}
              </span>
              <span v-if="Object.keys(parseBreakdown(todayShift.actual_breakdown)).length === 0"
                class="text-sm text-gray-400">暂无收款记录</span>
            </div>
          </div>
        </div>
        <div v-else class="text-center py-6">
          <div class="text-gray-400 text-lg mb-4">今日尚未交班</div>
          <button @click="createShift"
            :disabled="creating"
            class="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed">
            <span v-if="creating" class="inline-flex items-center gap-2">
              <span class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></span>
              正在生成...
            </span>
            <span v-else>交班</span>
          </button>
        </div>
      </div>

      <!-- History -->
      <div class="bg-white rounded-lg shadow p-6">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-lg font-semibold text-gray-700">交班记录</h2>
          <select v-model="filterStatus" @change="loadHistory" class="border rounded px-3 py-1 text-sm">
            <option value="">全部状态</option>
            <option value="pending">待审核</option>
            <option value="confirmed">已确认</option>
          </select>
        </div>

        <div v-if="historyTotal === 0" class="text-center py-8 text-gray-400">
          暂无交班记录
        </div>

        <div v-else class="space-y-3">
          <div v-for="record in historyRecords" :key="record.id"
            class="border rounded-lg p-4 hover:bg-gray-50 transition-colors">
            <div class="flex items-center justify-between">
              <div>
                <div class="font-medium text-gray-800">{{ record.shift_date.split('T')[0] }}</div>
                <div class="text-sm text-gray-500">{{ record.employee_name }}</div>
              </div>
              <div class="text-right">
                <span class="px-2 py-1 text-xs rounded-full"
                  :class="record.status === 'confirmed' ? 'bg-green-100 text-green-700' : 'bg-yellow-100 text-yellow-700'">
                  {{ record.status === 'confirmed' ? '已确认' : '待审核' }}
                </span>
                <div class="text-sm font-bold mt-1">¥{{ formatCents(record.actual_total_cents) }}</div>
              </div>
            </div>
            <div class="grid grid-cols-3 gap-2 mt-3 text-xs text-gray-500">
              <div>应收: ¥{{ formatCents(record.expected_total_cents) }}</div>
              <div>差异: ¥{{ formatCents(record.difference_cents) }}</div>
              <div>订单: {{ record.order_count }}笔</div>
            </div>
            <div v-if="record.confirmed_by_name" class="text-xs text-gray-400 mt-2">
              确认人: {{ record.confirmed_by_name }} · {{ formatTime(record.confirmed_at) }}
            </div>
          </div>

          <!-- Pagination -->
          <div class="flex items-center justify-between pt-4 border-t" v-if="historyTotal > historyPageSize">
            <button @click="prevPage" :disabled="historyPage <= 1"
              class="px-3 py-1 text-sm border rounded disabled:opacity-30">上一页</button>
            <span class="text-sm text-gray-500">第 {{ historyPage }} 页 / 共 {{ totalPages }} 页</span>
            <button @click="nextPage" :disabled="historyPage >= totalPages"
              class="px-3 py-1 text-sm border rounded disabled:opacity-30">下一页</button>
          </div>
        </div>
      </div>

      <!-- Error message -->
      <div v-if="errorMsg" class="fixed bottom-4 right-4 bg-red-500 text-white px-4 py-3 rounded-lg shadow-lg">
        {{ errorMsg }}
        <button @click="errorMsg = ''" class="ml-3 text-white/80 hover:text-white">&times;</button>
      </div>

      <!-- Success message -->
      <div v-if="successMsg" class="fixed bottom-4 right-4 bg-green-500 text-white px-4 py-3 rounded-lg shadow-lg">
        {{ successMsg }}
        <button @click="successMsg = ''" class="ml-3 text-white/80 hover:text-white">&times;</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { apiClient } from '@/api/client'

const loading = ref(true)
const creating = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

const todayShift = ref<any>(null)
const historyRecords = ref<any[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyPageSize = 20
const filterStatus = ref('')

const totalPages = computed(() => Math.max(1, Math.ceil(historyTotal.value / historyPageSize)))

onMounted(async () => {
  await loadTodayStatus()
  await loadHistory()
  loading.value = false
})

async function loadTodayStatus() {
  try {
    const data = await apiClient.getMerchantShiftToday()
    todayShift.value = data.shift
  } catch {
    // Not critical; show no shift
  }
}

async function loadHistory() {
  try {
    const params: any = { page: historyPage.value, page_size: historyPageSize }
    if (filterStatus.value) params.status = filterStatus.value
    const data = await apiClient.getMerchantShiftList(params)
    historyRecords.value = data.records
    historyTotal.value = data.total
  } catch (e: any) {
    errorMsg.value = e.message || '加载失败'
  }
}

async function createShift() {
  creating.value = true
  errorMsg.value = ''
  try {
    const data = await apiClient.createMerchantShift()
    todayShift.value = data
    successMsg.value = '交班成功！请重新登录后继续操作。'
    // Clear token to force re-login after shift
    setTimeout(() => {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      window.location.href = '/merchant/login'
    }, 2000)
  } catch (e: any) {
    errorMsg.value = e.message || '交班失败'
  } finally {
    creating.value = false
  }
}

function prevPage() {
  if (historyPage.value > 1) {
    historyPage.value--
    loadHistory()
  }
}

function nextPage() {
  if (historyPage.value < totalPages.value) {
    historyPage.value++
    loadHistory()
  }
}

function formatCents(cents: number): string {
  return (cents / 100).toFixed(2)
}

function formatTime(ts: string): string {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function methodLabel(m: string): string {
  const labels: Record<string, string> = {
    cash: '现金',
    wechat: '微信',
    alipay: '支付宝',
    balance: '储值',
    points: '积分',
    coupon: '优惠券',
  }
  return labels[m] || m
}

function parseBreakdown(bd: any): Record<string, number> {
  if (!bd) return {}
  if (typeof bd === 'string') {
    try { return JSON.parse(bd) } catch { return {} }
  }
  return bd
}
</script>
