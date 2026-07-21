// Lampiran A — Skenario 6: Impor massal aset (asset_import), web-only.
// Validasi worker lalu confirm membuka approval asset_import (amount = jumlah
// kolom harga). Band mengikuti asset_create; 72 jt -> office lalu wilayah.

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import { loginAdmin, loginDemo, loginMobile, resolveBranchCast, approve, getRequest, uniq } from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch (asset.manage) — MAKER, pemilik job
let officeAppr: Actor // Kepala Unit @ branch
let wilayahAppr: Actor // Kepala Kanwil

const FULL_HEADER = 'asset_tag,nama,kategori,kantor,tgl_beli,harga,vendor,lokasi'

function assetCsv(run: string, rows: Array<[string, string]>): string {
  const body = rows.map(([name, harga], i) => `,${name},KOM,BDG01,2026-06-0${i + 1},${harga},,R1-1`).join('\n')
  return `${FULL_HEADER}\n${body}\n`
}

function csvFile(name: string, csv: string) {
  return { name, mimeType: 'text/csv', buffer: Buffer.from(csv) }
}

async function uploadAsset(as: Actor, filename: string, csv: string) {
  return as.api.post('imports', { multipart: { target: 'asset', file: csvFile(filename, csv) } })
}

async function jobStatus(as: Actor, jobId: string): Promise<{ status: string, success_rows: number, request_id: string | null }> {
  return await (await as.api.get(`imports/${jobId}`)).json()
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[maker, officeAppr, wilayahAppr] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.officeApproverEmail), loginDemo(cast.wilayahApproverEmail)
  ])
})

test.afterAll(async () => {
  for (const a of [admin, maker, officeAppr, wilayahAppr]) await a?.api.dispose()
})

test('impor 2 aset (72 jt): validated -> confirm -> approval office lalu wilayah -> aset lahir', async () => {
  const run = uniq('imp')
  const n1 = `Impor Aset A ${run}`
  const n2 = `Impor Aset B ${run}`
  const up = await uploadAsset(maker, `assets-${run}.csv`, assetCsv(run, [[n1, '36000000'], [n2, '36000000']]))
  expect(up.status(), await up.text()).toBe(201)
  const jobId = (await up.json()).id

  // Worker memvalidasi (async).
  await expect.poll(async () => (await jobStatus(maker, jobId)).status, { timeout: 20_000 }).toBe('validated')
  expect((await jobStatus(maker, jobId)).success_rows).toBe(2)

  // Confirm oleh pemilik -> buka approval asset_import.
  expect((await maker.api.post(`imports/${jobId}/confirm`, {})).status()).toBe(200)
  let requestId = ''
  await expect.poll(async () => {
    requestId = (await jobStatus(maker, jobId)).request_id || ''
    return requestId
  }, { timeout: 20_000 }).not.toBe('')

  const req = await getRequest(admin.api, requestId)
  expect(Number(req.amount)).toBe(72000000)
  expect(req.steps.map(s => s.required_level)).toEqual(['office', 'wilayah'])

  // Approve office lalu wilayah (2 orang berbeda).
  expect((await approve(officeAppr, requestId)).status()).toBe(200)
  expect((await approve(wilayahAppr, requestId)).status()).toBe(200)
  expect((await getRequest(admin.api, requestId)).status).toBe('approved')

  // Aset hasil impor lahir.
  await expect.poll(async () =>
    (await (await admin.api.get(`assets?search=${encodeURIComponent(n1)}&limit=3`)).json()).total,
  { timeout: 10_000 }).toBeGreaterThan(0)
})

test('TIDAK boleh: token mobile ke grup impor (web-only) -> 403', async () => {
  const mob = await loginMobile(cast.makerEmail, 'Inventra123!')
  try {
    const run = uniq('mob')
    const res = await uploadAsset(mob, `m-${run}.csv`, assetCsv(run, [[`M ${run}`, '1000000']]))
    expect(res.status()).toBe(403)
  } finally {
    await mob.api.dispose()
  }
})

test('TIDAK boleh: user lain meng-confirm job milik maker -> 403 (assertOwner)', async () => {
  const run = uniq('own')
  const up = await uploadAsset(maker, `o-${run}.csv`, assetCsv(run, [[`O ${run}`, '2000000']]))
  const jobId = (await up.json()).id
  await expect.poll(async () => (await jobStatus(maker, jobId)).status, { timeout: 20_000 }).toBe('validated')
  // Kepala Unit (bukan pembuat job) mencoba confirm -> 403.
  expect((await officeAppr.api.post(`imports/${jobId}/confirm`, {})).status()).toBe(403)
})

test('TIDAK boleh: header CSV tak memuat semua kolom target -> job failed (ErrBadHeader)', async () => {
  const run = uniq('hdr')
  // Header tanpa kolom 'harga'. Kontrak header dicek WORKER (async), bukan saat
  // upload — job dibuat (201) lalu berakhir 'failed', tak pernah 'validated'.
  const badCsv = `asset_tag,nama,kategori,kantor,tgl_beli,vendor,lokasi\n,Bad ${run},KOM,BDG01,2026-06-01,,R1-1\n`
  const res = await uploadAsset(maker, `bad-${run}.csv`, badCsv)
  expect(res.status()).toBe(201)
  const jobId = (await res.json()).id
  await expect.poll(async () => (await jobStatus(maker, jobId)).status, { timeout: 20_000 })
    .toBe('failed')
  // Confirm hanya sah dari status 'validated' -> ErrBadState = 409.
  expect((await maker.api.post(`imports/${jobId}/confirm`, {})).status()).toBe(409)
})

test('TIDAK boleh: user lain meng-cancel job milik maker -> 403 (assertOwner)', async () => {
  const run = uniq('cxl')
  const up = await uploadAsset(maker, `c-${run}.csv`, assetCsv(run, [[`C ${run}`, '2000000']]))
  const jobId = (await up.json()).id
  await expect.poll(async () => (await jobStatus(maker, jobId)).status, { timeout: 20_000 }).toBe('validated')
  expect((await officeAppr.api.post(`imports/${jobId}/cancel`, {})).status()).toBe(403)
})
