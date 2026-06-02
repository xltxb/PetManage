<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold" style="color: var(--color-ink)">系统设置</h3>
      <button class="soft-btn" :disabled="loading" @click="load">刷新</button>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>

    <div class="grid grid-cols-[1fr_1.1fr] gap-4">
      <section class="kpi-card">
        <h4 class="text-sm font-semibold mb-3">配置项</h4>
        <div v-if="loading" class="muted">加载中...</div>
        <button v-for="item in visibleSettings" :key="item.key" class="setting-row" @click="select(item.key)">
          <span>
            <strong>{{ label(item.key) }}</strong>
            <small>{{ item.key }}</small>
          </span>
          <code>{{ valueSummary(item.value) }}</code>
        </button>
      </section>

      <section class="kpi-card space-y-3">
        <div class="flex items-center justify-between">
          <h4 class="font-semibold">编辑配置</h4>
          <select v-model="selectedKey" class="field key-select" @change="select(selectedKey)">
            <option v-for="key in keyOptions" :key="key" :value="key">{{ label(key) }}</option>
          </select>
        </div>
        <label class="label">配置键<input v-model.trim="selectedKey" class="field" /></label>
        <label class="label">配置值 JSON<textarea v-model="editor" class="field editor" spellcheck="false" /></label>
        <div class="quick-grid">
          <button class="soft-btn" @click="setBoolean(true)">true</button>
          <button class="soft-btn" @click="setBoolean(false)">false</button>
          <button class="soft-btn" @click="formatEditor">格式化</button>
        </div>
        <button class="primary-btn" :disabled="saving || !selectedKey" @click="save">保存设置</button>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import client from '../../api/client'

type SettingValue = boolean | number | string | Record<string, unknown> | unknown[]

const commonKeys = [
  'feature.sms_enabled',
  'feature.wechat_enabled',
  'feature.online_booking_enabled',
  'store.business_hours',
  'boarding.checkout_rule',
  'appointment.cancel_deadline_hours',
  'appointment.visit_reminder_hours',
  'pet.vaccine_remind_days',
  'inventory.allow_negative',
  'member.allow_downgrade',
  'member.churn_days',
  'points.rule',
]

const settings = ref<Record<string, SettingValue>>({})
const selectedKey = ref(commonKeys[0])
const editor = ref('false')
const loading = ref(false)
const saving = ref(false)
const error = ref('')

const keyOptions = computed(() => Array.from(new Set([...commonKeys, ...Object.keys(settings.value)])).sort())
const visibleSettings = computed(() => keyOptions.value.map((key) => ({ key, value: settings.value[key] })))

function label(key: string) {
  const map: Record<string, string> = {
    'feature.sms_enabled': '短信开关',
    'feature.wechat_enabled': '微信开关',
    'feature.online_booking_enabled': '线上预约',
    'store.business_hours': '营业时间',
    'boarding.checkout_rule': '寄养退房规则',
    'appointment.cancel_deadline_hours': '预约取消时限',
    'appointment.visit_reminder_hours': '到店提醒提前量',
    'pet.vaccine_remind_days': '疫苗提醒天数',
    'inventory.allow_negative': '库存允许负数',
    'member.allow_downgrade': '会员允许降级',
    'member.churn_days': '沉默会员天数',
    'points.rule': '积分规则',
  }
  return map[key] || key
}

function valueSummary(value: SettingValue | undefined) {
  if (value === undefined) return '未设置'
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > 42 ? `${text.slice(0, 42)}...` : text
}

function stringify(value: SettingValue | undefined) {
  if (value === undefined) return 'null'
  return JSON.stringify(value, null, 2)
}

function errorMessage(err: unknown) {
  const maybe = err as { response?: { data?: { message?: string } }; message?: string }
  return maybe.response?.data?.message || maybe.message || '操作失败'
}

function idem(action: string) {
  return { headers: { 'Idempotency-Key': `${action}-${Date.now()}-${Math.random().toString(16).slice(2)}` } }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await client.get('/settings')
    settings.value = data.data || {}
    select(selectedKey.value)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function select(key: string) {
  selectedKey.value = key
  editor.value = stringify(settings.value[key])
}

function setBoolean(value: boolean) {
  editor.value = JSON.stringify(value)
}

function formatEditor() {
  try {
    editor.value = JSON.stringify(JSON.parse(editor.value), null, 2)
    error.value = ''
  } catch {
    error.value = '配置值不是合法 JSON'
  }
}

async function save() {
  saving.value = true
  error.value = ''
  try {
    const parsed = JSON.parse(editor.value)
    await client.put(`/settings/${selectedKey.value}`, { value: parsed, updated_by: 0 }, idem('setting-save'))
    await load()
  } catch (err) {
    error.value = err instanceof SyntaxError ? '配置值不是合法 JSON' : errorMessage(err)
  } finally {
    saving.value = false
  }
}

onMounted(load)
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

.key-select {
  width: 200px;
}

.editor {
  min-height: 220px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  line-height: 1.5;
  resize: vertical;
}

.label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.muted {
  font-size: 14px;
  opacity: 0.55;
}

.setting-row {
  display: flex;
  width: 100%;
  justify-content: space-between;
  gap: 14px;
  padding: 12px 0;
  text-align: left;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.setting-row span {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.setting-row small {
  opacity: 0.55;
}

.setting-row code {
  max-width: 46%;
  overflow: hidden;
  color: var(--color-pine);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quick-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
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

@media (max-width: 960px) {
  .grid {
    grid-template-columns: 1fr;
  }
}
</style>
