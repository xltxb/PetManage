<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">商品库存</h3>
      <button @click="load" class="soft-btn">刷新</button>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="kpi-card">
      <div class="flex items-center justify-between mb-3">
        <h4 class="text-sm font-semibold">库存预警</h4>
        <span v-if="alerts.length" class="status-badge status-alert">{{ alerts.length }}</span>
      </div>
      <div v-if="loading" class="text-sm" style="opacity: 0.55">加载中...</div>
      <div v-else-if="alerts.length === 0" class="text-sm" style="color: var(--color-pine)">库存充足</div>
      <div
        v-for="item in alerts"
        :key="item.product_id"
        class="flex items-center justify-between py-2 border-b border-black/5 last:border-0 text-sm"
      >
        <span>{{ item.product_name || `商品 #${item.product_id}` }}</span>
        <span class="status-badge status-alert">{{ item.quantity }}/{{ item.safety_stock }}</span>
      </div>
    </div>

    <div class="grid grid-cols-3 gap-4">
      <form class="kpi-card space-y-3" @submit.prevent="saleOut">
        <h4 class="font-semibold">销售出库</h4>
        <label class="label">
          商品ID
          <input v-model.number="sale.product_id" class="field" type="number" min="1" required />
        </label>
        <label class="label">
          数量
          <input v-model.number="sale.quantity" class="field" type="number" min="1" required />
        </label>
        <button :disabled="saving" class="primary-btn">出库</button>
      </form>

      <form class="kpi-card space-y-3" @submit.prevent="purchaseIn">
        <h4 class="font-semibold">采购入库</h4>
        <label class="label">
          商品ID
          <input v-model.number="purchase.product_id" class="field" type="number" min="1" required />
        </label>
        <label class="label">
          数量
          <input v-model.number="purchase.quantity" class="field" type="number" min="1" required />
        </label>
        <button :disabled="saving" class="primary-btn">入库</button>
      </form>

      <form class="kpi-card space-y-3" @submit.prevent="adjustStock">
        <h4 class="font-semibold">库存调整</h4>
        <label class="label">
          商品ID
          <input v-model.number="adjust.product_id" class="field" type="number" min="1" required />
        </label>
        <label class="label">
          调整数
          <input v-model.number="adjust.delta" class="field" type="number" required />
        </label>
        <label class="label">
          原因
          <input v-model.trim="adjust.remark" class="field" required />
        </label>
        <button :disabled="saving" class="primary-btn">调整</button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import client from '../../api/client'

interface InventoryAlert {
  product_id: number
  product_name: string
  quantity: number
  safety_stock: number
}

const alerts = ref<InventoryAlert[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const sale = reactive({ product_id: 0, quantity: 1 })
const purchase = reactive({ product_id: 0, quantity: 1 })
const adjust = reactive({ product_id: 0, delta: 0, remark: '' })

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
    const { data } = await client.get('/inventory/alerts')
    alerts.value = data.data || []
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

async function saleOut() {
  await runAction(() => client.post('/inventory/sale-out', sale, idem('inventory-sale-out')).then(() => undefined))
}

async function purchaseIn() {
  await runAction(() => client.post('/inventory/purchase-in', purchase, idem('inventory-purchase-in')).then(() => undefined))
}

async function adjustStock() {
  if (!adjust.remark.trim()) {
    error.value = '库存调整必须填写原因'
    return
  }
  await runAction(() => client.post('/inventory/adjust', adjust, idem('inventory-adjust')).then(() => undefined))
}

onMounted(load)
</script>

<style scoped>
.label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.field {
  width: 100%;
  border: 1px solid rgba(35, 30, 24, 0.12);
  border-radius: 8px;
  padding: 8px 10px;
  background: white;
  font-size: 14px;
  color: var(--color-ink);
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
