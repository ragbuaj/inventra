// Lampiran A — Skenario 7: Pengecualian valuasi (valuation_exclusion).
// Selalu 1 langkah level WILAYAH, berapa pun nilainya. Hanya Kepala Kanwil /
// Pejabat Pusat yang bisa memutus (tier wilayah); Kepala Unit / Manager cabang
// di luar scope.

import { test, expect } from '@playwright/test'
import type { Actor, BranchCast } from './lampiran-helpers'
import {
  loginAdmin, loginDemo, resolveBranchCast, createApprovedAsset, approve, getRequest
} from './lampiran-helpers'

test.describe.configure({ mode: 'serial' })

let admin: Actor
let cast: BranchCast
let maker: Actor // Manager @ branch — MAKER
let officeAppr: Actor // Kepala Unit @ branch (tier office)
let wilayahAppr: Actor // Kepala Kanwil @ wilayah (tier wilayah)
let pusatAppr: Actor // Pejabat Pusat

async function newAsset() {
  return createApprovedAsset(admin, maker, { office: officeAppr, wilayah: wilayahAppr }, { officeId: cast.branch.id, cost: '8000000' })
}

async function submitExclusion(as: Actor, assetId: string) {
  return as.api.post('requests', {
    data: { type: 'valuation_exclusion', amount: '0', office_id: cast.branch.id, target_id: assetId, reason: 'aset idle, dikeluarkan dari penyusutan' }
  })
}

test.beforeAll(async () => {
  admin = await loginAdmin()
  cast = await resolveBranchCast(admin, 'BDG01')
  ;[maker, officeAppr, wilayahAppr, pusatAppr] = await Promise.all([
    loginDemo(cast.makerEmail), loginDemo(cast.officeApproverEmail),
    loginDemo(cast.wilayahApproverEmail), loginDemo(cast.pusatApproverEmail)
  ])
})

test.afterAll(async () => {
  for (const a of [admin, maker, officeAppr, wilayahAppr, pusatAppr]) await a?.api.dispose()
})

test('1 langkah wilayah: Kepala Kanwil approve -> aset excluded_from_valuation', async () => {
  const asset = await newAsset()
  const res = await submitExclusion(maker, asset.id)
  expect(res.status(), await res.text()).toBe(201)
  const reqId = (await res.json()).id
  expect((await getRequest(admin.api, reqId)).steps.map(s => s.required_level)).toEqual(['wilayah'])

  expect((await approve(wilayahAppr, reqId)).status()).toBe(200)
  expect((await getRequest(admin.api, reqId)).status).toBe('approved')
  const a = await (await admin.api.get(`assets/${asset.id}`)).json()
  expect(a.excluded_from_valuation).toBe(true)
})

test('Pejabat Pusat juga boleh memutus langkah wilayah', async () => {
  const asset = await newAsset()
  const reqId = (await (await submitExclusion(maker, asset.id)).json()).id
  expect((await approve(pusatAppr, reqId)).status()).toBe(200)
  expect((await getRequest(admin.api, reqId)).status).toBe('approved')
})

test('TIDAK boleh: Kepala Unit cabang memutus (tier wilayah di luar scope) -> 403', async () => {
  const asset = await newAsset()
  const reqId = (await (await submitExclusion(maker, asset.id)).json()).id
  expect((await approve(officeAppr, reqId)).status()).toBe(403)
})

test('TIDAK boleh: maker memutus permohonannya sendiri -> 403', async () => {
  const asset = await newAsset()
  const reqId = (await (await submitExclusion(maker, asset.id)).json()).id
  expect((await approve(maker, reqId)).status()).toBe(403)
})
