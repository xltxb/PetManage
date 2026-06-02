<template>
  <div class="space-y-4">
    <!-- Header -->
    <div class="flex justify-between items-center">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">预约管理</h3>
      <button @click="showCreate = true" class="px-4 py-2 rounded-lg text-white text-sm font-medium"
        style="background: var(--color-coral)">+ 新建预约</button>
    </div>

    <!-- Filters -->
    <div class="flex gap-2">
      <button v-for="f in filters" :key="f.value" @click="currentFilter = f.value"
        class="px-3 py-1 rounded-full text-xs font-medium transition-colors"
        :style="currentFilter === f.value ? { background: 'var(--color-ink)', color: 'white' } : { background: 'var(--color-surface)', color: 'var(--color-ink)' }">
        {{ f.label }}
      </button>
    </div>

    <!-- Appointment List -->
    <div class="kpi-card">
      <div v-if="appointments.length === 0" class="text-center py-8 text-sm" style="opacity: 0.5">暂无预约</div>
      <div v-for="a in appointments" :key="a.id"
        class="flex items-center justify-between py-3 border-b border-black/5 last:border-0 text-sm">
        <div class="flex items-center gap-4">
          <span class="status-badge text-xs" :class="'status-' + a.status">{{ statusMap[a.status] || a.status }}</span>
          <div>
            <p class="font-medium">{{ a.pet?.name || a.customer?.name || '散客' }}</p>
            <p class="text-xs" style="opacity: 0.5">{{ formatTime(a.scheduled_start) }} · {{ a.items?.[0]?.service_name || '—' }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <span class="text-xs font-mono" style="color: var(--color-ink)">¥{{ (a.total_amount / 100).toFixed(2) }}</span>
          <button v-if="a.status === 'pending'" @click="doTransition(a.id, 'arrive')"
            class="px-2 py-1 rounded text-xs text-white" style="background: var(--color-pine)">到店</button>
          <button v-if="a.status === 'arrived'" @click="doTransition(a.id, 'start')"
            class="px-2 py-1 rounded text-xs text-white" style="background: var(--color-coral)">开始</button>
          <button v-if="a.status === 'in_progress'" @click="doTransition(a.id, 'complete')"
            class="px-2 py-1 rounded text-xs text-white" style="background: var(--color-pine)">完成</button>
          <button v-if="a.status === 'pending'" @click="doCancel(a.id)"
            class="px-2 py-1 rounded text-xs" style="border: 1px solid var(--color-berry); color: var(--color-berry)">取消</button>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div class="flex justify-center gap-2" v-if="total > pageSize">
      <button @click="changePage(-1)" :disabled="page <= 1" class="px-3 py-1 rounded text-sm"
        style="background: var(--color-surface)">上一页</button>
      <span class="text-sm py-1">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <button @click="changePage(1)" :disabled="page * pageSize >= total" class="px-3 py-1 rounded text-sm"
        style="background: var(--color-surface)">下一页</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import client from '../../api/client'

interface Appointment {
  id: number; status: string; total_amount: number
  scheduled_start: string; scheduled_end: string
  pet?: { name: string }; customer?: { name: string }
  items?: { service_name: string }[]
}

const appointments = ref<Appointment[]>([])
const page = ref(1); const pageSize = 20; const total = ref(0)
const currentFilter = ref(''); const showCreate = ref(false)

const filters = [
  { label: '全部', value: '' }, { label: '待到店', value: 'pending' },
  { label: '已到店', value: 'arrived' }, { label: '进行中', value: 'in_progress' },
  { label: '已完成', value: 'completed' }, { label: '已取消', value: 'cancelled' },
]

const statusMap: Record<string, string> = {
  pending: '待到店', arrived: '已到店', in_progress: '进行中',
  completed: '已完成', cancelled: '已取消', no_show: '未到',
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

async function loadAppointments() {
  const { data } = await client.get('/appointments', {
    params: { page: page.value, page_size: pageSize, status: currentFilter.value || undefined }
  })
  appointments.value = data.data?.list || []
  total.value = data.data?.total || 0
}

async function doTransition(id: number, action: string) {
  await client.post(`/appointments/${id}/transitions`, { action })
  loadAppointments()
}

async function doCancel(id: number) {
  if (!confirm('确定取消该预约？')) return
  await client.post(`/appointments/${id}/cancel`, { reason: '操作员取消' })
  loadAppointments()
}

function changePage(delta: number) {
  page.value = Math.max(1, page.value + delta)
  loadAppointments()
}

onMounted(loadAppointments)
</script>
