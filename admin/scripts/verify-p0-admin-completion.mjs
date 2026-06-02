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
  'function parseValue',
  'settings editor must preserve raw text values when JSON parsing fails',
)
assertMatches(
  'src/views/setting/SettingView.vue',
  /value:\s*parseValue\(editor\.value\)/,
  'settings save must use parseValue rather than rejecting non-JSON text',
)

console.log('P0 admin completion source checks passed')
