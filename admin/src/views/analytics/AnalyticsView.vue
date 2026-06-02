<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">数据分析</h3>
      <form class="filters" @submit.prevent="load">
        <label class="label inline">开始<input v-model="start" class="field date-field" type="date" /></label>
        <label class="label inline">结束<input v-model="end" class="field date-field" type="date" /></label>
        <button class="soft-btn" :disabled="loading">刷新</button>
      </form>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="grid grid-cols-4 gap-4">
      <div class="kpi-card metric-card">
        <span>周期收入</span>
        <strong>¥{{ yuan(totalRevenue) }}</strong>
      </div>
      <div class="kpi-card metric-card">
        <span>服务分类</span>
        <strong>{{ report.service_breakdown.length }}</strong>
      </div>
      <div class="kpi-card metric-card">
        <span>高峰时段</span>
        <strong>{{ peakHourLabel }}</strong>
      </div>
      <div class="kpi-card metric-card">
        <span>会员分组</span>
        <strong>{{ report.retention_funnel.length }}</strong>
      </div>
    </div>

    <div class="grid grid-cols-[1.2fr_.8fr] gap-4">
      <section class="kpi-card">
        <div class="flex items-center justify-between mb-3">
          <h4 class="text-sm font-semibold">收入趋势</h4>
          <span class="text-xs" style="opacity: 0.55">{{ start }} 至 {{ end }}</span>
        </div>
        <div v-if="loading" class="muted">加载中...</div>
        <div v-else-if="report.revenue_trend.length === 0" class="muted">暂无收入数据</div>
        <div v-for="point in report.revenue_trend" :key="point.month" class="bar-row">
          <span>{{ point.month }}</span>
          <div class="bar-track"><i :style="{ width: barWidth(point.amount, maxRevenue) }" /></div>
          <strong>¥{{ yuan(point.amount) }}</strong>
        </div>
      </section>

      <section class="kpi-card">
        <h4 class="text-sm font-semibold mb-3">服务收入占比</h4>
        <div v-if="report.service_breakdown.length === 0" class="muted">暂无分类数据</div>
        <div v-for="item in report.service_breakdown" :key="item.category_name" class="share-row">
          <div class="flex items-center justify-between">
            <span>{{ item.category_name }}</span>
            <strong>{{ item.percentage.toFixed(1) }}%</strong>
          </div>
          <div class="bar-track thin"><i :style="{ width: `${item.percentage}%`, background: item.color || 'var(--color-coral)' }" /></div>
          <small>¥{{ yuan(item.amount) }}</small>
        </div>
      </section>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <section class="kpi-card">
        <h4 class="text-sm font-semibold mb-3">结算高峰</h4>
        <div v-if="report.peak_hours.length === 0" class="muted">暂无高峰数据</div>
        <div v-for="item in report.peak_hours" :key="item.hour" class="bar-row compact">
          <span>{{ String(item.hour).padStart(2, '0') }}:00</span>
          <div class="bar-track"><i :style="{ width: barWidth(item.count, maxPeakCount) }" /></div>
          <strong>{{ item.count }}</strong>
        </div>
      </section>

      <section class="kpi-card">
        <h4 class="text-sm font-semibold mb-3">客户留存分组</h4>
        <div v-if="report.retention_funnel.length === 0" class="muted">暂无留存数据</div>
        <div v-for="item in report.retention_funnel" :key="item.bucket" class="bar-row compact">
          <span>{{ item.bucket }}</span>
          <div class="bar-track"><i :style="{ width: barWidth(item.count, maxRetentionCount) }" /></div>
          <strong>{{ item.count }}</strong>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import client from '../../api/client'

interface RevenueTrendPoint {
  month: string
  amount: number
}

interface ServiceBreakdown {
  category_name: string
  amount: number
  percentage: number
  color?: string
}

interface PeakHourPoint {
  hour: number
  count: number
}

interface RetentionBucket {
  bucket: string
  count: number
}

interface AnalyticsReport {
  revenue_trend: RevenueTrendPoint[]
  service_breakdown: ServiceBreakdown[]
  peak_hours: PeakHourPoint[]
  retention_funnel: RetentionBucket[]
}

const now = new Date()
const oneYearAgo = new Date()
oneYearAgo.setFullYear(now.getFullYear() - 1)

const start = ref(dateInput(oneYearAgo))
const end = ref(dateInput(now))
const loading = ref(false)
const error = ref('')
const report = reactive<AnalyticsReport>({
  revenue_trend: [],
  service_breakdown: [],
  peak_hours: [],
  retention_funnel: [],
})

const totalRevenue = computed(() => report.revenue_trend.reduce((sum, item) => sum + item.amount, 0))
const maxRevenue = computed(() => Math.max(0, ...report.revenue_trend.map((item) => item.amount)))
const maxPeakCount = computed(() => Math.max(0, ...report.peak_hours.map((item) => item.count)))
const maxRetentionCount = computed(() => Math.max(0, ...report.retention_funnel.map((item) => item.count)))
const peakHourLabel = computed(() => {
  const peak = report.peak_hours.reduce<PeakHourPoint | null>((best, item) => (!best || item.count > best.count ? item : best), null)
  return peak ? `${String(peak.hour).padStart(2, '0')}:00` : '-'
})

function dateInput(date: Date) {
  return date.toISOString().slice(0, 10)
}

function yuan(cents: number) {
  return ((cents || 0) / 100).toFixed(2)
}

function barWidth(value: number, max: number) {
  if (!max) return '0%'
  return `${Math.max(4, Math.round((value / max) * 100))}%`
}

function errorMessage(err: unknown) {
  const maybe = err as { response?: { data?: { message?: string } }; message?: string }
  return maybe.response?.data?.message || maybe.message || '操作失败'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await client.get('/analytics/report', {
      params: { start: start.value, end: end.value },
    })
    const next = data.data || {}
    report.revenue_trend = next.revenue_trend || []
    report.service_breakdown = next.service_breakdown || []
    report.peak_hours = next.peak_hours || []
    report.retention_funnel = next.retention_funnel || []
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.field {
  width: 100%;
  border: 1px solid rgba(35, 30, 24, 0.12);
  border-radius: 8px;
  padding: 8px 10px;
  background: white;
  font-size: 14px;
  color: var(--color-ink);
}

.date-field {
  width: 150px;
}

.filters {
  display: flex;
  gap: 8px;
  align-items: end;
}

.label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.metric-card span,
.muted {
  font-size: 14px;
  opacity: 0.55;
}

.metric-card strong {
  color: var(--color-ink);
  font-size: 22px;
}

.bar-row,
.share-row {
  display: grid;
  grid-template-columns: 82px 1fr 92px;
  gap: 10px;
  align-items: center;
  padding: 9px 0;
  font-size: 13px;
}

.bar-row.compact {
  grid-template-columns: 74px 1fr 44px;
}

.bar-track {
  height: 10px;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(35, 30, 24, 0.08);
}

.bar-track.thin {
  grid-column: 1 / -1;
  height: 8px;
}

.bar-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--color-pine);
}

.share-row {
  display: block;
}

.share-row small {
  display: block;
  margin-top: 5px;
  opacity: 0.58;
}

.primary-btn,
.soft-btn {
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
  font-weight: 600;
}

.soft-btn {
  background: var(--color-surface);
}

button:disabled {
  opacity: 0.6;
}

@media (max-width: 1040px) {
  .grid {
    grid-template-columns: 1fr;
  }

  .filters {
    flex-wrap: wrap;
  }
}
</style>
