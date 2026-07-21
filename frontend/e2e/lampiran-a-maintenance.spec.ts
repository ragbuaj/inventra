// Lampiran A — Skenario 5: Maintenance.
// Jalur normal (jadwal/rekaman) LANGSUNG oleh pemegang maintenance.manage (Manager),
// tanpa approval. Jalur laporan kerusakan Staf lewat approval `maintenance`
// (amount 0, 1 langkah office).

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, resolveBranchCast, createApprovedAsset, approve, getRequest, assetStatus
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch (maintenance.manage)
let officeAppr: Actor // Kepala Unit @ branch — approver laporan Staf
let wilayahAppr: Actor
let staf: Actor // Staf @ branch — pelapor kerusakan
let problemCategoryId: string

const TODAY = new Date().toISOString().slice(0, 10)

async function newAsset() {
  return createApprovedAsset(admin, maker, { office: officeAppr, wilayah: wilayahAppr }, { officeId: cast.branch.id, cost: '8000000' })
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[maker, officeAppr, wilayahAppr, staf] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.officeApproverEmail),
    loginDemo(cast.wilayahApproverEmail), loginDemo(cast.stafEmail)
  ])
  const pcs = (await (await admin.api.get('problem-categories?limit=5')).json()).data as Array<{ id: string }>
  problemCategoryId = pcs[0].id
})

test.afterAll(async () => {
  for (const a of [admin, maker, officeAppr, wilayahAppr, staf]) await a?.api.dispose()
})

test('Manager: rekaman langsung scheduled -> in_progress (aset under_maintenance) -> completed (aset lepas)', async () => {
  const asset = await newAsset()
  const create = await maker.api.post('maintenance/records', {
    data: { asset_id: asset.id, type: 'corrective', status: 'scheduled', description: 'Servis berkala AC', scheduled_date: TODAY }
  })
  expect(create.status(), await create.text()).toBe(201)
  const recId = (await create.json()).id

  expect((await maker.api.patch(`maintenance/records/${recId}`, { data: { status: 'in_progress' } })).status()).toBe(200)
  expect(await assetStatus(admin, asset.id)).toBe('under_maintenance')

  expect((await maker.api.patch(`maintenance/records/${recId}`, {
    data: { status: 'completed', completed_date: TODAY, cost: '500000' }
  })).status()).toBe(200)
  expect(await assetStatus(admin, asset.id)).toBe('available')
})

test('laporan kerusakan Staf -> approval maintenance (1 langkah office) -> rekaman korektif', async () => {
  const asset = await newAsset()
  const report = await staf.api.post('maintenance/reports', {
    multipart: { asset_id: asset.id, problem_category_id: problemCategoryId, description: 'Layar tidak menyala' }
  })
  expect(report.status(), await report.text()).toBe(201)
  const reqId = (await report.json()).request_id
  expect((await getRequest(admin.api, reqId)).steps.map(s => s.required_level)).toEqual(['office'])

  // Staf tak boleh memutus laporannya sendiri; approver berbeda approve.
  expect((await approve(staf, reqId)).status()).toBe(403)
  expect((await approve(officeAppr, reqId)).status()).toBe(200)
  expect((await getRequest(admin.api, reqId)).status).toBe('approved')

  // Rekaman korektif kini ada untuk aset.
  const recs = (await (await admin.api.get(`assets/${asset.id}/maintenance`)).json()).data as Array<{ type: string }>
  expect(recs.some(r => r.type === 'corrective')).toBe(true)
})

test('TIDAK boleh: Staf membuat rekaman langsung (butuh maintenance.manage) -> 403', async () => {
  const asset = await newAsset()
  const res = await staf.api.post('maintenance/records', {
    data: { asset_id: asset.id, type: 'corrective', description: 'coba', status: 'scheduled' }
  })
  expect(res.status()).toBe(403)
})

test('TIDAK boleh: dua laporan kerusakan pending untuk (aset, pelapor) yang sama', async () => {
  const asset = await newAsset()
  const first = await staf.api.post('maintenance/reports', {
    multipart: { asset_id: asset.id, problem_category_id: problemCategoryId, description: 'panas berlebih' }
  })
  expect(first.status()).toBe(201)
  const second = await staf.api.post('maintenance/reports', {
    multipart: { asset_id: asset.id, problem_category_id: problemCategoryId, description: 'panas lagi' }
  })
  expect(second.status()).toBe(409) // ErrDuplicatePending: satu laporan pending per (aset, pelapor)
})
