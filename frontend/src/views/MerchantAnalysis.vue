<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api/client'

const route = useRoute()
const router = useRouter()

const merchantId = Number(route.params.id)

interface ProductRank {
  product_id: number
  product_name: string
  quantity: number
  revenue: number
  rank: number
}

interface ServiceItem {
  service_id: number
  service_name: string
  order_count: number
  revenue: number
  rank: number
}

interface RankItem {
  merchant_id: number
  merchant_name: string
  total_revenue: number
  rank: number
}

const analysis = ref<any>(null)
const ranking = ref<RankItem[]>([])
const loading = ref(true)
const error = ref('')
const period = ref('all')

const periodLabels: Record<string, string> = {
  all: '全部',
  today: '今日',
  week: '本周',
  month: '本月',
  year: '本年',
}

function formatRevenue(cents: number): string {
  return '¥' + (cents / 100).toFixed(2)
}

async function fetchData() {
  loading.value = true
  error.value = ''
  try {
    const [a, r] = await Promise.all([
      api.getMerchantAnalysis(merchantId, period.value),
      api.getMerchantsRevenueRanking(period.value),
    ])
    analysis.value = a
    ranking.value = r
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function switchPeriod(p: string) {
  period.value = p
  fetchData()
}

function goBack() {
  router.push('/')
}

const currentRank = computed(() => {
  if (!analysis.value) return '-'
  const r = ranking.value.find(item => item.merchant_id === merchantId)
  return r ? `#${r.rank}` : `#${analysis.value.revenue_rank}`
})

onMounted(() => {
  const token = api.getToken()
  if (!token) {
    router.push('/login')
    return
  }
  fetchData()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm">
      <div class="max-w-7xl mx-auto px-4 py-4 flex justify-between items-center">
        <div class="flex items-center gap-4">
          <button
            @click="goBack"
            class="text-gray-500 hover:text-gray-700 text-sm"
          >
            &larr; 返回
          </button>
          <h1 class="text-xl font-bold text-gray-900">
            商户经营分析 - {{ analysis?.merchant_name || '加载中...' }}
          </h1>
        </div>
        <span class="text-xs text-gray-400">平台管理后台</span>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-8">
      <!-- Period switcher -->
      <div class="flex justify-end mb-6">
        <div class="flex gap-1 bg-gray-100 rounded-lg p-1">
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

      <!-- Error state -->
      <div v-else-if="error" class="text-center py-20">
        <p class="text-red-500">{{ error }}</p>
        <button @click="fetchData" class="mt-4 text-blue-500 hover:underline text-sm">重试</button>
      </div>

      <!-- Data -->
      <template v-else-if="analysis">
        <!-- Core metrics -->
        <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-8">
          <div class="bg-white rounded-lg shadow-sm p-5">
            <p class="text-sm text-gray-500">今日营收</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">{{ formatRevenue(analysis.today_revenue) }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5">
            <p class="text-sm text-gray-500">今日订单</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">{{ analysis.today_orders }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5">
            <p class="text-sm text-gray-500">今日新增会员</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">{{ analysis.today_new_members }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5">
            <p class="text-sm text-gray-500">营收排名</p>
            <p class="text-2xl font-bold text-blue-600 mt-1">{{ currentRank }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5">
            <p class="text-sm text-gray-500">{{ periodLabels[period] }}总营收</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">{{ formatRevenue(analysis.total_revenue) }}</p>
          </div>
          <div class="bg-white rounded-lg shadow-sm p-5">
            <p class="text-sm text-gray-500">{{ periodLabels[period] }}总订单</p>
            <p class="text-2xl font-bold text-gray-900 mt-1">{{ analysis.total_orders }}</p>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
          <!-- Product sales TOP 10 -->
          <section class="bg-white rounded-lg shadow-sm p-6">
            <h2 class="text-lg font-semibold text-gray-900 mb-4">商品销售排行 TOP10</h2>
            <div v-if="analysis.product_sales_rank.length === 0" class="text-center py-8 text-gray-400">
              暂无商品销售数据
            </div>
            <table v-else class="w-full text-sm">
              <thead>
                <tr class="text-left text-gray-500 border-b">
                  <th class="pb-2 w-12">排名</th>
                  <th class="pb-2">商品名称</th>
                  <th class="pb-2 text-right">销量</th>
                  <th class="pb-2 text-right">营收</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in analysis.product_sales_rank" :key="item.product_id" class="border-b border-gray-50">
                  <td class="py-2.5">
                    <span :class="[
                      'inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold',
                      item.rank === 1 ? 'bg-yellow-100 text-yellow-700' :
                      item.rank === 2 ? 'bg-gray-100 text-gray-600' :
                      item.rank === 3 ? 'bg-orange-100 text-orange-700' :
                      'text-gray-400'
                    ]">{{ item.rank }}</span>
                  </td>
                  <td class="py-2.5">{{ item.product_name }}</td>
                  <td class="py-2.5 text-right">{{ item.quantity }}</td>
                  <td class="py-2.5 text-right">{{ formatRevenue(item.revenue) }}</td>
                </tr>
              </tbody>
            </table>
          </section>

          <!-- Service popularity TOP 10 -->
          <section class="bg-white rounded-lg shadow-sm p-6">
            <h2 class="text-lg font-semibold text-gray-900 mb-4">服务热度排行 TOP10</h2>
            <div v-if="analysis.service_popularity.length === 0" class="text-center py-8 text-gray-400">
              暂无服务数据
            </div>
            <table v-else class="w-full text-sm">
              <thead>
                <tr class="text-left text-gray-500 border-b">
                  <th class="pb-2 w-12">排名</th>
                  <th class="pb-2">服务名称</th>
                  <th class="pb-2 text-right">订单数</th>
                  <th class="pb-2 text-right">营收</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in analysis.service_popularity" :key="item.service_id" class="border-b border-gray-50">
                  <td class="py-2.5">
                    <span :class="[
                      'inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold',
                      item.rank === 1 ? 'bg-yellow-100 text-yellow-700' :
                      item.rank === 2 ? 'bg-gray-100 text-gray-600' :
                      item.rank === 3 ? 'bg-orange-100 text-orange-700' :
                      'text-gray-400'
                    ]">{{ item.rank }}</span>
                  </td>
                  <td class="py-2.5">{{ item.service_name }}</td>
                  <td class="py-2.5 text-right">{{ item.order_count }}</td>
                  <td class="py-2.5 text-right">{{ formatRevenue(item.revenue) }}</td>
                </tr>
              </tbody>
            </table>
          </section>
        </div>

        <!-- Revenue ranking -->
        <section class="bg-white rounded-lg shadow-sm p-6 mt-8">
          <h2 class="text-lg font-semibold text-gray-900 mb-4">商户营收排行</h2>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-gray-500 border-b">
                  <th class="pb-2 w-12">排名</th>
                  <th class="pb-2">商户名称</th>
                  <th class="pb-2 text-right">总营收</th>
                  <th class="pb-2 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in ranking"
                  :key="item.merchant_id"
                  :class="['border-b border-gray-50', item.merchant_id === merchantId ? 'bg-blue-50' : '']"
                >
                  <td class="py-2.5">
                    <span :class="[
                      'inline-flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold',
                      item.rank === 1 ? 'bg-yellow-100 text-yellow-700' :
                      item.rank === 2 ? 'bg-gray-100 text-gray-600' :
                      item.rank === 3 ? 'bg-orange-100 text-orange-700' :
                      'text-gray-400'
                    ]">{{ item.rank }}</span>
                  </td>
                  <td class="py-2.5">
                    {{ item.merchant_name }}
                    <span v-if="item.merchant_id === merchantId" class="text-blue-500 text-xs ml-1">(当前)</span>
                  </td>
                  <td class="py-2.5 text-right">{{ formatRevenue(item.total_revenue) }}</td>
                  <td class="py-2.5 text-right">
                    <router-link
                      v-if="item.merchant_id !== merchantId"
                      :to="`/merchants/${item.merchant_id}/analysis`"
                      class="text-blue-500 hover:underline text-xs"
                    >
                      查看分析
                    </router-link>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </main>
  </div>
</template>
