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
            <button class="remove-btn" @click="removeItem(idx)">&times;</button>
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
        <!-- Coupon deduction display -->
        <div class="summary-row discount" v-if="couponDeduction > 0">
          <span>优惠券抵扣</span>
          <span>-¥{{ (couponDeduction / 100).toFixed(2) }}</span>
        </div>
        <div class="summary-row total" v-if="couponDeduction > 0">
          <span>优惠后应付</span>
          <span class="total-amount">¥{{ (effectiveTotal / 100).toFixed(2) }}</span>
        </div>
        <!-- Available balance/points info -->
        <div class="summary-row member-info" v-if="currentMember">
          <span>储值余额: ¥{{ (currentMember.balance_cents || 0 / 100).toFixed(2) }}</span>
          <span>积分: {{ currentMember.points || 0 }}</span>
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

      <!-- Payment Panel -->
      <div class="payment-section" v-if="cartItems.length > 0 && !checkoutSuccess">
        <!-- Combined Payment Methods -->
        <div class="payment-panel">
          <div class="payment-header">
            <span class="payment-header-title">收款</span>
            <span class="remaining-label">
              待付: <strong>¥{{ (remainingCents / 100).toFixed(2) }}</strong>
            </span>
          </div>

          <!-- Active payment rows -->
          <div
            v-for="(pm, idx) in paymentRows"
            :key="idx"
            class="payment-row"
          >
            <select v-model="pm.method" class="method-select" @change="onMethodChange(idx)">
              <option value="">选择方式</option>
              <option value="cash">现金</option>
              <option value="wechat">微信</option>
              <option value="alipay">支付宝</option>
              <option value="balance">储值余额</option>
              <option value="points">积分抵扣</option>
              <option value="coupon">优惠券</option>
            </select>

            <!-- Cash: amount + received -->
            <template v-if="pm.method === 'cash'">
              <input
                v-model.number="pm.amount"
                class="amount-input"
                type="number"
                placeholder="收款金额"
                @input="recalcRemaining"
              />
              <input
                v-model.number="pm.received"
                class="amount-input"
                type="number"
                placeholder="实收(选填)"
                @input="recalcRemaining"
              />
              <span v-if="pm.received > 0 && pm.received > pm.amount" class="change-hint">
                找零: ¥{{ ((pm.received - pm.amount) / 100).toFixed(2) }}
              </span>
            </template>

            <!-- WeChat/Alipay: amount -->
            <template v-else-if="pm.method === 'wechat' || pm.method === 'alipay'">
              <input
                v-model.number="pm.amount"
                class="amount-input"
                type="number"
                placeholder="支付金额"
                @input="recalcRemaining"
              />
            </template>

            <!-- Balance: amount + available display -->
            <template v-else-if="pm.method === 'balance'">
              <input
                v-model.number="pm.amount"
                class="amount-input"
                type="number"
                placeholder="抵扣金额"
                :max="currentMember?.balance_cents || 0"
                @input="recalcRemaining"
              />
              <span class="avail-hint">
                可用: ¥{{ ((currentMember?.balance_cents || 0) / 100).toFixed(2) }}
              </span>
            </template>

            <!-- Points: amount + available display -->
            <template v-else-if="pm.method === 'points'">
              <input
                v-model.number="pm.amount"
                class="amount-input"
                type="number"
                placeholder="抵扣金额"
                :max="cartTotal.maxPointsDeductCents || 0"
                @input="recalcRemaining"
              />
              <span class="avail-hint">
                可用: {{ currentMember?.points || 0 }}积分 (最高抵¥{{ ((cartTotal.maxPointsDeductCents || 0) / 100).toFixed(2) }})
              </span>
            </template>

            <!-- Coupon: code input + verify -->
            <template v-else-if="pm.method === 'coupon'">
              <input
                v-model="pm.couponCode"
                class="coupon-input"
                placeholder="输入券码"
              />
              <button class="btn btn-sm btn-verify" @click="verifyCoupon(idx)" :disabled="pm.verified">验证</button>
              <span v-if="pm.verified" class="coupon-ok">已抵扣 ¥{{ (pm.amount / 100).toFixed(2) }}</span>
              <span v-if="pm.error" class="coupon-err">{{ pm.error }}</span>
            </template>

            <button class="remove-pm-btn" @click="removePaymentRow(idx)" v-if="paymentRows.length > 1">&times;</button>
          </div>

          <!-- Add payment method -->
          <button class="add-pm-btn" @click="addPaymentRow" v-if="remainingCents > 0">
            + 添加支付方式
          </button>

          <!-- Total paid and change summary -->
          <div class="payment-summary" v-if="totalPaid > 0">
            <div class="ps-row">
              <span>已收合计</span>
              <span>¥{{ (totalPaid / 100).toFixed(2) }}</span>
            </div>
            <div class="ps-row change" v-if="totalChange > 0">
              <span>找零</span>
              <span>¥{{ (totalChange / 100).toFixed(2) }}</span>
            </div>
          </div>

          <!-- Confirm Payment Button -->
          <button
            class="btn-confirm-pay"
            :disabled="remainingCents > 0 || paymentRows.some(p => !p.method || (p.method !== 'coupon' && !p.amount) || (p.method === 'coupon' && !p.verified))"
            @click="submitPayment"
          >
            <template v-if="totalChange > 0">
              确认收款 (找零 ¥{{ (totalChange / 100).toFixed(2) }})
            </template>
            <template v-else>
              确认收款 ¥{{ (totalPaid / 100).toFixed(2) }}
            </template>
          </button>
        </div>

        <div v-if="checkoutError" class="checkout-error">{{ checkoutError }}</div>
      </div>

      <!-- Checkout Success -->
      <div class="payment-section" v-if="checkoutSuccess">
        <div class="checkout-success">
          <div class="success-icon">&#10003;</div>
          <div class="success-text">
            <div>收银完成！订单号: <strong>{{ checkoutSuccess.order_id }}</strong></div>
            <div class="success-detail" v-if="checkoutSuccess.change_cents > 0">
              找零: ¥{{ (checkoutSuccess.change_cents / 100).toFixed(2) }}
            </div>
            <div class="success-detail" v-if="checkoutSuccess.payments">
              <template v-for="(p, i) in checkoutSuccess.payments" :key="i">
                <span class="pm-tag">{{ methodLabel(p.method) }}: ¥{{ (p.amount_cents / 100).toFixed(2) }}</span>
              </template>
            </div>
          </div>
          <div class="success-actions">
            <button class="btn btn-print" @click="printReceipt">&#128424; 打印小票</button>
            <button class="btn btn-new" @click="newOrder">新订单</button>
          </div>
        </div>
      </div>

      <div v-if="calculating" class="loading-overlay">
        <div class="spinner"></div>
      </div>
    </div>

    <!-- Print Receipt (hidden) -->
    <div id="receipt-print" v-if="checkoutSuccess" class="receipt-template" :style="{ maxWidth: receiptTemplate?.paper_width === '58mm' ? '220px' : '300px' }">
      <div class="receipt-header">
        <img v-if="receiptTemplate?.logo_url" :src="receiptTemplate.logo_url" class="r-logo" alt="Logo" />
        <h2>{{ receiptTemplate?.store_name || shopName || '宠物店' }}</h2>
        <p v-if="receiptTemplate?.contact_phone" class="r-contact">{{ receiptTemplate.contact_phone }}</p>
        <p v-if="receiptTemplate?.contact_address" class="r-contact">{{ receiptTemplate.contact_address }}</p>
        <p>订单号: {{ checkoutSuccess.order_id }}</p>
        <p>{{ new Date().toLocaleString('zh-CN') }}</p>
      </div>
      <hr />
      <div v-if="currentMember" class="receipt-member">
        <p>会员: {{ currentMember.name }} · {{ currentMember.card_no }}</p>
        <p>手机: {{ currentMember.phone }}</p>
      </div>
      <hr />
      <div class="receipt-items">
        <div v-for="(item, idx) in cartItems" :key="idx" class="receipt-item">
          <span class="r-item-name">{{ item.name }}</span>
          <span>x{{ item.quantity }}</span>
          <span>¥{{ (item.lineTotalCents / 100).toFixed(2) }}</span>
        </div>
      </div>
      <hr />
      <div class="receipt-total">
        <div class="r-total-row">
          <span>折扣前</span>
          <span>¥{{ (cartTotal.originalCents / 100).toFixed(2) }}</span>
        </div>
        <div class="r-total-row" v-if="cartTotal.discountCents > 0">
          <span>优惠</span>
          <span>-¥{{ (cartTotal.discountCents / 100).toFixed(2) }}</span>
        </div>
        <div class="r-total-row total">
          <span>合计</span>
          <strong>¥{{ (cartTotal.payableCents / 100).toFixed(2) }}</strong>
        </div>
        <div class="r-total-row" v-if="checkoutSuccess.payments">
          <template v-for="(p, i) in checkoutSuccess.payments" :key="i">
            <div class="r-payment-item">
              <span>{{ methodLabel(p.method) }}</span>
              <span>¥{{ (p.amount_cents / 100).toFixed(2) }}</span>
            </div>
          </template>
        </div>
        <div class="r-total-row change" v-if="checkoutSuccess.change_cents > 0">
          <span>找零</span>
          <span>¥{{ (checkoutSuccess.change_cents / 100).toFixed(2) }}</span>
        </div>
      </div>
      <hr />
      <div class="receipt-footer">
        <p>{{ receiptTemplate?.footer_note || '感谢惠顾，欢迎再次光临！' }}</p>
        <p v-if="orderNotes">备注: {{ orderNotes }}</p>
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
const checkoutSuccess = ref<any>(null)
const shopName = ref('')
const receiptTemplate = ref<any>(null)
let searchTimer: any = null

// Payment panel state
const paymentRows = ref<PaymentRow[]>([{ method: '', amount: 0, received: 0, couponCode: '', verified: false, error: '' }])
const couponDeduction = ref(0)

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
  memberBalanceCents: number
  memberPoints: number
  maxPointsDeductCents: number
}

interface PaymentRow {
  method: string
  amount: number
  received: number
  couponCode: string
  verified: boolean
  error: string
}

const cartTotal = ref<CartTotal>({
  originalCents: 0, discountCents: 0, payableCents: 0,
  memberBalanceCents: 0, memberPoints: 0, maxPointsDeductCents: 0,
})

// --- Computed ---
const filteredServiceItems = computed(() => {
  if (!selectedCategory.value) return serviceItems.value.filter((s: any) => s.status === 'active')
  return serviceItems.value.filter((s: any) => s.status === 'active' && s.category_id === selectedCategory.value)
})

const effectiveTotal = computed(() => {
  return Math.max(0, cartTotal.value.payableCents - couponDeduction.value)
})

const totalPaid = computed(() => {
  return paymentRows.value.reduce((sum, p) => {
    if (p.method === 'coupon') return sum // coupon doesn't count as cash paid
    if (!p.method || !p.amount) return sum
    return sum + (p.amount || 0)
  }, 0)
})

const totalChange = computed(() => {
  return paymentRows.value.reduce((sum, p) => {
    if (p.method === 'cash' && p.received > 0 && p.received > (p.amount || 0)) {
      return sum + (p.received - (p.amount || 0))
    }
    return sum
  }, 0)
})

const remainingCents = computed(() => {
  return Math.max(0, effectiveTotal.value - totalPaid.value)
})

// --- Methods ---
function methodLabel(m: string): string {
  const map: Record<string, string> = { cash: '现金', wechat: '微信', alipay: '支付宝', balance: '储值', points: '积分', coupon: '优惠券' }
  return map[m] || m
}

async function onBarcodeEnter() {
  const val = barcode.value.trim()
  if (!val) return

  try {
    const result = await api.getProducts({ keyword: val, page_size: 1 })
    if (result.total > 0 && result.products[0].barcode === val) {
      addProduct(result.products[0])
      barcode.value = ''
      barcodeInput.value?.focus()
      return
    }
  } catch (e) { /* ignore */ }

  productKeyword.value = val
  await doProductSearch(val)
  barcode.value = ''
  barcodeInput.value?.focus()
}

function onProductSearchDebounced() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => doProductSearch(productKeyword.value), 300)
}

async function doProductSearch(keyword: string) {
  if (!keyword || keyword.length < 1) { productResults.value = []; return }
  productLoading.value = true
  try {
    const result = await api.getProducts({ keyword, page_size: 8 })
    productResults.value = result.products || []
  } catch (e) { productResults.value = [] }
  finally { productLoading.value = false }
}

function addProduct(prod: any) {
  const dup = cartItems.value.find((i) => i.productId === prod.id && !i.skuId && !i.serviceItemId)
  if (dup) { dup.quantity++ }
  else {
    cartItems.value.push({
      productId: prod.id, name: prod.name, barcode: prod.barcode,
      unitPriceCents: prod.price_cents, discountCents: 0, quantity: 1, lineTotalCents: prod.price_cents,
    })
  }
  productKeyword.value = ''
  productResults.value = []
  recalculate()
}

function addServiceItem(svc: any) {
  const dup = cartItems.value.find((i) => i.serviceItemId === svc.id)
  if (dup) { dup.quantity++ }
  else {
    cartItems.value.push({
      serviceItemId: svc.id, name: svc.name,
      unitPriceCents: svc.price_cents, discountCents: 0, quantity: 1, lineTotalCents: svc.price_cents,
    })
  }
  recalculate()
}

function selectCategory(catId: number) {
  selectedCategory.value = selectedCategory.value === catId ? null : catId
}

function increaseQty(idx: number) { cartItems.value[idx].quantity++; recalculate() }
function decreaseQty(idx: number) { if (cartItems.value[idx].quantity > 1) { cartItems.value[idx].quantity--; recalculate() } }
function removeItem(idx: number) { cartItems.value.splice(idx, 1); recalculate() }

async function recalculate() {
  if (cartItems.value.length === 0) {
    cartTotal.value = { originalCents: 0, discountCents: 0, payableCents: 0, memberBalanceCents: 0, memberPoints: 0, maxPointsDeductCents: 0 }
    resetPayments()
    return
  }
  calculating.value = true
  try {
    const items = cartItems.value.map((i) => ({
      product_id: i.productId, sku_id: i.skuId, service_item_id: i.serviceItemId, quantity: i.quantity,
    }))
    const result = await api.posCartCalculate({
      member_id: currentMember.value?.member_id || null, items,
    })
    cartItems.value = result.items.map((ri) => ({
      productId: ri.product_id, skuId: ri.sku_id, serviceItemId: ri.service_item_id,
      name: ri.name, barcode: ri.barcode,
      unitPriceCents: ri.unit_price_cents, discountCents: ri.discount_cents,
      quantity: ri.quantity, lineTotalCents: ri.line_total_cents,
    }))
    cartTotal.value = {
      originalCents: result.original_cents, discountCents: result.discount_cents,
      payableCents: result.payable_cents,
      memberBalanceCents: result.member_balance_cents || 0,
      memberPoints: result.member_points || 0,
      maxPointsDeductCents: result.max_points_deduct_cents || 0,
    }
    // Update member info with latest balance/points
    if (currentMember.value) {
      currentMember.value.balance_cents = result.member_balance_cents || 0
      currentMember.value.points = result.member_points || 0
    }
    resetPayments()
  } catch (e) {
    console.error('Cart calculation failed:', e)
  } finally { calculating.value = false }
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
  resetPayments()
  recalculate()
}

// --- Payment Panel ---
function resetPayments() {
  couponDeduction.value = 0
  paymentRows.value = [{ method: '', amount: 0, received: 0, couponCode: '', verified: false, error: '' }]
}

function addPaymentRow() {
  paymentRows.value.push({ method: '', amount: 0, received: 0, couponCode: '', verified: false, error: '' })
}

function removePaymentRow(idx: number) {
  const removed = paymentRows.value[idx]
  paymentRows.value.splice(idx, 1)
  if (removed.method === 'coupon' && removed.verified) {
    couponDeduction.value = 0
  }
}

function onMethodChange(idx: number) {
  const pm = paymentRows.value[idx]
  pm.amount = 0
  pm.received = 0
  pm.couponCode = ''
  pm.verified = false
  pm.error = ''
  if (pm.method === 'coupon') {
    // Reset coupon deduction when method changes
    couponDeduction.value = 0
    for (const row of paymentRows.value) {
      if (row.method === 'coupon' && row.verified) {
        couponDeduction.value = row.amount
      }
    }
  }
}

function recalcRemaining() {
  // Reactive computation handles this
}

async function verifyCoupon(idx: number) {
  const pm = paymentRows.value[idx]
  const code = pm.couponCode.trim()
  if (!code) {
    pm.error = '请输入券码'
    return
  }
  pm.error = ''
  try {
    const coupon = await api.posCouponVerify(code)
    if (coupon.min_order_cents > cartTotal.value.payableCents) {
      pm.error = `订单需满¥${(coupon.min_order_cents / 100).toFixed(2)}`
      return
    }
    let deduction = coupon.value_cents
    if (coupon.discount_type === 'percent') {
      deduction = Math.floor(cartTotal.value.payableCents * coupon.value_cents / 100)
    }
    if (deduction > cartTotal.value.payableCents) deduction = cartTotal.value.payableCents
    pm.amount = deduction
    pm.verified = true
    couponDeduction.value = deduction
  } catch (e: any) {
    pm.error = e.message || '券码无效'
  }
}

async function submitPayment() {
  if (cartItems.value.length === 0) return
  checkoutError.value = ''

  try {
    const items = cartItems.value.map((i) => ({
      product_id: i.productId, sku_id: i.skuId, service_item_id: i.serviceItemId, quantity: i.quantity,
    }))
    const payments = paymentRows.value
      .filter(p => p.method && (p.amount > 0 || p.method === 'coupon'))
      .map(p => ({
        method: p.method,
        amount_cents: p.amount,
        received_cents: p.method === 'cash' && p.received > 0 ? p.received : undefined,
        coupon_code: p.method === 'coupon' ? p.couponCode : undefined,
      }))

    if (payments.length === 0) {
      checkoutError.value = '请至少选择一种支付方式'
      return
    }

    const result = await api.posCheckout({
      member_id: currentMember.value?.member_id || null,
      items,
      payments,
      order_notes: orderNotes.value || undefined,
    })
    checkoutSuccess.value = result
    // Auto-print receipt after checkout
    nextTick(() => { setTimeout(() => printReceipt(), 500) })
  } catch (e: any) {
    checkoutError.value = e.message || '收银失败'
  }
}

function printReceipt() {
  const el = document.getElementById('receipt-print')
  if (!el) return
  const paperWidth = receiptTemplate.value?.paper_width === '58mm' ? 58 : 80
  const win = window.open('', '_blank', `width=${paperWidth * 4},height=600`)
  if (!win) return
  win.document.write(`
    <html><head><title>小票</title>
    <style>
      @page { margin: 0; size: ${paperWidth}mm auto; }
      body { font-family: monospace; font-size: 12px; width: ${paperWidth - 10}mm; margin: 5mm auto; padding: 0; }
      h2 { text-align: center; margin: 0 0 4px 0; font-size: 16px; }
      p { margin: 2px 0; text-align: center; }
      hr { border: none; border-top: 1px dashed #999; margin: 8px 0; }
      .r-logo { display: block; max-width: 60px; max-height: 60px; margin: 0 auto 6px; }
      .r-contact { font-size: 11px; color: #666; }
      .receipt-items { margin: 8px 0; }
      .receipt-item { display: flex; justify-content: space-between; margin: 3px 0; gap: 6px; }
      .r-item-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
      .receipt-member { text-align: center; font-size: 11px; }
      .receipt-total { margin: 8px 0; }
      .r-total-row { display: flex; justify-content: space-between; margin: 2px 0; }
      .r-total-row.total { font-size: 14px; }
      .r-total-row.change { color: #4caf50; }
      .r-payment-item { display: flex; justify-content: space-between; width: 100%; }
      .receipt-footer { margin-top: 8px; text-align: center; }
    </style></head><body>
    ${el.innerHTML}
    </body></html>
  `)
  win.document.close()
  win.focus()
  setTimeout(() => { win.print(); win.close() }, 300)
}

function newOrder() {
  cartItems.value = []
  orderNotes.value = ''
  currentMember.value = null
  memberPhone.value = ''
  checkoutSuccess.value = null
  checkoutError.value = ''
  resetPayments()
  cartTotal.value = { originalCents: 0, discountCents: 0, payableCents: 0, memberBalanceCents: 0, memberPoints: 0, maxPointsDeductCents: 0 }
  nextTick(() => barcodeInput.value?.focus())
}

// --- Lifecycle ---
onMounted(async () => {
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
  try {
    const settings = await api.getShopSettings()
    shopName.value = settings.name || ''
  } catch (e) { /* ignore */ }
  try {
    receiptTemplate.value = await api.getReceiptTemplate()
  } catch (e) { /* ignore */ }
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

.barcode-section { padding: 12px; border-bottom: 1px solid #e0e0e0; }

.barcode-input {
  width: 100%; padding: 10px 14px; border: 2px solid #4caf50; border-radius: 6px;
  font-size: 15px; outline: none; box-sizing: border-box;
}
.barcode-input:focus { border-color: #2e7d32; box-shadow: 0 0 0 3px rgba(76,175,80,0.15); }

.search-section { padding: 8px 12px; border-bottom: 1px solid #e0e0e0; position: relative; }
.search-input {
  width: 100%; padding: 8px 12px; border: 1px solid #ccc; border-radius: 4px;
  font-size: 14px; outline: none; box-sizing: border-box;
}
.search-results {
  position: absolute; top: 100%; left: 12px; right: 12px; background: #fff;
  border: 1px solid #ddd; border-radius: 4px; max-height: 240px; overflow-y: auto;
  z-index: 100; box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}
.search-result-item {
  display: flex; align-items: center; padding: 8px 12px; cursor: pointer;
  gap: 8px; border-bottom: 1px solid #f0f0f0; font-size: 13px;
}
.search-result-item:hover { background: #e8f5e9; }
.result-name { flex: 1; font-weight: 500; }
.result-barcode { color: #888; font-size: 11px; }
.result-price { color: #e53935; font-weight: 600; }
.result-stock { color: #666; font-size: 11px; }
.no-results { padding: 12px; color: #999; text-align: center; font-size: 13px; }

.service-section { padding: 12px; flex: 1; overflow-y: auto; }
.section-title { font-size: 14px; font-weight: 600; color: #333; margin: 0 0 8px 0; }
.service-categories { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
.cat-btn {
  padding: 4px 12px; border: 1px solid #ddd; border-radius: 14px; background: #fff;
  cursor: pointer; font-size: 12px; color: #555; transition: all 0.15s;
}
.cat-btn.active { background: #1976d2; color: #fff; border-color: #1976d2; }
.service-items { display: flex; flex-direction: column; gap: 6px; }
.service-item-card {
  padding: 8px 12px; border: 1px solid #e8e8e8; border-radius: 6px; cursor: pointer;
  transition: background 0.15s;
}
.service-item-card:hover { background: #e3f2fd; border-color: #90caf9; }
.svc-name { font-size: 13px; font-weight: 500; }
.svc-meta { display: flex; justify-content: space-between; margin-top: 4px; font-size: 11px; color: #888; }
.svc-price { color: #e53935; font-weight: 600; }

.member-section { padding: 12px; border-top: 1px solid #e0e0e0; }
.member-input-row { display: flex; gap: 6px; align-items: center; }
.member-phone-input {
  flex: 1; padding: 7px 10px; border: 1px solid #ccc; border-radius: 4px;
  font-size: 13px; outline: none;
}
.btn { padding: 6px 14px; border: 1px solid #ccc; border-radius: 4px; background: #fff; cursor: pointer; font-size: 12px; white-space: nowrap; }
.btn-sm { padding: 4px 10px; font-size: 11px; }
.btn-outline { background: #fff; border-color: #1976d2; color: #1976d2; }
.btn-outline:hover { background: #e3f2fd; }
.member-found { margin-top: 8px; padding: 8px 10px; background: #e8f5e9; border-radius: 4px; font-size: 13px; display: flex; align-items: center; gap: 8px; }
.member-badge { background: #4caf50; color: #fff; padding: 1px 6px; border-radius: 3px; font-size: 11px; font-weight: 600; }
.member-phone { color: #888; margin-left: auto; }
.member-error { margin-top: 6px; color: #e53935; font-size: 12px; }

/* Right Panel */
.cart-title { font-size: 18px; font-weight: 600; margin: 0 0 12px 0; color: #333; }
.cart-items { flex: 1; overflow-y: auto; margin-bottom: 12px; }
.cart-empty { text-align: center; padding: 60px 20px; color: #bbb; font-size: 15px; }
.cart-item { display: flex; align-items: center; padding: 10px 0; border-bottom: 1px solid #eee; gap: 12px; }
.cart-item-info { flex: 1; min-width: 0; }
.cart-item-name { font-size: 14px; font-weight: 500; }
.cart-item-meta { font-size: 12px; color: #888; margin-top: 2px; }
.cart-item-price { color: #e53935; font-weight: 500; margin-left: 8px; }
.cart-item-actions { display: flex; align-items: center; gap: 4px; }
.qty-btn {
  width: 26px; height: 26px; border: 1px solid #ddd; border-radius: 4px;
  background: #f5f5f5; cursor: pointer; font-size: 14px; display: flex; align-items: center; justify-content: center;
}
.qty-btn:hover { background: #e0e0e0; }
.qty-display { width: 28px; text-align: center; font-size: 14px; font-weight: 600; }
.remove-btn { width: 26px; height: 26px; border: none; background: none; color: #e53935; cursor: pointer; font-size: 18px; margin-left: 4px; }
.cart-item-line-total { font-size: 14px; font-weight: 600; color: #e53935; min-width: 70px; text-align: right; }

/* Summary */
.cart-summary { border-top: 2px solid #333; padding: 12px 0; }
.summary-row { display: flex; justify-content: space-between; padding: 4px 0; font-size: 14px; color: #555; }
.summary-row.discount { color: #4caf50; }
.summary-row.total { font-size: 18px; font-weight: 700; color: #e53935; padding-top: 8px; }
.total-amount { font-size: 22px; }
.summary-row.member-info { font-size: 12px; color: #888; margin-top: 4px; }

/* Notes */
.order-notes-section { margin-bottom: 12px; }
.notes-input { width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 4px; font-size: 13px; outline: none; box-sizing: border-box; }

/* Payment Panel */
.payment-section { padding-top: 8px; }
.payment-panel { border: 1px solid #ddd; border-radius: 8px; padding: 12px 16px; background: #fafafa; }
.payment-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.payment-header-title { font-weight: 600; font-size: 15px; }
.remaining-label { font-size: 14px; color: #e53935; }
.remaining-label strong { font-size: 18px; }

.payment-row { display: flex; align-items: center; gap: 6px; margin-bottom: 8px; flex-wrap: wrap; }
.method-select { padding: 6px 8px; border: 1px solid #ccc; border-radius: 4px; font-size: 13px; min-width: 90px; }
.amount-input { width: 100px; padding: 6px 8px; border: 1px solid #ccc; border-radius: 4px; font-size: 13px; }
.coupon-input { width: 130px; padding: 6px 8px; border: 1px solid #ccc; border-radius: 4px; font-size: 13px; }
.btn-verify { background: #1976d2; color: #fff; border-color: #1976d2; }
.btn-verify:disabled { background: #ccc; }
.change-hint { color: #4caf50; font-size: 12px; font-weight: 600; }
.avail-hint { color: #888; font-size: 11px; }
.coupon-ok { color: #4caf50; font-size: 12px; font-weight: 600; }
.coupon-err { color: #e53935; font-size: 12px; }
.remove-pm-btn { border: none; background: none; color: #e53935; cursor: pointer; font-size: 16px; }

.add-pm-btn {
  width: 100%; padding: 8px; border: 1px dashed #1976d2; border-radius: 4px;
  background: none; color: #1976d2; cursor: pointer; font-size: 13px; margin-bottom: 10px;
}
.add-pm-btn:hover { background: #e3f2fd; }

.payment-summary { border-top: 1px solid #e0e0e0; padding: 8px 0; margin-top: 4px; }
.ps-row { display: flex; justify-content: space-between; font-size: 14px; padding: 2px 0; }
.ps-row.change { color: #4caf50; font-weight: 600; }

.btn-confirm-pay {
  width: 100%; padding: 14px 20px; border: none; border-radius: 8px;
  font-size: 16px; font-weight: 700; color: #fff; background: #43a047; cursor: pointer;
  margin-top: 8px; transition: opacity 0.15s;
}
.btn-confirm-pay:hover:not(:disabled) { opacity: 0.9; }
.btn-confirm-pay:disabled { background: #ccc; cursor: not-allowed; }

.checkout-error { margin-top: 10px; padding: 10px; background: #ffebee; border-radius: 4px; color: #c62828; font-size: 13px; }

/* Success */
.checkout-success {
  padding: 16px 20px; background: #e8f5e9; border-radius: 8px; border: 1px solid #a5d6a7;
}
.success-icon { font-size: 32px; color: #2e7d32; margin-bottom: 8px; }
.success-text { margin-bottom: 12px; font-size: 15px; }
.success-text strong { font-size: 16px; }
.success-detail { font-size: 13px; color: #555; margin-top: 4px; }
.pm-tag {
  display: inline-block; padding: 2px 8px; background: #fff; border-radius: 3px;
  margin-right: 6px; font-size: 12px; border: 1px solid #ddd;
}
.success-actions { display: flex; gap: 10px; }
.btn-print { background: #ff9800; color: #fff; border-color: #ff9800; font-size: 13px; padding: 8px 16px; }
.btn-print:hover { background: #f57c00; }
.btn-new { background: #1976d2; color: #fff; border-color: #1976d2; font-size: 13px; padding: 8px 16px; }
.btn-new:hover { background: #1565c0; }

.loading-overlay {
  position: absolute; inset: 0; background: rgba(255,255,255,0.7);
  display: flex; align-items: center; justify-content: center;
}
.spinner {
  width: 32px; height: 32px; border: 3px solid #e0e0e0; border-top-color: #1976d2;
  border-radius: 50%; animation: spin 0.6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Receipt (hidden on screen, visible on print) */
.receipt-template { display: none; }
</style>
