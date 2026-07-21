// Lampiran A — Skenario 2: Penghapusan aset (asset_disposal) nilai buku >= 50 jt.
// Band 50 jt ke atas = office, wilayah, pusat (3 langkah, 3 orang berbeda).
// Nilai buku dihitung SERVER (klien tak bisa mengirimnya) dan jadi amount approval.

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, resolveBranchCast, createApprovedAsset, approve, getRequest, currentLevel, assetStatus
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch (disposal.manage) — MAKER
let wilayahMgr: Actor // Manager berkantor di WILAYAH (scope mencakup wilayah, BUKAN pusat)
let officeAppr: Actor
let wilayahAppr: Actor
let pusatAppr: Actor

const TODAY = new Date().toISOString().slice(0, 10)

// Aset nilai buku >= 50 jt: dibuat baru (nilai buku ~= harga, belum tersusut).
async function newHighValueAsset() {
  return createApprovedAsset(admin, maker, { office: officeAppr, wilayah: wilayahAppr }, { officeId: cast.branch.id, cost: '60000000' })
}

async function submitDisposal(assetId: string, extra: Record<string, unknown> = {}) {
  return maker.api.post('disposals', {
    data: { asset_id: assetId, method: 'sale', disposal_date: TODAY, proceeds: '10000000', reason: 'tidak ekonomis', ...extra }
  })
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[maker, wilayahMgr, officeAppr, wilayahAppr, pusatAppr] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.wilayahManagerEmail), loginDemo(cast.officeApproverEmail),
    loginDemo(cast.wilayahApproverEmail), loginDemo(cast.pusatApproverEmail)
  ])
})

test.afterAll(async () => {
  for (const a of [admin, maker, wilayahMgr, officeAppr, wilayahAppr, pusatAppr]) await a?.api.dispose()
})

test('nilai buku dihitung server = amount request (klien tak menentukan)', async () => {
  const asset = await newHighValueAsset()
  // Kirim book_value palsu di body — harus DIABAIKAN (DTO tak punya field itu).
  const res = await submitDisposal(asset.id, { book_value_at_disposal: '1', proceeds: '5000000' })
  expect(res.status(), await res.text()).toBe(201)
  const reqId = (await res.json()).request_id
  const req = await getRequest(admin.api, reqId)
  // book_value palsu ('1') dari klien DIABAIKAN; amount = nilai buku dihitung server.
  // Aset baru belum tersusut -> nilai buku = harga perolehan (60 jt), bukan '1'.
  expect(Number(req.amount)).toBe(60000000)
})

test('3 langkah office/wilayah/pusat oleh 3 orang berbeda -> aset disposed', async () => {
  const asset = await newHighValueAsset()
  const reqId = (await (await submitDisposal(asset.id)).json()).request_id
  expect((await getRequest(admin.api, reqId)).steps.map(s => s.required_level)).toEqual(['office', 'wilayah', 'pusat'])

  expect((await approve(officeAppr, reqId)).status()).toBe(200)
  expect(currentLevel(await getRequest(admin.api, reqId))).toBe('wilayah')
  expect((await approve(wilayahAppr, reqId)).status()).toBe(200)
  expect(currentLevel(await getRequest(admin.api, reqId))).toBe('pusat')

  // TIDAK boleh: aktor ber-scope WILAYAH (Manager di kantor wilayah, belum memutus)
  // memutus langkah pusat — subtree wilayah tak naik ke Pusat. 403 murni scope.
  expect((await approve(wilayahMgr, reqId)).status()).toBe(403)

  expect((await approve(pusatAppr, reqId)).status()).toBe(200)
  expect((await getRequest(admin.api, reqId)).status).toBe('approved')
  expect(await assetStatus(admin, asset.id)).toBe('disposed')
})

test('TIDAK boleh: disposal aset yang sudah punya permintaan disposal aktif -> 422', async () => {
  const asset = await newHighValueAsset()
  expect((await submitDisposal(asset.id)).status()).toBe(201) // buka request pending
  expect((await submitDisposal(asset.id)).status()).toBe(422) // ErrDisposalExists
})

test('TIDAK boleh: method invalid -> 400', async () => {
  const asset = await newHighValueAsset()
  const res = await maker.api.post('disposals', {
    data: { asset_id: asset.id, method: 'gadai', disposal_date: TODAY }
  })
  expect(res.status()).toBe(400)
})
