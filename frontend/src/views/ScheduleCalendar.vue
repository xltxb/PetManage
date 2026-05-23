<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'

const router = useRouter()
const auth = useAuthStore()

interface Employee {
  id: number
  name: string
  position: string
  employee_no: string
  status: string
}

interface Schedule {
  id: number
  employee_id: number
  schedule_date: string
  shift_type: string
}

const employees = ref<Employee[]>([])
const selectedEmployeeId = ref<number | null>(null)
const schedules = ref<Schedule[]>([])
const weekStart = ref(getMonday(new Date()))
const loading = ref(false)
const error = ref('')
const success = ref('')

const shiftLabels: Record<string, string> = {
  morning: '早班 (09-17)',
  evening: '晚班 (13-21)',
  rest: '休息',
}

const shiftColors: Record<string, string> = {
  morning: 'bg-yellow-100 text-yellow-800 border-yellow-300',
  evening: 'bg-blue-100 text-blue-800 border-blue-300',
  rest: 'bg-gray-100 text-gray-500 border-gray-300',
}

function getMonday(d: Date): string {
  const date = new Date(d)
  const day = date.getDay()
  const diff = date.getDate() - day + (day === 0 ? -6 : 1)
  date.setDate(diff)
  return date.toISOString().split('T')[0]
}

const weekDays = computed(() => {
  const days: string[] = []
  const start = new Date(weekStart.value)
  for (let i = 0; i < 7; i++) {
    const d = new Date(start)
    d.setDate(d.getDate() + i)
    days.push(d.toISOString().split('T')[0])
  }
  return days
})

const weekLabel = computed(() => {
  const start = weekDays.value[0]
  const end = weekDays.value[6]
  return `${start} ~ ${end}`
})

function prevWeek() {
  const d = new Date(weekStart.value)
  d.setDate(d.getDate() - 7)
  weekStart.value = getMonday(d)
}

function nextWeek() {
  const d = new Date(weekStart.value)
  d.setDate(d.getDate() + 7)
  weekStart.value = getMonday(d)
}

async function loadEmployees() {
  try {
    const result = await api.getEmployees({ status: 'active', page_size: 100 })
    employees.value = result.employees || []
    if (employees.value.length > 0 && !selectedEmployeeId.value) {
      selectedEmployeeId.value = employees.value[0].id
    }
  } catch (e: any) {
    error.value = e.message || '加载员工失败'
  }
}

async function loadSchedules() {
  if (!selectedEmployeeId.value) return
  loading.value = true
  error.value = ''
  try {
    const result = await api.getSchedules({
      employee_id: selectedEmployeeId.value,
      start_date: weekDays.value[0],
      end_date: weekDays.value[6],
    })
    schedules.value = result.schedules || []
  } catch (e: any) {
    error.value = e.message || '加载排班失败'
  } finally {
    loading.value = false
  }
}

function getShift(date: string): string {
  const s = schedules.value.find((s) => {
    const sd = s.schedule_date.split('T')[0]
    return sd === date
  })
  return s?.shift_type || 'morning'
}

async function toggleShift(date: string) {
  const current = getShift(date)
  const next = current === 'morning' ? 'evening' : current === 'evening' ? 'rest' : 'morning'
  try {
    await api.upsertSchedule({
      employee_id: selectedEmployeeId.value!,
      schedule_date: date,
      shift_type: next,
    })
    await loadSchedules()
    success.value = `${date} 已更新为 ${shiftLabels[next]}`
    setTimeout(() => (success.value = ''), 2000)
  } catch (e: any) {
    error.value = e.message || '更新排班失败'
  }
}

async function batchSetDefault() {
  if (!selectedEmployeeId.value) return
  const schedules = weekDays.value.map((date) => {
    const d = new Date(date)
    const isWeekend = d.getDay() === 0 || d.getDay() === 6
    return { date, shift_type: isWeekend ? 'rest' : 'morning' }
  })
  try {
    await api.batchSetSchedules({ employee_id: selectedEmployeeId.value, schedules })
    await loadSchedules()
    success.value = '已快速设置为周一至周五早班，周末休息'
    setTimeout(() => (success.value = ''), 2000)
  } catch (e: any) {
    error.value = e.message || '批量设置失败'
  }
}

// Copy week dialog
const showCopyModal = ref(false)
const copyFromEmployeeId = ref<number | null>(null)
const copyFromWeekStart = ref('')

function openCopyModal() {
  copyFromWeekStart.value = weekStart.value
  showCopyModal.value = true
}

async function doCopyWeek() {
  if (!selectedEmployeeId.value || !copyFromEmployeeId.value) return
  try {
    await api.copyWeekSchedules({
      from_employee_id: copyFromEmployeeId.value,
      to_employee_id: selectedEmployeeId.value,
      from_week_start: copyFromWeekStart.value,
      to_week_start: weekStart.value,
    })
    await loadSchedules()
    showCopyModal.value = false
    success.value = '排班复制成功'
    setTimeout(() => (success.value = ''), 2000)
  } catch (e: any) {
    error.value = e.message || '复制排班失败'
  }
}

const selectedEmployee = computed(() =>
  employees.value.find((e) => e.id === selectedEmployeeId.value)
)

watch(selectedEmployeeId, () => loadSchedules())
watch(weekStart, () => loadSchedules())

onMounted(async () => {
  if (!auth.user) {
    router.replace('/merchant/login')
    return
  }
  await loadEmployees()
  if (selectedEmployeeId.value) await loadSchedules()
})
</script>

<template>
  <div class="p-6 max-w-7xl mx-auto">
    <h1 class="text-2xl font-bold mb-4">技师排班管理</h1>

    <div v-if="error" class="mb-4 p-3 bg-red-50 text-red-600 rounded text-sm">{{ error }}</div>
    <div v-if="success" class="mb-4 p-3 bg-green-50 text-green-600 rounded text-sm">{{ success }}</div>

    <!-- Top Controls -->
    <div class="flex flex-wrap items-center gap-3 mb-4">
      <!-- Employee Selector -->
      <select
        v-model="selectedEmployeeId"
        class="border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
      >
        <option :value="null" disabled>选择技师</option>
        <option v-for="e in employees" :key="e.id" :value="e.id">
          {{ e.name }} ({{ e.position }})
        </option>
      </select>

      <!-- Week Navigation -->
      <button
        @click="prevWeek"
        class="px-3 py-2 text-sm border rounded hover:bg-gray-50 cursor-pointer"
      >
        ← 上一周
      </button>
      <span class="text-sm font-medium">{{ weekLabel }}</span>
      <button
        @click="nextWeek"
        class="px-3 py-2 text-sm border rounded hover:bg-gray-50 cursor-pointer"
      >
        下一周 →
      </button>

      <!-- Actions -->
      <button
        @click="batchSetDefault"
        class="px-3 py-2 text-sm bg-green-600 text-white rounded hover:bg-green-700 cursor-pointer"
      >
        快速设置 (周一至五早班)
      </button>
      <button
        @click="openCopyModal"
        class="px-3 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 cursor-pointer"
      >
        从其他技师复制
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center py-8 text-gray-400">加载中...</div>

    <!-- Calendar Grid -->
    <div v-else-if="selectedEmployee" class="bg-white border rounded shadow-sm overflow-x-auto">
      <div class="grid grid-cols-7 gap-px bg-gray-200 min-w-[700px]">
        <!-- Day Headers -->
        <div
          v-for="(date, idx) in weekDays"
          :key="date"
          class="bg-gray-50 px-3 py-2 text-center text-sm font-medium"
          :class="idx >= 5 ? 'text-gray-400' : 'text-gray-700'"
        >
          <div>{{ ['一','二','三','四','五','六','日'][idx] }}</div>
          <div class="text-xs">{{ date.slice(5) }}</div>
        </div>

        <!-- Schedule Cells -->
        <div
          v-for="date in weekDays"
          :key="'sched-' + date"
          @click="toggleShift(date)"
          :class="[
            'px-3 py-4 text-center cursor-pointer border transition-colors hover:opacity-80',
            shiftColors[getShift(date)] || 'bg-gray-50',
          ]"
        >
          <div class="text-sm font-medium">{{ shiftLabels[getShift(date)] }}</div>
        </div>
      </div>
    </div>

    <!-- Legend -->
    <div class="flex gap-4 mt-4 text-sm">
      <div class="flex items-center gap-1">
        <div class="w-4 h-4 rounded bg-yellow-100 border border-yellow-300"></div>
        <span>早班 (09:00-17:00)</span>
      </div>
      <div class="flex items-center gap-1">
        <div class="w-4 h-4 rounded bg-blue-100 border border-blue-300"></div>
        <span>晚班 (13:00-21:00)</span>
      </div>
      <div class="flex items-center gap-1">
        <div class="w-4 h-4 rounded bg-gray-100 border border-gray-300"></div>
        <span>休息</span>
      </div>
    </div>

    <!-- Copy Week Modal -->
    <div v-if="showCopyModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div class="bg-white rounded-lg shadow-xl w-full max-w-md mx-4">
        <div class="px-6 py-4 border-b">
          <h2 class="text-lg font-semibold">复制排班</h2>
        </div>
        <div class="px-6 py-4 space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">从哪个技师复制?</label>
            <select
              v-model="copyFromEmployeeId"
              class="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              <option :value="null" disabled>选择技师</option>
              <option
                v-for="e in employees"
                :key="e.id"
                :value="e.id"
                :disabled="e.id === selectedEmployeeId"
              >
                {{ e.name }} ({{ e.position }})
              </option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">源周起始日期</label>
            <input
              type="date"
              v-model="copyFromWeekStart"
              class="w-full border rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>
          <p class="text-sm text-gray-500">
            将排班复制到 <strong>{{ selectedEmployee?.name }}</strong> 的当前周
            ({{ weekDays[0] }} ~ {{ weekDays[6] }})
          </p>
        </div>
        <div class="px-6 py-4 border-t flex justify-end gap-2">
          <button
            @click="showCopyModal = false"
            class="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded cursor-pointer"
          >
            取消
          </button>
          <button
            @click="doCopyWeek"
            :disabled="!copyFromEmployeeId"
            class="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 cursor-pointer"
          >
            确认复制
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
