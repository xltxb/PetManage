<template>
  <div class="pos-layout">
    <!-- Left Panel: Product & Service Selection -->
    <div class="pos-left">
      <!-- Barcode Input -->
      <div class="barcode-section">
        <input
          ref="barcodeInput"
          v-model="barcode"
          class="barcode-input"
          placeholder="扫描条码或输入商品关键字..."
          @keyup.enter="onBarcodeEnter"
          autofocus
        />
      </div>

      <!-- Product Search -->
      <div class="search-section">
        <input
          v-model="productKeyword"
          class="search-input"
          placeholder="搜索商品名称/条码..."
          @input="onProductSearchDebounced"
        />
        <div v-if="productResults.length > 0" class="search-results">
          <div
            v-for="prod in productResults"
            :key="'p' + prod.id"
            class="search-result-item"
            @click="addProduct(prod)"
          >
            <span class="result-name">{{ prod.name }}</span>
            <span class="result-barcode">{{ prod.barcode }}</span>
            <span class="result-price">¥{{ (prod.price_cents / 100).toFixed(2) }}</span>
            <span class="result-stock">库存:{{ prod.stock }}</span>
          </div>
        </div>
        <div v-if="productKeyword && productResults.length === 0 && !productLoading" class="no-results">
          未找到匹配商品
        </div>
      </div>

      <!-- Service Items -->
      <div class="service-section">
        <h3 class="section-title">服务项目</h3>
        <div class="service-categories">
          <button
            v-for="cat in serviceCategories"
            :key="cat.id"
            :class="['cat-btn', { active: selectedCategory === cat.id }]"
            @click="selectCategory(cat.id)"
          >
            {{ cat.name }}
          </button>
        </div>
        <div class="service-items">
          <div
            v-for="svc in filteredServiceItems"
            :key="'s' + svc.id"
            class="service-item-card"
            @click="addServiceItem(svc)"
          >
            <div class="svc-name">{{ svc.name }}</div>
            <div class="svc-meta">
              <span>{{ svc.duration_minutes }}分钟</span>
              <span class="svc-price">¥{{ (svc.price_cents / 100).toFixed(2) }}</span>
            </div>
          </div>
          <div v-if="filteredServiceItems.length === 0" class="no-results">
            暂无可选服务项目
          </div>
        </div>
      </div>

      <!-- Member Identification -->
      <div class="member-section">
        <h3 class="section-title">会员识别</h3>
        <div class="member-input-row">
          <input
            v-model="memberPhone"
            class="member-phone-input"
            placeholder="输入手机号查找会员..."
            @keyup.enter="lookupMember"
          />
          <button class="btn btn-sm btn-outline" @click="lookupMember">查找</button>
          <button class="btn btn-sm btn-outline" @click="clearMember" v-if="currentMember">清除</button>
        </div>
        <div v-if="currentMember" class="member-found">
          <span class="member-badge">会员</span>
          <span>{{ currentMember.name }} · {{ currentMember.card_no }}</span>
          <span class="member-phone">{{ currentMember.phone }}</span>
        </div>
        <div v-if="memberError" class="member-error">{{ memberError }}</div>
      </div>
    </div>

    <!-- Right Panel: Cart -->
    <div class="pos-right">
      <h2 class="cart-title">购物车</h2>

      <!-- Cart Items -->
      <div class="cart-items" ref="cartItemsContainer">
        <div v-if="cartItems.length === 0" class="cart-empty">
          购物车为空，请扫码或搜索添加商品/服务
        </div>
        <div
          v-for="(item, idx) in cartItems"
          :key="idx"
          class="cart-item"
        >
          <div class="cart-item-info">
            <div class="cart-item-name">{{ item.name }}</div>
            <div class="cart-item-meta">
              <span v-if="item.barcode">{{ item.barcode }}</span>
              <span class="cart-item-price">¥{{ (item.unitPriceCents / 100).toFixed(2) }}</span>
            </div>
          </div>
          <div class="cart-item-actions">
            <button class="qty-btn" @click="decreaseQty(idx)" :disabled="item.quantity <= 1">-</button>
            <span class="qty-display">{{ item.quantity }}</span>
            <button class="qty-btn" @click="increaseQty(idx)">+</button>
            <button class="remove-btn" @click="removeItem(idx)">×</button>
          </div>
          <div class="cart-item-line-total">
            ¥{{ ((item.lineTotalCents) / 100).toFixed(2) }}
          </div>
        </div>
      </div>

      <!-- Cart Summary -->
      <div class="cart-summary" v-if="cartItems.length > 0">
        <div class="summary-row">
          <span>原价</span>
          <span>¥{{ (cartTotal.originalCents / 100).toFixed(2) }}</span>
        </div>
        <div class="summary-row discount" v-if="cartTotal.discountCents > 0">
          <span>会员优惠</span>
          <span>-¥{{ (cartTotal.discountCents / 100).toFixed(2) }}</span>
        </div>
        <div class="summary-row total">
          <span>应收金额</span>
          <span class="total-amount">¥{{ (cartTotal.payableCents / 100).toFixed(2) }}</span>
        </div>
      </div>

      <!-- Order Notes -->
      <div class="order-notes-section" v-if="cartItems.length > 0">
        <input
          v-model="orderNotes"
          class="notes-input"
          placeholder="整单备注（可选）"
        />
      </div>

      <!-- Quick Payment Buttons -->
      <div class="payment-section" v-if="cartItems.length > 0">
        <div class="payment-buttons">
          <button class="btn-pay cash" @click="quickCheckout('cash')">
            现金收款 ¥{{ (cartTotal.payableCents / 100).toFixed(2) }}
          </button>
          <button class="btn-pay wechat" @click="quickCheckout('wechat')">
            微信收款 ¥{{ (cartTotal.payableCents / 100).toFixed(2) }}
          </button>
          <button class="btn-pay alipay" @click="quickCheckout('alipay')">
            支付宝 ¥{{ (cartTotal.payableCents / 100).toFixed(2) }}
          </button>
        </div>
        <div v-if="checkoutError" class="checkout-error">{{ checkoutError }}</div>
        <div v-if="checkoutSuccess" class="checkout-success">
          ✓ 收银完成！订单号: {{ checkoutSuccess }}
          <button class="btn btn-sm" @click="newOrder">新订单</button>
        </div>
      </div>

      <div v-if="calculating" class="loading-overlay">
        <div class="spinner"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { api } from '@/api/client'

// --- State ---
const barcode = ref('')
const barcodeInput = ref<HTMLInputElement | null>(null)
const productKeyword = ref('')
const productResults = ref<any[]>([])
const productLoading = ref(false)
const selectedCategory = ref<number | null>(null)
const serviceCategories = ref<any[]>([])
const serviceItems = ref<any[]>([])
const memberPhone = ref('')
const currentMember = ref<any>(null)
const memberError = ref('')
const orderNotes = ref('')
const cartItems = ref<CartItem[]>([])
const calculating = ref(false)
const checkoutError = ref('')
const checkoutSuccess = ref('')
let searchTimer: any = null

interface CartItem {
  productId?: number
  skuId?: number
  serviceItemId?: number
  name: string
  barcode?: string
  unitPriceCents: number
  discountCents: number
  quantity: number
  lineTotalCents: number
}

interface CartTotal {
  originalCents: number
  discountCents: number
  payableCents: number
}

const cartTotal = ref<CartTotal>({ originalCents: 0, discountCents: 0, payableCents: 0 })

// --- Computed ---
const filteredServiceItems = computed(() => {
  if (!selectedCategory.value) return serviceItems.value.filter((s: any) => s.status === 'active')
  return serviceItems.value.filter((s: any) => s.status === 'active' && s.category_id === selectedCategory.value)
})

// --- Methods ---
async function onBarcodeEnter() {
  const val = barcode.value.trim()
  if (!val) return

  // Try exact barcode match first
  try {
    const result = await api.getProducts({ keyword: val, page_size: 1 })
    if (result.total > 0 && result.products[0].barcode === val) {
      // Exact barcode match — auto add
      addProduct(result.products[0])
      barcode.value = ''
      barcodeInput.value?.focus()
      return
    }
  } catch (e) {
    // ignore
  }

  // Try as keyword search
  productKeyword.value = val
  await doProductSearch(val)
  barcode.value = ''
  barcodeInput.value?.focus()
}

function onProductSearchDebounced() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    doProductSearch(productKeyword.value)
  }, 300)
}

async function doProductSearch(keyword: string) {
  if (!keyword || keyword.length < 1) {
    productResults.value = []
    return
  }
  productLoading.value = true
  try {
    const result = await api.getProducts({ keyword, page_size: 8 })
    productResults.value = result.products || []
  } catch (e) {
    productResults.value = []
  } finally {
    productLoading.value = false
  }
}

function addProduct(prod: any) {
  // Check duplicate
  const dup = cartItems.value.find(
    (i) => i.productId === prod.id && !i.skuId && !i.serviceItemId
  )
  if (dup) {
    dup.quantity++
  } else {
    cartItems.value.push({
      productId: prod.id,
      name: prod.name,
      barcode: prod.barcode,
      unitPriceCents: prod.price_cents,
      discountCents: 0,
      quantity: 1,
      lineTotalCents: prod.price_cents,
    })
  }
  productKeyword.value = ''
  productResults.value = []
  recalculate()
}

function addServiceItem(svc: any) {
  const dup = cartItems.value.find((i) => i.serviceItemId === svc.id)
  if (dup) {
    dup.quantity++
  } else {
    cartItems.value.push({
      serviceItemId: svc.id,
      name: svc.name,
      unitPriceCents: svc.price_cents,
      discountCents: 0,
      quantity: 1,
      lineTotalCents: svc.price_cents,
    })
  }
  recalculate()
}

function selectCategory(catId: number) {
  selectedCategory.value = selectedCategory.value === catId ? null : catId
}

function increaseQty(idx: number) {
  cartItems.value[idx].quantity++
  recalculate()
}

function decreaseQty(idx: number) {
  if (cartItems.value[idx].quantity > 1) {
    cartItems.value[idx].quantity--
    recalculate()
  }
}

function removeItem(idx: number) {
  cartItems.value.splice(idx, 1)
  recalculate()
}

async function recalculate() {
  if (cartItems.value.length === 0) {
    cartTotal.value = { originalCents: 0, discountCents: 0, payableCents: 0 }
    return
  }

  calculating.value = true
  try {
    const items = cartItems.value.map((i) => ({
      product_id: i.productId,
      sku_id: i.skuId,
      service_item_id: i.serviceItemId,
      quantity: i.quantity,
    }))
    const result = await api.posCartCalculate({
      member_id: currentMember.value?.member_id || null,
      items,
    })
    // Update cart items with server-calculated values
    cartItems.value = result.items.map((ri) => ({
      productId: ri.product_id,
      skuId: ri.sku_id,
      serviceItemId: ri.service_item_id,
      name: ri.name,
      barcode: ri.barcode,
      unitPriceCents: ri.unit_price_cents,
      discountCents: ri.discount_cents,
      quantity: ri.quantity,
      lineTotalCents: ri.line_total_cents,
    }))
    cartTotal.value = {
      originalCents: result.original_cents,
      discountCents: result.discount_cents,
      payableCents: result.payable_cents,
    }
  } catch (e) {
    console.error('Cart calculation failed:', e)
  } finally {
    calculating.value = false
  }
}

async function lookupMember() {
  const phone = memberPhone.value.trim()
  if (!phone) return
  memberError.value = ''
  try {
    const member = await api.posMemberLookup(phone)
    currentMember.value = member
    recalculate()
  } catch (e: any) {
    currentMember.value = null
    memberError.value = e.message || '未找到会员'
    recalculate()
  }
}

function clearMember() {
  currentMember.value = null
  memberPhone.value = ''
  memberError.value = ''
  recalculate()
}

async function quickCheckout(method: string) {
  if (cartItems.value.length === 0) return
  checkoutError.value = ''
  checkoutSuccess.value = ''

  try {
    const items = cartItems.value.map((i) => ({
      product_id: i.productId,
      sku_id: i.skuId,
      service_item_id: i.serviceItemId,
      quantity: i.quantity,
    }))
    const result = await api.posCheckout({
      member_id: currentMember.value?.member_id || null,
      items,
      payments: [{ method, amount_cents: cartTotal.value.payableCents }],
      order_notes: orderNotes.value || undefined,
    })
    checkoutSuccess.value = String(result.order_id)
  } catch (e: any) {
    checkoutError.value = e.message || '收银失败'
  }
}

function newOrder() {
  cartItems.value = []
  orderNotes.value = ''
  currentMember.value = null
  memberPhone.value = ''
  checkoutSuccess.value = ''
  checkoutError.value = ''
  cartTotal.value = { originalCents: 0, discountCents: 0, payableCents: 0 }
  nextTick(() => barcodeInput.value?.focus())
}

// --- Lifecycle ---
onMounted(async () => {
  // Load service categories and items
  try {
    const [catResult, itemResult] = await Promise.all([
      api.getServiceCategories(),
      api.getServiceItems({ status: 'active', page_size: 100 }),
    ])
    serviceCategories.value = catResult.categories || []
    serviceItems.value = itemResult.items || []
  } catch (e) {
    console.error('Failed to load service data:', e)
  }
})
</script>

<style scoped>
.pos-layout {
  display: flex;
  height: calc(100vh - 60px);
  gap: 0;
  background: #f5f5f5;
}

.pos-left {
  width: 380px;
  min-width: 380px;
  background: #fff;
  border-right: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.pos-right {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 16px 24px;
  position: relative;
  overflow-y: auto;
}

.barcode-section {
  padding: 12px;
  border-bottom: 1px solid #e0e0e0;
}

.barcode-input {
  width: 100%;
  padding: 10px 14px;
  border: 2px solid #4caf50;
  border-radius: 6px;
  font-size: 15px;
  outline: none;
  box-sizing: border-box;
}

.barcode-input:focus {
  border-color: #2e7d32;
  box-shadow: 0 0 0 3px rgba(76, 175, 80, 0.15);
}

.search-section {
  padding: 8px 12px;
  border-bottom: 1px solid #e0e0e0;
  position: relative;
}

.search-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
  box-sizing: border-box;
}

.search-results {
  position: absolute;
  top: 100%;
  left: 12px;
  right: 12px;
  background: #fff;
  border: 1px solid #ddd;
  border-radius: 4px;
  max-height: 240px;
  overflow-y: auto;
  z-index: 100;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}

.search-result-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  cursor: pointer;
  gap: 8px;
  border-bottom: 1px solid #f0f0f0;
  font-size: 13px;
}

.search-result-item:hover {
  background: #e8f5e9;
}

.result-name {
  flex: 1;
  font-weight: 500;
}

.result-barcode {
  color: #888;
  font-size: 11px;
}

.result-price {
  color: #e53935;
  font-weight: 600;
}

.result-stock {
  color: #666;
  font-size: 11px;
}

.no-results {
  padding: 12px;
  color: #999;
  text-align: center;
  font-size: 13px;
}

.service-section {
  padding: 12px;
  flex: 1;
  overflow-y: auto;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin: 0 0 8px 0;
}

.service-categories {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 10px;
}

.cat-btn {
  padding: 4px 12px;
  border: 1px solid #ddd;
  border-radius: 14px;
  background: #fff;
  cursor: pointer;
  font-size: 12px;
  color: #555;
  transition: all 0.15s;
}

.cat-btn.active {
  background: #1976d2;
  color: #fff;
  border-color: #1976d2;
}

.service-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.service-item-card {
  padding: 8px 12px;
  border: 1px solid #e8e8e8;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
}

.service-item-card:hover {
  background: #e3f2fd;
  border-color: #90caf9;
}

.svc-name {
  font-size: 13px;
  font-weight: 500;
}

.svc-meta {
  display: flex;
  justify-content: space-between;
  margin-top: 4px;
  font-size: 11px;
  color: #888;
}

.svc-price {
  color: #e53935;
  font-weight: 600;
}

.member-section {
  padding: 12px;
  border-top: 1px solid #e0e0e0;
}

.member-input-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.member-phone-input {
  flex: 1;
  padding: 7px 10px;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
}

.btn {
  padding: 6px 14px;
  border: 1px solid #ccc;
  border-radius: 4px;
  background: #fff;
  cursor: pointer;
  font-size: 12px;
  white-space: nowrap;
}

.btn-sm {
  padding: 4px 10px;
  font-size: 11px;
}

.btn-outline {
  background: #fff;
  border-color: #1976d2;
  color: #1976d2;
}

.btn-outline:hover {
  background: #e3f2fd;
}

.member-found {
  margin-top: 8px;
  padding: 8px 10px;
  background: #e8f5e9;
  border-radius: 4px;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.member-badge {
  background: #4caf50;
  color: #fff;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: 600;
}

.member-phone {
  color: #888;
  margin-left: auto;
}

.member-error {
  margin-top: 6px;
  color: #e53935;
  font-size: 12px;
}

/* Right Panel */
.cart-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 12px 0;
  color: #333;
}

.cart-items {
  flex: 1;
  overflow-y: auto;
  margin-bottom: 12px;
}

.cart-empty {
  text-align: center;
  padding: 60px 20px;
  color: #bbb;
  font-size: 15px;
}

.cart-item {
  display: flex;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #eee;
  gap: 12px;
}

.cart-item-info {
  flex: 1;
  min-width: 0;
}

.cart-item-name {
  font-size: 14px;
  font-weight: 500;
}

.cart-item-meta {
  font-size: 12px;
  color: #888;
  margin-top: 2px;
}

.cart-item-price {
  color: #e53935;
  font-weight: 500;
  margin-left: 8px;
}

.cart-item-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.qty-btn {
  width: 26px;
  height: 26px;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: #f5f5f5;
  cursor: pointer;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.qty-btn:hover {
  background: #e0e0e0;
}

.qty-display {
  width: 28px;
  text-align: center;
  font-size: 14px;
  font-weight: 600;
}

.remove-btn {
  width: 26px;
  height: 26px;
  border: none;
  background: none;
  color: #e53935;
  cursor: pointer;
  font-size: 18px;
  margin-left: 4px;
}

.cart-item-line-total {
  font-size: 14px;
  font-weight: 600;
  color: #e53935;
  min-width: 70px;
  text-align: right;
}

/* Summary */
.cart-summary {
  border-top: 2px solid #333;
  padding: 12px 0;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  font-size: 14px;
  color: #555;
}

.summary-row.discount {
  color: #4caf50;
}

.summary-row.total {
  font-size: 18px;
  font-weight: 700;
  color: #e53935;
  padding-top: 8px;
}

.total-amount {
  font-size: 22px;
}

/* Notes */
.order-notes-section {
  margin-bottom: 12px;
}

.notes-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
}

/* Payment */
.payment-section {
  padding-top: 8px;
}

.payment-buttons {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.btn-pay {
  flex: 1;
  min-width: 160px;
  padding: 14px 20px;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 600;
  color: #fff;
  cursor: pointer;
  transition: opacity 0.15s;
}

.btn-pay:hover {
  opacity: 0.9;
}

.btn-pay:active {
  transform: scale(0.98);
}

.btn-pay.cash {
  background: #43a047;
}

.btn-pay.wechat {
  background: #07c160;
}

.btn-pay.alipay {
  background: #1677ff;
}

.checkout-error {
  margin-top: 10px;
  padding: 10px;
  background: #ffebee;
  border-radius: 4px;
  color: #c62828;
  font-size: 13px;
}

.checkout-success {
  margin-top: 10px;
  padding: 12px 16px;
  background: #e8f5e9;
  border-radius: 6px;
  color: #2e7d32;
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 12px;
}

.loading-overlay {
  position: absolute;
  inset: 0;
  background: rgba(255,255,255,0.7);
  display: flex;
  align-items: center;
  justify-content: center;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid #e0e0e0;
  border-top-color: #1976d2;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
