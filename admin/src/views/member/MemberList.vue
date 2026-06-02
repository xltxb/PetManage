<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">会员客户</h3>
      <form class="flex gap-2" @submit.prevent="load">
        <input v-model.trim="keyword" class="field search" aria-label="搜索会员" />
        <button class="soft-btn">搜索</button>
      </form>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="grid grid-cols-[1.2fr_1fr] gap-4">
      <div class="kpi-card">
        <h4 class="text-sm font-semibold mb-3">客户列表</h4>
        <div v-if="loading" class="text-sm" style="opacity: 0.55">加载中...</div>
        <div v-else-if="customers.length === 0" class="text-sm" style="opacity: 0.55">暂无客户</div>
        <button
          v-for="c in customers"
          :key="c.id"
          type="button"
          class="row-btn"
          :class="{ active: selected?.id === c.id }"
          @click="selectCustomer(c.id)"
        >
          <span>
            <span class="font-medium">{{ c.name }}</span>
            <span class="ml-2 text-xs" style="opacity: 0.55">{{ c.phone || '无手机号' }}</span>
          </span>
          <span class="font-mono">¥{{ yuan(c.wallet_balance) }}</span>
        </button>
      </div>

      <div class="space-y-4">
        <div class="kpi-card">
          <h4 class="text-sm font-semibold mb-3">客户详情</h4>
          <div v-if="!selected" class="text-sm" style="opacity: 0.55">选择客户查看详情</div>
          <div v-else class="grid grid-cols-2 gap-3 text-sm">
            <p><span class="muted">姓名</span><br />{{ selected.name }}</p>
            <p><span class="muted">手机号</span><br />{{ selected.phone || '—' }}</p>
            <p><span class="muted">储值余额</span><br />¥{{ yuan(selected.wallet_balance) }}</p>
            <p><span class="muted">积分</span><br />{{ selected.points_balance }}</p>
            <p><span class="muted">累计消费</span><br />¥{{ yuan(selected.total_spend) }}</p>
            <p><span class="muted">等级ID</span><br />{{ selected.tier_id }}</p>
          </div>
        </div>

        <form class="kpi-card space-y-3" @submit.prevent="doRecharge">
          <h4 class="font-semibold">储值充值</h4>
          <input v-model.number="recharge.amount" class="field" type="number" min="1" aria-label="充值金额分" required />
          <input v-model.trim="recharge.remark" class="field" aria-label="充值备注" />
          <button :disabled="saving || !selected" class="primary-btn">充值</button>
        </form>

        <form class="kpi-card space-y-3" @submit.prevent="doAdjust">
          <h4 class="font-semibold">人工调整</h4>
          <input v-model.number="adjust.amount" class="field" type="number" aria-label="调整金额分" required />
          <input v-model.trim="adjust.remark" class="field" aria-label="调整原因" required />
          <button :disabled="saving || !selected" class="primary-btn">调整</button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import client from '../../api/client'

interface Customer {
  id: number
  name: string
  phone: string
  tier_id: number
  wallet_balance: number
  points_balance: number
  total_spend: number
}

const customers = ref<Customer[]>([])
const selected = ref<Customer | null>(null)
const keyword = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const recharge = reactive({ amount: 50000, remark: '储值充值' })
const adjust = reactive({ amount: 0, remark: '' })

function yuan(cents: number) {
  return ((cents || 0) / 100).toFixed(2)
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
    const { data } = await client.get('/customers', {
      params: { keyword: keyword.value || undefined, page: 1, page_size: 20 },
    })
    customers.value = data.data?.list || []
    if (!selected.value && customers.value.length) {
      await selectCustomer(customers.value[0].id)
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function selectCustomer(id: number) {
  error.value = ''
  try {
    const { data } = await client.get(`/customers/${id}`)
    selected.value = data.data
  } catch (err) {
    error.value = errorMessage(err)
  }
}

async function runAction(action: () => Promise<void>) {
  saving.value = true
  error.value = ''
  try {
    await action()
    if (selected.value) await selectCustomer(selected.value.id)
    await load()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

async function doRecharge() {
  if (!selected.value) return
  await runAction(() => client.post(`/customers/${selected.value!.id}/wallet`, recharge, idem('wallet-recharge')).then(() => undefined))
}

async function doAdjust() {
  if (!selected.value) return
  if (!adjust.remark.trim()) {
    error.value = '人工调整必须填写原因'
    return
  }
  await runAction(() => client.put(`/customers/${selected.value!.id}/wallet`, adjust, idem('wallet-adjust')).then(() => undefined))
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

.search {
  width: 220px;
}

.row-btn {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  text-align: left;
  font-size: 14px;
}

.row-btn.active {
  color: var(--color-coral);
}

.muted {
  opacity: 0.55;
  font-size: 12px;
}

.primary-btn,
.soft-btn {
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

.primary-btn:disabled {
  opacity: 0.6;
}

.soft-btn {
  background: var(--color-surface);
  color: var(--color-ink);
}
</style>
