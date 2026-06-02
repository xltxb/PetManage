<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">宠物档案</h3>
      <form class="lookup" @submit.prevent="loadByCustomer">
        <label class="sr-only" for="customerId">客户ID</label>
        <input id="customerId" v-model.number="customerId" class="field compact" type="number" min="1" />
        <button class="soft-btn" :disabled="loading">查询客户宠物</button>
      </form>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="grid grid-cols-[1.05fr_1.15fr_.9fr] gap-4">
      <section class="kpi-card">
        <div class="flex items-center justify-between mb-3">
          <h4 class="text-sm font-semibold">客户宠物</h4>
          <span class="text-xs" style="opacity: 0.55">{{ pets.length }} 只</span>
        </div>
        <div v-if="loading" class="muted">加载中...</div>
        <div v-else-if="pets.length === 0" class="muted">输入客户ID查询宠物</div>
        <button v-for="pet in pets" :key="pet.id" class="pet-row" @click="loadDetail(pet.id)">
          <span class="avatar">{{ pet.avatar_text || pet.name.slice(0, 1) }}</span>
          <span class="pet-info">
            <strong>{{ pet.name }}</strong>
            <small>{{ speciesLabel(pet.species) }} · {{ pet.breed || '未填品种' }} · {{ kg(pet.weight_g) }}</small>
          </span>
        </button>
      </section>

      <section class="kpi-card">
        <div class="flex items-center justify-between mb-3">
          <h4 class="text-sm font-semibold">档案详情</h4>
          <form class="lookup" @submit.prevent="loadDetail(petIdInput)">
            <label class="sr-only" for="petId">宠物ID</label>
            <input id="petId" v-model.number="petIdInput" class="field id-field" type="number" min="1" />
            <button class="soft-btn" :disabled="loading">查询</button>
          </form>
        </div>
        <div v-if="!detail" class="muted">选择宠物查看健康和体重记录</div>
        <div v-else class="space-y-3">
          <div class="detail-head">
            <span class="avatar large">{{ detail.pet.avatar_text || detail.pet.name.slice(0, 1) }}</span>
            <div>
              <h4 class="font-semibold">{{ detail.pet.name }}</h4>
              <p class="text-xs" style="opacity: 0.6">
                {{ speciesLabel(detail.pet.species) }} · {{ genderLabel(detail.pet.gender) }} · {{ detail.age_years }}岁{{ detail.age_months }}个月
              </p>
            </div>
          </div>
          <div class="metrics">
            <span>客户 {{ detail.pet.customer_id }}</span>
            <span>体重 {{ kg(detail.pet.weight_g) }}</span>
            <span>芯片 {{ detail.pet.chip_no || '-' }}</span>
          </div>
          <div>
            <h5 class="section-title">健康记录</h5>
            <div v-if="detail.health_records.length === 0" class="muted small">暂无健康记录</div>
            <div v-for="record in detail.health_records" :key="record.id" class="record-row">
              <span>{{ healthLabel(record.type) }} · {{ record.title }}</span>
              <small>{{ dateLabel(record.performed_at) }} / 下次 {{ dateLabel(record.next_due_at) }}</small>
            </div>
          </div>
          <div>
            <h5 class="section-title">体重记录</h5>
            <div v-if="detail.weight_records.length === 0" class="muted small">暂无体重记录</div>
            <div v-for="record in detail.weight_records" :key="record.id" class="record-row">
              <span>{{ kg(record.weight_g) }}</span>
              <small>{{ dateLabel(record.recorded_at) }}</small>
            </div>
          </div>
        </div>
      </section>

      <div class="space-y-4">
        <form class="kpi-card space-y-3" @submit.prevent="createPet">
          <h4 class="font-semibold">新建档案</h4>
          <label class="label">客户ID<input v-model.number="form.customer_id" class="field" type="number" min="1" required /></label>
          <label class="label">宠物名<input v-model.trim="form.name" class="field" required /></label>
          <div class="form-grid">
            <label class="label">物种
              <select v-model.number="form.species" class="field">
                <option :value="1">犬</option>
                <option :value="2">猫</option>
                <option :value="9">其他</option>
              </select>
            </label>
            <label class="label">性别
              <select v-model.number="form.gender" class="field">
                <option :value="1">公</option>
                <option :value="2">母</option>
                <option :value="0">未知</option>
              </select>
            </label>
          </div>
          <label class="label">品种<input v-model.trim="form.breed" class="field" /></label>
          <label class="label">生日<input v-model="form.birthday" class="field" type="date" /></label>
          <label class="label">体重（克）<input v-model.number="form.weight_g" class="field" type="number" min="0" /></label>
          <label class="label">芯片号<input v-model.trim="form.chip_no" class="field" /></label>
          <label class="label">备注<input v-model.trim="form.note" class="field" /></label>
          <label class="check"><input v-model="form.neutered" type="checkbox" /> 已绝育</label>
          <button class="primary-btn" :disabled="saving">创建档案</button>
        </form>

        <form class="kpi-card space-y-3" @submit.prevent="addHealth">
          <h4 class="font-semibold">健康登记</h4>
          <label class="label">类型
            <select v-model="healthForm.type" class="field">
              <option value="vaccine">疫苗</option>
              <option value="deworm">驱虫</option>
              <option value="exam">体检</option>
              <option value="allergy">过敏</option>
              <option value="other">其他</option>
            </select>
          </label>
          <label class="label">标题<input v-model.trim="healthForm.title" class="field" required /></label>
          <div class="form-grid">
            <label class="label">日期<input v-model="healthForm.performed_at" class="field" type="date" /></label>
            <label class="label">下次提醒<input v-model="healthForm.next_due_at" class="field" type="date" /></label>
          </div>
          <label class="label">详情<input v-model.trim="healthForm.detail" class="field" /></label>
          <button class="primary-btn" :disabled="saving || !detail">保存健康记录</button>
        </form>

        <form class="kpi-card space-y-3" @submit.prevent="addWeight">
          <h4 class="font-semibold">体重更新</h4>
          <label class="label">体重（克）<input v-model.number="weightG" class="field" type="number" min="1" required /></label>
          <button class="primary-btn" :disabled="saving || !detail">保存体重</button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import client from '../../api/client'

interface Pet {
  id: number
  customer_id: number
  name: string
  species: number
  breed: string
  gender: number
  neutered: boolean
  birthday?: string
  weight_g: number
  chip_no: string
  avatar_text: string
  note: string
}

interface HealthRecord {
  id: number
  type: string
  title: string
  performed_at?: string
  next_due_at?: string
}

interface WeightRecord {
  id: number
  weight_g: number
  recorded_at: string
}

interface PetDetail {
  pet: Pet
  age_years: number
  age_months: number
  health_records: HealthRecord[]
  weight_records: WeightRecord[]
}

const customerId = ref(1)
const petIdInput = ref(1)
const pets = ref<Pet[]>([])
const detail = ref<PetDetail | null>(null)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const weightG = ref(5000)
const form = reactive({
  customer_id: 1,
  name: '布丁',
  species: 1,
  breed: '',
  gender: 0,
  neutered: false,
  birthday: '',
  weight_g: 5000,
  chip_no: '',
  note: '',
})
const healthForm = reactive({
  type: 'vaccine',
  title: '年度疫苗',
  performed_at: new Date().toISOString().slice(0, 10),
  next_due_at: '',
  detail: '',
})

function kg(grams: number) {
  return grams ? `${(grams / 1000).toFixed(1)}kg` : '-'
}

function dateLabel(value?: string) {
  return value ? new Date(value).toLocaleDateString('zh-CN') : '-'
}

function speciesLabel(value: number) {
  const map: Record<number, string> = { 1: '犬', 2: '猫', 9: '其他' }
  return map[value] || '未知'
}

function genderLabel(value: number) {
  const map: Record<number, string> = { 1: '公', 2: '母', 0: '未知' }
  return map[value] || '未知'
}

function healthLabel(value: string) {
  const map: Record<string, string> = { vaccine: '疫苗', deworm: '驱虫', exam: '体检', allergy: '过敏', other: '其他' }
  return map[value] || value
}

function birthdayValue(date: string) {
  return date ? new Date(`${date}T00:00:00`).toISOString() : null
}

function errorMessage(err: unknown) {
  const maybe = err as { response?: { data?: { message?: string } }; message?: string }
  return maybe.response?.data?.message || maybe.message || '操作失败'
}

function idem(action: string) {
  return { headers: { 'Idempotency-Key': `${action}-${Date.now()}-${Math.random().toString(16).slice(2)}` } }
}

async function loadByCustomer() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await client.get(`/customers/${customerId.value}/pets`)
    pets.value = data.data || []
    if (pets.value.length > 0) {
      await loadDetail(pets.value[0].id)
    }
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function loadDetail(id: number) {
  if (!id) return
  loading.value = true
  error.value = ''
  try {
    const { data } = await client.get(`/pets/${id}`)
    detail.value = data.data
    petIdInput.value = id
    weightG.value = detail.value?.pet.weight_g || weightG.value
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function runAction(action: () => Promise<void>) {
  saving.value = true
  error.value = ''
  try {
    await action()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

async function createPet() {
  await runAction(async () => {
    const { data } = await client.post('/pets', {
      customer_id: form.customer_id,
      name: form.name,
      species: form.species,
      breed: form.breed,
      gender: form.gender,
      neutered: form.neutered,
      birthday: birthdayValue(form.birthday),
      weight_g: form.weight_g,
      chip_no: form.chip_no,
      note: form.note,
    }, idem('pet-create'))
    customerId.value = form.customer_id
    await loadByCustomer()
    await loadDetail(data.data.id)
  })
}

async function addHealth() {
  if (!detail.value) return
  const petID = detail.value.pet.id
  await runAction(async () => {
    await client.post(`/pets/${petID}/health`, {
      type: healthForm.type,
      title: healthForm.title,
      performed_at: healthForm.performed_at,
      next_due_at: healthForm.next_due_at,
      detail: healthForm.detail,
      operator_id: 0,
    }, idem('pet-health'))
    await loadDetail(petID)
  })
}

async function addWeight() {
  if (!detail.value) return
  const petID = detail.value.pet.id
  await runAction(async () => {
    await client.post(`/pets/${petID}/weights`, {
      weight_g: weightG.value,
      recorded_at: new Date().toISOString().slice(0, 10),
    }, idem('pet-weight'))
    await loadDetail(petID)
  })
}
</script>

<style scoped>
.field {
  width: 100%;
  border: 1px solid rgba(35, 30, 24, 0.12);
  border-radius: 8px;
  padding: 8px 10px;
  background: white;
  font-size: 14px;
  color: var(--color-ink);
}

.compact {
  width: 110px;
}

.id-field {
  width: 82px;
}

.lookup {
  display: flex;
  gap: 8px;
  align-items: center;
}

.label,
.check {
  display: flex;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.label {
  flex-direction: column;
}

.check {
  align-items: center;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.muted {
  font-size: 14px;
  opacity: 0.55;
}

.small {
  font-size: 12px;
}

.pet-row {
  display: flex;
  width: 100%;
  gap: 10px;
  align-items: center;
  padding: 11px 0;
  text-align: left;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.avatar {
  display: inline-grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 8px;
  color: white;
  background: var(--color-pine);
  font-weight: 700;
}

.avatar.large {
  width: 48px;
  height: 48px;
  font-size: 18px;
}

.pet-info {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.pet-info small,
.record-row small {
  opacity: 0.6;
}

.detail-head {
  display: flex;
  gap: 12px;
  align-items: center;
}

.metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  font-size: 12px;
}

.metrics span {
  border-radius: 8px;
  padding: 8px;
  background: var(--color-surface);
}

.section-title {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 700;
}

.record-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  font-size: 13px;
}

.primary-btn,
.soft-btn {
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
  font-weight: 600;
}

.primary-btn {
  width: 100%;
  color: white;
  background: var(--color-coral);
}

.soft-btn {
  background: var(--color-surface);
}

button:disabled {
  opacity: 0.6;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@media (max-width: 1160px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
