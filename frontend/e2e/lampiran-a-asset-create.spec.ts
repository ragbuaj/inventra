// Lampiran A — Skenario 1: Pengadaan aset (asset_create) via maker-checker.
// docs/ALUR_PENGGUNA.md A.4/A.5. API-driven, strict exact-status assertions.
//
// Band nilai (tersegel, A.4):
//   0-10 jt        -> office (1 langkah)
//   10 jt - 100 jt -> office, wilayah (2 langkah)
//   100 jt ke atas -> office, wilayah, pusat (3 langkah, 3 orang berbeda)

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, resolveBranchCast, getRequest, currentLevel, stepLevels, approve,
  categoryIdByCode
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch (asset.manage) — MAKER
let maker2: Actor // a 2nd Manager @ branch (scope = branch only)
let officeAppr: Actor // Kepala Unit @ branch — office-tier approver
let wilayahAppr: Actor // Kepala Kanwil @ wilayah — wilayah-tier approver
let pusatAppr: Actor // Pejabat Kantor Pusat — pusat-tier approver
let staf: Actor // Staf @ branch
let categoryId: string

async function submitCreate(as: Actor, officeId: string, amount: string, purchaseCost: string, name: string) {
  return as.api.post('requests', {
    data: {
      type: 'asset_create', amount, office_id: officeId, reason: `create ${name}`,
      payload: { name, category_id: categoryId, office_id: officeId, asset_class: 'intangible', purchase_cost: purchaseCost }
    }
  })
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[maker, maker2, officeAppr, wilayahAppr, pusatAppr, staf] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.maker2Email), loginDemo(cast.officeApproverEmail),
    loginDemo(cast.wilayahApproverEmail), loginDemo(cast.pusatApproverEmail), loginDemo(cast.stafEmail)
  ])
  categoryId = await categoryIdByCode(admin.api, 'SWL')
})

test.afterAll(async () => {
  for (const a of [admin, maker, maker2, officeAppr, wilayahAppr, pusatAppr, staf]) await a?.api.dispose()
})

test('band 10-100jt: 2 langkah office lalu wilayah, aset lahir', async () => {
  const name = `Aset 18jt ${Date.now()}`
  const res = await submitCreate(maker, cast.branch.id, '18500000', '18500000', name)
  expect(res.status()).toBe(201)
  const reqId = (await res.json()).id

  let req = await getRequest(admin.api, reqId)
  expect(req.status).toBe('pending')
  expect(stepLevels(req)).toEqual(['office', 'wilayah'])
  expect(currentLevel(req)).toBe('office')

  // Langkah 1 (office) oleh Kepala Unit cabang.
  expect((await approve(officeAppr, reqId)).status()).toBe(200)
  req = await getRequest(admin.api, reqId)
  expect(req.status).toBe('pending')
  expect(currentLevel(req)).toBe('wilayah')

  // Langkah 2 (wilayah) oleh Kepala Kanwil.
  expect((await approve(wilayahAppr, reqId)).status()).toBe(200)
  req = await getRequest(admin.api, reqId)
  expect(req.status).toBe('approved')

  // Aset lahir, status available.
  const found = (await (await admin.api.get(`assets?search=${encodeURIComponent(name)}&limit=5`)).json()).data
  expect(found.length).toBeGreaterThan(0)
  expect(found[0].status).toBe('available')
})

test('band 100jt+: 3 langkah office/wilayah/pusat oleh 3 orang berbeda', async () => {
  const name = `Aset 150jt ${Date.now()}`
  const res = await submitCreate(maker, cast.branch.id, '150000000', '150000000', name)
  expect(res.status()).toBe(201)
  const reqId = (await res.json()).id

  const req = await getRequest(admin.api, reqId)
  expect(stepLevels(req)).toEqual(['office', 'wilayah', 'pusat'])

  expect((await approve(officeAppr, reqId)).status()).toBe(200)
  expect(currentLevel(await getRequest(admin.api, reqId))).toBe('wilayah')
  expect((await approve(wilayahAppr, reqId)).status()).toBe(200)
  expect(currentLevel(await getRequest(admin.api, reqId))).toBe('pusat')
  expect((await approve(pusatAppr, reqId)).status()).toBe(200)
  expect((await getRequest(admin.api, reqId)).status).toBe('approved')
})

test('TIDAK boleh: maker memutus permohonannya sendiri (SoD) -> 403', async () => {
  const res = await submitCreate(maker, cast.branch.id, '18500000', '18500000', `SoD ${Date.now()}`)
  const reqId = (await res.json()).id
  expect((await approve(maker, reqId)).status()).toBe(403)
})

test('TIDAK boleh: approver yang sama memutus dua langkah berturut -> 403', async () => {
  // pusatAppr scope mencakup semua tier; boleh langkah office, tapi tak boleh diulang di wilayah.
  const res = await submitCreate(maker, cast.branch.id, '18500000', '18500000', `Repeat ${Date.now()}`)
  const reqId = (await res.json()).id
  expect((await approve(pusatAppr, reqId)).status()).toBe(200) // langkah office (dalam scope)
  expect((await approve(pusatAppr, reqId)).status()).toBe(403) // langkah wilayah — approver berulang
})

test('TIDAK boleh: approver di luar tier (Manager cabang) memutus langkah wilayah -> 403', async () => {
  const res = await submitCreate(maker, cast.branch.id, '18500000', '18500000', `Scope ${Date.now()}`)
  const reqId = (await res.json()).id
  expect((await approve(officeAppr, reqId)).status()).toBe(200) // office ok
  // maker2 = Manager cabang (scope office_subtree dari cabang) tidak mencakup wilayah.
  expect((await approve(maker2, reqId)).status()).toBe(403)
})

test('TIDAK boleh: amount != payload.purchase_cost (understatement/overstatement/notasi) -> 400', async () => {
  const bad = [
    ['understatement', '10000000', '18500000'],
    ['overstatement', '20000000', '18500000'],
    ['eksponen', '1e7', '10000000'],
    ['pecahan', '1/3', '18500000']
  ] as const
  for (const [label, amount, cost] of bad) {
    const res = await submitCreate(maker, cast.branch.id, amount, cost, `Bad ${label} ${Date.now()}`)
    expect(res.status(), `amount=${amount} cost=${cost} (${label})`).toBe(400)
  }
})

test('TIDAK boleh: submit asset_create untuk kantor di luar scope pemohon -> 403', async () => {
  // Staf mengajukan untuk kantor SAUDARA (bukan kantornya) -> di luar assets-scope.
  const name = `OOScope ${Date.now()}`
  const res = await staf.api.post('requests', {
    data: {
      type: 'asset_create', amount: '5000000', office_id: cast.sibling.id, reason: 'oos',
      payload: { name, category_id: categoryId, office_id: cast.sibling.id, asset_class: 'intangible', purchase_cost: '5000000' }
    }
  })
  expect(res.status()).toBe(403)
})

test('cancel: hanya maker & hanya pending — non-maker / setelah approved -> 404', async () => {
  const res = await submitCreate(maker, cast.branch.id, '18500000', '18500000', `Cancel ${Date.now()}`)
  const reqId = (await res.json()).id

  // Non-maker mencoba cancel -> 404 (filter SQL WHERE requester = pemanggil).
  expect((await officeAppr.api.post(`requests/${reqId}/cancel`, { data: {} })).status()).toBe(404)
  // Maker cancel selagi pending -> 200.
  expect((await maker.api.post(`requests/${reqId}/cancel`, { data: {} })).status()).toBe(200)

  // Setelah selesai (approved), cancel -> 404.
  const res2 = await submitCreate(maker, cast.branch.id, '5000000', '5000000', `Done ${Date.now()}`)
  const reqId2 = (await res2.json()).id
  expect((await approve(officeAppr, reqId2)).status()).toBe(200) // band office 1 langkah
  expect((await getRequest(admin.api, reqId2)).status).toBe('approved')
  expect((await maker.api.post(`requests/${reqId2}/cancel`, { data: {} })).status()).toBe(404)
})
