// Lampiran A — guard regresi masking kolom finansial (field_permissions).
// authz.FilterView bersifat DEFAULT-ALLOW: masking bergantung SEPENUHNYA pada
// baris field_permissions yang di-seed (mirror migrasi 000016). Tanpa tes ini,
// menghapus/merusak baris itu akan membocorkan harga perolehan/nilai buku/akumulasi
// penyusutan ke Staf/Kepala TANPA satu pun tes gagal (persis bug review #1).
//
// Kebijakan kanonik (000016 + 000037, dirapikan jadi SATU tier konsisten):
//   purchase_cost, book_value, accumulated_depreciation
//     -> view: Superadmin + Manager + Pejabat Kantor Pusat
//     -> masked: Kepala Unit, Kepala Kanwil, Staf

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import { loginAdmin, loginDemo, resolveBranchCast, pickAvailableAsset } from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let manager: Actor // Manager @ branch (privileged)
let pejabat: Actor // Pejabat Kantor Pusat (privileged)
let kepalaUnit: Actor // Kepala Unit @ branch (masked)
let staf: Actor // Staf @ branch (masked)
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
  ;[manager, pejabat, kepalaUnit, staf] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.pusatApproverEmail),
    loginDemo(cast.officeApproverEmail), loginDemo(cast.stafEmail)
  ])
  assetId = (await pickAvailableAsset(admin, cast.branch.id)).id
})

test.afterAll(async () => {
  for (const a of [admin, manager, pejabat, kepalaUnit, staf]) await a?.api.dispose()
})

test('Staf & Kepala Unit: SEMUA kolom finansial ter-mask', async () => {
  for (const [label, actor] of [['Staf', staf], ['Kepala Unit', kepalaUnit]] as const) {
    const keys = await assetKeys(actor, assetId)
    for (const f of FINANCIAL) expect(keys.has(f), `${label} tidak boleh melihat ${f}`).toBe(false)
  }
})

test('Manager: lihat KETIGA kolom finansial (tier konsisten)', async () => {
  const keys = await assetKeys(manager, assetId)
  for (const f of FINANCIAL) expect(keys.has(f), `Manager harus melihat ${f}`).toBe(true)
})

test('Pejabat Kantor Pusat: lihat KETIGA kolom finansial', async () => {
  const keys = await assetKeys(pejabat, assetId)
  for (const f of FINANCIAL) expect(keys.has(f), `Pejabat Pusat harus melihat ${f}`).toBe(true)
})

test('Superadmin: lihat SEMUA kolom finansial (dengan nilai)', async () => {
  const res = await admin.api.get(`assets/${assetId}`)
  const body = await res.json()
  for (const f of FINANCIAL) expect(f in body, `Superadmin harus melihat ${f}`).toBe(true)
  expect(Number(body.purchase_cost)).toBeGreaterThan(0)
})
