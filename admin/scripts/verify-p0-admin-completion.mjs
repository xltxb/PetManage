import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')

function read(path) {
  return readFileSync(resolve(root, path), 'utf8')
}

function assertIncludes(file, text, reason) {
  const source = read(file)
  if (!source.includes(text)) {
    throw new Error(`${file} missing ${JSON.stringify(text)}: ${reason}`)
  }
}

function assertMatches(file, pattern, reason) {
  const source = read(file)
  if (!pattern.test(source)) {
    throw new Error(`${file} failed ${pattern}: ${reason}`)
  }
}

function assertNotIncludes(file, text, reason) {
  const source = read(file)
  if (source.includes(text)) {
    throw new Error(`${file} still includes ${JSON.stringify(text)}: ${reason}`)
  }
}

assertIncludes(
  'src/views/boarding/BoardingList.vue',
  'actual_check_in',
  'boarding rows must expose actual check-in time from backend orders',
)
assertIncludes(
  'src/views/boarding/BoardingList.vue',
  'actual_check_out',
  'boarding rows must expose actual check-out time from backend orders',
)
assertIncludes(
  'src/views/boarding/BoardingList.vue',
  'settlement_id',
  'boarding rows must expose checkout-generated settlement id',
)
assertIncludes(
  'src/views/boarding/BoardingList.vue',
  'order.nights',
  'boarding rows must expose calculated nights',
)
assertIncludes(
  'src/views/boarding/BoardingList.vue',
  'order.total_amount',
  'boarding rows must expose checkout total amount',
)

assertIncludes(
  'src/views/pet/PetList.vue',
  'form.chip_no',
  'pet creation must allow chip number entry',
)
assertMatches(
  'src/views/pet/PetList.vue',
  /chip_no:\s*form\.chip_no/,
  'pet creation payload must submit chip_no',
)

assertIncludes(
  'src/views/setting/SettingView.vue',
  'form.businessOpen',
  'settings page must expose business hours as form fields instead of raw JSON',
)
assertMatches(
  'src/views/setting/SettingView.vue',
  /v-model="form\.businessOpen"[\s\S]*type="time"/,
  'settings page must use a time input for opening time',
)
assertIncludes(
  'src/views/setting/SettingView.vue',
  'form.smsEnabled',
  'settings page must expose feature flags as page controls',
)
assertIncludes(
  'src/views/setting/SettingView.vue',
  'form.checkoutRound',
  'settings page must expose boarding checkout rule as page controls',
)
assertIncludes(
  'src/views/setting/SettingView.vue',
  'saveAllSettings',
  'settings page must save structured form values through the settings API',
)
assertIncludes(
  'src/views/setting/SettingView.vue',
  'validateForm',
  'settings page must validate structured controls before saving',
)
assertIncludes(
  'src/views/setting/SettingView.vue',
  'await load(false)',
  'settings page must preserve the save success message after refreshing settings',
)
assertNotIncludes(
  'src/views/setting/SettingView.vue',
  '配置值 JSON',
  'settings page must not ask operators to edit JSON directly',
)
assertNotIncludes(
  'src/views/setting/SettingView.vue',
  '<textarea',
  'settings page must not expose a raw JSON textarea',
)

console.log('P0 admin completion source checks passed')
