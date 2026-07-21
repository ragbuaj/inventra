// Lampiran A — guard regresi masking kolom finansial (field_permissions).
// authz.FilterView bersifat DEFAULT-ALLOW: masking bergantung SEPENUHNYA pada
// baris field_permissions yang di-seed (mirror migrasi 000016). Tanpa tes ini,
// menghapus/merusak baris itu akan membocorkan harga perolehan/nilai buku/akumulasi
// penyusutan ke Staf/Kepala TANPA satu pun tes gagal (persis bug review #1).
//
// Aturan kanonik (000016):
//   purchase_cost, book_value        -> hanya Superadmin + Manager yang view
//   accumulated_depreciation         -> hanya Superadmin yang view

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import { loginAdmin, loginDemo, resolveBranchCast, pickAvailableAsset } from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let manager: Actor // Manager @ branch
let staf: Actor // Staf @ branch
let assetId: string

const FINANCIAL = ['purchase_cost', 'book_value', 'accumulated_depreciation'] as const

async function assetKeys(as: Actor, id: string): Promise<Set<string>> {
  const res = await as.api.get(`assets/${id}`)
  expect(res.status(), await res.text()).toBe(200)
  return new Set(Object.keys(await res.json()))
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[manager, staf] = await Promise.all([loginDemo(cast.makerEmail), loginDemo(cast.stafEmail)])
  assetId = (await pickAvailableAsset(admin, cast.branch.id)).id
})

test.afterAll(async () => {
  for (const a of [admin, manager, staf]) await a?.api.dispose()
})

test('Staf: SEMUA kolom finansial ter-mask (tak muncul di response)', async () => {
  const keys = await assetKeys(staf, assetId)
  for (const f of FINANCIAL) expect(keys.has(f), `Staf tidak boleh melihat ${f}`).toBe(false)
})

test('Manager: lihat purchase_cost + book_value, TAPI bukan accumulated_depreciation', async () => {
  const keys = await assetKeys(manager, assetId)
  expect(keys.has('purchase_cost')).toBe(true)
  expect(keys.has('book_value')).toBe(true)
  expect(keys.has('accumulated_depreciation'), 'akumulasi penyusutan hanya untuk Superadmin').toBe(false)
})

test('Superadmin: lihat SEMUA kolom finansial (dengan nilai)', async () => {
  const res = await admin.api.get(`assets/${assetId}`)
  const body = await res.json()
  for (const f of FINANCIAL) expect(f in body, `Superadmin harus melihat ${f}`).toBe(true)
  expect(Number(body.purchase_cost)).toBeGreaterThan(0)
})
