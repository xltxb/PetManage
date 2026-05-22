<template>
  <div class="member-detail-container">
    <div class="page-header">
      <router-link to="/merchant/members" class="back-link">&larr; 返回会员列表</router-link>
      <h2>会员详情</h2>
    </div>

    <div v-if="loading" class="loading">加载中...</div>
    <div v-else-if="error" class="error">{{ error }}</div>

    <div v-else-if="member" class="content">
      <!-- Member info -->
      <div class="card">
        <h3>基础信息</h3>
        <div class="info-grid">
          <div class="info-item"><span class="label">卡号</span><span>{{ member.card_no }}</span></div>
          <div class="info-item"><span class="label">姓名</span><span>{{ member.name }}</span></div>
          <div class="info-item"><span class="label">手机号</span><span>{{ member.phone }}</span></div>
          <div class="info-item"><span class="label">微信</span><span>{{ member.wechat || '-' }}</span></div>
          <div class="info-item"><span class="label">性别</span><span>{{ genderLabel(member.gender) }}</span></div>
          <div class="info-item"><span class="label">生日</span><span>{{ member.birthday || '-' }}</span></div>
          <div class="info-item"><span class="label">地址</span><span>{{ member.address || '-' }}</span></div>
          <div class="info-item"><span class="label">备注</span><span>{{ member.remark || '-' }}</span></div>
          <div class="info-item"><span class="label">状态</span>
            <span :class="['status-badge', member.status]">{{ member.status === 'active' ? '活跃' : '已禁用' }}</span>
          </div>
          <div class="info-item"><span class="label">储值余额</span><span>&yen;{{ (member.balance_cents / 100).toFixed(2) }}</span></div>
          <div class="info-item"><span class="label">积分</span><span>{{ member.points }}</span></div>
        </div>
      </div>

      <!-- QR Code -->
      <div class="card qr-section">
        <h3>会员二维码</h3>
        <p class="qr-hint">扫描二维码可快速识别会员身份</p>
        <div class="qr-container">
          <img
            v-if="qrLoaded"
            :src="qrDataUrl"
            alt="会员二维码"
            class="qr-image"
          />
          <div v-else class="qr-loading">二维码加载中...</div>
        </div>
        <button class="btn-download" @click="downloadQR" :disabled="downloading">
          {{ downloading ? '下载中...' : '下载二维码 (打印会员卡)' }}
        </button>
      </div>

      <!-- Consumption records -->
      <div class="card">
        <h3>消费记录 ({{ detail.total_orders }} 笔)</h3>
        <table v-if="detail.consumption_records && detail.consumption_records.length > 0" class="data-table">
          <thead>
            <tr>
              <th>订单号</th>
              <th>金额</th>
              <th>已付</th>
              <th>状态</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="cr in detail.consumption_records" :key="cr.order_id">
              <td>#{{ cr.order_id }}</td>
              <td>&yen;{{ (cr.total_cents / 100).toFixed(2) }}</td>
              <td>&yen;{{ (cr.paid_cents / 100).toFixed(2) }}</td>
              <td>{{ cr.status }}</td>
              <td>{{ formatDate(cr.created_at) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="empty">暂无消费记录</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '@/api/client'

const route = useRoute()
const memberId = Number(route.params.id)

const member = ref<any>(null)
const detail = ref<any>({ consumption_records: [], total_orders: 0 })
const loading = ref(true)
const error = ref('')

const qrDataUrl = ref('')
const qrLoaded = ref(false)
const downloading = ref(false)

function genderLabel(g: string) {
  if (g === 'M') return '男'
  if (g === 'F') return '女'
  if (g === 'O') return '其他'
  return ''
}

function formatDate(s: string) {
  if (!s) return ''
  return new Date(s).toLocaleDateString('zh-CN')
}

async function loadQRCode() {
  try {
    const { blob } = await api.getMemberQRCodeBlob(memberId)
    qrDataUrl.value = URL.createObjectURL(blob)
    qrLoaded.value = true
  } catch {
    // QR code load failed — non-critical
  }
}

async function downloadQR() {
  downloading.value = true
  try {
    const { blob, cardNo } = await api.getMemberQRCodeBlob(memberId)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `member_${cardNo}_qrcode.png`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  } finally {
    downloading.value = false
  }
}

onMounted(async () => {
  try {
    const data = await api.getMember(memberId)
    member.value = data.member
    detail.value = data
    loadQRCode()
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.member-detail-container { padding: 20px; max-width: 800px; margin: 0 auto; }
.page-header { margin-bottom: 20px; }
.back-link { color: #3b82f6; text-decoration: none; font-size: 14px; display: inline-block; margin-bottom: 8px; }
.page-header h2 { margin: 0; }
.loading { text-align: center; padding: 60px; color: #6b7280; }
.error { text-align: center; padding: 60px; color: #ef4444; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; padding: 20px; margin-bottom: 16px; }
.card h3 { margin: 0 0 12px; font-size: 16px; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.info-item { display: flex; flex-direction: column; }
.info-item .label { font-size: 12px; color: #6b7280; }
.info-item span:last-child { font-size: 15px; color: #111827; margin-top: 2px; }
.status-badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 12px; }
.status-badge.active { background: #d1fae5; color: #065f46; }
.status-badge.inactive { background: #fee2e2; color: #991b1b; }

.qr-section { text-align: center; }
.qr-hint { color: #6b7280; font-size: 13px; margin-bottom: 12px; }
.qr-container { display: inline-block; border: 2px solid #e5e7eb; border-radius: 10px; padding: 12px; background: #fff; }
.qr-image { width: 200px; height: 200px; display: block; }
.qr-loading { width: 200px; height: 200px; display: flex; align-items: center; justify-content: center; color: #9ca3af; }
.btn-download { margin-top: 12px; padding: 8px 20px; background: #10b981; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; }
.btn-download:disabled { opacity: 0.6; cursor: not-allowed; }

.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { padding: 8px 10px; text-align: left; border-bottom: 1px solid #e5e7eb; font-size: 14px; }
.data-table th { background: #f9fafb; font-weight: 600; color: #374151; }
.empty { text-align: center; padding: 20px; color: #9ca3af; font-size: 14px; }
</style>
