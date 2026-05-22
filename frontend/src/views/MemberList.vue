<template>
  <div class="member-list-container">
    <div class="page-header">
      <h2>会员管理</h2>
      <button class="btn-primary" @click="showCreate = true">+ 新建会员</button>
    </div>

    <div class="search-bar">
      <input v-model="keyword" placeholder="搜索姓名/手机号..." @keyup.enter="search" />
      <select v-model="statusFilter" @change="search">
        <option value="">全部状态</option>
        <option value="active">活跃</option>
        <option value="inactive">已禁用</option>
      </select>
      <button @click="search">搜索</button>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else-if="error" class="error">
      {{ error }}
      <button @click="fetchMembers">重试</button>
    </div>

    <div v-else>
      <table v-if="members.length > 0" class="member-table">
        <thead>
          <tr>
            <th>卡号</th>
            <th>姓名</th>
            <th>手机号</th>
            <th>性别</th>
            <th>状态</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in members" :key="m.id">
            <td>{{ m.card_no }}</td>
            <td>{{ m.name }}</td>
            <td>{{ m.phone }}</td>
            <td>{{ genderLabel(m.gender) }}</td>
            <td>
              <span :class="['status-badge', m.status]">{{ m.status === 'active' ? '活跃' : '已禁用' }}</span>
            </td>
            <td>{{ formatDate(m.created_at) }}</td>
            <td>
              <router-link :to="`/merchant/members/${m.id}`" class="btn-link">详情</router-link>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="empty">暂无会员数据</div>

      <div v-if="total > pageSize" class="pagination">
        <button :disabled="page <= 1" @click="goPage(page - 1)">上一页</button>
        <span>第 {{ page }} / {{ totalPages }} 页 (共 {{ total }} 条)</span>
        <button :disabled="page >= totalPages" @click="goPage(page + 1)">下一页</button>
      </div>
    </div>

    <!-- Create dialog -->
    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <h3>新建会员</h3>
        <form @submit.prevent="handleCreate">
          <label>姓名 *<input v-model="form.name" required /></label>
          <label>手机号 *<input v-model="form.phone" required /></label>
          <label>微信<input v-model="form.wechat" /></label>
          <label>性别
            <select v-model="form.gender">
              <option value="">请选择</option>
              <option value="M">男</option>
              <option value="F">女</option>
              <option value="O">其他</option>
            </select>
          </label>
          <label>生日<input v-model="form.birthday" type="date" /></label>
          <label>地址<input v-model="form.address" /></label>
          <label>备注<input v-model="form.remark" /></label>
          <div class="modal-actions">
            <button type="button" @click="showCreate = false">取消</button>
            <button type="submit" class="btn-primary" :disabled="creating">{{ creating ? '保存中...' : '保存' }}</button>
          </div>
        </form>
        <p v-if="createError" class="error-msg">{{ createError }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/api/client'

const members = ref<any[]>([])
const loading = ref(true)
const error = ref('')
const keyword = ref('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)

const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const form = ref({ name: '', phone: '', wechat: '', gender: '', birthday: '', address: '', remark: '' })

const totalPages = computed(() => Math.ceil(total.value / pageSize) || 1)

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

async function fetchMembers() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getMembers({ keyword: keyword.value, status: statusFilter.value || undefined, page: page.value, page_size: pageSize })
    members.value = data.members
    total.value = data.total
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  fetchMembers()
}

function goPage(p: number) {
  page.value = p
  fetchMembers()
}

async function handleCreate() {
  creating.value = true
  createError.value = ''
  try {
    await api.createMember({
      name: form.value.name,
      phone: form.value.phone,
      wechat: form.value.wechat || undefined,
      gender: form.value.gender || undefined,
      birthday: form.value.birthday || undefined,
      address: form.value.address || undefined,
      remark: form.value.remark || undefined,
    })
    showCreate.value = false
    form.value = { name: '', phone: '', wechat: '', gender: '', birthday: '', address: '', remark: '' }
    fetchMembers()
  } catch (e: any) {
    createError.value = e.message || '创建失败'
  } finally {
    creating.value = false
  }
}

onMounted(fetchMembers)
</script>

<style scoped>
.member-list-container { padding: 20px; max-width: 1200px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-header h2 { margin: 0; }
.btn-primary { background: #3b82f6; color: #fff; border: none; padding: 8px 16px; border-radius: 6px; cursor: pointer; }
.btn-link { color: #3b82f6; text-decoration: none; }
.search-bar { display: flex; gap: 8px; margin-bottom: 16px; }
.search-bar input { flex: 1; padding: 8px; border: 1px solid #d1d5db; border-radius: 6px; }
.search-bar select { padding: 8px; border: 1px solid #d1d5db; border-radius: 6px; }
.search-bar button { padding: 8px 16px; background: #3b82f6; color: #fff; border: none; border-radius: 6px; cursor: pointer; }
.loading { text-align: center; padding: 40px; color: #6b7280; }
.error { text-align: center; padding: 40px; color: #ef4444; }
.empty { text-align: center; padding: 40px; color: #9ca3af; }
.member-table { width: 100%; border-collapse: collapse; }
.member-table th, .member-table td { padding: 10px 12px; text-align: left; border-bottom: 1px solid #e5e7eb; }
.member-table th { background: #f9fafb; font-weight: 600; color: #374151; }
.status-badge { padding: 2px 8px; border-radius: 10px; font-size: 12px; }
.status-badge.active { background: #d1fae5; color: #065f46; }
.status-badge.inactive { background: #fee2e2; color: #991b1b; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; margin-top: 16px; }
.pagination button { padding: 6px 12px; border: 1px solid #d1d5db; border-radius: 4px; background: #fff; cursor: pointer; }
.pagination button:disabled { opacity: 0.5; cursor: not-allowed; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: #fff; border-radius: 10px; padding: 24px; width: 420px; max-height: 90vh; overflow-y: auto; }
.modal h3 { margin: 0 0 16px; }
.modal label { display: block; margin-bottom: 10px; font-size: 14px; color: #374151; }
.modal input, .modal select { width: 100%; padding: 8px; border: 1px solid #d1d5db; border-radius: 4px; margin-top: 4px; box-sizing: border-box; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
.modal-actions button { padding: 8px 16px; border: 1px solid #d1d5db; border-radius: 6px; background: #fff; cursor: pointer; }
.error-msg { color: #ef4444; margin-top: 8px; font-size: 13px; }
</style>
