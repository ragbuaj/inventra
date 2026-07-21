// Lampiran A — A.7: Stock Opname.
// Semua tulis butuh stockopname.manage + scope kantor sesi. Opname TIDAK menerapkan
// maker-checker internal; segregasi baru muncul saat tindak lanjut (disposal/
// transfer/maintenance) yang masuk approval sendiri. Staf tak bisa menulis apa pun.

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, resolveBranchCast, createApprovedAsset
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let manager: Actor // Manager @ branch (stockopname.manage)
let officeAppr: Actor
let wilayahAppr: Actor
let staf: Actor // Staf @ branch (tanpa stockopname.manage)

const PERIOD = '2026-07'

interface Item { id: string, asset_id: string, asset_tag: string, result: string, expected: boolean }

async function newAsset() {
  return createApprovedAsset(admin, manager, { office: officeAppr, wilayah: wilayahAppr }, { officeId: cast.branch.id, cost: '8000000' })
}

async function createSession(as: Actor, officeId: string, name: string) {
  return as.api.post('stock-opname/sessions', { data: { office_id: officeId, period: PERIOD, name } })
}

async function items(sid: string): Promise<Item[]> {
  return (await (await admin.api.get(`stock-opname/sessions/${sid}/items?limit=2000`)).json()).data
}

async function itemByTag(sid: string, tag: string): Promise<Item> {
  const found = (await items(sid)).find(i => i.asset_tag === tag)
  if (!found) throw new Error(`itemByTag: item for ${tag} not in session ${sid}`)
  return found
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[manager, officeAppr, wilayahAppr, staf] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.officeApproverEmail),
    loginDemo(cast.wilayahApproverEmail), loginDemo(cast.stafEmail)
  ])
})

test.afterAll(async () => {
  for (const a of [admin, manager, officeAppr, wilayahAppr, staf]) await a?.api.dispose()
})

test('lifecycle penuh: snapshot -> counting -> reconcile -> follow-up per varian -> close -> report', async () => {
  // Aset available segar agar tindak lanjut varian (disposal/transfer/maintenance) valid.
  const [aNF, aMP, aDM, aScan] = await Promise.all([newAsset(), newAsset(), newAsset(), newAsset()])

  const create = await createSession(manager, cast.branch.id, `SO ${Date.now()}`)
  expect(create.status(), await create.text()).toBe(201)
  const session = await create.json()
  const sid = session.id
  expect(session.total).toBeGreaterThan(0)

  expect((await manager.api.post(`stock-opname/sessions/${sid}/start`, {})).status()).toBe(200)

  // Scan satu tag: untuk item yang diharapkan (expected) scan mengembalikan item
  // apa adanya (hanya menyisipkan item baru untuk aset tak terduga) — hasil di-set
  // via PATCH. Di sini scan hanya memastikan endpoint aktif saat counting.
  expect((await manager.api.post(`stock-opname/sessions/${sid}/scan`, { data: { asset_tag: aScan.tag } })).status()).toBe(200)

  // Set hasil varian via PATCH.
  const nf = await itemByTag(sid, aNF.tag)
  const mp = await itemByTag(sid, aMP.tag)
  const dm = await itemByTag(sid, aDM.tag)
  const scanned = await itemByTag(sid, aScan.tag)
  expect((await manager.api.patch(`stock-opname/sessions/${sid}/items/${scanned.id}`, { data: { result: 'found' } })).status()).toBe(200)
  expect((await manager.api.patch(`stock-opname/sessions/${sid}/items/${nf.id}`, { data: { result: 'not_found' } })).status()).toBe(200)
  expect((await manager.api.patch(`stock-opname/sessions/${sid}/items/${mp.id}`, { data: { result: 'misplaced' } })).status()).toBe(200)
  expect((await manager.api.patch(`stock-opname/sessions/${sid}/items/${dm.id}`, { data: { result: 'damaged' } })).status()).toBe(200)

  // Rekonsiliasi.
  expect((await manager.api.post(`stock-opname/sessions/${sid}/reconcile`, {})).status()).toBe(200)

  // Tindak lanjut: not_found -> disposal, misplaced -> transfer, damaged -> maintenance.
  expect((await manager.api.post(`stock-opname/sessions/${sid}/items/${nf.id}/follow-up`, { data: { reason: 'hilang' } })).status()).toBe(200)
  // misplaced tanpa to_office_id -> 422.
  expect((await manager.api.post(`stock-opname/sessions/${sid}/items/${mp.id}/follow-up`, { data: {} })).status()).toBe(422)
  expect((await manager.api.post(`stock-opname/sessions/${sid}/items/${mp.id}/follow-up`, { data: { to_office_id: cast.sibling.id } })).status()).toBe(200)
  expect((await manager.api.post(`stock-opname/sessions/${sid}/items/${dm.id}/follow-up`, { data: {} })).status()).toBe(200)

  // found/pending -> ErrInvalidState (409).
  expect((await manager.api.post(`stock-opname/sessions/${sid}/items/${scanned.id}/follow-up`, { data: {} })).status()).toBe(409)
  // Tindak lanjut ganda item yang sama -> 409 ErrAlreadyFollowedUp.
  expect((await manager.api.post(`stock-opname/sessions/${sid}/items/${nf.id}/follow-up`, { data: { reason: 'lagi' } })).status()).toBe(409)

  // Tutup + unduh Berita Acara.
  expect((await manager.api.post(`stock-opname/sessions/${sid}/close`, {})).status()).toBe(200)
  const rep = await admin.api.get(`stock-opname/sessions/${sid}/report?format=pdf`)
  expect(rep.status()).toBe(200)
  expect(rep.headers()['content-type']).toContain('pdf')
})

test('TIDAK boleh: Staf menulis apa pun (buat sesi) -> 403', async () => {
  expect((await createSession(staf, cast.branch.id, 'SO staf')).status()).toBe(403)
})

test('TIDAK boleh: scan / set-result saat status bukan counting (masih open) -> 409', async () => {
  const sid = (await (await createSession(manager, cast.branch.id, `SO open ${Date.now()}`)).json()).id
  // Belum start (status open).
  expect((await manager.api.post(`stock-opname/sessions/${sid}/scan`, { data: { asset_tag: 'X' } })).status()).toBe(409)
  const it = (await items(sid))[0]
  expect((await manager.api.patch(`stock-opname/sessions/${sid}/items/${it.id}`, { data: { result: 'found' } })).status()).toBe(409)
})

test('TIDAK boleh: transisi ilegal (close langsung dari open) -> 409', async () => {
  const sid = (await (await createSession(manager, cast.branch.id, `SO skip ${Date.now()}`)).json()).id
  expect((await manager.api.post(`stock-opname/sessions/${sid}/close`, {})).status()).toBe(409)
})

test('TIDAK boleh: buat sesi untuk kantor di luar scope -> 403 ErrOutOfScope', async () => {
  // Manager cabang membuat sesi untuk kantor SAUDARA (di luar scope-nya). CreateSession
  // mengecek InScope sebelum query DB apa pun -> selalu 403 (bukan 404).
  const res = await createSession(manager, cast.sibling.id, 'SO luar scope')
  expect(res.status()).toBe(403)
})
