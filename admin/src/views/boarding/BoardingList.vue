<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">寄养业务</h3>
      <div class="flex gap-2">
        <select v-model="status" class="field compact" @change="load">
          <option value="">全部</option>
          <option value="checked_in">在住</option>
          <option value="checked_out">已退房</option>
          <option value="booked">已预约</option>
          <option value="cancelled">已取消</option>
        </select>
        <button class="soft-btn" @click="load">刷新</button>
      </div>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="grid grid-cols-[1.35fr_.95fr] gap-4">
      <section class="kpi-card">
        <div class="flex items-center justify-between mb-3">
          <h4 class="text-sm font-semibold">寄养订单</h4>
          <span class="text-xs" style="opacity: 0.55">共 {{ orders.length }} 单</span>
        </div>
        <div v-if="loading" class="muted">加载中...</div>
        <div v-else-if="orders.length === 0" class="muted">暂无寄养订单</div>
        <article v-for="order in orders" :key="order.id" class="order-row">
          <div class="order-main">
            <div class="flex items-center gap-2">
              <span class="font-medium">订单 #{{ order.id }}</span>
              <span class="status-badge" :class="`status-${order.status}`">{{ statusLabel(order.status) }}</span>
            </div>
            <p class="text-xs mt-1" style="opacity: 0.58">
              客户 {{ order.customer_id }} · 宠物 {{ order.pet_id }} · 笼位 {{ order.room_id || '-' }} · {{ order.room_type_snapshot || '-' }}
            </p>
            <p class="text-xs mt-1" style="opacity: 0.58">
              {{ dateLabel(order.planned_check_in) }} 至 {{ dateLabel(order.planned_check_out) }} · ¥{{ yuan(order.price_per_night) }}/晚
            </p>
            <p class="text-xs mt-1" style="opacity: 0.58">
              实际 {{ dateTimeLabel(order.actual_check_in) }} / {{ dateTimeLabel(order.actual_check_out) }} ·
              {{ order.nights || '-' }} 晚 · 合计 ¥{{ yuan(order.total_amount || 0) }} · 结算 {{ order.settlement_id || '-' }}
            </p>
          </div>
          <div class="row-actions">
            <button v-if="order.status === 'checked_in'" class="mini-btn pine" :disabled="saving" @click="postCare(order, 'feeding')">喂食</button>
            <button v-if="order.status === 'checked_in'" class="mini-btn coral" :disabled="saving" @click="postCare(order, 'walking')">遛宠</button>
            <button v-if="order.status === 'checked_in'" class="mini-btn berry" :disabled="saving" @click="checkOut(order)">退房</button>
            <button class="mini-btn ghost" :disabled="saving" @click="selectOrder(order)">详情</button>
          </div>
        </article>
      </section>

      <div class="space-y-4">
        <form class="kpi-card space-y-3" @submit.prevent="checkIn">
          <h4 class="font-semibold">办理入住</h4>
          <div class="form-grid">
            <label class="label">客户ID<input v-model.number="form.customer_id" class="field" type="number" min="1" required /></label>
            <label class="label">宠物ID<input v-model.number="form.pet_id" class="field" type="number" min="1" required /></label>
            <label class="label">笼位ID<input v-model.number="form.room_id" class="field" type="number" min="1" required /></label>
            <label class="label">房型
              <select v-model="form.room_type_code" class="field">
                <option value="small">小型犬舍</option>
                <option value="medium">中型犬舍</option>
                <option value="large">大型犬舍</option>
                <option value="cat">猫舍</option>
              </select>
            </label>
            <label class="label">入住日期<input v-model="form.planned_check_in" class="field" type="date" required /></label>
            <label class="label">离店日期<input v-model="form.planned_check_out" class="field" type="date" required /></label>
          </div>
          <label class="label">每晚价格（分）<input v-model.number="form.price_per_night" class="field" type="number" min="1" required /></label>
          <label class="label">备注<input v-model.trim="form.remark" class="field" /></label>
          <button class="primary-btn" :disabled="saving">确认入住</button>
        </form>

        <section v-if="selected" class="kpi-card space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="font-semibold">照护记录 #{{ selected.id }}</h4>
            <button class="soft-btn" @click="loadCareLogs(selected)">刷新</button>
          </div>
          <div class="form-grid">
            <label class="label">任务
              <select v-model="careForm.task" class="field">
                <option value="feeding">喂食</option>
                <option value="walking">遛宠</option>
                <option value="medication">用药</option>
                <option value="photo">拍照</option>
              </select>
            </label>
            <label class="label">状态
              <select v-model="careForm.status" class="field">
                <option value="done">完成</option>
                <option value="pending">待处理</option>
              </select>
            </label>
          </div>
          <label class="label">说明<input v-model.trim="careForm.note" class="field" /></label>
          <button class="primary-btn" :disabled="saving || selected.status !== 'checked_in'" @click="postCare(selected)">登记照护</button>
          <div v-if="careLogs.length === 0" class="muted">暂无照护记录</div>
          <div v-for="log in careLogs" :key="log.id" class="care-log">
            <span>{{ taskLabel(log.task) }} · {{ careStatusLabel(log.status) }}</span>
            <span>{{ dateTimeLabel(log.created_at) }}</span>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import client from '../../api/client'

interface BoardingOrder {
  id: number
  customer_id: number
  pet_id: number
  room_id?: number
  room_type_snapshot: string
  price_per_night: number
  status: string
  planned_check_in: string
  planned_check_out: string
  actual_check_in?: string
  actual_check_out?: string
  nights?: number
  total_amount?: number
  settlement_id?: number
}

interface CareLog {
  id: number
  task: string
  status: string
  note: string
  created_at: string
}

const today = new Date()
const tomorrow = new Date(Date.now() + 24 * 60 * 60 * 1000)
const orders = ref<BoardingOrder[]>([])
const careLogs = ref<CareLog[]>([])
const selected = ref<BoardingOrder | null>(null)
const status = ref('checked_in')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const form = reactive({
  customer_id: 1,
  pet_id: 1,
  room_id: 1,
  room_type_code: 'small',
  price_per_night: 8800,
  planned_check_in: dateInput(today),
  planned_check_out: dateInput(tomorrow),
  remark: '',
})
const careForm = reactive({
  task: 'feeding',
  status: 'done',
  note: '状态正常',
})

function dateInput(date: Date) {
  return date.toISOString().slice(0, 10)
}

function toAPIDate(date: string, hour: number) {
  return new Date(`${date}T${String(hour).padStart(2, '0')}:00:00`).toISOString()
}

function yuan(cents: number) {
  return ((cents || 0) / 100).toFixed(2)
}

function dateLabel(value: string) {
  return value ? new Date(value).toLocaleDateString('zh-CN') : '-'
}

function dateTimeLabel(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
}

function statusLabel(value: string) {
  const map: Record<string, string> = { booked: '已预约', checked_in: '在住', checked_out: '已退房', cancelled: '已取消' }
  return map[value] || value
}

function taskLabel(value: string) {
  const map: Record<string, string> = { feeding: '喂食', walking: '遛宠', medication: '用药', photo: '拍照' }
  return map[value] || value
}

function careStatusLabel(value: string) {
  return value === 'done' ? '完成' : '待处理'
}

function errorMessage(err: unknown) {
  const maybe = err as { response?: { data?: { message?: string } }; message?: string }
  return maybe.response?.data?.message || maybe.message || '操作失败'
}

function idem(action: string) {
  return { headers: { 'Idempotency-Key': `${action}-${Date.now()}-${Math.random().toString(16).slice(2)}` } }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await client.get('/boarding-orders', {
      params: { status: status.value || undefined, page: 1, page_size: 20 },
    })
    orders.value = data.data?.list || []
    if (selected.value) {
      selected.value = orders.value.find((order) => order.id === selected.value?.id) || null
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function runAction(action: () => Promise<void>) {
  saving.value = true
  error.value = ''
  try {
    await action()
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

async function checkIn() {
  await runAction(() => client.post('/boarding-orders/check-in', {
    customer_id: form.customer_id,
    pet_id: form.pet_id,
    room_id: form.room_id,
    room_type_code: form.room_type_code,
    price_per_night: form.price_per_night,
    planned_check_in: toAPIDate(form.planned_check_in, 9),
    planned_check_out: toAPIDate(form.planned_check_out, 18),
    source: 1,
    remark: form.remark,
  }, idem('boarding-check-in')).then(() => undefined))
}

async function checkOut(order: BoardingOrder) {
  if (!confirm(`确认办理订单 #${order.id} 退房？`)) return
  await runAction(() => client.post(`/boarding-orders/${order.id}/check-out`, {}, idem('boarding-check-out')).then(() => undefined))
}

async function postCare(order: BoardingOrder, task?: string) {
  const body = {
    task: task || careForm.task,
    status: task ? 'done' : careForm.status,
    note: task ? '快捷照护登记' : careForm.note,
    operator_id: 0,
  }
  await runAction(() => client.post(`/boarding-orders/${order.id}/care-logs`, body, idem('boarding-care')).then(() => loadCareLogs(order)))
}

async function selectOrder(order: BoardingOrder) {
  selected.value = order
  await loadCareLogs(order)
}

async function loadCareLogs(order: BoardingOrder) {
  error.value = ''
  try {
    const { data } = await client.get(`/boarding-orders/${order.id}/care-logs`)
    careLogs.value = data.data || []
  } catch (err) {
    error.value = errorMessage(err)
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

.compact {
  width: 128px;
}

.label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.muted {
  font-size: 14px;
  opacity: 0.55;
}

.order-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  padding: 13px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  font-size: 14px;
}

.order-main {
  min-width: 0;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.primary-btn,
.soft-btn,
.mini-btn {
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
  font-weight: 600;
}

.primary-btn {
  width: 100%;
  color: white;
  background: var(--color-coral);
}

.soft-btn {
  background: var(--color-surface);
}

.mini-btn {
  color: white;
  font-size: 12px;
  padding: 6px 8px;
}

.mini-btn.pine { background: var(--color-pine); }
.mini-btn.coral { background: var(--color-coral); }
.mini-btn.berry { background: var(--color-berry); }
.mini-btn.ghost { color: var(--color-pine); border: 1px solid var(--color-pine); background: transparent; }

.care-log {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  font-size: 13px;
}

button:disabled {
  opacity: 0.6;
}

@media (max-width: 960px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
