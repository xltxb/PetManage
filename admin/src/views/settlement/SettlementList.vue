<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">结算收银</h3>
      <div class="flex gap-2">
        <select v-model="status" class="field compact" @change="load">
          <option value="">全部</option>
          <option value="unpaid">未支付</option>
          <option value="paid">已支付</option>
          <option value="refunded">已退款</option>
          <option value="void">已作废</option>
        </select>
        <button @click="load" class="soft-btn">刷新</button>
      </div>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="grid grid-cols-[1.4fr_.9fr] gap-4">
      <div class="kpi-card">
        <h4 class="text-sm font-semibold mb-3">结算单</h4>
        <div v-if="loading" class="text-sm" style="opacity: 0.55">加载中...</div>
        <div v-else-if="settlements.length === 0" class="text-sm" style="opacity: 0.55">暂无结算单</div>
        <div v-for="row in settlements" :key="row.id" class="settlement-row">
          <div>
            <div class="flex items-center gap-2">
              <span class="font-medium">{{ row.code || `#${row.id}` }}</span>
              <span class="status-badge" :class="`status-${row.status}`">{{ statusLabel(row.status) }}</span>
            </div>
            <p class="text-xs mt-1" style="opacity: 0.55">{{ row.biz_type }} · 应收 ¥{{ yuan(row.total_amount) }} · 实收 ¥{{ yuan(row.paid_amount) }}</p>
          </div>
          <div class="flex items-center gap-2">
            <button v-if="row.status === 'unpaid'" @click="pay(row, 'cash')" :disabled="saving" class="mini-btn pine">现金</button>
            <button v-if="row.status === 'unpaid'" @click="pay(row, 'wallet')" :disabled="saving" class="mini-btn coral">钱包</button>
            <button v-if="row.status === 'paid'" @click="refund(row)" :disabled="saving" class="mini-btn berry">退款</button>
            <button v-if="row.status === 'unpaid'" @click="voidSettlement(row)" :disabled="saving" class="mini-btn ghost">作废</button>
          </div>
        </div>
      </div>

      <form class="kpi-card space-y-3" @submit.prevent="createSettlement">
        <h4 class="font-semibold">新建结算</h4>
        <label class="label">客户ID<input v-model.number="form.customer_id" class="field" type="number" min="0" /></label>
        <label class="label">业务类型
          <select v-model="form.biz_type" class="field">
            <option value="service">服务</option>
            <option value="retail">零售</option>
            <option value="boarding">寄养</option>
            <option value="mixed">混合</option>
          </select>
        </label>
        <label class="label">来源类型<input v-model.trim="form.source_type" class="field" required /></label>
        <label class="label">来源ID<input v-model.number="form.source_id" class="field" type="number" min="0" /></label>
        <label class="label">项目名称<input v-model.trim="form.name" class="field" required /></label>
        <label class="label">单价（分）<input v-model.number="form.unit_price" class="field" type="number" min="1" required /></label>
        <label class="label">数量<input v-model.number="form.quantity" class="field" type="number" min="1" required /></label>
        <label class="label">备注<input v-model.trim="form.remark" class="field" /></label>
        <button :disabled="saving" class="primary-btn">创建结算单</button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import client from '../../api/client'

interface Settlement {
  id: number
  code: string
  biz_type: string
  status: string
  total_amount: number
  paid_amount: number
}

const settlements = ref<Settlement[]>([])
const status = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const form = reactive({
  customer_id: 0,
  biz_type: 'service',
  source_type: 'manual',
  source_id: 0,
  name: '手工项目',
  unit_price: 1000,
  quantity: 1,
  remark: '',
})

function yuan(cents: number) {
  return ((cents || 0) / 100).toFixed(2)
}

function statusLabel(s: string) {
  const map: Record<string, string> = { unpaid: '未支付', paid: '已支付', refunded: '已退款', void: '已作废' }
  return map[s] || s
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
    const { data } = await client.get('/settlements', {
      params: { status: status.value || undefined, page: 1, page_size: 20 },
    })
    settlements.value = data.data?.list || []
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

async function createSettlement() {
  await runAction(() => client.post('/settlements', {
    customer_id: form.customer_id || 0,
    biz_type: form.biz_type,
    remark: form.remark,
    items: [{
      source_type: form.source_type,
      source_id: form.source_id || 0,
      name: form.name,
      unit_price: form.unit_price,
      quantity: form.quantity,
    }],
  }, idem('settlement-create')).then(() => undefined))
}

async function pay(row: Settlement, method: string) {
  await runAction(() => client.post(`/settlements/${row.id}/pay`, {
    method,
    amount: row.total_amount,
    operator_id: 0,
  }, idem('settlement-pay')).then(() => undefined))
}

async function refund(row: Settlement) {
  if (!confirm(`确认退款 ${row.code || row.id}？`)) return
  await runAction(() => client.post(`/settlements/${row.id}/refund`, {
    reason: '顾客要求退款',
    operator_id: 0,
  }, idem('settlement-refund')).then(() => undefined))
}

async function voidSettlement(row: Settlement) {
  if (!confirm(`确认作废 ${row.code || row.id}？`)) return
  await runAction(() => client.post(`/settlements/${row.id}/void`, {}, idem('settlement-void')).then(() => undefined))
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
  width: 120px;
}

.label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.settlement-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  font-size: 14px;
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
.mini-btn.ghost { color: var(--color-berry); border: 1px solid var(--color-berry); background: transparent; }

button:disabled {
  opacity: 0.6;
}
</style>
