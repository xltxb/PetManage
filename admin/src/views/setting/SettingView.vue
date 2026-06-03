<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-lg font-semibold" style="color: var(--color-ink)">系统设置</h3>
        <p class="muted mt-1">按门店保存运营参数，无需编辑 JSON。</p>
      </div>
      <button class="soft-btn" :disabled="loading" @click="() => load()">刷新</button>
    </div>

    <p v-if="error" class="text-sm" style="color: var(--color-berry)">{{ error }}</p>
    <p v-if="success" class="text-sm" style="color: var(--color-pine)">{{ success }}</p>

    <div class="settings-grid">
      <section class="kpi-card setting-section">
        <h4>功能开关</h4>
        <label class="toggle-row">
          <span>
            <strong>短信通知</strong>
            <small>控制 sms 渠道是否实际发送。</small>
          </span>
          <input v-model="form.smsEnabled" type="checkbox" />
        </label>
        <label class="toggle-row">
          <span>
            <strong>微信公众号通知</strong>
            <small>开启后 mock WeChat MP 通知记为 sent。</small>
          </span>
          <input v-model="form.wechatEnabled" type="checkbox" />
        </label>
        <label class="toggle-row">
          <span>
            <strong>线上预约</strong>
            <small>允许顾客端提交自助预约。</small>
          </span>
          <input v-model="form.onlineBookingEnabled" type="checkbox" />
        </label>
      </section>

      <section class="kpi-card setting-section">
        <h4>营业与预约</h4>
        <div class="field-grid two">
          <label class="label">开门时间<input v-model="form.businessOpen" type="time" class="field" /></label>
          <label class="label">打烊时间<input v-model="form.businessClose" type="time" class="field" /></label>
        </div>
        <div class="field-grid three">
          <label class="label">取消截止小时<input v-model.number="form.cancelDeadlineHours" type="number" min="0" max="720" step="1" class="field" /></label>
          <label class="label">到店提醒提前小时<input v-model.number="form.visitReminderHours" type="number" min="0" max="720" step="1" class="field" /></label>
          <label class="label">疫苗提醒天数<input v-model.number="form.vaccineRemindDays" type="number" min="1" max="3650" step="1" class="field" /></label>
        </div>
      </section>

      <section class="kpi-card setting-section">
        <h4>寄养退房规则</h4>
        <div class="field-grid two">
          <label class="label">计费取整
            <select v-model="form.checkoutRound" class="field">
              <option value="ceil">向上取整</option>
              <option value="floor">向下取整</option>
              <option value="round">四舍五入</option>
            </select>
          </label>
          <label class="label">最少计费晚数<input v-model.number="form.minNights" type="number" min="1" max="365" step="1" class="field" /></label>
        </div>
        <label class="toggle-row compact">
          <span>
            <strong>寄养参与会员折扣</strong>
            <small>默认关闭，寄养按原价计费。</small>
          </span>
          <input v-model="form.applyMemberDiscount" type="checkbox" />
        </label>
      </section>

      <section class="kpi-card setting-section">
        <h4>库存、会员与积分</h4>
        <label class="toggle-row">
          <span>
            <strong>允许负库存</strong>
            <small>关闭时库存不足会阻止出库。</small>
          </span>
          <input v-model="form.allowNegativeInventory" type="checkbox" />
        </label>
        <label class="toggle-row">
          <span>
            <strong>允许会员降级</strong>
            <small>关闭时会员等级只升不降。</small>
          </span>
          <input v-model="form.allowMemberDowngrade" type="checkbox" />
        </label>
        <div class="field-grid three">
          <label class="label">沉默会员天数<input v-model.number="form.churnDays" type="number" min="1" max="3650" step="1" class="field" /></label>
          <label class="label">每元积分<input v-model.number="form.pointsPerYuan" type="number" min="0" max="100" step="0.1" class="field" /></label>
          <label class="label">按等级倍率
            <select v-model="form.pointsByTierRate" class="field">
              <option :value="true">开启</option>
              <option :value="false">关闭</option>
            </select>
          </label>
        </div>
        <label class="toggle-row compact">
          <span>
            <strong>充值赠送积分</strong>
            <small>默认关闭，充值不计积分。</small>
          </span>
          <input v-model="form.rechargeEarnPoints" type="checkbox" />
        </label>
      </section>
    </div>

    <div class="save-bar">
      <button class="primary-btn" :disabled="loading || saving" @click="saveAllSettings">
        {{ saving ? '保存中...' : '保存全部设置' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import client from '../../api/client'

type SettingsResponse = Record<string, unknown>

interface SettingForm {
  smsEnabled: boolean
  wechatEnabled: boolean
  onlineBookingEnabled: boolean
  businessOpen: string
  businessClose: string
  checkoutRound: 'ceil' | 'floor' | 'round'
  minNights: number
  applyMemberDiscount: boolean
  cancelDeadlineHours: number
  visitReminderHours: number
  vaccineRemindDays: number
  allowNegativeInventory: boolean
  allowMemberDowngrade: boolean
  churnDays: number
  pointsPerYuan: number
  pointsByTierRate: boolean
  rechargeEarnPoints: boolean
}

const form = reactive<SettingForm>({
  smsEnabled: false,
  wechatEnabled: false,
  onlineBookingEnabled: true,
  businessOpen: '09:00',
  businessClose: '21:00',
  checkoutRound: 'ceil',
  minNights: 1,
  applyMemberDiscount: false,
  cancelDeadlineHours: 2,
  visitReminderHours: 24,
  vaccineRemindDays: 7,
  allowNegativeInventory: false,
  allowMemberDowngrade: false,
  churnDays: 30,
  pointsPerYuan: 1,
  pointsByTierRate: true,
  rechargeEarnPoints: false,
})

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const success = ref('')

function boolValue(value: unknown, fallback: boolean) {
  return typeof value === 'boolean' ? value : fallback
}

function numberValue(value: unknown, fallback: number) {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function objectValue(value: unknown) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {}
}

function applySettings(settings: SettingsResponse) {
  form.smsEnabled = boolValue(settings['feature.sms_enabled'], false)
  form.wechatEnabled = boolValue(settings['feature.wechat_enabled'], false)
  form.onlineBookingEnabled = boolValue(settings['feature.online_booking_enabled'], true)

  const hours = objectValue(settings['store.business_hours'])
  form.businessOpen = typeof hours.open === 'string' ? hours.open : '09:00'
  form.businessClose = typeof hours.close === 'string' ? hours.close : '21:00'

  const checkout = objectValue(settings['boarding.checkout_rule'])
  form.checkoutRound = checkout.round === 'floor' || checkout.round === 'round' ? checkout.round : 'ceil'
  form.minNights = numberValue(checkout.min_nights, 1)
  form.applyMemberDiscount = boolValue(checkout.apply_member_discount, false)

  form.cancelDeadlineHours = numberValue(settings['appointment.cancel_deadline_hours'], 2)
  form.visitReminderHours = numberValue(settings['appointment.visit_reminder_hours'], 24)
  form.vaccineRemindDays = numberValue(settings['pet.vaccine_remind_days'], 7)
  form.allowNegativeInventory = boolValue(settings['inventory.allow_negative'], false)
  form.allowMemberDowngrade = boolValue(settings['member.allow_downgrade'], false)
  form.churnDays = numberValue(settings['member.churn_days'], 30)

  const points = objectValue(settings['points.rule'])
  form.pointsPerYuan = numberValue(points.per_yuan, 1)
  form.pointsByTierRate = boolValue(points.by_tier_rate, true)
  form.rechargeEarnPoints = boolValue(points.recharge_earn, false)
}

function settingsPayload() {
  return {
    'feature.sms_enabled': form.smsEnabled,
    'feature.wechat_enabled': form.wechatEnabled,
    'feature.online_booking_enabled': form.onlineBookingEnabled,
    'store.business_hours': { open: form.businessOpen, close: form.businessClose },
    'boarding.checkout_rule': {
      round: form.checkoutRound,
      min_nights: form.minNights,
      apply_member_discount: form.applyMemberDiscount,
    },
    'appointment.cancel_deadline_hours': form.cancelDeadlineHours,
    'appointment.visit_reminder_hours': form.visitReminderHours,
    'pet.vaccine_remind_days': form.vaccineRemindDays,
    'inventory.allow_negative': form.allowNegativeInventory,
    'member.allow_downgrade': form.allowMemberDowngrade,
    'member.churn_days': form.churnDays,
    'points.rule': {
      per_yuan: form.pointsPerYuan,
      by_tier_rate: form.pointsByTierRate,
      recharge_earn: form.rechargeEarnPoints,
    },
  }
}

function isWholeNumber(value: number) {
  return Number.isFinite(value) && Math.trunc(value) === value
}

function validateIntegerRange(label: string, value: number, min: number, max: number) {
  if (!isWholeNumber(value) || value < min || value > max) {
    return `${label}必须是 ${min}-${max} 的整数`
  }
  return ''
}

function validateForm() {
  const problems = [
    form.businessOpen && form.businessClose ? '' : '营业时间不能为空',
    form.businessOpen === form.businessClose ? '开门时间和打烊时间不能相同' : '',
    validateIntegerRange('最少计费晚数', form.minNights, 1, 365),
    validateIntegerRange('取消截止小时', form.cancelDeadlineHours, 0, 720),
    validateIntegerRange('到店提醒提前小时', form.visitReminderHours, 0, 720),
    validateIntegerRange('疫苗提醒天数', form.vaccineRemindDays, 1, 3650),
    validateIntegerRange('沉默会员天数', form.churnDays, 1, 3650),
    Number.isFinite(form.pointsPerYuan) && form.pointsPerYuan >= 0 && form.pointsPerYuan <= 100
      ? ''
      : '每元积分必须是 0-100 的数字',
  ].filter(Boolean)
  return problems[0] || ''
}

function errorMessage(err: unknown) {
  const maybe = err as { response?: { data?: { message?: string } }; message?: string }
  return maybe.response?.data?.message || maybe.message || '操作失败'
}

function idem(action: string) {
  return { headers: { 'Idempotency-Key': `${action}-${Date.now()}-${Math.random().toString(16).slice(2)}` } }
}

async function load(clearFeedback = true) {
  loading.value = true
  if (clearFeedback) {
    error.value = ''
    success.value = ''
  }
  try {
    const { data } = await client.get('/settings')
    applySettings(data.data || {})
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

async function saveAllSettings() {
  error.value = ''
  success.value = ''
  const validationError = validateForm()
  if (validationError) {
    error.value = validationError
    return
  }
  saving.value = true
  try {
    const payload = settingsPayload()
    await Promise.all(
      Object.entries(payload).map(([key, value]) =>
        client.put(`/settings/${key}`, { value, updated_by: 0 }, idem(`setting-${key}`)),
      ),
    )
    success.value = '设置已保存'
    await load(false)
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.setting-section {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 14px;
}

.setting-section h4 {
  font-size: 15px;
  font-weight: 700;
}

.field-grid {
  display: grid;
  gap: 12px;
}

.field-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.field-grid.three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.field {
  width: 100%;
  border: 1px solid rgba(35, 30, 24, 0.12);
  border-radius: 8px;
  padding: 8px 10px;
  background: white;
  font-size: 14px;
  color: var(--color-ink);
}

.label {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
  color: rgba(35, 30, 24, 0.7);
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border-bottom: 1px solid rgba(35, 30, 24, 0.06);
  padding-bottom: 12px;
}

.toggle-row.compact {
  border-bottom: 0;
  padding-bottom: 0;
}

.toggle-row span {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.toggle-row strong {
  font-size: 14px;
}

.toggle-row small,
.muted {
  color: rgba(35, 30, 24, 0.58);
  font-size: 12px;
}

.toggle-row input[type='checkbox'] {
  width: 42px;
  height: 22px;
  flex: 0 0 auto;
  accent-color: var(--color-coral);
}

.save-bar {
  display: flex;
  justify-content: flex-end;
}

.primary-btn,
.soft-btn {
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
  font-weight: 600;
}

.primary-btn {
  min-width: 160px;
  color: white;
  background: var(--color-coral);
}

.soft-btn {
  background: var(--color-surface);
}

button:disabled {
  opacity: 0.6;
}

@media (max-width: 1100px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .field-grid.two,
  .field-grid.three {
    grid-template-columns: 1fr;
  }
}
</style>
