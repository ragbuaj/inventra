// Lampiran A — Skenario 4: Peminjaman (assignment) oleh Staf + check-out Manager.
// Peminjaman Staf = approval `assignment` (amount 0, 1 langkah office). Check-out
// Manager = langsung tanpa approval.

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, loginActor, resolveBranchCast, createApprovedAsset, approve,
  getRequest, assetStatus, findRoleId, uniq
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch (assignment.manage) — check-out langsung
let officeAppr: Actor // Kepala Unit @ branch — approver peminjaman
let wilayahAppr: Actor
let staf: Actor // Staf @ branch (punya employee) — MAKER peminjaman
let stafEmployeeId: string

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
  const me = await (await staf.api.get('auth/me')).json()
  stafEmployeeId = me.employee_id
})

test.afterAll(async () => {
  for (const a of [admin, maker, officeAppr, wilayahAppr, staf]) await a?.api.dispose()
})

test('peminjaman Staf: approver berbeda approve -> aset assigned; checkin -> available', async () => {
  const asset = await newAsset()
  const res = await staf.api.post('assignments/borrow', { data: { asset_id: asset.id, due_date: TODAY, condition_out: 'baik' } })
  expect(res.status(), await res.text()).toBe(201)
  const reqId = (await res.json()).request_id
  expect((await getRequest(admin.api, reqId)).steps.map(s => s.required_level)).toEqual(['office'])

  // Staf TIDAK boleh memutus permohonannya sendiri.
  expect((await approve(staf, reqId)).status()).toBe(403)
  // Approver berbeda (Kepala Unit) approve -> executor set aset assigned.
  expect((await approve(officeAppr, reqId)).status()).toBe(200)
  expect(await assetStatus(admin, asset.id)).toBe('assigned')

  // Check-in oleh Manager -> assignment returned, aset available.
  const list = (await (await admin.api.get(`assets/${asset.id}/assignments`)).json()).data as Array<{ id: string, status: string, employee_id: string }>
  const active = list.find(a => a.status === 'active')
  expect(active).toBeTruthy()
  // Executor menautkan aset ke pegawai MILIK Staf (diresolusi dari JWT, bukan input).
  expect(active!.employee_id).toBe(stafEmployeeId)
  expect((await maker.api.post(`assignments/${active!.id}/checkin`, { data: { condition_in: 'baik' } })).status()).toBe(200)
  expect(await assetStatus(admin, asset.id)).toBe('available')
})

test('check-out langsung oleh Manager (tanpa approval) -> aset assigned', async () => {
  const asset = await newAsset()
  const res = await maker.api.post('assignments', {
    data: { asset_id: asset.id, employee_id: stafEmployeeId, checkout_date: TODAY, condition_out: 'baik' }
  })
  expect(res.status(), await res.text()).toBe(201)
  expect(await assetStatus(admin, asset.id)).toBe('assigned')
})

test('TIDAK boleh: borrow aset yang tidak available -> 422', async () => {
  const asset = await newAsset()
  // Jadikan assigned lebih dulu via check-out Manager.
  expect((await maker.api.post('assignments', {
    data: { asset_id: asset.id, employee_id: stafEmployeeId, checkout_date: TODAY }
  })).status()).toBe(201)
  const res = await staf.api.post('assignments/borrow', { data: { asset_id: asset.id } })
  expect(res.status()).toBe(422)
})

test('TIDAK boleh: check-out Manager atas aset tidak available -> 422', async () => {
  const asset = await newAsset()
  expect((await maker.api.post('assignments', {
    data: { asset_id: asset.id, employee_id: stafEmployeeId, checkout_date: TODAY }
  })).status()).toBe(201)
  const res = await maker.api.post('assignments', {
    data: { asset_id: asset.id, employee_id: stafEmployeeId, checkout_date: TODAY }
  })
  expect(res.status()).toBe(422)
})

test('TIDAK boleh: Staf tanpa pegawai tertaut mengajukan borrow -> 422 ErrNoEmployee', async () => {
  const stafRoleId = await findRoleId(admin.api, 'Staf')
  const email = `${uniq('noemp').toLowerCase()}@demo.inventra.local`
  const password = 'Inventra123!'
  const created = await admin.api.post('users', {
    data: { name: 'Staf Tanpa Pegawai', email, password, role_id: stafRoleId, office_id: cast.branch.id }
  })
  expect(created.status(), await created.text()).toBe(201)
  const userId = (await created.json()).id
  try {
    const noemp = await loginActor(email, password)
    try {
      const asset = await newAsset()
      const res = await noemp.api.post('assignments/borrow', { data: { asset_id: asset.id } })
      expect(res.status()).toBe(422)
    } finally {
      await noemp.api.dispose()
    }
  } finally {
    await admin.api.delete(`users/${userId}`)
  }
})
