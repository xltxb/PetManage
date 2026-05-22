<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'
import type { Category } from '@/api/client'

const router = useRouter()
const auth = useAuthStore()

const categories = ref<Category[]>([])
const loading = ref(true)
const error = ref('')

// Dialog state
const showDialog = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingCategory = ref<Category | null>(null)
const formName = ref('')
const formParentId = ref<number | null>(null)
const formError = ref('')
const saving = ref(false)

// Delete state
const showDeleteDialog = ref(false)
const deletingCategory = ref<Category | null>(null)
const deleteError = ref('')
const deleting = ref(false)

if (!auth.user) {
  router.replace('/merchant/login')
}

// Flatten tree for parent selection dropdown
function flattenTree(nodes: Category[], depth = 0): { id: number; label: string; depth: number }[] {
  const result: { id: number; label: string; depth: number }[] = []
  for (const node of nodes) {
    const prefix = depth > 0 ? '  '.repeat(depth) + '└ ' : ''
    result.push({ id: node.id, label: prefix + node.name, depth })
    if (node.children && node.children.length > 0) {
      result.push(...flattenTree(node.children, depth + 1))
    }
  }
  return result
}

async function loadCategories() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getCategories()
    categories.value = data.categories || []
  } catch (e: any) {
    error.value = e.message || 'Failed to load categories'
  } finally {
    loading.value = false
  }
}

function openCreateDialog(parentId?: number) {
  dialogMode.value = 'create'
  editingCategory.value = null
  formName.value = ''
  formParentId.value = parentId || null
  formError.value = ''
  showDialog.value = true
}

function openEditDialog(cat: Category) {
  dialogMode.value = 'edit'
  editingCategory.value = cat
  formName.value = cat.name
  formParentId.value = cat.parent_id
  formError.value = ''
  showDialog.value = true
}

async function handleSave() {
  formError.value = ''
  if (!formName.value.trim()) {
    formError.value = '分类名称不能为空'
    return
  }

  saving.value = true
  try {
    if (dialogMode.value === 'create') {
      await api.createCategory({
        name: formName.value.trim(),
        parent_id: formParentId.value,
      })
    } else {
      await api.updateCategory(editingCategory.value!.id, {
        name: formName.value.trim(),
        parent_id: formParentId.value,
      })
    }
    showDialog.value = false
    await loadCategories()
  } catch (e: any) {
    formError.value = e.message || '操作失败'
  } finally {
    saving.value = false
  }
}

function openDeleteDialog(cat: Category) {
  deletingCategory.value = cat
  deleteError.value = ''
  showDeleteDialog.value = true
}

async function handleDelete() {
  if (!deletingCategory.value) return
  deleteError.value = ''
  deleting.value = true
  try {
    await api.deleteCategory(deletingCategory.value.id)
    showDeleteDialog.value = false
    deletingCategory.value = null
    await loadCategories()
  } catch (e: any) {
    deleteError.value = e.message || '删除失败'
  } finally {
    deleting.value = false
  }
}

function getIndentStyle(depth: number) {
  return { paddingLeft: `${depth * 24}px` }
}

// Recursive tree node component
function renderTreeNode(node: Category, depth: number = 0) {
  const indent = { paddingLeft: `${depth * 28 + 8}px` }
  const hasChildren = node.children && node.children.length > 0

  return [
    h('div', {
      key: node.id,
      class: 'flex items-center justify-between py-2 px-3 hover:bg-gray-50 rounded group border-b border-gray-100',
    }, [
      h('div', { class: 'flex items-center gap-2', style: indent }, [
        h('span', { class: hasChildren ? 'text-yellow-500 text-sm' : 'text-gray-300 text-sm' }, hasChildren ? '\u{1F4C1}' : '\u{1F4C4}'),
        h('span', { class: 'text-sm font-medium text-gray-700' }, node.name),
      ]),
      h('div', { class: 'flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity' }, [
        h('button', {
          class: 'text-xs text-blue-500 hover:text-blue-700 px-2 py-0.5 rounded hover:bg-blue-50',
          onClick: () => openCreateDialog(node.id),
        }, '添加子分类'),
        h('button', {
          class: 'text-xs text-gray-500 hover:text-gray-700 px-2 py-0.5 rounded hover:bg-gray-100',
          onClick: () => openEditDialog(node),
        }, '编辑'),
        h('button', {
          class: 'text-xs text-red-400 hover:text-red-600 px-2 py-0.5 rounded hover:bg-red-50',
          onClick: () => openDeleteDialog(node),
        }, '删除'),
      ]),
    ]),
    ...(hasChildren ? node.children!.flatMap(c => renderTreeNode(c, depth + 1)) : []),
  ]
}

import { h } from 'vue'

onMounted(() => {
  loadCategories()
})
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b border-gray-200">
      <div class="max-w-4xl mx-auto px-4 py-4 flex items-center justify-between">
        <div>
          <h1 class="text-xl font-bold text-gray-800">商品分类管理</h1>
          <p class="text-sm text-gray-500 mt-0.5">管理商品分类，支持多级分类结构</p>
        </div>
        <button
          class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 transition-colors flex items-center gap-1"
          @click="openCreateDialog()"
        >
          <span class="text-lg leading-none">+</span> 新建分类
        </button>
      </div>
    </header>

    <!-- Content -->
    <div class="max-w-4xl mx-auto px-4 py-6">
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
      <div v-else-if="categories.length === 0" class="text-center py-20">
        <div class="text-5xl mb-4">📂</div>
        <p class="text-gray-500 text-sm mb-4">暂无商品分类，点击上方按钮创建第一个分类</p>
        <button
          class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700"
          @click="openCreateDialog()"
        >创建分类</button>
      </div>

      <!-- Tree -->
      <div v-else class="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
        <div class="space-y-0">
          <template v-for="cat in categories" :key="cat.id">
            <div class="flex items-center justify-between py-2.5 px-3 hover:bg-gray-50 rounded group border-b border-gray-50 last:border-0">
              <div class="flex items-center gap-2">
                <span class="text-yellow-500 text-sm">📁</span>
                <span class="text-sm font-semibold text-gray-800">{{ cat.name }}</span>
                <span v-if="cat.children && cat.children.length > 0" class="text-xs text-gray-400">({{ cat.children.length }}个子分类)</span>
              </div>
              <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button
                  class="text-xs text-blue-500 hover:text-blue-700 px-2 py-0.5 rounded hover:bg-blue-50"
                  @click="openCreateDialog(cat.id)"
                >添加子分类</button>
                <button
                  class="text-xs text-gray-500 hover:text-gray-700 px-2 py-0.5 rounded hover:bg-gray-100"
                  @click="openEditDialog(cat)"
                >编辑</button>
                <button
                  class="text-xs text-red-400 hover:text-red-600 px-2 py-0.5 rounded hover:bg-red-50"
                  @click="openDeleteDialog(cat)"
                >删除</button>
              </div>
            </div>
            <!-- Level 2 children -->
            <template v-if="cat.children && cat.children.length > 0">
              <div v-for="child in cat.children" :key="child.id" class="flex items-center justify-between py-2 px-3 hover:bg-gray-50 rounded group border-b border-gray-50 last:border-0" style="padding-left: 44px">
                <div class="flex items-center gap-2">
                  <span class="text-gray-300 text-sm">📄</span>
                  <span class="text-sm text-gray-700">{{ child.name }}</span>
                </div>
                <div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    class="text-xs text-gray-500 hover:text-gray-700 px-2 py-0.5 rounded hover:bg-gray-100"
                    @click="openEditDialog(child)"
                  >编辑</button>
                  <button
                    class="text-xs text-red-400 hover:text-red-600 px-2 py-0.5 rounded hover:bg-red-50"
                    @click="openDeleteDialog(child)"
                  >删除</button>
                </div>
              </div>
            </template>
          </template>
        </div>
      </div>
    </div>

    <!-- Create/Edit Dialog -->
    <div v-if="showDialog" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50" @click.self="showDialog = false">
      <div class="bg-white rounded-xl shadow-xl w-full max-w-md mx-4 p-6">
        <h2 class="text-lg font-bold text-gray-800 mb-4">
          {{ dialogMode === 'create' ? '新建分类' : '编辑分类' }}
        </h2>

        <div class="space-y-4">
          <!-- Name -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">分类名称 <span class="text-red-500">*</span></label>
            <input
              v-model="formName"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="请输入分类名称"
              @keyup.enter="handleSave"
            />
          </div>

          <!-- Parent -->
          <div v-if="dialogMode === 'create'">
            <label class="block text-sm font-medium text-gray-700 mb-1">父级分类</label>
            <select v-model="formParentId" class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent">
              <option :value="null">无（作为一级分类）</option>
              <option v-for="opt in flattenTree(categories)" :key="opt.id" :value="opt.id" v-html="opt.label"></option>
            </select>
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
        <p class="text-sm text-gray-600 mb-4">
          确定要删除分类「{{ deletingCategory?.name }}」吗？
          <span class="block mt-1 text-red-500">删除前将检查是否有关联商品，如有则不可删除。</span>
        </p>

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
