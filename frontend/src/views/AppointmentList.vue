<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
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
  confirmed: '已接单',
  arrived: '已到店',
  in_progress: '服务中',
  completed: '待取宠',
  picked_up: '已完成',
  cancelled: '已取消',
}

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  confirmed: 'bg-blue-100 text-blue-800',
  arrived: 'bg-indigo-100 text-indigo-800',
  in_progress: 'bg-purple-100 text-purple-800',
  completed: 'bg-orange-100 text-orange-800',
  picked_up: 'bg-green-100 text-green-800',
  cancelled: 'bg-gray-100 text-gray-500',
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
    if (appointmentDate.value && appointmentHour.value && appointmentMinute.value) {
      // Filter by on-duty employees for the selected date/time
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone
      const apptTime = `${appointmentDate.value}T${appointmentHour.value.padStart(2, '0')}:${appointmentMinute.value.padStart(2, '0')}:00`
      const result = await api.getOnDutyEmployees(apptTime)
      employees.value = (result.employees || []).map((e: any) => ({
        ...e,
        employee_no: '',
        status: 'active',
      }))
    } else {
      const result = await api.getEmployees({ status: 'active', page_size: 100 })
      employees.value = result.employees || []
    }
  } catch (e: any) {
    createError.value = e.message || '加载员工失败'
  } finally {
    employeesLoading.value = false
  }
}

// Reload employees when the appointment time changes in step 4.
watch([appointmentDate, appointmentHour, appointmentMinute], () => {
  if (createStep.value === 3 && appointmentDate.value && appointmentHour.value && appointmentMinute.value) {
    selectedEmployee.value = null
    loadEmployees()
  }
})

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

// Confirm dialog state
const confirmTarget = ref<Appointment | null>(null)
const actionLoading = ref(false)

// Reschedule modal state
const rescheduleTarget = ref<Appointment | null>(null)
const rsDate = ref('')
const rsHour = ref('')
const rsMinute = ref('00')
const rsReason = ref('')

// Cancel modal state
const cancelTarget = ref<Appointment | null>(null)
const cancelReason = ref('')
const actionError = ref('')

// New action states
const arriveTarget = ref<Appointment | null>(null)
const startTarget = ref<Appointment | null>(null)
const completeTarget = ref<Appointment | null>(null)
const pickupTarget = ref<Appointment | null>(null)

// Change log state
const changeLogTarget = ref<Appointment | null>(null)
const changeLogs = ref<any[]>([])
const logsLoading = ref(false)

const actionLabels: Record<string, string> = {
  confirmed: '接单',
  arrived: '到店',
  started: '开始服务',
  completed: '服务完成',
  picked_up: '取宠',
  rescheduled: '改期',
  cancelled: '取消',
}

const actionColors: Record<string, string> = {
  confirmed: 'bg-green-100 text-green-700',
  arrived: 'bg-indigo-100 text-indigo-700',
  started: 'bg-purple-100 text-purple-700',
  completed: 'bg-orange-100 text-orange-700',
  picked_up: 'bg-teal-100 text-teal-700',
  rescheduled: 'bg-blue-100 text-blue-700',
  cancelled: 'bg-red-100 text-red-700',
}

function openConfirmDialog(apt: Appointment) {
  confirmTarget.value = apt
}

async function doConfirm() {
  if (!confirmTarget.value) return
  actionLoading.value = true
  try {
    await api.confirmAppointment(confirmTarget.value.id)
    confirmTarget.value = null
    loadAppointments()
  } catch (e: any) {
    alert(e.message || '确认失败')
  } finally {
    actionLoading.value = false
  }
}

function openRescheduleModal(apt: Appointment) {
  rescheduleTarget.value = apt
  rsDate.value = ''
  rsHour.value = ''
  rsMinute.value = '00'
  rsReason.value = ''
  actionError.value = ''
}

async function doReschedule() {
  if (!rescheduleTarget.value) return
  if (!rsDate.value || !rsHour.value) {
    actionError.value = '请选择新的预约时间'
    return
  }
  const hour = rsHour.value.padStart(2, '0')
  const minute = rsMinute.value || '00'
  const timeStr = `${rsDate.value}T${hour}:${minute}:00+08:00`

  actionLoading.value = true
  actionError.value = ''
  try {
    await api.rescheduleAppointment(rescheduleTarget.value.id, {
      new_time: timeStr,
      reason: rsReason.value || undefined,
    })
    rescheduleTarget.value = null
    loadAppointments()
  } catch (e: any) {
    actionError.value = e.message || '改期失败'
  } finally {
    actionLoading.value = false
  }
}

function openCancelModal(apt: Appointment) {
  cancelTarget.value = apt
  cancelReason.value = ''
  actionError.value = ''
}

async function doCancel() {
  if (!cancelTarget.value) return
  actionLoading.value = true
  actionError.value = ''
  try {
    await api.cancelAppointment(cancelTarget.value.id, {
      reason: cancelReason.value || undefined,
    })
    cancelTarget.value = null
    loadAppointments()
  } catch (e: any) {
    actionError.value = e.message || '取消失败'
  } finally {
    actionLoading.value = false
  }
}

async function openChangeLogs(apt: Appointment) {
  changeLogTarget.value = apt
  logsLoading.value = true
  try {
    const result = await api.getAppointmentChangeLogs(apt.id)
    changeLogs.value = result.logs || []
  } catch (e: any) {
    changeLogs.value = []
  } finally {
    logsLoading.value = false
  }
}

// --- New life cycle actions ---

function openArriveDialog(apt: Appointment) {
  arriveTarget.value = apt
}

async function doArrive() {
  if (!arriveTarget.value) return
  actionLoading.value = true
  try {
    await api.arriveAppointment(arriveTarget.value.id)
    arriveTarget.value = null
    loadAppointments()
  } catch (e: any) {
    alert(e.message || '\u5230\u5e97\u786e\u8ba4\u5931\u8d25')
  } finally {
    actionLoading.value = false
  }
}

function openStartDialog(apt: Appointment) {
  startTarget.value = apt
}

async function doStart() {
  if (!startTarget.value) return
  actionLoading.value = true
  try {
    await api.startAppointment(startTarget.value.id)
    startTarget.value = null
    loadAppointments()
  } catch (e: any) {
    alert(e.message || '\u5f00\u59cb\u670d\u52a1\u5931\u8d25')
  } finally {
    actionLoading.value = false
  }
}

function openCompleteDialog(apt: Appointment) {
  completeTarget.value = apt
}

async function doComplete() {
  if (!completeTarget.value) return
  actionLoading.value = true
  try {
    await api.completeAppointment(completeTarget.value.id)
    completeTarget.value = null
    loadAppointments()
  } catch (e: any) {
    alert(e.message || '\u5b8c\u6210\u670d\u52a1\u5931\u8d25')
  } finally {
    actionLoading.value = false
  }
}

function openPickupDialog(apt: Appointment) {
  pickupTarget.value = apt
}

async function doPickup() {
  if (!pickupTarget.value) return
  actionLoading.value = true
  try {
    await api.pickupAppointment(pickupTarget.value.id)
    pickupTarget.value = null
    loadAppointments()
  } catch (e: any) {
    alert(e.message || '\u53d6\u5ba0\u786e\u8ba4\u5931\u8d25')
  } finally {
    actionLoading.value = false
  }
}

// Progress step definitions for visualization
const progressSteps = ['pending', 'confirmed', 'arrived', 'in_progress', 'completed', 'picked_up']
const progressStepLabels: Record<string, string> = {
  pending: '\u5f85\u786e\u8ba4',
  confirmed: '\u5df2\u63a5\u5355',
  arrived: '\u5df2\u5230\u5e97',
  in_progress: '\u670d\u52a1\u4e2d',
  completed: '\u5f85\u53d6\u5ba0',
  picked_up: '\u5df2\u5b8c\u6210',
}

function getProgressStepIndex(status: string): number {
  return progressSteps.indexOf(status)
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
          <button
            @click="router.push('/merchant/schedules')"
            class="text-sm text-blue-600 hover:text-blue-800 cursor-pointer ml-4"
          >
            排班管理 →
          </button>
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
          v-for="tab in [{k:'',l:'全部'},{k:'pending',l:'待确认'},{k:'confirmed',l:'已接单'},{k:'arrived',l:'已到店'},{k:'in_progress',l:'服务中'},{k:'completed',l:'待取宠'},{k:'picked_up',l:'已完成'},{k:'cancelled',l:'已取消'}]"
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
              <th class="text-left px-4 py-3 font-medium">操作</th>
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
              <td class="px-4 py-3">
                <div class="flex gap-1 flex-wrap">
                  <button
                    v-if="apt.status === 'pending'"
                    @click="openConfirmDialog(apt)"
                    class="text-xs bg-green-100 text-green-700 px-2 py-1 rounded hover:bg-green-200 cursor-pointer"
                  >接单</button>
                  <button
                    v-if="apt.status === 'confirmed'"
                    @click="openArriveDialog(apt)"
                    class="text-xs bg-indigo-100 text-indigo-700 px-2 py-1 rounded hover:bg-indigo-200 cursor-pointer"
                  >到店</button>
                  <button
                    v-if="apt.status === 'arrived'"
                    @click="openStartDialog(apt)"
                    class="text-xs bg-purple-100 text-purple-700 px-2 py-1 rounded hover:bg-purple-200 cursor-pointer"
                  >开始服务</button>
                  <button
                    v-if="apt.status === 'in_progress'"
                    @click="openCompleteDialog(apt)"
                    class="text-xs bg-orange-100 text-orange-700 px-2 py-1 rounded hover:bg-orange-200 cursor-pointer"
                  >完成服务</button>
                  <button
                    v-if="apt.status === 'completed'"
                    @click="openPickupDialog(apt)"
                    class="text-xs bg-teal-100 text-teal-700 px-2 py-1 rounded hover:bg-teal-200 cursor-pointer"
                  >确认取宠</button>
                  <button
                    v-if="apt.status === 'pending' || apt.status === 'confirmed' || apt.status === 'arrived' || apt.status === 'in_progress'"
                    @click="openRescheduleModal(apt)"
                    class="text-xs bg-blue-100 text-blue-700 px-2 py-1 rounded hover:bg-blue-200 cursor-pointer"
                  >改期</button>
                  <button
                    v-if="apt.status !== 'cancelled' && apt.status !== 'picked_up' && apt.status !== 'completed'"
                    @click="openCancelModal(apt)"
                    class="text-xs bg-red-100 text-red-700 px-2 py-1 rounded hover:bg-red-200 cursor-pointer"
                  >取消</button>
                  <button
                    @click="openChangeLogs(apt)"
                    class="text-xs bg-gray-100 text-gray-600 px-2 py-1 rounded hover:bg-gray-200 cursor-pointer"
                  >记录</button>
                </div>
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

    <!-- Confirm Dialog -->
    <Teleport to="body">
      <div v-if="confirmTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="confirmTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">接单确认</h2>
          </div>
          <div class="px-6 py-4">
            <p class="text-sm text-gray-600">接单后状态将变为「已接单」，会员将收到通知。</p>
            <div class="bg-gray-50 rounded p-3 mt-3 text-sm text-gray-600">
              <p>会员：{{ confirmTarget.member_name }}</p>
              <p>时间：{{ formatTime(confirmTarget.appointment_time) }}</p>
            </div>
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="confirmTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doConfirm" :disabled="actionLoading" class="px-4 py-2 text-sm bg-green-600 text-white rounded hover:bg-green-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '确认中...' : '确认接单' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Reschedule Modal -->
    <Teleport to="body">
      <div v-if="rescheduleTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="rescheduleTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">改期预约</h2>
          </div>
          <div class="px-6 py-4">
            <div v-if="actionError" class="bg-red-50 text-red-600 text-sm p-3 rounded mb-3">{{ actionError }}</div>
            <div class="bg-gray-50 rounded p-3 mb-3 text-sm text-gray-600">
              <p>当前时间：{{ formatTime(rescheduleTarget.appointment_time) }}</p>
              <p>技师：{{ rescheduleTarget.employee_name }}</p>
            </div>
            <label class="block text-sm font-medium text-gray-700 mb-2">新预约时间</label>
            <div class="flex gap-2 mb-3">
              <input type="date" v-model="rsDate" :min="new Date().toISOString().split('T')[0]" class="flex-1 border rounded px-3 py-2 text-sm" />
              <select v-model="rsHour" class="border rounded px-3 py-2 text-sm"><option value="">时</option><option v-for="h in 24" :key="h-1" :value="String(h-1).padStart(2,'0')">{{ String(h-1).padStart(2,'0') }}</option></select>
              <span class="text-gray-400 self-center">:</span>
              <select v-model="rsMinute" class="border rounded px-3 py-2 text-sm"><option value="00">00</option><option value="15">15</option><option value="30">30</option><option value="45">45</option></select>
            </div>
            <label class="block text-sm font-medium text-gray-700 mb-2">改期原因</label>
            <input v-model="rsReason" placeholder="可选，填写改期原因..." class="w-full border rounded px-3 py-2 text-sm mb-3" />
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="rescheduleTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doReschedule" :disabled="actionLoading" class="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '改期中...' : '确认改期' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Cancel Modal -->
    <Teleport to="body">
      <div v-if="cancelTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="cancelTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">取消预约</h2>
          </div>
          <div class="px-6 py-4">
            <div v-if="actionError" class="bg-red-50 text-red-600 text-sm p-3 rounded mb-3">{{ actionError }}</div>
            <div class="bg-gray-50 rounded p-3 mb-3 text-sm text-gray-600">
              <p>会员：{{ cancelTarget.member_name }}</p>
              <p>时间：{{ formatTime(cancelTarget.appointment_time) }}</p>
              <p>技师：{{ cancelTarget.employee_name }}</p>
            </div>
            <label class="block text-sm font-medium text-gray-700 mb-2">取消原因</label>
            <textarea v-model="cancelReason" placeholder="请填写取消原因..." rows="2" class="w-full border rounded px-3 py-2 text-sm mb-3"></textarea>
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="cancelTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doCancel" :disabled="actionLoading" class="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '取消中...' : '确认取消' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Arrive Dialog -->
    <Teleport to="body">
      <div v-if="arriveTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="arriveTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">确认到店</h2>
          </div>
          <div class="px-6 py-4">
            <p class="text-sm text-gray-600">确认宠物已到店，状态将变为「已到店」。</p>
            <div class="bg-gray-50 rounded p-3 mt-3 text-sm text-gray-600">
              <p>会员：{{ arriveTarget.member_name }}</p>
              <p>宠物：{{ arriveTarget.pet_name }}</p>
              <p>时间：{{ formatTime(arriveTarget.appointment_time) }}</p>
            </div>
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="arriveTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doArrive" :disabled="actionLoading" class="px-4 py-2 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '确认中...' : '确认到店' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Start Dialog -->
    <Teleport to="body">
      <div v-if="startTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="startTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">开始服务</h2>
          </div>
          <div class="px-6 py-4">
            <p class="text-sm text-gray-600">确认开始服务，状态将变为「服务中」。</p>
            <div class="bg-gray-50 rounded p-3 mt-3 text-sm text-gray-600">
              <p>会员：{{ startTarget.member_name }}</p>
              <p>宠物：{{ startTarget.pet_name }}</p>
              <p>服务：{{ startTarget.service_item_name }}</p>
            </div>
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="startTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doStart" :disabled="actionLoading" class="px-4 py-2 text-sm bg-purple-600 text-white rounded hover:bg-purple-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '确认中...' : '开始服务' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Complete Dialog -->
    <Teleport to="body">
      <div v-if="completeTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="completeTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">完成服务</h2>
          </div>
          <div class="px-6 py-4">
            <p class="text-sm text-gray-600">确认服务已完成，状态将变为「待取宠」，会员将收到取宠通知。</p>
            <div class="bg-gray-50 rounded p-3 mt-3 text-sm text-gray-600">
              <p>会员：{{ completeTarget.member_name }}</p>
              <p>宠物：{{ completeTarget.pet_name }}</p>
              <p>服务：{{ completeTarget.service_item_name }}</p>
            </div>
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="completeTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doComplete" :disabled="actionLoading" class="px-4 py-2 text-sm bg-orange-600 text-white rounded hover:bg-orange-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '确认中...' : '完成服务' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Pickup Dialog -->
    <Teleport to="body">
      <div v-if="pickupTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="pickupTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-sm mx-4">
          <div class="px-6 py-4 border-b">
            <h2 class="text-lg font-semibold">确认取宠</h2>
          </div>
          <div class="px-6 py-4">
            <p class="text-sm text-gray-600">确认客户已取宠，状态将变为「已完成」。</p>
            <div class="bg-gray-50 rounded p-3 mt-3 text-sm text-gray-600">
              <p>会员：{{ pickupTarget.member_name }}</p>
              <p>宠物：{{ pickupTarget.pet_name }}</p>
              <p>服务：{{ pickupTarget.service_item_name }}</p>
            </div>
          </div>
          <div class="px-6 py-4 border-t flex justify-end gap-2">
            <button @click="pickupTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">取消</button>
            <button @click="doPickup" :disabled="actionLoading" class="px-4 py-2 text-sm bg-teal-600 text-white rounded hover:bg-teal-700 cursor-pointer disabled:opacity-50">
              {{ actionLoading ? '确认中...' : '确认取宠' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Change Logs Modal -->
    <Teleport to="body">
      <div v-if="changeLogTarget" class="fixed inset-0 bg-black bg-opacity-40 z-50 flex items-center justify-center" @click.self="changeLogTarget = null">
        <div class="bg-white rounded-lg shadow-xl w-full max-w-lg mx-4 max-h-[80vh] overflow-y-auto">
          <div class="px-6 py-4 border-b sticky top-0 bg-white z-10">
            <h2 class="text-lg font-semibold">变更记录 - {{ changeLogTarget.member_name }}</h2>
          </div>
          <div class="px-6 py-4">
            <!-- Progress Stepper -->
            <div v-if="changeLogTarget" class="mb-4">
              <p class="text-sm font-medium text-gray-700 mb-2">服务进度</p>
              <div class="flex items-center gap-1 overflow-x-auto pb-2">
                <template v-for="(step, idx) in progressSteps" :key="step">
                  <div class="flex flex-col items-center gap-1 min-w-[60px]">
                    <div
                      :class="[
                        'w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium',
                        getProgressStepIndex(changeLogTarget.status) >= idx
                          ? changeLogTarget.status === 'cancelled'
                            ? 'bg-red-500 text-white'
                            : 'bg-green-500 text-white'
                          : 'bg-gray-200 text-gray-500',
                      ]"
                    >
                      {{ getProgressStepIndex(changeLogTarget.status) >= idx ? '✓' : idx + 1 }}
                    </div>
                    <span class="text-[10px] whitespace-nowrap"
                      :class="getProgressStepIndex(changeLogTarget.status) >= idx ? 'text-gray-700 font-medium' : 'text-gray-400'"
                    >{{ progressStepLabels[step] }}</span>
                  </div>
                  <div
                    v-if="idx < progressSteps.length - 1"
                    :class="[
                      'h-0.5 flex-1 min-w-[8px]',
                      getProgressStepIndex(changeLogTarget.status) > idx ? 'bg-green-500' : 'bg-gray-200',
                    ]"
                  ></div>
                </template>
              </div>
            </div>
            <div v-if="logsLoading" class="text-center py-4 text-gray-400 text-sm">加载中...</div>
            <div v-else-if="!changeLogs.length" class="text-center py-4 text-gray-400 text-sm">暂无变更记录</div>
            <div v-else class="space-y-3">
              <div v-for="log in changeLogs" :key="log.id" class="border rounded p-3 text-sm">
                <div class="flex items-center justify-between mb-1">
                  <span :class="['px-2 py-0.5 rounded text-xs', actionColors[log.action] || 'bg-gray-100']">
                    {{ actionLabels[log.action] || log.action }}
                  </span>
                  <span class="text-xs text-gray-400">{{ new Date(log.created_at).toLocaleString('zh-CN') }}</span>
                </div>
                <div class="text-gray-600" v-if="log.action === 'confirmed' || log.action === 'arrived' || log.action === 'started' || log.action === 'completed' || log.action === 'picked_up'">
                  状态：{{ log.old_value?.status }} → {{ log.new_value?.status }}
                </div>
                <div class="text-gray-600" v-if="log.action === 'rescheduled'">时间：{{ log.new_value?.appointment_time }}</div>
                <div class="text-gray-600" v-if="log.action === 'cancelled'">状态：{{ log.old_value?.status }} → 已取消</div>
                <div v-if="log.reason" class="text-gray-400 text-xs mt-1">原因：{{ log.reason }}</div>
              </div>
            </div>
          </div>
          <div class="px-6 py-4 border-t flex justify-end">
            <button @click="changeLogTarget = null" class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer">关闭</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
