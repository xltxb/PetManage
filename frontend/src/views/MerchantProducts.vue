<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const router = useRouter()
const auth = useAuthStore()

interface Product {
  id: number
  barcode: string
  name: string
  brand: string
  specification: string
  price_cents: number
  cost_cents: number
  stock: number
  alert_stock: number
  expiry_date: string | null
  category_id: number | null
  status: string
  created_at: string
  updated_at: string
}

const products = ref<Product[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(true)
const error = ref('')
const searchKeyword = ref('')

// Dialog state
const showDialog = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingProduct = ref<Product | null>(null)
const formData = ref({
  barcode: '',
  name: '',
  brand: '',
  specification: '',
  price_cents: 0,
  cost_cents: 0,
  stock: 0,
  alert_stock: 0,
  expiry_date: '',
  category_id: null as number | null,
})
const formError = ref('')
const saving = ref(false)

// Delete state
const showDeleteDialog = ref(false)
const deletingProduct = ref<Product | null>(null)
const deleteError = ref('')
const deleting = ref(false)

if (!auth.user) {
  router.replace('/merchant/login')
}

async function loadProducts() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getProducts({
      keyword: searchKeyword.value || undefined,
      page: page.value,
      page_size: pageSize.value,
    })
    products.value = data.products || []
    total.value = data.total || 0
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  dialogMode.value = 'create'
  editingProduct.value = null
  formData.value = {
    barcode: '',
    name: '',
    brand: '',
    specification: '',
    price_cents: 0,
    cost_cents: 0,
    stock: 0,
    alert_stock: 0,
    expiry_date: '',
    category_id: null,
  }
  formError.value = ''
  showDialog.value = true
}

function openEditDialog(product: Product) {
  dialogMode.value = 'edit'
  editingProduct.value = product
  formData.value = {
    barcode: product.barcode,
    name: product.name,
    brand: product.brand,
    specification: product.specification,
    price_cents: product.price_cents,
    cost_cents: product.cost_cents,
    stock: product.stock,
    alert_stock: product.alert_stock,
    expiry_date: product.expiry_date ? product.expiry_date.slice(0, 10) : '',
    category_id: product.category_id,
  }
  formError.value = ''
  showDialog.value = true
}

async function handleSave() {
  formError.value = ''
  if (!formData.value.name.trim()) {
    formError.value = '商品名称不能为空'
    return
  }
  if (formData.value.price_cents < 0) {
    formError.value = '售价不能为负数'
    return
  }

  saving.value = true
  try {
    const payload = {
      ...formData.value,
      name: formData.value.name.trim(),
      barcode: formData.value.barcode.trim(),
    }
    if (dialogMode.value === 'create') {
      await api.createProduct(payload)
    } else {
      await api.updateProduct(editingProduct.value!.id, payload)
    }
    showDialog.value = false
    await loadProducts()
  } catch (e: any) {
    formError.value = e.message || '操作失败'
  } finally {
    saving.value = false
  }
}

function openDeleteDialog(product: Product) {
  deletingProduct.value = product
  deleteError.value = ''
  showDeleteDialog.value = true
}

async function handleDelete() {
  if (!deletingProduct.value) return
  deleteError.value = ''
  deleting.value = true
  try {
    await api.deleteProduct(deletingProduct.value.id)
    showDeleteDialog.value = false
    deletingProduct.value = null
    await loadProducts()
  } catch (e: any) {
    deleteError.value = e.message || '删除失败'
  } finally {
    deleting.value = false
  }
}

async function handleToggleStatus(product: Product) {
  try {
    await api.toggleProductStatus(product.id)
    await loadProducts()
  } catch (e: any) {
    alert(e.message || '操作失败')
  }
}

function handleSearch() {
  page.value = 1
  loadProducts()
}

function formatPrice(cents: number) {
  return '¥' + (cents / 100).toFixed(2)
}

onMounted(() => {
  loadProducts()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b border-gray-200">
      <div class="max-w-6xl mx-auto px-4 py-4 flex items-center justify-between">
        <div>
          <button
            @click="router.push('/merchant')"
            class="text-sm text-blue-600 hover:text-blue-800 mb-1 cursor-pointer"
          >
            &larr; 返回首页
          </button>
          <h1 class="text-xl font-bold text-gray-800">商品管理</h1>
          <p class="text-sm text-gray-500 mt-0.5">维护商品基础信息，支持上架/下架操作</p>
        </div>
        <button
          class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
          @click="openCreateDialog()"
        >
          <span class="text-lg leading-none">+</span> 新建商品
        </button>
      </div>
    </header>

    <!-- Content -->
    <div class="max-w-6xl mx-auto px-4 py-6">
      <!-- Search bar -->
      <div class="mb-4 flex gap-2">
        <input
          v-model="searchKeyword"
          type="text"
          class="flex-1 max-w-sm px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          placeholder="搜索条码或名称..."
          @keyup.enter="handleSearch"
        />
        <button
          class="px-4 py-2 bg-gray-100 text-gray-600 text-sm rounded-lg hover:bg-gray-200"
          @click="handleSearch"
        >搜索</button>
        <button
          v-if="searchKeyword"
          class="px-3 py-2 text-sm text-gray-400 hover:text-gray-600"
          @click="searchKeyword = ''; handleSearch()"
        >清除</button>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="text-center py-20 text-gray-500">
        <div class="inline-block w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin mb-2"></div>
        <p class="text-sm">加载中...</p>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 text-sm">
        {{ error }}
      </div>

      <!-- Empty state -->
      <div v-else-if="products.length === 0" class="text-center py-20">
        <div class="text-5xl mb-4">📦</div>
        <p class="text-gray-500 text-sm mb-4">暂无商品，点击上方按钮创建第一个商品</p>
        <button
          class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700"
          @click="openCreateDialog()"
        >创建商品</button>
      </div>

      <!-- Table -->
      <div v-else class="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 text-gray-600 border-b border-gray-200">
            <tr>
              <th class="text-left px-4 py-3 font-medium">条码</th>
              <th class="text-left px-4 py-3 font-medium">名称</th>
              <th class="text-left px-4 py-3 font-medium">品牌</th>
              <th class="text-left px-4 py-3 font-medium">规格</th>
              <th class="text-right px-4 py-3 font-medium">进价</th>
              <th class="text-right px-4 py-3 font-medium">售价</th>
              <th class="text-right px-4 py-3 font-medium">库存</th>
              <th class="text-center px-4 py-3 font-medium">状态</th>
              <th class="text-center px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="product in products"
              :key="product.id"
              class="border-b border-gray-100 hover:bg-gray-50"
              :class="{ 'opacity-60': product.status === 'inactive' }"
            >
              <td class="px-4 py-3 text-gray-600 font-mono text-xs">{{ product.barcode || '-' }}</td>
              <td class="px-4 py-3 font-medium text-gray-800">{{ product.name }}</td>
              <td class="px-4 py-3 text-gray-600">{{ product.brand || '-' }}</td>
              <td class="px-4 py-3 text-gray-600">{{ product.specification || '-' }}</td>
              <td class="px-4 py-3 text-right text-gray-600">{{ formatPrice(product.cost_cents) }}</td>
              <td class="px-4 py-3 text-right text-gray-800 font-medium">{{ formatPrice(product.price_cents) }}</td>
              <td class="px-4 py-3 text-right">
                <span
                  class="font-medium"
                  :class="product.stock <= product.alert_stock && product.stock > 0 ? 'text-orange-500' : product.stock === 0 ? 'text-red-500' : 'text-gray-700'"
                >{{ product.stock }}</span>
              </td>
              <td class="px-4 py-3 text-center">
                <span
                  class="inline-block px-2 py-0.5 text-xs rounded-full font-medium"
                  :class="product.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'"
                >{{ product.status === 'active' ? '上架' : '下架' }}</span>
              </td>
              <td class="px-4 py-3 text-center">
                <div class="flex gap-1 justify-center">
                  <button
                    class="text-xs px-2 py-1 rounded hover:bg-blue-50"
                    :class="product.status === 'active' ? 'text-orange-500 hover:text-orange-700' : 'text-green-500 hover:text-green-700'"
                    @click="handleToggleStatus(product)"
                  >{{ product.status === 'active' ? '下架' : '上架' }}</button>
                  <button
                    class="text-xs text-blue-500 hover:text-blue-700 px-2 py-1 rounded hover:bg-blue-50"
                    @click="openEditDialog(product)"
                  >编辑</button>
                  <button
                    class="text-xs text-red-400 hover:text-red-600 px-2 py-1 rounded hover:bg-red-50"
                    @click="openDeleteDialog(product)"
                  >删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="total > pageSize" class="px-4 py-3 border-t border-gray-100 flex items-center justify-between text-sm text-gray-600">
          <span>共 {{ total }} 个商品</span>
          <div class="flex gap-1">
            <button
              class="px-3 py-1 rounded border border-gray-200 hover:bg-gray-50 disabled:opacity-30"
              :disabled="page <= 1"
              @click="page--; loadProducts()"
            >上一页</button>
            <span class="px-3 py-1">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
            <button
              class="px-3 py-1 rounded border border-gray-200 hover:bg-gray-50 disabled:opacity-30"
              :disabled="page >= Math.ceil(total / pageSize)"
              @click="page++; loadProducts()"
            >下一页</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create/Edit Dialog -->
    <div v-if="showDialog" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="showDialog = false">
      <div class="bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 p-6 max-h-[90vh] overflow-y-auto">
        <h2 class="text-lg font-bold text-gray-800 mb-4">
          {{ dialogMode === 'create' ? '新建商品' : '编辑商品' }}
        </h2>

        <div class="space-y-3">
          <!-- Name -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">商品名称 <span class="text-red-500">*</span></label>
            <input
              v-model="formData.name"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="请输入商品名称"
            />
          </div>

          <!-- Barcode -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">条码</label>
            <input
              v-model="formData.barcode"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="商品条码"
            />
          </div>

          <!-- Brand -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">品牌</label>
            <input
              v-model="formData.brand"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="品牌名称"
            />
          </div>

          <!-- Specification -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">规格</label>
            <input
              v-model="formData.specification"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="如：10kg/袋"
            />
          </div>

          <!-- Price / Cost -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">售价（元） <span class="text-red-500">*</span></label>
              <input
                v-model.number="formData.price_cents"
                type="number"
                min="0"
                step="0.01"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="0.00"
                @input="formData.price_cents = Math.round($event.target.value * 100) || 0"
                :value="formData.price_cents / 100"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">进价（元）</label>
              <input
                v-model="formData.cost_cents"
                type="number"
                min="0"
                step="0.01"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="0.00"
                :value="formData.cost_cents / 100"
                @input="formData.cost_cents = Math.round(($event.target as HTMLInputElement).valueAsNumber * 100) || 0"
              />
            </div>
          </div>

          <!-- Stock / Alert stock -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">库存</label>
              <input
                v-model.number="formData.stock"
                type="number"
                min="0"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="0"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">库存预警值</label>
              <input
                v-model.number="formData.alert_stock"
                type="number"
                min="0"
                class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="0"
              />
            </div>
          </div>

          <!-- Expiry date -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">有效期</label>
            <input
              v-model="formData.expiry_date"
              type="date"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>

          <!-- Error -->
          <div v-if="formError" class="text-sm text-red-600 bg-red-50 rounded-lg p-3">{{ formError }}</div>
        </div>

        <!-- Actions -->
        <div class="flex justify-end gap-3 mt-6">
          <button class="px-4 py-2 text-sm text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200" @click="showDialog = false">取消</button>
          <button class="px-4 py-2 text-sm text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50" :disabled="saving" @click="handleSave">
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Delete Dialog -->
    <div v-if="showDeleteDialog" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="showDeleteDialog = false">
      <div class="bg-white rounded-xl shadow-xl w-full max-w-sm mx-4 p-6">
        <h2 class="text-lg font-bold text-gray-800 mb-2">确认删除</h2>
        <p class="text-sm text-gray-600 mb-1">
          确定要删除商品「{{ deletingProduct?.name }}」吗？
        </p>
        <p class="text-xs text-red-500 mb-4">删除前将检查该商品是否有剩余库存和未完成订单，如有则不可删除。</p>

        <div v-if="deleteError" class="text-sm text-red-600 bg-red-50 rounded-lg p-3 mb-4">{{ deleteError }}</div>

        <div class="flex justify-end gap-3">
          <button class="px-4 py-2 text-sm text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200" @click="showDeleteDialog = false">取消</button>
          <button class="px-4 py-2 text-sm text-white bg-red-500 rounded-lg hover:bg-red-600 disabled:opacity-50" :disabled="deleting" @click="handleDelete">
            {{ deleting ? '删除中...' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
