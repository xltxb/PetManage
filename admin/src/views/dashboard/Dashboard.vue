<template>
  <div>
    <!-- KPI Row -->
    <div class="grid grid-cols-4 gap-4 mb-6">
      <div class="kpi-card">
        <p class="text-sm" style="color: var(--color-sky)">今日营业额</p>
        <p class="text-2xl font-bold mt-1" style="color: var(--color-ink)">¥{{ fmtYuan(summary.revenue_today) }}</p>
      </div>
      <div class="kpi-card">
        <p class="text-sm" style="color: var(--color-coral)">今日预约</p>
        <p class="text-2xl font-bold mt-1" style="color: var(--color-ink)">{{ summary.appointment_count }}</p>
      </div>
      <div class="kpi-card">
        <p class="text-sm" style="color: var(--color-pine)">在店宠物</p>
        <p class="text-2xl font-bold mt-1" style="color: var(--color-ink)">{{ summary.pets_in_store }}</p>
      </div>
      <div class="kpi-card">
        <p class="text-sm" style="color: var(--color-honey)">今日新会员</p>
        <p class="text-2xl font-bold mt-1" style="color: var(--color-ink)">{{ summary.new_members_count }}</p>
      </div>
    </div>

    <!-- Mid Section -->
    <div class="grid grid-cols-3 gap-4 mb-6">
      <div class="kpi-card col-span-2">
        <h3 class="text-sm font-semibold mb-3" style="color: var(--color-ink)">近14天营收趋势</h3>
        <div class="flex items-end gap-1 h-32">
          <div v-for="(p, i) in summary.revenue_trend" :key="i"
            class="flex-1 rounded-t-sm transition-all"
            style="background: var(--color-coral); opacity: 0.8"
            :style="{ height: p.amount ? Math.max(4, (p.amount / maxRevenue) * 100) + '%' : '4px' }"
            :title="p.date + ': ¥' + fmtYuan(p.amount)">
          </div>
        </div>
        <p class="text-xs text-center mt-2" style="color: var(--color-ink); opacity: 0.5">每日营收</p>
      </div>
      <div class="kpi-card">
        <h3 class="text-sm font-semibold mb-3" style="color: var(--color-ink)">会员构成</h3>
        <div v-for="t in summary.member_composition" :key="t.tier_name"
          class="flex justify-between text-sm py-1">
          <span>{{ t.tier_name }}</span>
          <span class="font-semibold">{{ t.count }}</span>
        </div>
      </div>
    </div>

    <!-- Bottom Section -->
    <div class="grid grid-cols-2 gap-4">
      <div class="kpi-card">
        <h3 class="text-sm font-semibold mb-3" style="color: var(--color-ink)">今日预约</h3>
        <div v-if="summary.today_appointments.length === 0" class="text-sm text-center py-4" style="opacity: 0.5">暂无预约</div>
        <div v-for="a in summary.today_appointments" :key="a.id"
          class="flex items-center justify-between py-2 border-b border-black/5 last:border-0 text-sm">
          <span class="font-medium">{{ a.pet_name || '—' }}</span>
          <span>{{ a.service_name }}</span>
          <span class="status-badge" :class="'status-' + a.status">{{ statusLabel(a.status) }}</span>
        </div>
      </div>
      <div class="kpi-card">
        <h3 class="text-sm font-semibold mb-3" style="color: var(--color-ink)">
          库存预警
          <span v-if="summary.inventory_alerts.length" class="status-badge status-alert ml-2">{{ summary.inventory_alerts.length }}</span>
        </h3>
        <div v-if="summary.inventory_alerts.length === 0" class="text-sm text-center py-4" style="color: var(--color-pine)">✅ 库存充足</div>
        <div v-for="item in summary.inventory_alerts" :key="item.product_id"
          class="flex items-center justify-between py-2 border-b border-black/5 last:border-0 text-sm">
          <span>{{ item.product_name }}</span>
          <span style="color: var(--color-berry)">{{ item.quantity }}/{{ item.safety_stock }}{{ item.unit }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import client from '../../api/client'

interface DashboardSummary {
  revenue_today: number
  appointment_count: number
  pets_in_store: number
  new_members_count: number
  revenue_trend: { date: string; amount: number }[]
  today_appointments: any[]
  popular_services: any[]
  inventory_alerts: any[]
  member_composition: any[]
}

const summary = ref<DashboardSummary>({
  revenue_today: 0, appointment_count: 0, pets_in_store: 0, new_members_count: 0,
  revenue_trend: [], today_appointments: [], popular_services: [], inventory_alerts: [], member_composition: [],
})

const maxRevenue = computed(() => {
  const max = Math.max(...summary.value.revenue_trend.map((p) => p.amount), 1)
  return max
})

function fmtYuan(cents: number) {
  return (cents / 100).toFixed(2)
}

function statusLabel(s: string) {
  const m: Record<string, string> = { pending: '待到店', arrived: '已到店', in_progress: '进行中', completed: '已完成', cancelled: '已取消', no_show: '未到' }
  return m[s] || s
}

onMounted(async () => {
  try {
    const { data } = await client.get('/dashboard/summary')
    summary.value = data.data
  } catch (e) { /* handle error */ }
})
</script>
