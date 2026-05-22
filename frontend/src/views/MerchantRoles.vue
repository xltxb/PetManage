<template>
  <div class="p-6 max-w-5xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-gray-800">角色管理</h1>
      <button
        @click="showCreate = true"
        class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition"
      >
        + 新建角色
      </button>
    </div>

    <!-- Role list -->
    <div v-if="loading" class="text-gray-500">加载中...</div>
    <div v-else-if="roles.length === 0" class="text-gray-400 text-center py-12">
      暂无角色，点击上方按钮创建
    </div>
    <div v-else class="space-y-4">
      <div
        v-for="role in roles"
        :key="role.id"
        class="border rounded-lg p-4 hover:shadow-md transition cursor-pointer"
        @click="editRole(role)"
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-lg font-semibold text-gray-800">{{ role.name }}</h3>
            <p class="text-sm text-gray-500">{{ role.code }}</p>
            <p v-if="role.description" class="text-sm text-gray-400 mt-1">{{ role.description }}</p>
          </div>
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-400">
              {{ role.permissions.length }} 项权限
            </span>
            <button
              @click.stop="confirmDelete(role)"
              class="px-3 py-1 text-sm text-red-600 border border-red-300 rounded hover:bg-red-50 transition"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div class="bg-white rounded-xl shadow-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <h2 class="text-xl font-bold mb-4">{{ editing ? '编辑角色' : '新建角色' }}</h2>

        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">角色名称</label>
            <input
              v-model="form.name"
              class="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="如：收银员"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">角色编码</label>
            <input
              v-model="form.code"
              :disabled="editing"
              class="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-gray-100"
              placeholder="如：cashier"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">描述</label>
            <input
              v-model="form.description"
              class="w-full border rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="角色职责说明"
            />
          </div>

          <!-- Permission configuration -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-2">菜单/操作权限</label>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div v-for="cat in permissionCategories" :key="cat.category" class="border rounded-lg p-3">
                <h4 class="font-medium text-sm text-gray-700 mb-2">{{ cat.category }}</h4>
                <label
                  v-for="perm in cat.items"
                  :key="perm.key"
                  class="flex items-center gap-2 py-1 cursor-pointer text-sm"
                >
                  <input
                    type="checkbox"
                    :value="perm.key"
                    v-model="form.permissions"
                    class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span class="text-gray-700">{{ perm.name }}</span>
                  <span class="text-xs text-gray-400 ml-auto">{{ perm.description }}</span>
                </label>
              </div>
            </div>
          </div>
        </div>

        <div class="flex justify-end gap-3 mt-6 pt-4 border-t">
          <button
            @click="showModal = false"
            class="px-4 py-2 border rounded-lg text-gray-600 hover:bg-gray-50 transition"
          >
            取消
          </button>
          <button
            @click="saveRole"
            :disabled="saving"
            class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 transition"
          >
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Delete confirmation -->
    <div v-if="showDelete" class="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div class="bg-white rounded-xl shadow-2xl p-6 max-w-sm">
        <h3 class="text-lg font-bold mb-2">确认删除</h3>
        <p class="text-gray-600 mb-4">确定要删除角色「{{ deletingRole?.name }}」吗？此操作不可撤销。</p>
        <div class="flex justify-end gap-3">
          <button
            @click="showDelete = false"
            class="px-4 py-2 border rounded-lg text-gray-600 hover:bg-gray-50 transition"
          >
            取消
          </button>
          <button
            @click="doDelete"
            :disabled="deleting"
            class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 transition"
          >
            {{ deleting ? '删除中...' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '@/api/client'

interface Role {
  id: number
  merchant_id: number
  name: string
  code: string
  description: string
  permissions: string[]
  created_at: string
  updated_at: string
}

interface PermissionItem {
  key: string
  name: string
  category: string
  description: string
}

const roles = ref<Role[]>([])
const loading = ref(true)
const showCreate = ref(false)
const showModal = ref(false)
const showDelete = ref(false)
const editing = ref(false)
const saving = ref(false)
const deleting = ref(false)
const deletingRole = ref<Role | null>(null)
const editingRoleId = ref<number | null>(null)
const allPermissions = ref<PermissionItem[]>([])

const form = ref({
  name: '',
  code: '',
  description: '',
  permissions: [] as string[],
})

const permissionCategories = computed(() => {
  const map = new Map<string, PermissionItem[]>()
  for (const p of allPermissions.value) {
    if (!map.has(p.category)) map.set(p.category, [])
    map.get(p.category)!.push(p)
  }
  return Array.from(map.entries()).map(([category, items]) => ({ category, items }))
})

async function loadRoles() {
  loading.value = true
  try {
    const data = await api.request<{ roles: Role[] }>('/api/v1/merchant/roles')
    roles.value = data.roles
  } catch (e) {
    console.error('Failed to load roles', e)
  } finally {
    loading.value = false
  }
}

async function loadPermissions() {
  try {
    const data = await api.request<{ permissions: PermissionItem[] }>('/api/v1/merchant/roles/permissions')
    allPermissions.value = data.permissions
  } catch (e) {
    console.error('Failed to load permissions', e)
  }
}

function editRole(role: Role) {
  editing.value = true
  editingRoleId.value = role.id
  form.value = {
    name: role.name,
    code: role.code,
    description: role.description,
    permissions: [...role.permissions],
  }
  showModal.value = true
}

function openCreate() {
  editing.value = false
  editingRoleId.value = null
  form.value = { name: '', code: '', description: '', permissions: [] }
  showModal.value = true
}

async function saveRole() {
  saving.value = true
  try {
    if (editing.value && editingRoleId.value) {
      await api.request(`/api/v1/merchant/roles/${editingRoleId.value}`, {
        method: 'PUT',
        body: {
          name: form.value.name,
          description: form.value.description,
          permissions: form.value.permissions,
        },
      })
    } else {
      await api.request('/api/v1/merchant/roles', {
        method: 'POST',
        body: {
          name: form.value.name,
          code: form.value.code,
          description: form.value.description,
          permissions: form.value.permissions,
        },
      })
    }
    showModal.value = false
    await loadRoles()
  } catch (e: any) {
    alert('保存失败: ' + (e.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

function confirmDelete(role: Role) {
  deletingRole.value = role
  showDelete.value = true
}

async function doDelete() {
  if (!deletingRole.value) return
  deleting.value = true
  try {
    await api.request(`/api/v1/merchant/roles/${deletingRole.value.id}`, { method: 'DELETE' })
    showDelete.value = false
    await loadRoles()
  } catch (e: any) {
    alert('删除失败: ' + (e.message || '未知错误'))
  } finally {
    deleting.value = false
  }
}

// Watch showCreate to open the modal
const showCreateWatcher = computed(() => {
  if (showCreate.value) {
    openCreate()
    showCreate.value = false
  }
  return showCreate.value
})
// Trigger it
showCreateWatcher

onMounted(() => {
  loadRoles()
  loadPermissions()
})
</script>
