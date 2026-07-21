// Lampiran A — Skenario 3: Mutasi aset (asset_transfer) KC A ke KC B.
// Band 0-50 jt = office (1 langkah). Kirim = scope kantor ASAL; terima = scope
// kantor TUJUAN. Semua assertion status code persis (peta transfer module).

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, resolveBranchCast, createApprovedAsset, approve, getRequest
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch (transfer.manage) — MAKER (kantor asal)
let officeAppr: Actor // Kepala Unit @ branch — approver office
let wilayahAppr: Actor
let siblingAppr: Actor // Kepala Unit @ sibling branch — penerima (scope tujuan)

const TODAY = new Date().toISOString().slice(0, 10)

async function newAsset(cost = '8000000') {
  return createApprovedAsset(admin, maker, { office: officeAppr, wilayah: wilayahAppr }, { officeId: cast.branch.id, cost })
}

/** Submit a transfer, approve its single office step, and return the transfer row id. */
async function openApprovedTransfer(assetId: string, toOfficeId: string): Promise<string> {
  const res = await maker.api.post('transfers', {
    data: { asset_id: assetId, to_office_id: toOfficeId, condition_sent: 'baik', reason: 'relokasi', transfer_date: TODAY }
  })
  expect(res.status(), await res.text()).toBe(201)
  const reqId = (await res.json()).request_id
  expect((await approve(officeAppr, reqId)).status()).toBe(200)
  expect((await getRequest(admin.api, reqId)).status).toBe('approved')
  const rows = (await (await admin.api.get(`assets/${assetId}/transfers`)).json()).data as Array<{ id: string, status: string }>
  const open = rows.find(r => r.status === 'approved' || r.status === 'in_transit')
  if (!open) throw new Error('openApprovedTransfer: no open transfer row found')
  return open.id
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[maker, officeAppr, wilayahAppr, siblingAppr] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.officeApproverEmail),
    loginDemo(cast.wilayahApproverEmail), loginDemo(cast.siblingApproverEmail)
  ])
})

test.afterAll(async () => {
  for (const a of [admin, maker, officeAppr, wilayahAppr, siblingAppr]) await a?.api.dispose()
})

test('alur lengkap: approve office, ship (asal), receive+BAST (tujuan) -> aset pindah', async () => {
  const asset = await newAsset()
  const trId = await openApprovedTransfer(asset.id, cast.sibling.id)
  // Catatan: alur transfer app TIDAK mengubah status aset menjadi 'in_transfer';
  // aset tetap 'available' selama transit (guard dobel-mutasi lewat baris transfer
  // terbuka, bukan status aset). Yang berubah saat receive adalah office_id.

  // Kirim oleh pihak kantor ASAL.
  expect((await maker.api.post(`transfers/${trId}/ship`, { data: {} })).status()).toBe(200)
  // Terima + BAST oleh pihak kantor TUJUAN.
  const rec = await siblingAppr.api.post(`transfers/${trId}/receive`, {
    data: { bast_no: `BAST-TRF-${Date.now()}`, received_date: TODAY }
  })
  expect(rec.status(), await rec.text()).toBe(200)

  // Aset kini di kantor tujuan, status available.
  const a = await (await admin.api.get(`assets/${asset.id}`)).json()
  expect(a.office_id).toBe(cast.sibling.id)
  expect(a.status).toBe('available')
})

test('TIDAK boleh: mutasi ke kantor yang sama (to == from) -> 422 ErrSameOffice', async () => {
  const asset = await newAsset()
  const res = await maker.api.post('transfers', {
    data: { asset_id: asset.id, to_office_id: cast.branch.id, condition_sent: 'baik', transfer_date: TODAY }
  })
  expect(res.status()).toBe(422)
})

test('TIDAK boleh: mutasi aset yang sudah punya mutasi terbuka -> 422 ErrAssetInTransit', async () => {
  // 422 datang dari guard baris-transfer-terbuka (GetOpenTransferForAsset), BUKAN
  // dari status aset (app tak pernah menyetel aset ke in_transfer — lihat catatan di atas).
  const asset = await newAsset()
  await openApprovedTransfer(asset.id, cast.sibling.id) // buka mutasi (aset tetap available)
  const res = await maker.api.post('transfers', {
    data: { asset_id: asset.id, to_office_id: cast.sibling.id, condition_sent: 'baik', transfer_date: TODAY }
  })
  expect(res.status()).toBe(422)
})

test('TIDAK boleh: ship oleh user di luar scope kantor asal -> 403', async () => {
  const asset = await newAsset()
  const trId = await openApprovedTransfer(asset.id, cast.sibling.id)
  // siblingAppr berkantor di kantor tujuan; scope-nya tak mencakup kantor asal.
  expect((await siblingAppr.api.post(`transfers/${trId}/ship`, { data: {} })).status()).toBe(403)
})

test('TIDAK boleh: receive oleh user di luar scope kantor tujuan -> 403', async () => {
  const asset = await newAsset()
  const trId = await openApprovedTransfer(asset.id, cast.sibling.id)
  expect((await maker.api.post(`transfers/${trId}/ship`, { data: {} })).status()).toBe(200)
  // maker berkantor di asal; scope-nya tak mencakup kantor tujuan.
  const res = await maker.api.post(`transfers/${trId}/receive`, { data: { bast_no: 'X', received_date: TODAY } })
  expect(res.status()).toBe(403)
})

test('TIDAK boleh: receive sebelum di-ship (state approved, bukan in_transit) -> 409', async () => {
  const asset = await newAsset()
  const trId = await openApprovedTransfer(asset.id, cast.sibling.id)
  const res = await siblingAppr.api.post(`transfers/${trId}/receive`, { data: { bast_no: 'Y', received_date: TODAY } })
  expect(res.status()).toBe(409)
})

test('reject-receive oleh kantor tujuan -> returned (aset tak pindah)', async () => {
  const asset = await newAsset()
  const trId = await openApprovedTransfer(asset.id, cast.sibling.id)
  expect((await maker.api.post(`transfers/${trId}/ship`, { data: {} })).status()).toBe(200)
  const res = await siblingAppr.api.post(`transfers/${trId}/reject-receive`, { data: { note: 'kondisi tak sesuai' } })
  expect(res.status(), await res.text()).toBe(200)
  const a = await (await admin.api.get(`assets/${asset.id}`)).json()
  expect(a.office_id).toBe(cast.branch.id) // tetap di kantor asal
})
