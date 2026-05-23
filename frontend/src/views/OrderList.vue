<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'

const orders = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)

// Filters
const filterKeyword = ref('')
const filterStatus = ref('')
const filterDateFrom = ref('')
const filterDateTo = ref('')

// Detail modal
const showDetail = ref(false)
const selectedOrder = ref<any>(null)
const detailLoading = ref(false)

// Refund dialog
const showRefund = ref(false)
const refundType = ref<'full' | 'partial'>('full')
const refundReason = ref('')
const refundItems = ref<Array<{ order_item_id: number; quantity: number }>>([])
const refundLoading = ref(false)

const totalPages = computed(() => Math.ceil(total.value / pageSize.value))

const statusLabels: Record<string, string> = {
  completed: '已完成',
  refunded: '已退款',
  partially_refunded: '部分退款',
}

const statusColors: Record<string, string> = {
  completed: 'bg-green-100 text-green-800',
  refunded: 'bg-red-100 text-red-800',
  partially_refunded: 'bg-yellow-100 text-yellow-800',
}

async function loadOrders() {
  loading.value = true
  try {
    const res = await api.getOrders({
      keyword: filterKeyword.value || undefined,
      status: filterStatus.value || undefined,
      date_from: filterDateFrom.value || undefined,
      date_to: filterDateTo.value || undefined,
      page: page.value,
      page_size: pageSize.value,
    })
    orders.value = res.orders
    total.value = res.total
  } catch (e: any) {
    alert('加载订单失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function viewDetail(order: any) {
  showDetail.value = true
  detailLoading.value = true
  try {
    selectedOrder.value = await api.getOrder(order.id)
  } catch (e: any) {
    alert('加载订单详情失败: ' + e.message)
  } finally {
    detailLoading.value = false
  }
}

function openRefund(type: 'full' | 'partial') {
  refundType.value = type
  refundReason.value = ''
  refundItems.value = []
  showRefund.value = true
}

function toggleRefundItem(itemId: number, quantity: number) {
  const idx = refundItems.value.findIndex(i => i.order_item_id === itemId)
  if (idx >= 0) {
    refundItems.value.splice(idx, 1)
  } else {
    refundItems.value.push({ order_item_id: itemId, quantity })
  }
}

async function submitRefund() {
  if (!selectedOrder.value) return
  refundLoading.value = true
  try {
    const payload: any = { refund_type: refundType.value, reason: refundReason.value }
    if (refundType.value === 'partial') {
      payload.items = refundItems.value
    }
    const result = await api.refundOrder(selectedOrder.value.id, payload)
    if (result.needs_approval) {
      alert('退款已提交，金额超过500元需店长审批')
    } else {
      alert('退款成功')
    }
    showRefund.value = false
    // Refresh
    selectedOrder.value = await api.getOrder(selectedOrder.value.id)
    await loadOrders()
  } catch (e: any) {
    alert('退款失败: ' + e.message)
  } finally {
    refundLoading.value = false
  }
}

function closeDetail() {
  showDetail.value = false
  selectedOrder.value = null
}

function formatCents(cents: number) {
  return '¥' + (cents / 100).toFixed(2)
}

function formatDate(d: string) {
  return new Date(d).toLocaleString('zh-CN')
}

onMounted(() => {
  loadOrders()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center">
        <router-link to="/merchant" class="text-sm text-blue-600 hover:text-blue-800 mr-4">
          &larr; 返回首页
        </router-link>
        <h1 class="text-lg font-semibold text-gray-800">订单管理</h1>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-6">
      <!-- Filter Bar -->
      <div class="bg-white rounded-lg shadow-sm p-4 mb-4">
        <div class="flex flex-wrap gap-3 items-end">
          <div>
            <label class="block text-xs text-gray-500 mb-1">搜索</label>
            <input
              v-model="filterKeyword"
              placeholder="订单号 / 会员名 / 手机号"
              class="border rounded px-3 py-1.5 text-sm w-48"
              @keyup.enter="page = 1; loadOrders()"
            />
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1">状态</label>
            <select v-model="filterStatus" class="border rounded px-3 py-1.5 text-sm" @change="page = 1; loadOrders()">
              <option value="">全部</option>
              <option value="completed">已完成</option>
              <option value="refunded">已退款</option>
              <option value="partially_refunded">部分退款</option>
            </select>
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1">开始日期</label>
            <input v-model="filterDateFrom" type="date" class="border rounded px-3 py-1.5 text-sm" @change="page = 1; loadOrders()" />
          </div>
          <div>
            <label class="block text-xs text-gray-500 mb-1">结束日期</label>
            <input v-model="filterDateTo" type="date" class="border rounded px-3 py-1.5 text-sm" @change="page = 1; loadOrders()" />
          </div>
          <button
            @click="page = 1; loadOrders()"
            class="bg-blue-600 text-white px-4 py-1.5 rounded text-sm hover:bg-blue-700 cursor-pointer"
          >
            查询
          </button>
        </div>
      </div>

      <!-- Order Table -->
      <div class="bg-white rounded-lg shadow-sm overflow-hidden">
        <div v-if="loading" class="p-8 text-center text-gray-500">加载中...</div>
        <div v-else-if="orders.length === 0" class="p-8 text-center text-gray-500">暂无订单记录</div>
        <table v-else class="w-full text-sm">
          <thead class="bg-gray-50 border-b">
            <tr>
              <th class="text-left px-4 py-2 font-medium text-gray-600">订单号</th>
              <th class="text-left px-4 py-2 font-medium text-gray-600">会员</th>
              <th class="text-right px-4 py-2 font-medium text-gray-600">金额</th>
              <th class="text-center px-4 py-2 font-medium text-gray-600">状态</th>
              <th class="text-left px-4 py-2 font-medium text-gray-600">时间</th>
              <th class="text-center px-4 py-2 font-medium text-gray-600">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="o in orders" :key="o.id" class="border-b hover:bg-gray-50">
              <td class="px-4 py-2 font-mono">#{{ o.id }}</td>
              <td class="px-4 py-2">{{ o.member_name || '-' }}</td>
              <td class="px-4 py-2 text-right">{{ formatCents(o.total_cents) }}</td>
              <td class="px-4 py-2 text-center">
                <span class="inline-block px-2 py-0.5 rounded text-xs" :class="statusColors[o.status] || 'bg-gray-100'">
                  {{ statusLabels[o.status] || o.status }}
                </span>
              </td>
              <td class="px-4 py-2 text-gray-500">{{ formatDate(o.created_at) }}</td>
              <td class="px-4 py-2 text-center">
                <button
                  @click="viewDetail(o)"
                  class="text-blue-600 hover:text-blue-800 cursor-pointer mr-3 text-xs"
                >
                  详情
                </button>
                <button
                  v-if="o.status === 'completed'"
                  @click="viewDetail(o); openRefund('full')"
                  class="text-red-600 hover:text-red-800 cursor-pointer text-xs"
                >
                  退款
                </button>
                <button
                  v-if="o.status === 'completed' || o.status === 'partially_refunded'"
                  @click="viewDetail(o); openRefund('partial')"
                  class="text-orange-600 hover:text-orange-800 cursor-pointer text-xs ml-2"
                >
                  部分退
                </button>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="totalPages > 1" class="flex items-center justify-between px-4 py-3 border-t">
          <span class="text-sm text-gray-500">共 {{ total }} 条</span>
          <div class="flex gap-1">
            <button
              :disabled="page <= 1"
              @click="page--; loadOrders()"
              class="px-3 py-1 border rounded text-sm disabled:opacity-30"
            >
              上一页
            </button>
            <span class="px-3 py-1 text-sm">{{ page }} / {{ totalPages }}</span>
            <button
              :disabled="page >= totalPages"
              @click="page++; loadOrders()"
              class="px-3 py-1 border rounded text-sm disabled:opacity-30"
            >
              下一页
            </button>
          </div>
        </div>
      </div>

      <!-- Detail Modal -->
      <div v-if="showDetail" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="closeDetail">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[80vh] overflow-y-auto mx-4">
          <div class="flex items-center justify-between px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">订单详情 #{{ selectedOrder?.id }}</h2>
            <button @click="closeDetail" class="text-gray-400 hover:text-gray-600 cursor-pointer text-xl">&times;</button>
          </div>

          <div v-if="detailLoading" class="p-8 text-center text-gray-500">加载中...</div>
          <div v-else-if="selectedOrder" class="p-6 space-y-4">
            <!-- Order Info -->
            <div class="grid grid-cols-2 gap-3 text-sm">
              <div><span class="text-gray-500">订单号：</span>#{{ selectedOrder.id }}</div>
              <div><span class="text-gray-500">会员：</span>{{ selectedOrder.member_name || '-' }}</div>
              <div><span class="text-gray-500">总金额：</span>{{ formatCents(selectedOrder.total_cents) }}</div>
              <div><span class="text-gray-500">已付：</span>{{ formatCents(selectedOrder.paid_cents) }}</div>
              <div>
                <span class="text-gray-500">状态：</span>
                <span class="inline-block px-2 py-0.5 rounded text-xs" :class="statusColors[selectedOrder.status] || 'bg-gray-100'">
                  {{ statusLabels[selectedOrder.status] || selectedOrder.status }}
                </span>
              </div>
              <div><span class="text-gray-500">时间：</span>{{ formatDate(selectedOrder.created_at) }}</div>
              <div v-if="selectedOrder.notes" class="col-span-2"><span class="text-gray-500">备注：</span>{{ selectedOrder.notes }}</div>
            </div>

            <!-- Items -->
            <div>
              <h3 class="font-medium text-sm mb-2">商品明细</h3>
              <table class="w-full text-sm border">
                <thead class="bg-gray-50">
                  <tr>
                    <th class="text-left px-3 py-1.5">商品</th>
                    <th class="text-right px-3 py-1.5">单价</th>
                    <th class="text-right px-3 py-1.5">数量</th>
                    <th class="text-right px-3 py-1.5">小计</th>
                    <th v-if="showRefund && refundType === 'partial'" class="text-center px-3 py-1.5">退款</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in selectedOrder.items" :key="item.id" class="border-b">
                    <td class="px-3 py-1.5">{{ item.product_name }}</td>
                    <td class="px-3 py-1.5 text-right">{{ formatCents(item.price_cents) }}</td>
                    <td class="px-3 py-1.5 text-right">{{ item.quantity }}</td>
                    <td class="px-3 py-1.5 text-right">{{ formatCents(item.price_cents * item.quantity) }}</td>
                    <td v-if="showRefund && refundType === 'partial'" class="px-3 py-1.5 text-center">
                      <input
                        type="checkbox"
                        :checked="refundItems.some(i => i.order_item_id === item.id)"
                        @change="toggleRefundItem(item.id, item.quantity)"
                      />
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Payments -->
            <div>
              <h3 class="font-medium text-sm mb-2">支付明细</h3>
              <table class="w-full text-sm border">
                <thead class="bg-gray-50">
                  <tr>
                    <th class="text-left px-3 py-1.5">支付方式</th>
                    <th class="text-right px-3 py-1.5">金额</th>
                    <th class="text-left px-3 py-1.5">时间</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="p in selectedOrder.payments" :key="p.id" class="border-b">
                    <td class="px-3 py-1.5">{{ p.method }}</td>
                    <td class="px-3 py-1.5 text-right">{{ formatCents(p.amount_cents) }}</td>
                    <td class="px-3 py-1.5 text-gray-500">{{ formatDate(p.created_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Refunds -->
            <div v-if="selectedOrder.refunds && selectedOrder.refunds.length > 0">
              <h3 class="font-medium text-sm mb-2">退款记录</h3>
              <table class="w-full text-sm border">
                <thead class="bg-gray-50">
                  <tr>
                    <th class="text-left px-3 py-1.5">类型</th>
                    <th class="text-left px-3 py-1.5">原因</th>
                    <th class="text-right px-3 py-1.5">金额</th>
                    <th class="text-center px-3 py-1.5">状态</th>
                    <th class="text-left px-3 py-1.5">时间</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="r in selectedOrder.refunds" :key="r.id" class="border-b">
                    <td class="px-3 py-1.5">{{ r.refund_type === 'full' ? '全额退款' : '部分退款' }}</td>
                    <td class="px-3 py-1.5">{{ r.reason }}</td>
                    <td class="px-3 py-1.5 text-right text-red-600">{{ formatCents(r.amount_cents) }}</td>
                    <td class="px-3 py-1.5 text-center">
                      <span class="inline-block px-2 py-0.5 rounded text-xs" :class="r.status === 'completed' ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800'">
                        {{ r.status === 'completed' ? '已完成' : r.status === 'pending_approval' ? '待审批' : r.status }}
                      </span>
                    </td>
                    <td class="px-3 py-1.5 text-gray-500">{{ formatDate(r.created_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- Refund Form -->
            <div v-if="showRefund" class="border-t pt-4">
              <h3 class="font-medium text-sm mb-3">
                {{ refundType === 'full' ? '全额退款' : '部分退款' }}
              </h3>
              <div class="mb-3">
                <label class="block text-xs text-gray-500 mb-1">退款原因</label>
                <input v-model="refundReason" class="border rounded px-3 py-1.5 text-sm w-full" placeholder="请输入退款原因" />
              </div>
              <div class="flex gap-2">
                <button
                  @click="submitRefund"
                  :disabled="refundLoading || (refundType === 'partial' && refundItems.length === 0)"
                  class="bg-red-600 text-white px-4 py-1.5 rounded text-sm hover:bg-red-700 cursor-pointer disabled:opacity-50"
                >
                  {{ refundLoading ? '处理中...' : '确认退款' }}
                </button>
                <button
                  @click="showRefund = false"
                  class="bg-gray-200 text-gray-700 px-4 py-1.5 rounded text-sm hover:bg-gray-300 cursor-pointer"
                >
                  取消
                </button>
              </div>
            </div>

            <!-- Action buttons for non-refund mode -->
            <div v-if="!showRefund && (selectedOrder.status === 'completed' || selectedOrder.status === 'partially_refunded')" class="border-t pt-4 flex gap-2">
              <button
                @click="openRefund('full')"
                class="bg-red-600 text-white px-4 py-1.5 rounded text-sm hover:bg-red-700 cursor-pointer"
              >
                全额退款
              </button>
              <button
                @click="openRefund('partial')"
                class="bg-orange-600 text-white px-4 py-1.5 rounded text-sm hover:bg-orange-700 cursor-pointer"
              >
                部分退款
              </button>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>
