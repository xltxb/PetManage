<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const router = useRouter()
const auth = useAuthStore()

interface SettingsForm {
  name: string
  address: string
  contact_phone: string
  contact_email: string
  business_hours: string
  notice: string
}

const form = ref<SettingsForm>({
  name: '',
  address: '',
  contact_phone: '',
  contact_email: '',
  business_hours: '',
  notice: '',
})

const logoUrl = ref('')
const logoFile = ref<File | null>(null)
const logoPreview = ref('')
const loading = ref(true)
const saving = ref(false)
const uploadingLogo = ref(false)
const error = ref('')
const successMsg = ref('')

if (!auth.user) {
  router.replace('/merchant/login')
}

async function loadSettings() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getShopSettings()
    form.value = {
      name: data.name || '',
      address: data.address || '',
      contact_phone: data.contact_phone || '',
      contact_email: data.contact_email || '',
      business_hours: data.business_hours || '',
      notice: data.notice || '',
    }
    logoUrl.value = data.logo_url || ''
    if (logoUrl.value) {
      logoPreview.value = logoUrl.value
    }
  } catch (e: any) {
    error.value = e.message || '加载失败'
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
  try {
    const data = await api.uploadShopLogo(logoFile.value)
    logoUrl.value = data.logo_url
    logoFile.value = null
    successMsg.value = 'Logo 上传成功'
    setTimeout(() => { successMsg.value = '' }, 3000)
  } catch (e: any) {
    error.value = e.message || 'Logo 上传失败'
  } finally {
    uploadingLogo.value = false
  }
}

async function saveSettings() {
  if (!form.value.name.trim()) {
    error.value = '店铺名称不能为空'
    return
  }
  saving.value = true
  error.value = ''
  successMsg.value = ''
  try {
    const data = await api.updateShopSettings({
      name: form.value.name.trim(),
      address: form.value.address.trim(),
      contact_phone: form.value.contact_phone.trim(),
      contact_email: form.value.contact_email.trim(),
      business_hours: form.value.business_hours.trim(),
      notice: form.value.notice.trim(),
    })
    // Update auth store merchant name
    if (auth.user) {
      auth.user.merchant_name = data.name
      localStorage.setItem('auth_user', JSON.stringify(auth.user))
    }
    successMsg.value = '保存成功'
    setTimeout(() => { successMsg.value = '' }, 3000)
  } catch (e: any) {
    error.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}

function goBack() {
  router.push('/merchant')
}

onMounted(loadSettings)
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-4xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-3">
          <button
            @click="goBack"
            class="text-gray-500 hover:text-gray-700 cursor-pointer"
          >
            &larr; 返回
          </button>
          <h1 class="text-lg font-semibold text-gray-800">店铺设置</h1>
        </div>
        <span class="text-sm text-gray-600">
          {{ auth.user?.merchant_name || '我的店铺' }}
        </span>
      </div>
    </header>

    <!-- Content -->
    <main class="max-w-4xl mx-auto px-4 py-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <div class="animate-spin h-8 w-8 border-4 border-blue-500 border-t-transparent rounded-full"></div>
        <span class="ml-3 text-gray-500">加载中...</span>
      </div>

      <!-- Error -->
      <div v-else-if="error && !form.name" class="bg-red-50 border border-red-200 rounded-lg p-6 text-center">
        <p class="text-red-600">{{ error }}</p>
        <button @click="loadSettings" class="mt-3 text-sm text-blue-600 hover:text-blue-800 cursor-pointer">
          重新加载
        </button>
      </div>

      <!-- Form -->
      <div v-else class="space-y-6">
        <!-- Success Message -->
        <div v-if="successMsg" class="bg-green-50 border border-green-200 text-green-700 rounded-lg px-4 py-3 text-sm">
          {{ successMsg }}
        </div>

        <!-- Error Message -->
        <div v-if="error" class="bg-red-50 border border-red-200 text-red-600 rounded-lg px-4 py-3 text-sm flex justify-between items-center">
          <span>{{ error }}</span>
          <button @click="error = ''" class="text-red-400 hover:text-red-600 cursor-pointer">&times;</button>
        </div>

        <!-- Logo Section -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h2 class="text-base font-semibold text-gray-800 mb-4">店铺 Logo</h2>
          <div class="flex items-start gap-6">
            <div class="w-24 h-24 rounded-lg border-2 border-dashed border-gray-300 flex items-center justify-center overflow-hidden bg-gray-50">
              <img v-if="logoPreview" :src="logoPreview" alt="Logo" class="w-full h-full object-contain" />
              <span v-else class="text-gray-400 text-sm">暂无 Logo</span>
            </div>
            <div class="flex-1">
              <label class="block mb-2">
                <input
                  type="file"
                  accept="image/*"
                  @change="onLogoChange"
                  class="block w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100 cursor-pointer"
                />
              </label>
              <p class="text-xs text-gray-400 mt-1">支持 JPG、PNG、SVG 格式，建议尺寸 200x200</p>
              <button
                v-if="logoFile"
                @click="uploadLogo"
                :disabled="uploadingLogo"
                class="mt-3 px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50 cursor-pointer"
              >
                {{ uploadingLogo ? '上传中...' : '上传 Logo' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Basic Info Form -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h2 class="text-base font-semibold text-gray-800 mb-4">基本信息</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                店铺名称 <span class="text-red-500">*</span>
              </label>
              <input
                v-model="form.name"
                type="text"
                placeholder="请输入店铺名称"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">联系电话</label>
                <input
                  v-model="form.contact_phone"
                  type="text"
                  placeholder="如 010-88888888"
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">联系邮箱</label>
                <input
                  v-model="form.contact_email"
                  type="email"
                  placeholder="如 contact@shop.com"
                  class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                />
              </div>
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">店铺地址</label>
              <input
                v-model="form.address"
                type="text"
                placeholder="请输入店铺详细地址"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">营业时间</label>
              <input
                v-model="form.business_hours"
                type="text"
                placeholder="如 09:00-21:00"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">门店公告</label>
              <textarea
                v-model="form.notice"
                rows="3"
                placeholder="请输入门店公告内容，将展示在首页"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
              ></textarea>
            </div>
          </div>
        </div>

        <!-- Save Button -->
        <div class="flex justify-end">
          <button
            @click="saveSettings"
            :disabled="saving"
            class="px-6 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 cursor-pointer"
          >
            {{ saving ? '保存中...' : '保存设置' }}
          </button>
        </div>
      </div>
    </main>
  </div>
</template>
