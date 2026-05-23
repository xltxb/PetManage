<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const router = useRouter()
const auth = useAuthStore()

interface Appointment {
  id: number
  merchant_id: number
  member_id: number
  pet_id: number
  service_item_id: number
  employee_id: number
  appointment_time: string
  status: string
  remark: string
  created_at: string
  updated_at: string
  member_name: string
  member_phone: string
  pet_name: string
  service_item_name: string
  employee_name: string
}

interface Member {
  id: number
  name: string
  phone: string
  card_no: string
  status: string
}

interface Pet {
  id: number
  name: string
  breed: string
  gender: string
}

interface ServiceItem {
  id: number
  name: string
  duration_minutes: number
  price_cents: number
  member_price_cents: number
  status: string
}

interface Employee {
  id: number
  name: string
  employee_no: string
  position: string
  status: string
}

const appointments = ref<Appointment[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const statusFilter = ref('')
const loading = ref(true)
const error = ref('')

// Create modal state
const showCreateModal = ref(false)
const createStep = ref(0)
const createLoading = ref(false)
const createError = ref('')
const createSuccess = ref('')

// Step 1: Select member
const memberKeyword = ref('')
const members = ref<Member[]>([])
const selectedMember = ref<Member | null>(null)
const memberSearching = ref(false)

// Step 2: Select pet
const pets = ref<Pet[]>([])
const selectedPet = ref<Pet | null>(null)
const petsLoading = ref(false)

// Step 3: Select service item
const serviceItems = ref<ServiceItem[]>([])
const selectedService = ref<ServiceItem | null>(null)
const servicesLoading = ref(false)

// Step 4: Select time & technician
const appointmentDate = ref('')
const appointmentHour = ref('')
const appointmentMinute = ref('')
const employees = ref<Employee[]>([])
const selectedEmployee = ref<Employee | null>(null)
const remark = ref('')
const employeesLoading = ref(false)

const statusLabels: Record<string, string> = {
  pending: '待确认',
  confirmed: '已确认',
  cancelled: '已取消',
  completed: '已完成',
}

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  confirmed: 'bg-blue-100 text-blue-800',
  cancelled: 'bg-gray-100 text-gray-500',
  completed: 'bg-green-100 text-green-800',
}

if (!auth.user) {
  router.replace('/merchant/login')
}

async function loadAppointments() {
  loading.value = true
  error.value = ''
  try {
    const result = await api.getAppointments({
      status: statusFilter.value || undefined,
      page: page.value,
      page_size: pageSize,
    })
    appointments.value = result.appointments
    total.value = result.total
    page.value = result.page
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function setStatusFilter(s: string) {
  statusFilter.value = s
  page.value = 1
  loadAppointments()
}

function goToPage(p: number) {
  page.value = p
  loadAppointments()
}

function formatTime(iso: string) {
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', {
    month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

function openCreateModal() {
  createStep.value = 0
  createError.value = ''
  createSuccess.value = ''
  selectedMember.value = null
  selectedPet.value = null
  selectedService.value = null
  selectedEmployee.value = null
  memberKeyword.value = ''
  members.value = []
  pets.value = []
  serviceItems.value = []
  employees.value = []
  appointmentDate.value = ''
  appointmentHour.value = ''
  appointmentMinute.value = ''
  remark.value = ''
  showCreateModal.value = true
}

async function searchMembers() {
  memberSearching.value = true
  try {
    const result = await api.getMembers({ keyword: memberKeyword.value || undefined, page_size: 50 })
    members.value = result.members || []
  } catch (e: any) {
    createError.value = e.message || '搜索会员失败'
  } finally {
    memberSearching.value = false
  }
}

function selectMember(m: Member) {
  selectedMember.value = m
  createError.value = ''
  createStep.value = 1
  loadPets()
}

async function loadPets() {
  if (!selectedMember.value) return
  petsLoading.value = true
  try {
    const result = await api.getMember(selectedMember.value.id)
    pets.value = result.pets || []
  } catch (e: any) {
    createError.value = e.message || '加载宠物失败'
  } finally {
    petsLoading.value = false
  }
}

function selectPet(p: Pet) {
  selectedPet.value = p
  createError.value = ''
  createStep.value = 2
  loadServices()
}

async function loadServices() {
  servicesLoading.value = true
  try {
    const result = await api.getServiceItems({ status: 'active', page_size: 100 })
    serviceItems.value = result.items || []
  } catch (e: any) {
    createError.value = e.message || '加载服务项目失败'
  } finally {
    servicesLoading.value = false
  }
}

function selectService(s: ServiceItem) {
  selectedService.value = s
  createError.value = ''
  createStep.value = 3
  loadEmployees()
}

async function loadEmployees() {
  employeesLoading.value = true
  try {
    const result = await api.getEmployees({ status: 'active', page_size: 100 })
    employees.value = result.employees || []
  } catch (e: any) {
    createError.value = e.message || '加载员工失败'
  } finally {
    employeesLoading.value = false
  }
}

function selectEmployee(e: Employee) {
  selectedEmployee.value = e
  createError.value = ''
}

async function submitAppointment() {
  if (!selectedMember.value || !selectedPet.value || !selectedService.value || !selectedEmployee.value) {
    createError.value = '请完成所有选择'
    return
  }
  if (!appointmentDate.value || !appointmentHour.value) {
    createError.value = '请选择预约时间'
    return
  }

  const dateStr = appointmentDate.value
  const hour = appointmentHour.value.padStart(2, '0')
  const minute = appointmentMinute.value || '00'
  const timeStr = `${dateStr}T${hour}:${minute}:00+08:00`

  createLoading.value = true
  createError.value = ''

  try {
    await api.createAppointment({
      member_id: selectedMember.value.id,
      pet_id: selectedPet.value.id,
      service_item_id: selectedService.value.id,
      employee_id: selectedEmployee.value.id,
      appointment_time: timeStr,
      remark: remark.value || undefined,
    })
    createSuccess.value = '预约创建成功！'
    setTimeout(() => {
      showCreateModal.value = false
      loadAppointments()
    }, 1000)
  } catch (e: any) {
    createError.value = e.message || '创建失败'
  } finally {
    createLoading.value = false
  }
}

function goToMemberDetail(memberId: number) {
  router.push(`/merchant/members/${memberId}`)
}

function disableDateBeforeToday(dateStr: string) {
  const today = new Date()
  const yyyy = today.getFullYear()
  const mm = String(today.getMonth() + 1).padStart(2, '0')
  const dd = String(today.getDate()).padStart(2, '0')
  return dateStr >= `${yyyy}-${mm}-${dd}`
}

onMounted(loadAppointments)
</script>

<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow-sm border-b">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between">
        <div class="flex items-center gap-4">
          <button
            @click="router.push('/merchant')"
            class="text-sm text-blue-600 hover:text-blue-800 cursor-pointer"
          >
            &larr; 返回首页
          </button>
          <h1 class="text-lg font-semibold text-gray-800">预约管理</h1>
        </div>
        <button
          @click="openCreateModal"
          class="bg-blue-600 text-white px-4 py-1.5 rounded text-sm hover:bg-blue-700 cursor-pointer"
        >
          + 新建预约
        </button>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 py-6">
      <!-- Status tabs -->
      <div class="flex gap-2 mb-4">
        <button
          v-for="tab in [{k:'',l:'全部'},{k:'pending',l:'待确认'},{k:'confirmed',l:'已确认'},{k:'completed',l:'已完成'},{k:'cancelled',l:'已取消'}]"
          :key="tab.k"
          @click="setStatusFilter(tab.k)"
          :class="[
            'px-3 py-1.5 rounded text-sm cursor-pointer',
            statusFilter === tab.k ? 'bg-blue-600 text-white' : 'bg-white text-gray-600 hover:bg-gray-100 border',
          ]"
        >
          {{ tab.l }}
        </button>
      </div>

      <!-- Error -->
      <div v-if="error" class="bg-red-50 text-red-600 text-sm p-3 rounded mb-4">
        {{ error }}
      </div>

      <!-- Loading -->
      <div v-if="loading" class="text-center py-12 text-gray-400">加载中...</div>

      <!-- Empty -->
      <div v-else-if="appointments.length === 0" class="bg-white rounded-lg shadow-sm p-12 text-center">
        <p class="text-gray-500">暂无预约记录</p>
        <button
          @click="openCreateModal"
          class="mt-4 text-blue-600 text-sm hover:text-blue-800 cursor-pointer"
        >
          创建第一个预约
        </button>
      </div>

      <!-- Appointment list -->
      <div v-else class="bg-white rounded-lg shadow-sm overflow-hidden">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 text-gray-500">
            <tr>
              <th class="text-left px-4 py-3 font-medium">预约时间</th>
              <th class="text-left px-4 py-3 font-medium">会员</th>
              <th class="text-left px-4 py-3 font-medium">宠物</th>
              <th class="text-left px-4 py-3 font-medium">服务项目</th>
              <th class="text-left px-4 py-3 font-medium">技师</th>
              <th class="text-left px-4 py-3 font-medium">状态</th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="apt in appointments" :key="apt.id" class="hover:bg-gray-50">
              <td class="px-4 py-3 text-gray-800">{{ formatTime(apt.appointment_time) }}</td>
              <td class="px-4 py-3">
                <button
                  @click="goToMemberDetail(apt.member_id)"
                  class="text-blue-600 hover:text-blue-800 cursor-pointer"
                >
                  {{ apt.member_name }}
                </button>
                <div class="text-xs text-gray-400">{{ apt.member_phone }}</div>
              </td>
              <td class="px-4 py-3 text-gray-700">{{ apt.pet_name }}</td>
              <td class="px-4 py-3 text-gray-700">{{ apt.service_item_name }}</td>
              <td class="px-4 py-3 text-gray-700">{{ apt.employee_name }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-0.5 rounded-full text-xs', statusColors[apt.status] || 'bg-gray-100']">
                  {{ statusLabels[apt.status] || apt.status }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="total > pageSize" class="px-4 py-3 border-t flex items-center justify-between text-sm text-gray-500">
          <span>共 {{ total }} 条</span>
          <div class="flex gap-1">
            <button
              v-for="p in Math.ceil(total / pageSize)"
              :key="p"
              @click="goToPage(p)"
              :class="[
                'px-3 py-1 rounded cursor-pointer',
                page === p ? 'bg-blue-600 text-white' : 'hover:bg-gray-100',
              ]"
            >
              {{ p }}
            </button>
          </div>
        </div>
      </div>
    </main>

    <!-- Create Appointment Modal -->
    <Teleport to="body">
      <div
        v-if="showCreateModal"
        class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center"
        @click.self="showCreateModal = false"
      >
        <div class="bg-white rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
          <!-- Modal Header -->
          <div class="px-6 py-4 border-b flex items-center justify-between sticky top-0 bg-white z-10">
            <h2 class="text-lg font-semibold">新建预约</h2>
            <button @click="showCreateModal = false" class="text-gray-400 hover:text-gray-600 text-xl cursor-pointer">&times;</button>
          </div>

          <!-- Step Indicator -->
          <div class="px-6 py-3 border-b">
            <div class="flex items-center justify-between text-xs">
              <span
                v-for="(step, idx) in ['选择会员', '选择宠物', '选择服务', '预约信息']"
                :key="idx"
                :class="[
                  'flex items-center gap-1',
                  createStep === idx ? 'text-blue-600 font-medium' :
                  createStep > idx ? 'text-green-600' : 'text-gray-400',
                ]"
              >
                <span
                  :class="[
                    'w-5 h-5 rounded-full flex items-center justify-center text-xs',
                    createStep === idx ? 'bg-blue-600 text-white' :
                    createStep > idx ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-500',
                  ]"
                >
                  {{ createStep > idx ? '✓' : idx + 1 }}
                </span>
                {{ step }}
              </span>
            </div>
          </div>

          <!-- Modal Body -->
          <div class="px-6 py-4">
            <!-- Error / Success -->
            <div v-if="createError" class="bg-red-50 text-red-600 text-sm p-3 rounded mb-4">
              {{ createError }}
            </div>
            <div v-if="createSuccess" class="bg-green-50 text-green-600 text-sm p-3 rounded mb-4">
              {{ createSuccess }}
            </div>

            <!-- Step 0: Select Member -->
            <div v-if="createStep === 0">
              <label class="block text-sm font-medium text-gray-700 mb-2">搜索会员</label>
              <div class="flex gap-2 mb-3">
                <input
                  v-model="memberKeyword"
                  @keyup.enter="searchMembers"
                  placeholder="输入姓名或手机号..."
                  class="flex-1 border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
                <button
                  @click="searchMembers"
                  class="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 cursor-pointer"
                  :disabled="memberSearching"
                >
                  {{ memberSearching ? '搜索中...' : '搜索' }}
                </button>
              </div>
              <p v-if="!members.length && !memberSearching" class="text-sm text-gray-400 text-center py-4">
                输入关键词搜索会员
              </p>
              <div v-else class="max-h-60 overflow-y-auto border rounded divide-y">
                <div
                  v-for="m in members"
                  :key="m.id"
                  @click="selectMember(m)"
                  class="px-4 py-3 hover:bg-blue-50 cursor-pointer flex items-center justify-between"
                >
                  <div>
                    <span class="text-sm font-medium text-gray-800">{{ m.name }}</span>
                    <span class="text-xs text-gray-400 ml-2">{{ m.phone }}</span>
                  </div>
                  <span class="text-xs text-gray-400">{{ m.card_no }}</span>
                </div>
              </div>
            </div>

            <!-- Step 1: Select Pet -->
            <div v-if="createStep === 1">
              <div class="mb-3">
                <span class="text-sm text-gray-500">会员：</span>
                <span class="text-sm font-medium">{{ selectedMember?.name }}</span>
                <button @click="createStep = 0" class="text-xs text-blue-600 ml-2 cursor-pointer">修改</button>
              </div>
              <label class="block text-sm font-medium text-gray-700 mb-2">选择宠物</label>
              <div v-if="petsLoading" class="text-center py-4 text-gray-400 text-sm">加载中...</div>
              <div v-else-if="!pets.length" class="text-center py-4 text-gray-400 text-sm">
                该会员暂无绑定宠物
              </div>
              <div v-else class="max-h-60 overflow-y-auto border rounded divide-y">
                <div
                  v-for="p in pets"
                  :key="p.id"
                  @click="selectPet(p)"
                  class="px-4 py-3 hover:bg-blue-50 cursor-pointer flex items-center justify-between"
                >
                  <div>
                    <span class="text-sm font-medium text-gray-800">{{ p.name }}</span>
                    <span class="text-xs text-gray-400 ml-2">{{ p.breed }}</span>
                  </div>
                  <span class="text-xs text-gray-400">{{ p.gender === 'M' ? '♂' : p.gender === 'F' ? '♀' : '' }}</span>
                </div>
              </div>
            </div>

            <!-- Step 2: Select Service -->
            <div v-if="createStep === 2">
              <div class="mb-3 space-x-4 text-sm text-gray-500">
                <span>会员：{{ selectedMember?.name }}</span>
                <button @click="createStep = 0" class="text-xs text-blue-600 cursor-pointer">修改</button>
                <span>| 宠物：{{ selectedPet?.name }}</span>
                <button @click="createStep = 1" class="text-xs text-blue-600 cursor-pointer">修改</button>
              </div>
              <label class="block text-sm font-medium text-gray-700 mb-2">选择服务项目</label>
              <div v-if="servicesLoading" class="text-center py-4 text-gray-400 text-sm">加载中...</div>
              <div v-else-if="!serviceItems.length" class="text-center py-4 text-gray-400 text-sm">
                暂无可用服务项目
              </div>
              <div v-else class="max-h-60 overflow-y-auto border rounded divide-y">
                <div
                  v-for="s in serviceItems"
                  :key="s.id"
                  @click="selectService(s)"
                  class="px-4 py-3 hover:bg-blue-50 cursor-pointer flex items-center justify-between"
                >
                  <div>
                    <span class="text-sm font-medium text-gray-800">{{ s.name }}</span>
                    <span class="text-xs text-gray-400 ml-2">{{ s.duration_minutes }}分钟</span>
                  </div>
                  <span class="text-sm text-gray-700">
                    ¥{{ s.member_price_cents > 0 ? (s.member_price_cents / 100).toFixed(2) : (s.price_cents / 100).toFixed(2) }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Step 3: Appointment Info (time + technician) -->
            <div v-if="createStep === 3">
              <div class="mb-3 space-x-4 text-sm text-gray-500">
                <span>会员：{{ selectedMember?.name }}</span>
                <span>| 宠物：{{ selectedPet?.name }}</span>
                <span>| 服务：{{ selectedService?.name }}</span>
              </div>

              <!-- Time -->
              <label class="block text-sm font-medium text-gray-700 mb-2">预约时间</label>
              <div class="flex gap-2 mb-4">
                <input
                  type="date"
                  v-model="appointmentDate"
                  :min="new Date().toISOString().split('T')[0]"
                  class="flex-1 border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
                <select
                  v-model="appointmentHour"
                  class="border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
                >
                  <option value="">时</option>
                  <option v-for="h in 24" :key="h-1" :value="String(h-1).padStart(2,'0')">
                    {{ String(h-1).padStart(2, '0') }}
                  </option>
                </select>
                <span class="text-gray-400 self-center">:</span>
                <select
                  v-model="appointmentMinute"
                  class="border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
                >
                  <option value="">分</option>
                  <option value="00">00</option>
                  <option value="15">15</option>
                  <option value="30">30</option>
                  <option value="45">45</option>
                </select>
              </div>

              <!-- Technician -->
              <label class="block text-sm font-medium text-gray-700 mb-2">指定技师</label>
              <div v-if="employeesLoading" class="text-center py-4 text-gray-400 text-sm">加载中...</div>
              <div v-else-if="!employees.length" class="text-center py-4 text-gray-400 text-sm">
                暂无可用技师
              </div>
              <div v-else class="max-h-40 overflow-y-auto border rounded divide-y mb-4">
                <div
                  v-for="e in employees"
                  :key="e.id"
                  @click="selectEmployee(e)"
                  :class="[
                    'px-4 py-3 cursor-pointer flex items-center justify-between',
                    selectedEmployee?.id === e.id ? 'bg-blue-50 border border-blue-300' : 'hover:bg-gray-50',
                  ]"
                >
                  <div>
                    <span class="text-sm font-medium text-gray-800">{{ e.name }}</span>
                    <span class="text-xs text-gray-400 ml-2">{{ e.position }}</span>
                  </div>
                  <span class="text-xs text-gray-400">{{ e.employee_no }}</span>
                </div>
              </div>

              <!-- Remark -->
              <label class="block text-sm font-medium text-gray-700 mb-2">备注</label>
              <textarea
                v-model="remark"
                placeholder="可选，备注信息..."
                rows="2"
                class="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500 mb-4"
              ></textarea>

              <!-- Info preview -->
              <div class="bg-gray-50 rounded p-3 text-sm text-gray-600 mb-4">
                <p><strong>会员：</strong>{{ selectedMember?.name }} ({{ selectedMember?.phone }})</p>
                <p><strong>宠物：</strong>{{ selectedPet?.name }} ({{ selectedPet?.breed }})</p>
                <p><strong>服务：</strong>{{ selectedService?.name }} ({{ selectedService?.duration_minutes }}分钟)</p>
                <p><strong>时间：</strong>{{ appointmentDate }} {{ appointmentHour }}:{{ appointmentMinute || '00' }}</p>
                <p><strong>技师：</strong>{{ selectedEmployee?.name || '未选择' }}</p>
              </div>
            </div>
          </div>

          <!-- Modal Footer -->
          <div class="px-6 py-4 border-t flex justify-end gap-2 sticky bottom-0 bg-white">
            <button
              v-if="createStep > 0"
              @click="createStep--"
              class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer"
            >
              上一步
            </button>
            <button
              @click="showCreateModal = false"
              class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer"
            >
              取消
            </button>
            <button
              v-if="createStep === 3"
              @click="submitAppointment"
              :disabled="createLoading"
              class="px-6 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 cursor-pointer disabled:opacity-50"
            >
              {{ createLoading ? '提交中...' : '提交预约' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
