<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api/client'

const route = useRoute()
const router = useRouter()

interface Reminder {
  pet_id: number
  pet_name: string
  member_id: number
  member_name: string
  card_no: string
  reminder_type: string
  item_name: string
  last_date: string
  next_date: string
  days_left: number
  notes: string
}

const reminders = ref<Reminder[]>([])
const total = ref(0)
const loading = ref(true)
const error = ref('')
const activeTab = ref((route.query.type as string) || 'all')
const page = ref(1)
const pageSize = 20

function formatDate(d: string): string {
  if (!d) return '-'
  return d
}

function statusClass(days: number): string {
  if (days < 0) return 'text-red-600 bg-red-50'
  if (days <= 3) return 'text-orange-600 bg-orange-50'
  if (days <= 7) return 'text-yellow-600 bg-yellow-50'
  return 'text-green-600 bg-green-50'
}

function statusText(days: number): string {
  if (days < 0) return `已逾期${-days}天`
  if (days === 0) return '今天到期'
  if (days === 1) return '明天到期'
  return `剩余${days}天`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.getHealthReminders({
      type: activeTab.value,
      days: 30,
      page: page.value,
      page_size: pageSize,
    })
    reminders.value = res.reminders
    total.value = res.total
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function switchTab(tab: string) {
  activeTab.value = tab
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-4">
          <button
            @click="router.push('/merchant')"
            class="text-sm text-blue-600 hover:text-blue-800 cursor-pointer"
          >
            &larr; 返回首页
          </button>
          <h1 class="text-lg font-semibold text-gray-800">健康提醒</h1>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-6">
      <!-- Tabs -->
      <div class="flex gap-2 mb-6">
        <button
          v-for="tab in [
            { key: 'all', label: '全部' },
            { key: 'vaccine', label: '疫苗提醒' },
            { key: 'deworming', label: '驱虫提醒' },
          ]"
          :key="tab.key"
          @click="switchTab(tab.key)"
          class="px-4 py-2 rounded-lg text-sm font-medium transition-colors cursor-pointer"
          :class="activeTab === tab.key
            ? 'bg-blue-600 text-white'
            : 'bg-white text-gray-600 hover:bg-gray-100 border border-gray-200'"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full"></div>
        <span class="ml-3 text-gray-500">加载提醒数据...</span>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
        <p class="text-red-600">{{ error }}</p>
        <button @click="load" class="mt-3 text-sm text-blue-600 hover:text-blue-800 cursor-pointer">
          重新加载
        </button>
      </div>

      <!-- Empty -->
      <div v-else-if="reminders.length === 0" class="bg-white rounded-lg shadow-sm p-12 text-center">
        <p class="text-gray-400 text-lg mb-2">暂无到期提醒</p>
        <p class="text-gray-300 text-sm">宠物的疫苗和驱虫记录都已在有效期内</p>
      </div>

      <!-- Table -->
      <div v-else class="bg-white rounded-lg shadow-sm overflow-hidden">
        <div class="px-6 py-4 border-b border-gray-100">
          <p class="text-sm text-gray-500">共 {{ total }} 条提醒</p>
        </div>
        <table class="w-full">
          <thead>
            <tr class="bg-gray-50 text-left text-xs text-gray-500 uppercase">
              <th class="px-6 py-3">宠物</th>
              <th class="px-6 py-3">会员</th>
              <th class="px-6 py-3">类型</th>
              <th class="px-6 py-3">项目名称</th>
              <th class="px-6 py-3">上次日期</th>
              <th class="px-6 py-3">下次到期</th>
              <th class="px-6 py-3">状态</th>
              <th class="px-6 py-3">备注</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-for="r in reminders" :key="`${r.pet_id}-${r.reminder_type}-${r.next_date}`"
              class="hover:bg-gray-50 text-sm">
              <td class="px-6 py-3 font-medium text-gray-800">{{ r.pet_name }}</td>
              <td class="px-6 py-3 text-gray-600">{{ r.member_name }}
                <span class="text-xs text-gray-400 ml-1">{{ r.card_no }}</span>
              </td>
              <td class="px-6 py-3">
                <span class="px-2 py-0.5 rounded text-xs font-medium"
                  :class="r.reminder_type === 'vaccine'
                    ? 'bg-cyan-100 text-cyan-700'
                    : 'bg-emerald-100 text-emerald-700'">
                  {{ r.reminder_type === 'vaccine' ? '疫苗' : '驱虫' }}
                </span>
              </td>
              <td class="px-6 py-3 text-gray-800">{{ r.item_name }}</td>
              <td class="px-6 py-3 text-gray-500">{{ formatDate(r.last_date) }}</td>
              <td class="px-6 py-3 text-gray-800 font-medium">{{ formatDate(r.next_date) }}</td>
              <td class="px-6 py-3">
                <span class="px-2 py-0.5 rounded text-xs font-medium" :class="statusClass(r.days_left)">
                  {{ statusText(r.days_left) }}
                </span>
              </td>
              <td class="px-6 py-3 text-gray-400 max-w-40 truncate">{{ r.notes || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>
