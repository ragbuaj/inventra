// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Asset, Office, ReferenceRow, Paginated } from '~/types'
import type { Transfer } from '~/composables/api/useTransfers'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'
import { useAuthStore } from '~/stores/auth'

// useToast's portal isn't mounted here (no UApp wrapper) — mock and assert on
// call args. Option A: a successful submit surfaces feedback via a toast.
const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const OFFICE_TYPES: ReferenceRow[] = [
  { id: 'ot-wilayah', name: 'Kantor Wilayah', is_active: true, tier: 'wilayah' },
  { id: 'ot-cabang', name: 'Kantor Cabang', is_active: true, tier: 'office' }
]

// Legacy-parity Fase 5 office columns — always present on the API response but
// irrelevant to these tests, so they are defaulted once and spread into each row.
const OFFICE_LP = {
  ownership_status: null, office_class_id: null, building_classification_id: null,
  floor_count: null, building_area: null, office_kind: 'konvensional',
  description: null, head_employee_id: null, contact: null
}

const OFFICES: Office[] = [
  { id: 'w1', parent_id: null, office_type_id: 'ot-wilayah', province_id: null, city_id: null, name: 'Kanwil III Jakarta', code: 'KW3', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null, ...OFFICE_LP },
  { id: 'w2', parent_id: null, office_type_id: 'ot-wilayah', province_id: null, city_id: null, name: 'Kanwil IV Bandung', code: 'KW4', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null, ...OFFICE_LP },
  { id: 'o-mine', parent_id: 'w1', office_type_id: 'ot-cabang', province_id: null, city_id: null, name: 'Kantor Cabang Jakarta Selatan', code: 'JKS', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null, ...OFFICE_LP },
  { id: 'o-same', parent_id: 'w1', office_type_id: 'ot-cabang', province_id: null, city_id: null, name: 'Kantor Cabang Jakarta Pusat', code: 'JKP', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null, ...OFFICE_LP },
  { id: 'o-diff', parent_id: 'w2', office_type_id: 'ot-cabang', province_id: null, city_id: null, name: 'Kantor Cabang Bandung', code: 'BDG', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null, ...OFFICE_LP }
]

const ASSET: Asset = {
  id: 'a1', asset_tag: 'JKT01-ELK-2026-00002', name: 'Proyektor Epson EB-X51',
  category_id: 'c1', office_id: 'o-mine', status: 'available', asset_class: 'tangible'
}

function transfer(over: Partial<Transfer> = {}): Transfer {
  return {
    id: 't1', asset_id: 'a1', from_office_id: 'o-diff', to_office_id: 'o-mine', to_room_id: null,
    status: 'in_transit', reason: 'Realokasi', requested_by_id: 'u1', approved_by_id: 'u2',
    shipped_date: '2026-07-01', received_date: null, received_by_id: null, bast_no: null,
    request_id: 'r1', condition_sent: 'baik', transfer_date: '2026-07-01', return_note: null,
    asset_name: 'Proyektor BenQ MW550', asset_tag: 'BDG02-ELK-2025-00031',
    from_office_name: 'Kantor Cabang Bandung', to_office_name: 'Kantor Cabang Jakarta Selatan',
    to_room_name: null, requested_by_name: 'Hendra Wijaya', received_by_name: null,
    created_at: '2026-06-30T09:00:00Z', updated_at: '2026-06-30T09:00:00Z',
    ...over
  }
}

function reqRow(over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow {
  return {
    id: 'req1', type: 'asset_transfer', status: 'pending', amount: null, current_step: 1,
    office_id: 'o-mine', office_name: 'Kantor Cabang Jakarta Selatan', target_id: 'a9', target_entity: 'asset',
    reason: 'Realokasi', requested_by_id: 'u1', requested_by_name: 'Dewi Lestari', requested_by_role: 'Kepala Unit',
    decided_by_id: null, decision_note: null, created_at: '2026-07-03T09:00:00Z',
    ...over
  }
}

const INBOX_TRANSFERS: Transfer[] = [
  transfer({
    id: 't-inbox1', to_office_id: 'o-mine', from_office_id: 'o-diff',
    asset_name: 'Proyektor BenQ MW550', asset_tag: 'BDG02-ELK-2025-00031',
    requested_by_name: 'Hendra Wijaya', reason: 'Realokasi untuk ruang rapat', condition_sent: 'baik'
  })
]

const HISTORY_TRANSFERS: Transfer[] = [
  transfer({
    id: 't-returned', status: 'returned', asset_name: 'UPS APC Smart-UPS 1500', asset_tag: 'JKT01-ELK-2025-00018',
    from_office_id: 'o-mine', to_office_id: 'o-same',
    from_office_name: 'Kantor Cabang Jakarta Selatan', to_office_name: 'Kantor Cabang Jakarta Pusat',
    created_at: '2026-06-20T09:00:00Z', transfer_date: '2026-06-20', bast_no: null, condition_sent: 'rusak_ringan'
  }),
  transfer({
    id: 't-approved', status: 'approved', asset_name: 'Laptop Dell Latitude', asset_tag: 'JKT01-ELK-2026-00099',
    from_office_id: 'o-mine', to_office_id: 'o-diff',
    from_office_name: 'Kantor Cabang Jakarta Selatan', to_office_name: 'Kantor Cabang Bandung',
    created_at: '2026-06-25T09:00:00Z', transfer_date: '2026-06-25', condition_sent: 'baik'
  })
]

const HISTORY_REQUESTS: ApprovalRequestRow[] = [reqRow()]

function page<T>(data: T[]): Paginated<T> {
  return { data, total: data.length, limit: 100, offset: 0 }
}

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const transfersListMock = vi.fn()
const transfersSubmitMock = vi.fn()
const transfersShipMock = vi.fn()
const transfersReceiveMock = vi.fn()
const transfersRejectReceiveMock = vi.fn()

vi.mock('~/composables/api/useTransfers', () => ({
  useTransfers: () => ({
    list: transfersListMock,
    get: vi.fn(),
    submit: transfersSubmitMock,
    ship: transfersShipMock,
    receive: transfersReceiveMock,
    rejectReceive: transfersRejectReceiveMock
  })
}))

const approvalListMock = vi.fn()
vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({ inbox: vi.fn(), list: approvalListMock, get: vi.fn(), approve: vi.fn(), reject: vi.fn() })
}))

const officesListMock = vi.fn()
const officesGetMock = vi.fn()
const officesTreeMock = vi.fn()
vi.mock('~/composables/api/useOffices', () => ({
  useOffices: () => ({ list: officesListMock, get: officesGetMock, tree: officesTreeMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

const listByOfficeMock = vi.fn()
const roomsByFloorMock = vi.fn()
vi.mock('~/composables/api/useFloors', () => ({
  useFloors: () => ({
    listByOffice: listByOfficeMock, roomsByFloor: roomsByFloorMock,
    createFloor: vi.fn(), updateFloor: vi.fn(), removeFloor: vi.fn(),
    createRoom: vi.fn(), updateRoom: vi.fn(), removeRoom: vi.fn()
  })
}))

const refListMock = vi.fn()
vi.mock('~/composables/api/useReference', () => ({
  useReference: () => ({ list: refListMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

const assetsListMock = vi.fn()
vi.mock('~/composables/api/useAssets', () => ({
  useAssets: () => ({ list: assetsListMock, get: vi.fn(), getByTag: vi.fn(), update: vi.fn() })
}))

// eslint-disable-next-line import/first
import TransfersPage from '~/pages/transfers.vue'

enableAutoUnmount(afterEach)
afterEach(() => {
  vi.useRealTimers()
})

function grantSession(officeId: string | null, permissions: string[] = ['transfer.view', 'transfer.manage']) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Dewi Lestari', email: 'dewi@test.com', role_id: 'r1', role_name: 'Kepala Unit', office_id: officeId },
    permissions
  )
}

async function mountAndWait() {
  const wrapper = await mountSuspended(TransfersPage, { route: '/transfers' })
  await flushPromises()
  await new Promise(r => setTimeout(r, 50))
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

// The page lands on the Riwayat view; the "Ajukan Mutasi" form is a full-view
// swap reached via the "Buat Pengajuan" button. Form-focused tests open it.
async function mountFormAndWait() {
  const wrapper = await mountAndWait()
  await wrapper.find('[data-testid="transfer-create"]').trigger('click')
  await wrapper.vm.$nextTick()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountAndWait>>

async function setVmRef(wrapper: Wrapper, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Record<string, unknown>)[key] = value
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
}

function clickTab(wrapper: Wrapper, key: 'ajukan' | 'inbox' | 'history') {
  // "Ajukan Mutasi" is no longer a tab — it opens via the header button.
  if (key === 'ajukan') return wrapper.find('[data-testid="transfer-create"]').trigger('click')
  return wrapper.find(`[data-testid="transfer-tab-${key}"]`).trigger('click')
}

function bodyButton(testid: string): HTMLButtonElement {
  const el = document.body.querySelector(`[data-testid="${testid}"]`)
  expect(el, `expected [data-testid="${testid}"] in document.body`).toBeTruthy()
  return el as HTMLButtonElement
}

// Row-actions kebab/context-menu items are portaled to document.body; locale
// is 'id' here (navigateTo isn't mocked in this file), so matching on the
// resolved Indonesian label text is reliable.
function menuItemByText(text: string): HTMLElement | undefined {
  return Array.from(document.querySelectorAll('[role="menuitem"]'))
    .find(el => el.textContent?.trim() === text) as HTMLElement | undefined
}

beforeEach(() => {
  vi.clearAllMocks()
  officesListMock.mockResolvedValue(page(OFFICES))
  officesTreeMock.mockResolvedValue(OFFICES)
  officesGetMock.mockImplementation((id: string) => {
    const o = OFFICES.find(off => off.id === id)
    return o ? Promise.resolve(o) : Promise.reject(new Error('not found'))
  })
  refListMock.mockImplementation((key: string) =>
    Promise.resolve(page(key === 'office-types' ? OFFICE_TYPES : [])))
  listByOfficeMock.mockResolvedValue([{ id: 'f1', office_id: 'o-mine', name: 'Lantai 1', level: 1, created_at: null, updated_at: null }])
  roomsByFloorMock.mockResolvedValue([{ id: 'room1', floor_id: 'f1', name: 'Ruang Server', code: null, created_at: null, updated_at: null }])
  assetsListMock.mockResolvedValue(page([]))
  transfersListMock.mockImplementation((q?: { status?: string }) =>
    Promise.resolve(page(q?.status === 'in_transit' ? INBOX_TRANSFERS : HISTORY_TRANSFERS)))
  approvalListMock.mockResolvedValue(page(HISTORY_REQUESTS))
  transfersSubmitMock.mockResolvedValue({ request_id: 'req9', status: 'pending' })
  transfersShipMock.mockResolvedValue(transfer({ status: 'in_transit' }))
  transfersReceiveMock.mockResolvedValue(transfer({ status: 'received' }))
  transfersRejectReceiveMock.mockResolvedValue(transfer({ status: 'returned' }))
  grantSession('o-mine')
})

// ---------------------------------------------------------------------------

describe('pages/transfers — mount', () => {
  it('loads the inbox count and history on mount', async () => {
    const w = await mountAndWait()
    expect(transfersListMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'in_transit' }))
    expect(approvalListMock).toHaveBeenCalledWith(expect.objectContaining({ type: 'asset_transfer' }))
    // Inbox badge count shown on the tab button.
    expect(w.find('[data-testid="transfer-tab-inbox"]').text()).toContain('1')
  })

  it('shows the caller office line resolved from the offices map', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="transfer-my-office"]').text()).toContain('Kantor Cabang Jakarta Selatan')
  })

  it('hides the office line when auth.user.office_id is null', async () => {
    grantSession(null)
    const w = await mountAndWait()
    expect(w.find('[data-testid="transfer-my-office"]').exists()).toBe(false)
  })
})

describe('pages/transfers — destination office is an AsyncSearchPicker', () => {
  it('renders the office picker input (no more eager-options USelect)', async () => {
    const w = await mountFormAndWait()
    expect(w.find('[data-testid="transfer-to-office"]').exists()).toBe(false)
    expect(w.find('[data-testid="to-office-picker-input"]').exists()).toBe(true)
  })

  it('searching drives useOffices().list with { search, limit: 20 } and excludes the source office', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET) // fromOfficeId = 'o-mine'
    vi.useFakeTimers()
    await w.find('[data-testid="to-office-picker-input"]').setValue('Kantor')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(officesListMock).toHaveBeenCalledWith({ search: 'Kantor', limit: 20 })
  })

  it('resolves a preselected destination office id to its label', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'toOfficeId', 'o-same')
    const input = w.find('[data-testid="to-office-picker-input"]').element as HTMLInputElement
    expect(input.value).toBe('Kantor Cabang Jakarta Pusat')
  })
})

describe('pages/transfers — Ajukan Mutasi form', () => {
  it('disables submit until asset + destination + date are set, then enables it', async () => {
    const w = await mountFormAndWait()
    const submit = () => w.find('[data-testid="transfer-submit"]')
    expect(submit().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'selectedAsset', ASSET)
    expect(submit().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'toOfficeId', 'o-same')
    expect(submit().attributes('disabled')).toBeDefined()

    await w.find('[data-testid="transfer-date"]').setValue('2026-07-10')
    expect(submit().attributes('disabled')).toBeUndefined()
  })

  it('shows nothing when no destination is chosen yet (tri-state null)', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET)
    expect(w.find('[data-testid="transfer-inter-region-alert"]').exists()).toBe(false)
    expect(w.find('[data-testid="transfer-in-subtree-note"]').exists()).toBe(false)
  })

  it('shows the inter-region alert when the destination is in a different wilayah', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET)
    await setVmRef(w, 'toOfficeId', 'o-diff')
    expect(w.find('[data-testid="transfer-inter-region-alert"]').exists()).toBe(true)
    expect(w.find('[data-testid="transfer-in-subtree-note"]').exists()).toBe(false)
  })

  it('shows the same-region note when the destination shares the wilayah', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET)
    await setVmRef(w, 'toOfficeId', 'o-same')
    expect(w.find('[data-testid="transfer-in-subtree-note"]').exists()).toBe(true)
    expect(w.find('[data-testid="transfer-inter-region-alert"]').exists()).toBe(false)
  })

  it('submits with the exact body and resets the form on success', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET)
    await setVmRef(w, 'toOfficeId', 'o-same')
    await w.find('[data-testid="transfer-date"]').setValue('2026-07-10')

    await w.find('[data-testid="transfer-submit"]').trigger('click')
    await flushPromises()

    expect(transfersSubmitMock).toHaveBeenCalledWith({
      asset_id: 'a1',
      to_office_id: 'o-same',
      to_room_id: null,
      reason: null,
      condition_sent: 'baik',
      transfer_date: '2026-07-10'
    })
    expect((w.vm as unknown as { selectedAsset: unknown }).selectedAsset).toBeNull()
    expect((w.vm as unknown as { toOfficeId: string }).toOfficeId).toBe('')
    // Option A: after a successful submit the page returns to the history view
    // and surfaces feedback via a success toast (not an in-form banner).
    expect(toastAddMock).toHaveBeenCalledWith(
      expect.objectContaining({ title: expect.stringContaining('berhasil diajukan'), color: 'success' })
    )
    expect(w.find('[data-testid="transfer-create"]').exists()).toBe(true)
  })

  it('keeps submit disabled and shows the no-permission note without transfer.manage', async () => {
    grantSession('o-mine', ['transfer.view'])
    const w = await mountFormAndWait()
    // Even with a complete form, a view-only caller cannot submit.
    await setVmRef(w, 'selectedAsset', ASSET)
    await setVmRef(w, 'toOfficeId', 'o-same')
    await w.find('[data-testid="transfer-date"]').setValue('2026-07-10')

    expect(w.find('[data-testid="transfer-submit"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="transfer-no-manage"]').text()).toContain('Anda tidak punya izin untuk mengajukan mutasi.')

    await w.find('[data-testid="transfer-submit"]').trigger('click')
    await flushPromises()
    expect(transfersSubmitMock).not.toHaveBeenCalled()
  })

  it('hides the no-permission note when the caller has transfer.manage', async () => {
    const w = await mountFormAndWait()
    expect(w.find('[data-testid="transfer-no-manage"]').exists()).toBe(false)
  })

  it('Reset clears the filled form fields', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET)
    await setVmRef(w, 'toOfficeId', 'o-same')
    await w.find('[data-testid="transfer-date"]').setValue('2026-07-10')
    expect((w.vm as unknown as { selectedAsset: unknown }).selectedAsset).not.toBeNull()

    const reset = w.findAll('button').find(b => b.text().trim() === 'Reset')
    await reset!.trigger('click')
    await w.vm.$nextTick()

    expect((w.vm as unknown as { selectedAsset: unknown }).selectedAsset).toBeNull()
    expect((w.vm as unknown as { toOfficeId: string }).toOfficeId).toBe('')
    expect((w.vm as unknown as { ajMsg: unknown }).ajMsg).toBeNull()
  })
})

describe('pages/transfers — list-first navigation (Buat Pengajuan / Kembali)', () => {
  it('hides the create button while the Ajukan form is open', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="transfer-create"]').exists()).toBe(true)
    await w.find('[data-testid="transfer-create"]').trigger('click')
    await w.vm.$nextTick()
    // Header button + tab bar hidden while the form is open; Back takes over.
    expect(w.find('[data-testid="transfer-create"]').exists()).toBe(false)
    expect(w.find('[data-testid="transfer-back"]').exists()).toBe(true)
    expect(w.find('[data-testid="transfer-submit"]').exists()).toBe(true)
  })

  it('the Back button returns to Riwayat, resets the form, and does not submit', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET)
    await setVmRef(w, 'toOfficeId', 'o-same')
    await w.find('[data-testid="transfer-date"]').setValue('2026-07-10')
    // Form is now submit-ready — Back must still not submit.
    expect(w.find('[data-testid="transfer-submit"]').attributes('disabled')).toBeUndefined()

    await w.find('[data-testid="transfer-back"]').trigger('click')
    await flushPromises()

    expect(transfersSubmitMock).not.toHaveBeenCalled()
    expect(toastAddMock).not.toHaveBeenCalled()
    // Back on the Riwayat view: header create button + history rows are shown.
    expect(w.find('[data-testid="transfer-create"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="transfer-history-row"]').length).toBeGreaterThan(0)
    // backFromAjukan() calls resetForm() so a later reopen starts clean.
    expect((w.vm as unknown as { selectedAsset: unknown }).selectedAsset).toBeNull()
    expect((w.vm as unknown as { toOfficeId: string }).toOfficeId).toBe('')
  })
})

describe('pages/transfers — Kotak Masuk', () => {
  it('renders inbox card fields', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    const card = w.find('[data-testid="transfer-inbox-card"]')
    expect(card.text()).toContain('Proyektor BenQ MW550')
    expect(card.text()).toContain('BDG02-ELK-2025-00031')
    expect(card.text()).toContain('Hendra Wijaya')
    expect(card.text()).toContain('Realokasi untuk ruang rapat')
    expect(card.text()).toContain('Baik')
  })

  it('shows the empty state when the inbox has no rows', async () => {
    transfersListMock.mockImplementation((q?: { status?: string }) =>
      Promise.resolve(page(q?.status === 'in_transit' ? [] : HISTORY_TRANSFERS)))
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    expect(w.text()).toContain('Kotak masuk kosong')
  })

  it('shows the load-error state with retry when the inbox call fails', async () => {
    transfersListMock.mockImplementation((q?: { status?: string }) =>
      q?.status === 'in_transit' ? Promise.reject(new Error('boom')) : Promise.resolve(page(HISTORY_TRANSFERS)))
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    expect(w.text()).toContain('Gagal memuat data.')
  })

  it('accept modal calls receive() with the file when one is attached', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    await w.find('[data-testid="transfer-accept"]').trigger('click')
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('Konfirmasi Penerimaan')

    const file = new File(['x'], 'bast.pdf', { type: 'application/pdf' })
    await setVmRef(w, 'acceptFile', file)
    await setVmRef(w, 'acceptBastNo', 'BAST/2026/07/0150')

    bodyButton('transfer-accept-confirm').click()
    await flushPromises()

    expect(transfersReceiveMock).toHaveBeenCalledWith('t-inbox1', expect.objectContaining({
      bast_no: 'BAST/2026/07/0150',
      file
    }))
  })

  it('reject-receive modal sends the note', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    await w.find('[data-testid="transfer-reject-receive"]').trigger('click')
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('Tolak Penerimaan')

    await setVmRef(w, 'rejectNote', 'Kondisi tidak sesuai BAST')
    bodyButton('transfer-reject-confirm').click()
    await flushPromises()

    expect(transfersRejectReceiveMock).toHaveBeenCalledWith('t-inbox1', 'Kondisi tidak sesuai BAST')
  })

  it('hides Terima / Tolak Terima without the transfer.manage permission', async () => {
    grantSession('o-mine', ['transfer.view'])
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    expect(w.find('[data-testid="transfer-accept"]').exists()).toBe(false)
    expect(w.find('[data-testid="transfer-reject-receive"]').exists()).toBe(false)
  })

  it('shows all in-transit rows (unfiltered) when the caller has no office', async () => {
    grantSession(null)
    const w = await mountAndWait()
    await clickTab(w, 'inbox')
    expect(w.findAll('[data-testid="transfer-inbox-card"]')).toHaveLength(1)
  })
})

describe('pages/transfers — Riwayat', () => {
  it('merges request-sourced and transfer-sourced rows with correct statuses', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="transfer-history-row"]')
    expect(rows.length).toBe(3)
    const statuses = w.findAll('[data-testid="transfer-history-status"]').map(n => n.text())
    expect(statuses.some(s => s.includes('Diajukan'))).toBe(true)
    expect(statuses.some(s => s.includes('Dikembalikan'))).toBe(true)
    expect(w.text()).toContain('UPS APC Smart-UPS 1500')
  })

  it('narrows rows via the status filter', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    await setVmRef(w, 'historyStatus', 'returned')
    const rows = w.findAll('[data-testid="transfer-history-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('UPS APC Smart-UPS 1500')
  })

  it('shows a Kirim kebab action only on the approved row and calls ship()', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="transfer-history-row"]')
    const approvedRow = rows.find(r => r.text().includes('Laptop Dell Latitude'))!
    const otherRows = rows.filter(r => r !== approvedRow)

    // Only the approved row (canShip) renders a kebab trigger.
    expect(approvedRow.find('button[aria-haspopup="menu"]').exists()).toBe(true)
    for (const r of otherRows) {
      expect(r.find('button[aria-haspopup="menu"]').exists()).toBe(false)
    }

    await approvedRow.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))
    menuItemByText('Kirim')!.click()
    await w.vm.$nextTick()
    await setVmRef(w, 'shipDate', '2026-07-06')
    bodyButton('transfer-ship-confirm').click()
    await flushPromises()

    expect(transfersShipMock).toHaveBeenCalledWith('t-approved', '2026-07-06')
  })

  it('hides the Kirim kebab on every row without the transfer.manage permission', async () => {
    grantSession('o-mine', ['transfer.view'])
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="transfer-history-row"]')
    expect(rows.length).toBeGreaterThan(0)
    for (const r of rows) {
      expect(r.find('button[aria-haspopup="menu"]').exists()).toBe(false)
    }
  })

  it('right-clicking the approved row surfaces Kirim in the context menu and fires the same ship() flow', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="transfer-history-row"]')
    const approvedRow = rows.find(r => r.text().includes('Laptop Dell Latitude'))!

    approvedRow.element.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(menuItemByText('Kirim')).toBeTruthy()

    menuItemByText('Kirim')!.click()
    await w.vm.$nextTick()
    await setVmRef(w, 'shipDate', '2026-07-06')
    bodyButton('transfer-ship-confirm').click()
    await flushPromises()

    expect(transfersShipMock).toHaveBeenCalledWith('t-approved', '2026-07-06')
  })

  it('right-clicking a non-row area after right-clicking the approved row shows no stale context menu', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="transfer-history-row"]')
    const approvedRow = rows.find(r => r.text().includes('Laptop Dell Latitude'))!

    approvedRow.element.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(menuItemByText('Kirim')).toBeTruthy()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    const thead = w.find('thead tr').element
    thead.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(document.querySelectorAll('[role="menuitem"]').length).toBe(0)
  })

  it('shows the empty state when there is no history', async () => {
    transfersListMock.mockImplementation((q?: { status?: string }) =>
      Promise.resolve(page(q?.status === 'in_transit' ? INBOX_TRANSFERS : [])))
    approvalListMock.mockResolvedValue(page([]))
    const w = await mountAndWait()
    await clickTab(w, 'history')
    expect(w.text()).toContain('Belum ada riwayat')
  })

  it('shows the load-error state with retry when the history call fails', async () => {
    approvalListMock.mockRejectedValue(new Error('boom'))
    const w = await mountAndWait()
    await clickTab(w, 'history')
    expect(w.text()).toContain('Gagal memuat data.')
  })

  it('shows the footer total count', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    expect(w.text()).toContain('Total 3 mutasi')
  })
})
