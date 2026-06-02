# PawPrint P0 Admin Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace static admin in-progress views with minimal working Vue screens backed by existing P0 APIs.

**Architecture:** Keep the current Vue 3 route-view structure. Each page remains a self-contained `.vue` file using `admin/src/api/client.ts`, local `ref` state, inline page-specific helpers, and the existing `kpi-card`/status badge styles. No new frontend dependencies are introduced.

**Tech Stack:** Vue 3, TypeScript, Vite, Pinia auth, Axios client, existing Go/Gin backend APIs

**Spec:** `docs/superpowers/specs/2026-06-03-pawprint-p0-admin-completion-design.md`

---

## File Map

- Modify: `admin/src/views/inventory/InventoryList.vue`
  - Safety-alert list plus sale out, purchase in, and adjustment forms.
- Modify: `admin/src/views/member/MemberList.vue`
  - Customer list, selected-customer detail, recharge, and adjustment forms.
- Modify: `admin/src/views/settlement/SettlementList.vue`
  - Settlement list, simple one-item creation, pay, refund, void actions.
- Modify: `admin/src/views/boarding/BoardingList.vue`
  - Boarding order list, check-in, check-out, care-log action.
- Modify: `admin/src/views/pet/PetList.vue`
  - Customer pet lookup, pet creation, health and weight forms.
- Modify: `admin/src/views/setting/SettingView.vue`
  - Settings list, selected setting editor, save action.
- Modify: `admin/src/views/analytics/AnalyticsView.vue`
  - Analytics report summary grids/lists.

## Shared Frontend Patterns

Every page should use this error helper pattern:

```ts
function errorMessage(err: unknown) {
  const maybe = err as { response?: { data?: { message?: string } }; message?: string }
  return maybe.response?.data?.message || maybe.message || '操作失败'
}
```

Every write action should wrap loading state and reload its page data:

```ts
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
```

Use idempotency headers for mutating P0 financial/inventory operations:

```ts
function idem(action: string) {
  return { headers: { 'Idempotency-Key': `${action}-${Date.now()}-${Math.random().toString(16).slice(2)}` } }
}
```

## Task 1: Inventory Page

**Files:**
- Modify: `admin/src/views/inventory/InventoryList.vue`

- [ ] **Step 1: Replace static inventory view**

Replace `InventoryList.vue` with a full page containing:

```vue
<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">商品库存</h3>
      <button @click="load" class="px-3 py-2 rounded text-sm" style="background: var(--color-surface)">刷新</button>
    </div>
    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>
    <div class="kpi-card">
      <h4 class="text-sm font-semibold mb-3">库存预警</h4>
      <div v-if="loading" class="text-sm" style="opacity:.55">加载中...</div>
      <div v-else-if="alerts.length === 0" class="text-sm" style="color: var(--color-pine)">库存充足</div>
      <div v-for="item in alerts" :key="item.product_id" class="flex justify-between py-2 border-b border-black/5 last:border-0 text-sm">
        <span>{{ item.product_name || ('商品 #' + item.product_id) }}</span>
        <span class="status-badge status-alert">{{ item.quantity }}/{{ item.safety_stock }}</span>
      </div>
    </div>
    <div class="grid grid-cols-3 gap-4">
      <form class="kpi-card space-y-3" @submit.prevent="saleOut">
        <h4 class="font-semibold">销售出库</h4>
        <input v-model.number="sale.product_id" class="field" type="number" min="1" aria-label="商品ID" required />
        <input v-model.number="sale.quantity" class="field" type="number" min="1" aria-label="数量" required />
        <button :disabled="saving" class="primary-btn">出库</button>
      </form>
      <form class="kpi-card space-y-3" @submit.prevent="purchaseIn">
        <h4 class="font-semibold">采购入库</h4>
        <input v-model.number="purchase.product_id" class="field" type="number" min="1" aria-label="商品ID" required />
        <input v-model.number="purchase.quantity" class="field" type="number" min="1" aria-label="数量" required />
        <button :disabled="saving" class="primary-btn">入库</button>
      </form>
      <form class="kpi-card space-y-3" @submit.prevent="adjustStock">
        <h4 class="font-semibold">库存调整</h4>
        <input v-model.number="adjust.product_id" class="field" type="number" min="1" aria-label="商品ID" required />
        <input v-model.number="adjust.delta" class="field" type="number" aria-label="调整数量，可为负" required />
        <input v-model.trim="adjust.remark" class="field" aria-label="调整原因" required />
        <button :disabled="saving" class="primary-btn">调整</button>
      </form>
    </div>
  </div>
</template>
```

Script requirements:

```ts
import { onMounted, reactive, ref } from 'vue'
import client from '../../api/client'
```

State and actions:

```ts
interface InventoryAlert { product_id: number; product_name: string; quantity: number; safety_stock: number }
const alerts = ref<InventoryAlert[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const sale = reactive({ product_id: 0, quantity: 1 })
const purchase = reactive({ product_id: 0, quantity: 1 })
const adjust = reactive({ product_id: 0, delta: 0, remark: '' })
async function load() { loading.value = true; error.value = ''; try { const { data } = await client.get('/inventory/alerts'); alerts.value = data.data || [] } catch (err) { error.value = errorMessage(err) } finally { loading.value = false } }
async function saleOut() { await runAction(() => client.post('/inventory/sale-out', sale, idem('inventory-sale-out')).then(() => undefined)) }
async function purchaseIn() { await runAction(() => client.post('/inventory/purchase-in', purchase, idem('inventory-purchase-in')).then(() => undefined)) }
async function adjustStock() { await runAction(() => client.post('/inventory/adjust', adjust, idem('inventory-adjust')).then(() => undefined)) }
onMounted(load)
```

Add local scoped CSS classes:

```css
.field { width: 100%; border: 1px solid rgba(35,30,24,.12); border-radius: 8px; padding: 8px 10px; background: white; font-size: 14px; }
.primary-btn { width: 100%; border-radius: 8px; padding: 8px 12px; color: white; background: var(--color-coral); font-size: 14px; font-weight: 600; }
```

- [ ] **Step 2: Verify inventory page build**

```bash
cd admin && npm run build
rg -n "开发中|图表开发中" admin/src/views/inventory admin/src/views
```

Expected: build passes; inventory file has no in-progress marker.

- [ ] **Step 3: Commit**

```bash
git add admin/src/views/inventory/InventoryList.vue
git commit -m "feat(admin): complete inventory operations page"
```

## Task 2: Member Page

**Files:**
- Modify: `admin/src/views/member/MemberList.vue`

- [ ] **Step 1: Replace static member view**

Implement a page with:

- search input bound to `keyword`
- customer list from `GET /customers`
- selected customer detail from `GET /customers/:id`
- recharge form posting `{ amount, remark }` to `POST /customers/:id/wallet`
- adjustment form putting `{ amount, remark }` to `PUT /customers/:id/wallet`
- yuan display via `(amount / 100).toFixed(2)`

Required script API:

```ts
interface Customer { id: number; name: string; phone: string; tier_id: number; wallet_balance: number; points_balance: number; total_spend: number }
const customers = ref<Customer[]>([])
const selected = ref<Customer | null>(null)
const keyword = ref('')
const recharge = reactive({ amount: 50000, remark: '储值充值' })
const adjust = reactive({ amount: 0, remark: '' })
async function load() { const { data } = await client.get('/customers', { params: { keyword: keyword.value || undefined, page: 1, page_size: 20 } }); customers.value = data.data?.list || [] }
async function selectCustomer(id: number) { const { data } = await client.get(`/customers/${id}`); selected.value = data.data }
async function doRecharge() { if (!selected.value) return; await runAction(() => client.post(`/customers/${selected.value!.id}/wallet`, recharge, idem('wallet-recharge')).then(() => selectCustomer(selected.value!.id))) }
async function doAdjust() { if (!selected.value || !adjust.remark.trim()) { error.value = '人工调整必须填写原因'; return }; await runAction(() => client.put(`/customers/${selected.value!.id}/wallet`, adjust, idem('wallet-adjust')).then(() => selectCustomer(selected.value!.id))) }
```

- [ ] **Step 2: Verify**

```bash
cd admin && npm run build
rg -n "开发中|图表开发中" admin/src/views/member
```

Expected: build passes; member file has no in-progress marker.

- [ ] **Step 3: Commit**

```bash
git add admin/src/views/member/MemberList.vue
git commit -m "feat(admin): complete member wallet page"
```

## Task 3: Settlement Page

**Files:**
- Modify: `admin/src/views/settlement/SettlementList.vue`

- [ ] **Step 1: Replace static settlement view**

Implement:

- settlement list from `GET /settlements`
- status filter: all, unpaid, paid, refunded, void
- one-item create form posting:

```ts
{
  customer_id: form.customer_id || 0,
  biz_type: form.biz_type,
  items: [{ source_type: form.source_type, source_id: form.source_id || 0, name: form.name, unit_price: form.unit_price, quantity: form.quantity }],
  remark: form.remark
}
```

- pay action:

```ts
client.post(`/settlements/${row.id}/pay`, { method, amount: row.total_amount, operator_id: 0 }, idem('settlement-pay'))
```

- refund action:

```ts
client.post(`/settlements/${row.id}/refund`, { reason: refundReason.value || '顾客要求退款', operator_id: 0 }, idem('settlement-refund'))
```

- void action:

```ts
client.post(`/settlements/${row.id}/void`, {}, idem('settlement-void'))
```

UI should show `code`, `biz_type`, `status`, `total_amount`, `paid_amount`, and actions only when status allows them.

- [ ] **Step 2: Verify**

```bash
cd admin && npm run build
rg -n "开发中|图表开发中" admin/src/views/settlement
```

Expected: build passes; settlement file has no in-progress marker.

- [ ] **Step 3: Commit**

```bash
git add admin/src/views/settlement/SettlementList.vue
git commit -m "feat(admin): complete settlement cashier page"
```

## Task 4: Boarding Page

**Files:**
- Modify: `admin/src/views/boarding/BoardingList.vue`

- [ ] **Step 1: Replace static boarding view**

Implement:

- list from `GET /boarding-orders?page=1&page_size=20&status=...`
- check-in form posting to `/boarding-orders/check-in` with `customer_id`, `pet_id`, `room_id`, `room_type_code`, `price_per_night`, `planned_check_in`, `planned_check_out`
- check-out action to `/boarding-orders/:id/check-out`
- care-log action to `/boarding-orders/:id/care-logs` with `{ task, status: 'done', note, operator_id: 0 }`
- order rows showing status, room id, customer id, pet id, actual check-in/out, nights, total amount, settlement id

- [ ] **Step 2: Verify**

```bash
cd admin && npm run build
rg -n "开发中|图表开发中" admin/src/views/boarding
```

Expected: build passes; boarding file has no in-progress marker.

- [ ] **Step 3: Commit**

```bash
git add admin/src/views/boarding/BoardingList.vue
git commit -m "feat(admin): complete boarding operations page"
```

## Task 5: Pet Page

**Files:**
- Modify: `admin/src/views/pet/PetList.vue`

- [ ] **Step 1: Replace static pet view**

Implement:

- customer ID lookup calling `GET /customers/:id/pets`
- selected pet detail calling `GET /pets/:id`
- create pet form posting to `/pets` with `customer_id`, `name`, `species`, `breed`, `gender`, `birthday`, `weight_g`, `chip_no`
- health form posting to `/pets/:id/health` with `type`, `title`, `performed_at`, `next_due_at`, `detail`
- weight form posting to `/pets/:id/weights` with `weight_g`

Dates should use browser `input type="date"` and send ISO date strings accepted by the backend.

- [ ] **Step 2: Verify**

```bash
cd admin && npm run build
rg -n "开发中|图表开发中" admin/src/views/pet
```

Expected: build passes; pet file has no in-progress marker.

- [ ] **Step 3: Commit**

```bash
git add admin/src/views/pet/PetList.vue
git commit -m "feat(admin): complete pet record page"
```

## Task 6: Settings and Analytics Pages

**Files:**
- Modify: `admin/src/views/setting/SettingView.vue`
- Modify: `admin/src/views/analytics/AnalyticsView.vue`

- [ ] **Step 1: Replace static settings view**

Settings page behavior:

- `GET /settings`
- select setting by key
- edit as text in a textarea
- save using `PUT /settings/:key` with `{ value: parsedOrRawValue, updated_by: 0 }`

Parser:

```ts
function parseValue(raw: string) {
  try { return JSON.parse(raw) } catch { return raw }
}
```

- [ ] **Step 2: Replace static analytics view**

Analytics page behavior:

- date range inputs `start` and `end`
- `GET /analytics/report?start=YYYY-MM-DD&end=YYYY-MM-DD`
- render `revenue_trend`, `service_breakdown`, `peak_hours`, and `retention_funnel`
- show empty states for empty arrays

- [ ] **Step 3: Verify**

```bash
cd admin && npm run build
rg -n "开发中|图表开发中" admin/src/views/setting admin/src/views/analytics
```

Expected: build passes; settings and analytics files have no in-progress marker.

- [ ] **Step 4: Commit**

```bash
git add admin/src/views/setting/SettingView.vue admin/src/views/analytics/AnalyticsView.vue
git commit -m "feat(admin): complete settings and analytics pages"
```

## Task 7: Final Admin Completion Gate

**Files:**
- Modify only files needed by gate failures.

- [ ] **Step 1: Run frontend build**

```bash
cd admin && npm run build
```

Expected: PASS.

- [ ] **Step 2: Scan for remaining admin in-progress markers**

```bash
rg -n "开发中|图表开发中" admin/src/views
```

Expected: no matches.

- [ ] **Step 3: Run backend gate**

```bash
cd backend && GOCACHE=/private/tmp/pet-gocache go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit gate fixes if needed**

If gate commands required changes:

```bash
git add admin backend
git commit -m "test: pass P0 admin completion gate"
```

If no files changed, record the passing command outputs in the final implementation report.

## Completion Criteria

- All seven admin route views are functional screens rather than static in-progress views.
- `admin/src/views` has no `开发中` or `图表开发中` marker.
- Each page has loading, empty, and error behavior.
- Financial and inventory writes use `Idempotency-Key`.
- `cd admin && npm run build` passes.
- `cd backend && GOCACHE=/private/tmp/pet-gocache go test ./...` passes.
