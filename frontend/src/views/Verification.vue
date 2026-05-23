<template>
  <div class="verification-page">
    <h1 class="page-title">券码核销</h1>

    <!-- Code Input -->
    <div class="scan-section">
      <input
        ref="codeInput"
        v-model="code"
        class="code-input"
        placeholder="扫描或输入券码..."
        @keyup.enter="verifyCode"
        autofocus
      />
      <button class="btn-verify" @click="verifyCode" :disabled="verifying || !code.trim()">
        核销
      </button>
    </div>

    <!-- Tab Switcher -->
    <div class="tab-row">
      <button
        :class="['tab-btn', { active: activeTab === 'coupon' }]"
        @click="activeTab = 'coupon'"
      >优惠券</button>
      <button
        :class="['tab-btn', { active: activeTab === 'third_party' }]"
        @click="activeTab = 'third_party'"
      >团购券</button>
      <button
        :class="['tab-btn', { active: activeTab === 'service_card' }]"
        @click="activeTab = 'service_card'"
      >服务次卡</button>
    </div>

    <!-- Verification Result -->
    <div v-if="lastResult" :class="['result-card', lastResult.error ? 'result-error' : 'result-success']">
      <div class="result-icon">{{ lastResult.error ? '✗' : '✓' }}</div>
      <div class="result-body">
        <div class="result-title">{{ lastResult.error ? '核销失败' : '核销成功' }}</div>
        <div class="result-message">{{ lastResult.error ? lastResult.error : lastResult.message }}</div>
        <div class="result-details" v-if="!lastResult.error && lastResult.details">
          <div v-for="(v, k) in lastResult.details" :key="k" class="detail-row">
            <span class="detail-label">{{ k }}</span>
            <span class="detail-value">{{ v }}</span>
          </div>
        </div>
      </div>
      <button class="btn-close" @click="lastResult = null">&times;</button>
    </div>

    <!-- Verification Records -->
    <div class="records-section">
      <h2 class="section-title">
        核销记录
        <span class="record-count" v-if="recordsTotal > 0">({{ recordsTotal }})</span>
      </h2>

      <div class="records-filter">
        <select v-model="filterType" @change="loadRecords">
          <option value="">全部类型</option>
          <option value="coupon">优惠券</option>
          <option value="service_card">服务次卡</option>
          <option value="third_party_voucher">团购券</option>
        </select>
      </div>

      <div class="records-table-wrap" v-if="records.length > 0">
        <table class="records-table">
          <thead>
            <tr>
              <th>券码</th>
              <th>类型</th>
              <th>结果</th>
              <th>详情</th>
              <th>核销时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in records" :key="r.id" :class="r.result === 'success' ? 'row-ok' : 'row-fail'">
              <td class="code-cell">{{ r.code }}</td>
              <td>
                <span class="type-tag">{{ typeLabel(r.verification_type) }}</span>
              </td>
              <td>
                <span :class="['status-tag', r.result === 'success' ? 'tag-success' : 'tag-fail']">
                  {{ r.result === 'success' ? '成功' : '失败' }}
                </span>
              </td>
              <td class="detail-cell">{{ r.detail }}</td>
              <td class="time-cell">{{ formatTime(r.verified_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="records-empty">
        暂无核销记录
      </div>

      <div class="pagination" v-if="recordsTotal > pageSize">
        <button :disabled="page <= 1" @click="changePage(-1)">上一页</button>
        <span>第 {{ page }} / {{ totalPages }} 页</span>
        <button :disabled="page >= totalPages" @click="changePage(1)">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { api } from '@/api/client'

const code = ref('')
const codeInput = ref<HTMLInputElement | null>(null)
const activeTab = ref('coupon')
const verifying = ref(false)
const lastResult = ref<any>(null)

// Records state
const records = ref<any[]>([])
const recordsTotal = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filterType = ref('')

const totalPages = computed(() => Math.max(1, Math.ceil(recordsTotal.value / pageSize.value)))

function typeLabel(t: string): string {
  const map: Record<string, string> = {
    coupon: '优惠券', service_card: '服务次卡', third_party_voucher: '团购券',
  }
  return map[t] || t
}

function formatTime(t: string): string {
  if (!t) return '-'
  try {
    return new Date(t).toLocaleString('zh-CN')
  } catch { return t }
}

async function verifyCode() {
  const val = code.value.trim()
  if (!val || verifying.value) return

  verifying.value = true
  lastResult.value = null

  try {
    let result: any
    const tab = activeTab.value
    if (tab === 'coupon') {
      result = await api.verifyCoupon(val)
      lastResult.value = {
        error: false,
        message: `优惠券 ${result.code} 核销成功`,
        details: {
          '抵扣金额': `¥${(result.value_cents / 100).toFixed(2)}`,
          '券类型': result.discount_type === 'fixed' ? '固定金额' : '百分比',
          '核销状态': '已使用',
        },
      }
    } else if (tab === 'third_party') {
      result = await api.verifyThirdPartyVoucher(val)
      lastResult.value = {
        error: false,
        message: `团购券 ${result.code} 核销成功`,
        details: {
          '来源': result.source,
          '券名称': result.name,
          '面值': `¥${(result.amount_cents / 100).toFixed(2)}`,
          '核销状态': '已验证',
        },
      }
    } else {
      result = await api.verifyServiceCard(val)
      lastResult.value = {
        error: false,
        message: `服务次卡 ${result.code} 核销成功`,
        details: {
          '卡名称': result.name,
          '服务项目': result.service_name || '-',
          '总次数': result.total_uses,
          '已用次数': result.used_count,
          '剩余次数': result.remaining_uses,
          '状态': result.remaining_uses > 0 ? '有效' : '已用完',
        },
      }
    }
    code.value = ''
    await loadRecords()
  } catch (e: any) {
    lastResult.value = { error: true, error: e.message || '核销失败' }
  } finally {
    verifying.value = false
    nextTick(() => codeInput.value?.focus())
  }
}

async function loadRecords() {
  try {
    const params: any = { page: page.value, page_size: pageSize.value }
    if (filterType.value) params.type = filterType.value
    const result = await api.getVerificationRecords(params)
    records.value = result.records || []
    recordsTotal.value = result.total || 0
  } catch (e) {
    console.error('Failed to load records:', e)
  }
}

function changePage(delta: number) {
  const newPage = page.value + delta
  if (newPage < 1 || newPage > totalPages.value) return
  page.value = newPage
  loadRecords()
}

onMounted(() => {
  loadRecords()
})
</script>

<style scoped>
.verification-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 20px 0;
  color: #333;
}

/* Scan Section */
.scan-section {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.code-input {
  flex: 1;
  padding: 12px 16px;
  border: 2px solid #4caf50;
  border-radius: 8px;
  font-size: 16px;
  outline: none;
}
.code-input:focus {
  border-color: #2e7d32;
  box-shadow: 0 0 0 3px rgba(76,175,80,0.15);
}

.btn-verify {
  padding: 12px 32px;
  background: #43a047;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}
.btn-verify:hover:not(:disabled) { opacity: 0.9; }
.btn-verify:disabled { background: #ccc; cursor: not-allowed; }

/* Tab Row */
.tab-row {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}
.tab-btn {
  padding: 8px 24px;
  border: 1px solid #ddd;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.15s;
}
.tab-btn.active {
  background: #1976d2;
  color: #fff;
  border-color: #1976d2;
}
.tab-btn:hover:not(.active) {
  background: #f5f5f5;
}

/* Result Card */
.result-card {
  display: flex;
  align-items: flex-start;
  padding: 16px 20px;
  border-radius: 8px;
  margin-bottom: 20px;
  position: relative;
}
.result-success {
  background: #e8f5e9;
  border: 1px solid #a5d6a7;
}
.result-error {
  background: #ffebee;
  border: 1px solid #ef9a9a;
}

.result-icon {
  font-size: 24px;
  margin-right: 14px;
  margin-top: 2px;
}
.result-success .result-icon { color: #2e7d32; }
.result-error .result-icon { color: #c62828; }

.result-body { flex: 1; }

.result-title {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 4px;
}
.result-success .result-title { color: #2e7d32; }
.result-error .result-title { color: #c62828; }

.result-message {
  font-size: 14px;
  color: #555;
  margin-bottom: 8px;
}

.result-details {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.detail-row {
  display: flex;
  gap: 6px;
  font-size: 13px;
}
.detail-label { color: #888; }
.detail-value { font-weight: 600; color: #333; }

.btn-close {
  position: absolute;
  top: 8px;
  right: 12px;
  border: none;
  background: none;
  font-size: 20px;
  color: #999;
  cursor: pointer;
}
.btn-close:hover { color: #333; }

/* Records Section */
.records-section {
  margin-top: 8px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 12px 0;
  color: #333;
}

.record-count { color: #888; font-weight: 400; }

.records-filter {
  margin-bottom: 12px;
}
.records-filter select {
  padding: 6px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 13px;
}

.records-table-wrap {
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  overflow-x: auto;
}

.records-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.records-table th {
  background: #f5f5f5;
  padding: 10px 12px;
  text-align: left;
  font-weight: 600;
  color: #555;
  border-bottom: 1px solid #e0e0e0;
}
.records-table td {
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
}

.code-cell { font-family: monospace; font-size: 12px; }
.detail-cell { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.time-cell { white-space: nowrap; font-size: 12px; color: #888; }

.type-tag {
  display: inline-block;
  padding: 2px 8px;
  background: #e3f2fd;
  color: #1565c0;
  border-radius: 3px;
  font-size: 12px;
}

.status-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 12px;
  font-weight: 600;
}
.tag-success { background: #e8f5e9; color: #2e7d32; }
.tag-fail { background: #ffebee; color: #c62828; }

.records-empty {
  text-align: center;
  padding: 40px;
  color: #bbb;
  font-size: 14px;
}

/* Pagination */
.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  margin-top: 16px;
  font-size: 13px;
  color: #555;
}
.pagination button {
  padding: 6px 16px;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: #fff;
  cursor: pointer;
}
.pagination button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.pagination button:hover:not(:disabled) {
  background: #f5f5f5;
}
</style>
