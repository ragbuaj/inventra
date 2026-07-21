// Helpers for the Lampiran A (docs/ALUR_PENGGUNA.md) maker-checker e2e suite.
//
// These tests are API-driven: each business actor (maker, office/wilayah/pusat
// approver, staf) is a separate `APIRequestContext` holding that user's Bearer
// token, so a full tiered approval chain runs with a DISTINCT person per step —
// exactly what the SoD + data-scope rules require. Assertions check EXACT HTTP
// status codes (403/422/409/400) for every "tidak boleh" path, per the backend
// error map, so hidden regressions in authorization/state cannot slip through.
//
// The cast is RESOLVED from the seeded demo org (backend/db/seed/seed_demo.sql):
// every office has role coverage per tier — Staf/Manager/Kepala Unit at a branch,
// Kepala Kanwil at its wilayah, Pejabat Kantor Pusat at Pusat — all with password
// `Inventra123!`. Nothing is hardcoded to a specific email; we look users up by
// (office, role) so the suite survives seed changes.

import { request } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'

const API_BASE = `${(process.env.NUXT_PUBLIC_API_BASE || process.env.E2E_API_BASE || 'http://localhost:8080/api/v1').replace(/\/+$/, '')}/`

export const DEMO_PASSWORD = process.env.E2E_DEMO_PASSWORD || 'Inventra123!'
export const ADMIN_EMAIL = process.env.E2E_EMAIL || 'admin@inventra.local'
export const ADMIN_PASSWORD = process.env.E2E_PASSWORD || 'admin12345'

export interface Actor {
  email: string
  token: string
  api: APIRequestContext
  userId: string
  officeId: string | null
}

export interface Office {
  id: string
  code: string
  name: string
  parent_id: string | null
  office_type_id: string
}

/** Log in against the real backend and return an authed context + identity. */
export async function loginActor(email: string, password: string): Promise<Actor> {
  const anon = await request.newContext({ baseURL: API_BASE })
  try {
    const res = await anon.post('auth/login', { data: { email, password } })
    if (!res.ok()) throw new Error(`loginActor(${email}) failed: ${res.status()} ${await res.text()}`)
    const body = await res.json() as { access_token: string }
    const api = await request.newContext({
      baseURL: API_BASE,
      extraHTTPHeaders: { Authorization: `Bearer ${body.access_token}` }
    })
    const me = await (await api.get('auth/me')).json() as { id: string, office_id: string | null }
    return { email, token: body.access_token, api, userId: me.id, officeId: me.office_id ?? null }
  } finally {
    await anon.dispose()
  }
}

export const loginDemo = (email: string): Promise<Actor> => loginActor(email, DEMO_PASSWORD)
export const loginAdmin = (): Promise<Actor> => loginActor(ADMIN_EMAIL, ADMIN_PASSWORD)

/**
 * Log in with a mobile-audience token (RequireAudience(web) routes reject it).
 * The audience is selected by the `X-Client-Type: mobile` request header at login
 * (see identity/handler.go `clientAudience`); the resulting token's `aud=mobile`
 * claim is denied by the web-only route groups (import, report export, authz admin).
 */
export async function loginMobile(email: string, password: string): Promise<Actor> {
  const anon = await request.newContext({ baseURL: API_BASE })
  try {
    const res = await anon.post('auth/login', {
      headers: { 'X-Client-Type': 'mobile' },
      data: { email, password }
    })
    if (!res.ok()) throw new Error(`loginMobile(${email}) failed: ${res.status()} ${await res.text()}`)
    const body = await res.json() as { access_token: string }
    const api = await request.newContext({
      baseURL: API_BASE,
      extraHTTPHeaders: { Authorization: `Bearer ${body.access_token}` }
    })
    const me = await (await api.get('auth/me')).json() as { id: string, office_id: string | null }
    return { email, token: body.access_token, api, userId: me.id, officeId: me.office_id ?? null }
  } finally {
    await anon.dispose()
  }
}

// ---------------------------------------------------------------------------
// Resolution against the seeded org (all reads via an admin/global context).
// ---------------------------------------------------------------------------

export async function findRoleId(admin: APIRequestContext, name: string): Promise<string> {
  const body = await (await admin.get('authz/roles')).json() as { data: Array<{ id: string, name: string }> }
  const role = body.data.find(r => r.name === name)
  if (!role) throw new Error(`findRoleId: role "${name}" not found`)
  return role.id
}

export async function findOffice(admin: APIRequestContext, code: string): Promise<Office> {
  const body = await (await admin.get(`offices?search=${encodeURIComponent(code)}&limit=50`)).json() as { data: Office[] }
  const office = body.data.find(o => o.code === code)
  if (!office) throw new Error(`findOffice: office code "${code}" not found`)
  return office
}

export async function getOfficeById(admin: APIRequestContext, id: string): Promise<Office> {
  const res = await admin.get(`offices/${id}`)
  if (!res.ok()) throw new Error(`getOfficeById(${id}) failed: ${res.status()}`)
  return await res.json() as Office
}

/** Users of a given (office, role) — active only. Includes employee_id linkage. */
export async function usersByOfficeRole(
  admin: APIRequestContext, officeId: string, roleName: string
): Promise<Array<{ id: string, email: string, employee_id: string | null }>> {
  const roleId = await findRoleId(admin, roleName)
  const body = await (await admin.get(
    `users?office_id=${officeId}&role_id=${roleId}&status=active&limit=100`
  )).json() as { data: Array<{ id: string, email: string, employee_id: string | null }> }
  return body.data
}

/** Direct child offices of a parent (admin/global scope). */
export async function childOffices(admin: APIRequestContext, parentId: string): Promise<Office[]> {
  const body = await (await admin.get('offices?limit=100')).json() as { data: Office[] }
  return body.data.filter(o => o.parent_id === parentId)
}

/** First room id of an office (for tangible assets that require a location). */
export async function firstRoomId(admin: APIRequestContext, officeId: string): Promise<string> {
  const floors = await (await admin.get(`floors?office_id=${officeId}`)).json() as { data: Array<{ id: string }> }
  if (!floors.data.length) throw new Error(`firstRoomId: no floors for office ${officeId}`)
  const rooms = await (await admin.get(`rooms?floor_id=${floors.data[0].id}`)).json() as { data: Array<{ id: string }> }
  if (!rooms.data.length) throw new Error(`firstRoomId: no rooms for office ${officeId}`)
  return rooms.data[0].id
}

export async function categoryIdByCode(admin: APIRequestContext, code: string): Promise<string> {
  const body = await (await admin.get('categories?limit=50')).json() as { data: Array<{ id: string, code: string }> }
  const cat = body.data.find(c => c.code === code)
  if (!cat) throw new Error(`categoryIdByCode: category "${code}" not found`)
  return cat.id
}

// ---------------------------------------------------------------------------
// The tiered cast for a branch scenario, resolved once per spec.
// ---------------------------------------------------------------------------

export interface BranchCast {
  admin: Actor
  branch: Office
  wilayah: Office
  pusat: Office
  sibling: Office // another branch under the same wilayah (for transfers)
  makerEmail: string // a Manager at the branch (has asset.manage + request.create)
  maker2Email: string // a second Manager at the branch (repeat-approver / distinct-maker tests)
  officeApproverEmail: string // Kepala Unit at the branch (office-tier approver)
  wilayahApproverEmail: string // Kepala Kanwil at the wilayah
  wilayahManagerEmail: string // a Manager stationed AT the wilayah (scope covers wilayah, NOT pusat)
  pusatApproverEmail: string // Pejabat Kantor Pusat (pusat-tier approver)
  stafEmail: string // a Staf at the branch (has linked employee) — self-borrow maker
  siblingApproverEmail: string // Kepala Unit at the sibling branch (transfer receiver scope)
}

/**
 * Resolves the full Lampiran A cast around a branch office code (default the
 * Bandung branch used in the doc's scenarios). Every returned email is a
 * distinct person so SoD holds across steps.
 */
export async function resolveBranchCast(admin: Actor, branchCode = 'BDG01'): Promise<BranchCast> {
  const branch = await findOffice(admin.api, branchCode)
  if (!branch.parent_id) throw new Error(`resolveBranchCast: branch ${branchCode} has no parent wilayah`)
  const wilayah = await getOfficeById(admin.api, branch.parent_id)
  const pusat = await findOffice(admin.api, 'PST')

  const managers = await usersByOfficeRole(admin.api, branch.id, 'Manager')
  const kepalaUnit = await usersByOfficeRole(admin.api, branch.id, 'Kepala Unit')
  const kepalaKanwil = await usersByOfficeRole(admin.api, wilayah.id, 'Kepala Kanwil')
  const wilayahManagers = await usersByOfficeRole(admin.api, wilayah.id, 'Manager')
  const pejabat = await usersByOfficeRole(admin.api, pusat.id, 'Pejabat Kantor Pusat')
  const stafs = await usersByOfficeRole(admin.api, branch.id, 'Staf')

  if (managers.length < 2) throw new Error(`resolveBranchCast: branch ${branchCode} needs >=2 Managers`)
  if (!kepalaUnit.length) throw new Error(`resolveBranchCast: branch ${branchCode} has no Kepala Unit`)
  if (!kepalaKanwil.length) throw new Error(`resolveBranchCast: wilayah ${wilayah.code} has no Kepala Kanwil`)
  if (!wilayahManagers.length) throw new Error(`resolveBranchCast: wilayah ${wilayah.code} has no Manager`)
  if (!pejabat.length) throw new Error('resolveBranchCast: Pusat has no Pejabat Kantor Pusat')
  const staf = stafs.find(s => s.employee_id)
  if (!staf) throw new Error(`resolveBranchCast: branch ${branchCode} has no Staf with a linked employee`)

  const siblings = (await childOffices(admin.api, wilayah.id)).filter(o => o.id !== branch.id)
  if (!siblings.length) throw new Error(`resolveBranchCast: wilayah ${wilayah.code} has no sibling branch`)
  const sibling = siblings[0]
  const siblingKepala = await usersByOfficeRole(admin.api, sibling.id, 'Kepala Unit')
  const siblingManagers = await usersByOfficeRole(admin.api, sibling.id, 'Manager')
  const siblingApprover = siblingKepala[0] || siblingManagers[0]
  if (!siblingApprover) throw new Error(`resolveBranchCast: sibling ${sibling.code} has no decider`)

  return {
    admin, branch, wilayah, pusat, sibling,
    makerEmail: managers[0].email,
    maker2Email: managers[1].email,
    officeApproverEmail: kepalaUnit[0].email,
    wilayahApproverEmail: kepalaKanwil[0].email,
    wilayahManagerEmail: wilayahManagers[0].email,
    pusatApproverEmail: pejabat[0].email,
    stafEmail: staf.email,
    siblingApproverEmail: siblingApprover.email
  }
}

// ---------------------------------------------------------------------------
// Approval-chain driving.
// ---------------------------------------------------------------------------

export interface RequestDetail {
  id: string
  type: string
  status: string
  amount: string
  current_step: number
  // The per-step approval chain is serialized as `steps` (GET /requests/:id).
  steps: Array<{ step_order: number, required_level: string, decision: string, approver_id: string | null }>
}

export async function getRequest(api: APIRequestContext, id: string): Promise<RequestDetail> {
  const res = await api.get(`requests/${id}`)
  if (!res.ok()) throw new Error(`getRequest(${id}) failed: ${res.status()}`)
  return await res.json() as RequestDetail
}

/** POST an approve decision for the current step. Returns the raw response so callers assert status. */
export function approve(actor: Actor, requestId: string, note = 'Disetujui') {
  return actor.api.post(`requests/${requestId}/approve`, { data: { decision: 'approve', note } })
}

export function reject(actor: Actor, requestId: string, note = 'Ditolak') {
  return actor.api.post(`requests/${requestId}/reject`, { data: { decision: 'reject', note } })
}

/** The tier (`required_level`) of the request's current pending step. */
export function currentLevel(req: RequestDetail): string {
  const step = (req.steps || []).find(a => a.step_order === req.current_step)
  return step ? step.required_level : ''
}

/** The ordered list of tier levels for the request's chain. */
export function stepLevels(req: RequestDetail): string[] {
  return (req.steps || []).map(s => s.required_level)
}

/** Approvers keyed by the tier they cover. */
export interface Approvers {
  office: Actor // office-tier step (Kepala Unit / Manager at the branch)
  wilayah?: Actor // wilayah-tier step (Kepala Kanwil)
  pusat?: Actor // pusat-tier step (Pejabat Kantor Pusat)
}

export function approverForLevel(approvers: Approvers, level: string): Actor {
  if (level === 'office' || level === 'office_subtree') return approvers.office
  if (level === 'wilayah') {
    if (!approvers.wilayah) throw new Error('approverForLevel: no wilayah approver supplied')
    return approvers.wilayah
  }
  if (level === 'pusat') {
    if (!approvers.pusat) throw new Error('approverForLevel: no pusat approver supplied')
    return approvers.pusat
  }
  throw new Error(`approverForLevel: unknown required_level "${level}"`)
}

/** Approves every remaining step with the tier-appropriate approver until `approved`. */
export async function driveToApproved(readApi: APIRequestContext, approvers: Approvers, requestId: string): Promise<void> {
  for (let i = 0; i < 6; i++) {
    const req = await getRequest(readApi, requestId)
    if (req.status === 'approved') return
    if (req.status !== 'pending') throw new Error(`driveToApproved: request ${requestId} is ${req.status}, not pending`)
    const level = currentLevel(req)
    const res = await approve(approverForLevel(approvers, level), requestId)
    if (!res.ok()) throw new Error(`driveToApproved: approve (level ${level}) failed: ${res.status()} ${await res.text()}`)
  }
  throw new Error('driveToApproved: exceeded step budget')
}

let counter = 0
/** A short unique suffix for fixture names (dev DB is not reset between runs). */
export function uniq(prefix = 'E2E'): string {
  counter += 1
  return `${prefix}-${Date.now().toString(36)}-${counter}`
}

/**
 * Creates an asset through the REAL asset_create maker-checker flow (submit as
 * maker, approve every tier step) and resolves the resulting asset. Intangible
 * by default (no room needed); pass klass:'tangible' + roomId for a physical one.
 */
export async function createApprovedAsset(
  admin: Actor,
  maker: Actor,
  approvers: Approvers,
  opts: { officeId: string, cost: string, klass?: 'tangible' | 'intangible', roomId?: string, name?: string }
): Promise<{ id: string, tag: string, name: string, requestId: string }> {
  const klass = opts.klass || 'intangible'
  const name = opts.name || `Aset ${uniq()}`
  const categoryId = await categoryIdByCode(admin.api, klass === 'intangible' ? 'SWL' : 'KOM')
  const payload: Record<string, unknown> = {
    name, category_id: categoryId, office_id: opts.officeId, asset_class: klass, purchase_cost: opts.cost
  }
  if (klass === 'tangible') {
    if (!opts.roomId) throw new Error('createApprovedAsset: tangible asset needs roomId')
    payload.room_id = opts.roomId
    payload.purchase_date = new Date().toISOString().slice(0, 10)
  }
  const submitRes = await maker.api.post('requests', {
    data: { type: 'asset_create', amount: opts.cost, office_id: opts.officeId, reason: `create ${name}`, payload }
  })
  if (submitRes.status() !== 201) throw new Error(`createApprovedAsset submit failed: ${submitRes.status()} ${await submitRes.text()}`)
  const requestId = (await submitRes.json() as { id: string }).id
  await driveToApproved(admin.api, approvers, requestId)

  const search = await admin.api.get(`assets?search=${encodeURIComponent(name)}&limit=5`)
  const found = (await search.json() as { data: Array<{ id: string, asset_tag: string }> }).data
  if (!found.length) throw new Error(`createApprovedAsset: asset "${name}" not found after approval`)
  return { id: found[0].id, tag: found[0].asset_tag, name, requestId }
}

/** Picks an existing available asset in an office (admin/global read). */
export async function pickAvailableAsset(
  admin: Actor, officeId: string, opts: { minCost?: number } = {}
): Promise<{ id: string, tag: string, purchase_cost: string }> {
  const body = await (await admin.api.get(`assets?office_id=${officeId}&status=available&limit=100`))
    .json() as { data: Array<{ id: string, asset_tag: string, purchase_cost: string }> }
  const items = body.data.filter(a => !opts.minCost || Number(a.purchase_cost) >= opts.minCost)
  if (!items.length) throw new Error(`pickAvailableAsset: no available asset (minCost ${opts.minCost}) in office ${officeId}`)
  return { id: items[0].id, tag: items[0].asset_tag, purchase_cost: items[0].purchase_cost }
}

/** Reads an asset's current status (admin/global). */
export async function assetStatus(admin: Actor, assetId: string): Promise<string> {
  const res = await admin.api.get(`assets/${assetId}`)
  if (!res.ok()) throw new Error(`assetStatus(${assetId}) failed: ${res.status()}`)
  return (await res.json() as { status: string }).status
}
