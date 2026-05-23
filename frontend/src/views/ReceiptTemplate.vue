<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '@/api/client'

interface TemplateForm {
  logo_url: string
  store_name: string
  contact_phone: string
  contact_address: string
  footer_note: string
  paper_width: string
  show_qrcode: boolean
}

const form = ref<TemplateForm>({
  logo_url: '',
  store_name: '',
  contact_phone: '',
  contact_address: '',
  footer_note: '',
  paper_width: '80mm',
  show_qrcode: false,
})

const logoFile = ref<File | null>(null)
const logoPreview = ref('')
const loading = ref(true)
const saving = ref(false)
const uploadingLogo = ref(false)
const error = ref('')
const successMsg = ref('')
const showPreview = ref(false)

async function loadTemplate() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getReceiptTemplate()
    form.value = {
      logo_url: data.logo_url || '',
      store_name: data.store_name || '',
      contact_phone: data.contact_phone || '',
      contact_address: data.contact_address || '',
      footer_note: data.footer_note || '',
      paper_width: data.paper_width || '80mm',
      show_qrcode: data.show_qrcode || false,
    }
    if (form.value.logo_url) {
      logoPreview.value = form.value.logo_url
    }
  } catch (e: any) {
    error.value = e.message || '加载模板失败'
  } finally {
    loading.value = false
  }
}

function onLogoChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    logoFile.value = input.files[0]
    logoPreview.value = URL.createObjectURL(input.files[0])
  }
}

async function uploadLogo() {
  if (!logoFile.value) return
  uploadingLogo.value = true
  error.value = ''
  try {
    const data = await api.uploadReceiptLogo(logoFile.value)
    form.value.logo_url = data.logo_url
    logoFile.value = null
    successMsg.value = 'Logo 上传成功'
    setTimeout(() => { successMsg.value = '' }, 3000)
  } catch (e: any) {
    error.value = e.message || 'Logo 上传失败'
  } finally {
    uploadingLogo.value = false
  }
}

async function saveTemplate() {
  saving.value = true
  error.value = ''
  successMsg.value = ''
  try {
    await api.updateReceiptTemplate({
      logo_url: form.value.logo_url,
      store_name: form.value.store_name.trim(),
      contact_phone: form.value.contact_phone.trim(),
      contact_address: form.value.contact_address.trim(),
      footer_note: form.value.footer_note.trim(),
      paper_width: form.value.paper_width,
      show_qrcode: form.value.show_qrcode,
    })
    successMsg.value = '模板保存成功'
    setTimeout(() => { successMsg.value = '' }, 3000)
  } catch (e: any) {
    error.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

// Preview sample data
const previewItems = computed(() => [
  { name: '皇家狗粮 1.5kg', quantity: 1, price_cents: 12800, total_cents: 12800 },
  { name: '宠物洗澡服务', quantity: 1, price_cents: 8800, total_cents: 8800 },
  { name: '狗咬胶', quantity: 2, price_cents: 1500, total_cents: 3000 },
])

const previewTotal = computed(() => previewItems.value.reduce((s, i) => s + i.total_cents, 0))
onMounted(() => loadTemplate())
</script>

<template>
  <div class="receipt-template-page">
    <h1 class="page-title">小票模板设置</h1>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="template-layout">
      <!-- Editor Panel -->
      <div class="editor-panel">
        <!-- Logo Upload -->
        <div class="form-group">
          <label class="form-label">小票 Logo</label>
          <div class="logo-upload">
            <div v-if="logoPreview" class="logo-preview">
              <img :src="logoPreview" alt="Logo" />
            </div>
            <div class="logo-actions">
              <input
                type="file"
                accept="image/*"
                @change="onLogoChange"
                class="file-input"
                id="logo-input"
              />
              <label for="logo-input" class="btn btn-upload">选择图片</label>
              <button
                v-if="logoFile"
                class="btn btn-primary"
                @click="uploadLogo"
                :disabled="uploadingLogo"
              >
                {{ uploadingLogo ? '上传中...' : '确认上传' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Store Name -->
        <div class="form-group">
          <label class="form-label">门店名称</label>
          <input
            v-model="form.store_name"
            class="form-input"
            placeholder="如：XX宠物生活馆"
          />
        </div>

        <!-- Contact Phone -->
        <div class="form-group">
          <label class="form-label">联系电话</label>
          <input
            v-model="form.contact_phone"
            class="form-input"
            placeholder="如：010-88886666"
          />
        </div>

        <!-- Contact Address -->
        <div class="form-group">
          <label class="form-label">门店地址</label>
          <input
            v-model="form.contact_address"
            class="form-input"
            placeholder="如：北京市朝阳区XX路XX号"
          />
        </div>

        <!-- Footer Note -->
        <div class="form-group">
          <label class="form-label">底部备注</label>
          <textarea
            v-model="form.footer_note"
            class="form-textarea"
            rows="3"
            placeholder="如：凭小票7天内可退换、感谢您的惠顾"
          ></textarea>
        </div>

        <!-- Paper Width -->
        <div class="form-group">
          <label class="form-label">打印纸宽度</label>
          <select v-model="form.paper_width" class="form-input">
            <option value="58mm">58mm (小票机)</option>
            <option value="80mm">80mm (厨房打印机)</option>
          </select>
        </div>

        <!-- Show QR Code -->
        <div class="form-group">
          <label class="form-label">
            <input type="checkbox" v-model="form.show_qrcode" />
            显示公众号/小程序二维码
          </label>
        </div>

        <!-- Buttons -->
        <div class="form-actions">
          <button class="btn btn-secondary" @click="showPreview = !showPreview">
            {{ showPreview ? '隐藏预览' : '预览小票' }}
          </button>
          <button class="btn btn-primary" @click="saveTemplate" :disabled="saving">
            {{ saving ? '保存中...' : '保存模板' }}
          </button>
        </div>

        <div v-if="error" class="msg error">{{ error }}</div>
        <div v-if="successMsg" class="msg success">{{ successMsg }}</div>
      </div>

      <!-- Preview Panel -->
      <div class="preview-panel" v-if="showPreview">
        <h2 class="preview-title">小票预览</h2>
        <div class="receipt-preview" :style="{ maxWidth: form.paper_width === '58mm' ? '220px' : '300px' }">
          <!-- Header -->
          <div class="r-header">
            <img v-if="form.logo_url" :src="form.logo_url" class="r-logo" alt="Logo" />
            <h3 class="r-store-name">{{ form.store_name || '门店名称' }}</h3>
            <p class="r-contact" v-if="form.contact_phone">{{ form.contact_phone }}</p>
            <p class="r-contact" v-if="form.contact_address">{{ form.contact_address }}</p>
            <p class="r-meta">订单号: SAMPLE-001</p>
            <p class="r-meta">{{ new Date().toLocaleString('zh-CN') }}</p>
          </div>
          <hr class="r-divider" />

          <!-- Items -->
          <div class="r-items">
            <div v-for="(item, idx) in previewItems" :key="idx" class="r-item">
              <span class="r-item-name">{{ item.name }}</span>
              <span class="r-item-qty">x{{ item.quantity }}</span>
              <span class="r-item-price">¥{{ (item.total_cents / 100).toFixed(2) }}</span>
            </div>
          </div>
          <hr class="r-divider" />

          <!-- Totals -->
          <div class="r-totals">
            <div class="r-total-row">
              <span>合计</span>
              <strong>¥{{ (previewTotal / 100).toFixed(2) }}</strong>
            </div>
            <div class="r-total-row">
              <span>现金</span>
              <span>¥{{ (previewTotal / 100).toFixed(2) }}</span>
            </div>
            <div class="r-total-row">
              <span>找零</span>
              <span>¥0.00</span>
            </div>
          </div>
          <hr class="r-divider" />

          <!-- Footer -->
          <div class="r-footer">
            <p v-if="form.footer_note">{{ form.footer_note }}</p>
            <p v-else>感谢惠顾，欢迎再次光临！</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.receipt-template-page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 20px 0;
  color: #333;
}

.loading {
  text-align: center;
  padding: 40px;
  color: #999;
}

.template-layout {
  display: flex;
  gap: 32px;
}

.editor-panel {
  flex: 1;
  max-width: 480px;
}

.preview-panel {
  flex: 1;
  position: sticky;
  top: 24px;
  align-self: flex-start;
}

.preview-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 16px 0;
  color: #666;
}

/* Form */
.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: #555;
  margin-bottom: 6px;
}

.form-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
  box-sizing: border-box;
}

.form-input:focus {
  border-color: #1976d2;
  box-shadow: 0 0 0 3px rgba(25, 118, 210, 0.1);
}

.form-textarea {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 14px;
  outline: none;
  resize: vertical;
  box-sizing: border-box;
}

.form-textarea:focus {
  border-color: #1976d2;
  box-shadow: 0 0 0 3px rgba(25, 118, 210, 0.1);
}

.logo-upload {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.logo-preview {
  width: 100px;
  height: 100px;
  border: 1px dashed #ddd;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #fafafa;
}

.logo-preview img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.logo-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.file-input {
  display: none;
}

.form-actions {
  display: flex;
  gap: 10px;
  margin-top: 20px;
}

.btn {
  padding: 8px 18px;
  border: 1px solid #ccc;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
}

.btn-primary {
  background: #1976d2;
  color: #fff;
  border-color: #1976d2;
}

.btn-primary:hover { background: #1565c0; }
.btn-primary:disabled { background: #90caf9; cursor: not-allowed; }

.btn-secondary {
  background: #fff;
  color: #555;
}

.btn-secondary:hover { background: #f5f5f5; }

.btn-upload {
  background: #ff9800;
  color: #fff;
  border-color: #ff9800;
  cursor: pointer;
  display: inline-block;
}

.btn-upload:hover { background: #f57c00; }

.msg {
  margin-top: 12px;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 13px;
}

.msg.error {
  background: #ffebee;
  color: #c62828;
}

.msg.success {
  background: #e8f5e9;
  color: #2e7d32;
}

/* Receipt Preview */
.receipt-preview {
  background: #fff;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
  font-family: 'Courier New', monospace;
  font-size: 12px;
}

.r-header {
  text-align: center;
}

.r-logo {
  max-width: 80px;
  max-height: 80px;
  margin: 0 auto 8px;
  display: block;
}

.r-store-name {
  font-size: 15px;
  font-weight: bold;
  margin: 0 0 4px 0;
}

.r-contact {
  margin: 2px 0;
  font-size: 11px;
  color: #666;
}

.r-meta {
  margin: 2px 0;
  font-size: 11px;
  color: #888;
}

.r-divider {
  border: none;
  border-top: 1px dashed #ccc;
  margin: 10px 0;
}

.r-items {
  margin: 8px 0;
}

.r-item {
  display: flex;
  align-items: baseline;
  margin: 3px 0;
}

.r-item-name {
  flex: 1;
}

.r-item-qty {
  margin: 0 6px;
  color: #888;
}

.r-item-price {
  text-align: right;
  min-width: 60px;
}

.r-totals {
  margin: 8px 0;
}

.r-total-row {
  display: flex;
  justify-content: space-between;
  margin: 3px 0;
  font-size: 13px;
}

.r-total-row strong {
  font-size: 14px;
}

.r-footer {
  text-align: center;
  font-size: 11px;
  color: #666;
  margin-top: 8px;
}

.r-footer p {
  margin: 3px 0;
}
</style>
