// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Asset, Office, Paginated } from '~/types'
import type { Disposal } from '~/composables/api/useDisposals'
import type { ApprovalRequestRow, ApprovalRequestDetail, ApprovalStep } from '~/composables/api/useApproval'
import type { PreviewStep } from '~/composables/api/useApprovalPreview'
import type { AssetDepreciationResponse } from '~/composables/api/useDepreciation'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// Legacy-parity Fase 5 office columns — present on the API response but irrelevant here.
const OFFICE_LP = {
  ownership_status: null, office_class_id: null, building_classification_id: null,
  floor_count: null, building_area: null, office_kind: 'konvensional',
  description: null, head_employee_id: null, contact: null
}

const OFFICES: Office[] = [
  { id: 'o1', parent_id: null, office_type_id: 'ot1', province_id: null, city_id: null, name: 'Kantor Cabang Jakarta Selatan', code: 'JKS', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null, ...OFFICE_LP }
]

const ASSET_LABA: Asset = {
  id: 'a1', asset_tag: 'JKT01-ELK-2024-00021', name: 'Printer HP LaserJet Pro',
  category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible',
  purchase_cost: '6800000', accumulated_depreciation: '6120000', book_value: '680000'
}

const ASSET_RUGI: Asset = {
  id: 'a2', asset_tag: 'JKT01-KEN-2019-00003', name: 'Toyota Innova 2019',
  category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible',
  purchase_cost: '315000000', accumulated_depreciation: '236000000', book_value: '79000000'
}

// Fully masked (purchase_cost/book_value/accumulated_depreciation all absent).
const ASSET_MASKED: Asset = {
  id: 'a3', asset_tag: 'JKT01-ELK-2022-00099', name: 'Server IBM System x3650 (Lama)',
  category_id: 'c1', office_id: 'o1', status: 'under_maintenance', asset_class: 'tangible'
}

// purchase_cost visible, book_value specifically masked (a rarer but valid field-permission split).
const ASSET_BV_MASKED: Asset = {
  id: 'a4', asset_tag: 'JKT01-MBL-2020-00040', name: 'Partisi Kubikel (30 unit)',
  category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible',
  purchase_cost: '42000000', accumulated_depreciation: '33600000'
}

function disposal(over: Partial<Disposal> = {}): Disposal {
  return {
    id: 'd1', asset_id: 'a9', method: 'sale', disposal_date: '2026-06-15',
    proceeds: '1200000', book_value_at_disposal: '450000', gain_loss: '750000',
    bast_no: 'BAP/2026/06/010', approved_by_id: 'u2', request_id: 'req-old', created_by_id: 'u1',
    asset_name: 'Laptop Asus X441 (Lama)', asset_tag: 'JKT01-ELK-2021-00055', office_name: 'Kantor Cabang Jakarta Selatan',
    created_by_name: 'Dewi Lestari', created_at: '2026-06-15T09:00:00Z', updated_at: '2026-06-15T09:00:00Z',
    ...over
  }
}

function reqRow(over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow {
  return {
    id: 'req1', type: 'asset_disposal', status: 'pending', amount: '79000000', current_step: 1,
    office_id: 'o1', office_name: 'Kantor Cabang Jakarta Selatan', target_id: 'a9', target_entity: 'asset',
    reason: 'Rusak berat', requested_by_id: 'u1', requested_by_name: 'Dewi Lestari', requested_by_role: 'Kepala Unit',
    decided_by_id: null, decision_note: null, created_at: '2026-07-02T09:00:00Z',
    ...over
  }
}

function page<T>(data: T[]): Paginated<T> {
  return { data, total: data.length, limit: 100, offset: 0 }
}

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const disposalsListMock = vi.fn()
const disposalsSubmitMock = vi.fn()
const disposalsAttachDocumentMock = vi.fn()
vi.mock('~/composables/api/useDisposals', () => ({
  useDisposals: () => ({
    list: disposalsListMock,
    get: vi.fn(),
    submit: disposalsSubmitMock,
    attachDocument: disposalsAttachDocumentMock
  })
}))

const approvalListMock = vi.fn()
const approvalGetMock = vi.fn()
vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({ inbox: vi.fn(), list: approvalListMock, get: approvalGetMock, approve: vi.fn(), reject: vi.fn() })
}))

const previewMock = vi.fn()
vi.mock('~/composables/api/useApprovalPreview', () => ({
  useApprovalPreview: () => ({ preview: previewMock })
}))

const depAssetScheduleMock = vi.fn()
vi.mock('~/composables/api/useDepreciation', () => ({
  useDepreciation: () => ({
    periods: vi.fn(), compute: vi.fn(), close: vi.fn(), schedule: vi.fn(), journal: vi.fn(), exportJournal: vi.fn(),
    assetSchedule: depAssetScheduleMock, recordImpairment: vi.fn()
  })
}))

const attachmentsUploadMock = vi.fn()
const attachmentsRemoveMock = vi.fn()
vi.mock('~/composables/api/useAssetAttachments', () => ({
  useAssetAttachments: () => ({
    list: vi.fn(), upload: attachmentsUploadMock, remove: attachmentsRemoveMock,
    thumbnailBlob: vi.fn(), contentBlob: vi.fn()
  })
}))

const officesListMock = vi.fn()
const officesTreeMock = vi.fn()
vi.mock('~/composables/api/useOffices', () => ({
  useOffices: () => ({ list: officesListMock, get: vi.fn(), tree: officesTreeMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

const assetsListMock = vi.fn()
vi.mock('~/composables/api/useAssets', () => ({
  useAssets: () => ({ list: assetsListMock, get: vi.fn(), getByTag: vi.fn(), update: vi.fn() })
}))

// eslint-disable-next-line import/first
import DisposalsPage from '~/pages/disposals.vue'

enableAutoUnmount(afterEach)

function grantSession(permissions: string[] = ['disposal.view', 'disposal.manage']) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Dewi Lestari', email: 'dewi@test.com', role_id: 'r1', role_name: 'Kepala Unit', office_id: 'o1' },
    permissions
  )
}

async function mountAndWait() {
  const wrapper = await mountSuspended(DisposalsPage, { route: '/disposals' })
  await flushPromises()
  await new Promise(r => setTimeout(r, 50))
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

// The page lands on the Riwayat view; the "Ajukan Penghapusan" form is a
// full-view swap reached via the "Buat Pengajuan" button. Form-focused tests
// open it first.
async function mountFormAndWait() {
  const wrapper = await mountAndWait()
  await wrapper.find('[data-testid="disposal-create"]').trigger('click')
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

function clickTab(wrapper: Wrapper, key: 'ajukan' | 'history') {
  if (key === 'ajukan') return wrapper.find('[data-testid="disposal-create"]').trigger('click')
  // History is the default landing view; return to it via Back if the form is open.
  const back = wrapper.find('[data-testid="disposal-back"]')
  return back.exists() ? back.trigger('click') : wrapper.vm.$nextTick()
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

function previewStep(over: Partial<PreviewStep> = {}): PreviewStep {
  return { step_order: 2, required_level: 'wilayah', ...over }
}

function scheduleResp(over: Partial<AssetDepreciationResponse> = {}): AssetDepreciationResponse {
  return { masked: false, computed_book_value: null, entries: [], ...over }
}

beforeEach(() => {
  vi.clearAllMocks()
  officesListMock.mockResolvedValue(page(OFFICES))
  officesTreeMock.mockResolvedValue(OFFICES)
  assetsListMock.mockResolvedValue(page([]))
  disposalsListMock.mockResolvedValue(page([disposal()]))
  approvalListMock.mockResolvedValue(page([reqRow()]))
  previewMock.mockResolvedValue([previewStep({ step_order: 2, required_level: 'wilayah' })])
  depAssetScheduleMock.mockResolvedValue(scheduleResp())
  disposalsSubmitMock.mockResolvedValue({ request_id: 'req9', status: 'pending' })
  disposalsAttachDocumentMock.mockResolvedValue({ document_id: 'doc1', disposal_id: 'd1' })
  attachmentsUploadMock.mockResolvedValue({ id: 'att1', asset_id: 'a1', kind: 'evidence', original_filename: 'foto.jpg', size_bytes: 1024, mime_type: 'image/jpeg', has_thumbnail: false, created_at: '2026-07-04T00:00:00Z' })
  attachmentsRemoveMock.mockResolvedValue(undefined)
  grantSession()
})

// ---------------------------------------------------------------------------

describe('pages/disposals — Ringkasan Valuasi', () => {
  it('renders acquisition, accumulated depreciation and book value once an asset is selected', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-valuation-acquisition"]').text()).toContain('6.800.000')
    expect(w.find('[data-testid="disposal-valuation-accum"]').text()).toContain('6.120.000')
    expect(w.find('[data-testid="disposal-valuation-book-commercial"]').text()).toContain('680.000')
  })

  it('renders masked "•••" for absent money fields instead of Rp 0', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_MASKED)
    expect(w.find('[data-testid="disposal-valuation-acquisition"]').text()).toContain('•••')
    expect(w.find('[data-testid="disposal-valuation-accum"]').text()).toContain('•••')
    expect(w.find('[data-testid="disposal-valuation-book-commercial"]').text()).toContain('•••')
  })

  it('shows "—" for the fiscal book value chip when the schedule has no fiscal entries yet', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(depAssetScheduleMock).toHaveBeenCalledWith('a1')
    expect(w.find('[data-testid="disposal-valuation-book-fiscal"]').text()).toBe('—')
  })

  it('renders the real fiscal book value from the stubbed depreciation schedule (closing of the last fiscal entry)', async () => {
    depAssetScheduleMock.mockResolvedValue(scheduleResp({
      computed_book_value: '650000',
      entries: [
        { basis: 'fiscal', period: '2026-04', opening: '750000', amount: '35000', closing: '715000', method: 'declining_balance' },
        { basis: 'fiscal', period: '2026-05', opening: '715000', amount: '35000', closing: '680000', method: 'declining_balance' },
        { basis: 'commercial', period: '2026-05', opening: '700000', amount: '20000', closing: '680000', method: 'straight_line' }
      ]
    }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-valuation-book-fiscal"]').text()).toContain('680.000')
  })

  it('prefers the schedule\'s computed_book_value over the asset\'s stale book_value column for the commercial cell', async () => {
    depAssetScheduleMock.mockResolvedValue(scheduleResp({ computed_book_value: '555000' }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-valuation-book-commercial"]').text()).toContain('555.000')
  })
})

describe('pages/disposals — Laba/Rugi card', () => {
  it('shows the empty state when no asset is selected', async () => {
    const w = await mountFormAndWait()
    expect(w.find('[data-testid="disposal-gainloss-empty"]').exists()).toBe(true)
  })

  it('shows the empty state when an asset is selected but no proceeds are entered', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-gainloss-empty"]').exists()).toBe(true)
  })

  it('renders a green gain (+ Rp) when proceeds exceed book value', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    const value = w.find('[data-testid="disposal-gainloss-value"]')
    expect(value.text()).toContain('+')
    expect(value.classes()).toContain('text-success')
  })

  it('renders a red loss (− Rp) when proceeds are below book value', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_RUGI)
    await setVmRef(w, 'proceedsRaw', '50000000')
    const value = w.find('[data-testid="disposal-gainloss-value"]')
    expect(value.text()).toContain('−')
    expect(value.classes()).toContain('text-error')
  })

  it('renders a neutral break-even when proceeds equal book value', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '680000')
    const value = w.find('[data-testid="disposal-gainloss-value"]')
    expect(value.classes()).not.toContain('text-success')
    expect(value.classes()).not.toContain('text-error')
  })

  it('computes the commercial gain/loss from the server-computed book value, not the stale asset column', async () => {
    // computed_book_value (500000) differs from the asset's stale book_value
    // (680000). The maker's preview must match what the backend records
    // (gain_loss from BookValueAsOf), i.e. 1500000 − 500000 = 1000000, not
    // the stale 1500000 − 680000 = 820000.
    depAssetScheduleMock.mockResolvedValue(scheduleResp({ computed_book_value: '500000' }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    const value = w.find('[data-testid="disposal-gainloss-value"]')
    expect(value.text()).toContain('1.000.000')
    expect(value.text()).not.toContain('820.000')
    // The "− Nilai Buku (komersial)" breakdown line shows the same computed value.
    expect(w.text()).toContain('500.000')
    expect(w.text()).not.toContain('680.000')
  })

  it('shows a masked "—" + note when book value is hidden for the caller\'s role', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_BV_MASKED)
    await setVmRef(w, 'proceedsRaw', '1000000')
    expect(w.find('[data-testid="disposal-gainloss-masked"]').text()).toBe('—')
    expect(w.text()).toContain('Nilai buku tersembunyi untuk peran Anda.')
  })

  it('shows "—" for the fiscal gain/loss row when no fiscal entries exist yet', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    expect(w.text()).toContain('Laba/rugi fiskal')
    expect(w.find('[data-testid="disposal-gainloss-fiscal-value"]').text()).toBe('—')
  })

  it('computes the real fiscal gain/loss as proceeds minus the fiscal book value', async () => {
    depAssetScheduleMock.mockResolvedValue(scheduleResp({
      entries: [{ basis: 'fiscal', period: '2026-05', opening: '750000', amount: '70000', closing: '680000', method: 'declining_balance' }]
    }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    expect(w.find('[data-testid="disposal-gainloss-fiscal-value"]').text()).toContain('820.000')
  })
})

describe('pages/disposals — Jenjang Persetujuan (chain) card', () => {
  it('falls back to the acquisition cost for the preview amount when no computed book value exists yet', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(previewMock).toHaveBeenCalledWith('asset_disposal', '6800000')
    expect(w.text()).toContain('berdasar nilai buku')
    expect(w.text()).toContain('6.800.000')
    expect(w.find('[data-testid="disposal-chain-card"]').text()).toContain('Dewi Lestari')
    expect(w.find('[data-testid="disposal-chain-steps"]').text()).toContain('Kanwil')
  })

  it('mirrors the server\'s basis switch: previews with computed_book_value once the depreciation schedule resolves it', async () => {
    depAssetScheduleMock.mockResolvedValue(scheduleResp({ computed_book_value: '4200000' }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(previewMock).toHaveBeenCalledWith('asset_disposal', '4200000')
    expect(w.text()).toContain('berdasar nilai buku')
    expect(w.text()).toContain('4.200.000')
  })

  it('falls back to "band belum dikonfigurasi" on a 422 from the preview endpoint', async () => {
    previewMock.mockRejectedValue(Object.assign(new Error('no band'), { statusCode: 422 }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-chain-not-configured"]').text()).toContain('belum dikonfigurasi')
  })

  it('skips the preview call and shows "nilai perolehan tersembunyi" when purchase_cost is masked', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_MASKED)
    expect(previewMock).not.toHaveBeenCalled()
    expect(w.find('[data-testid="disposal-chain-masked"]').text()).toContain('tersembunyi')
  })
})

describe('pages/disposals — Ajukan Penghapusan form', () => {
  it('keeps the right summary column sticky on large screens (mockup parity)', async () => {
    const w = await mountFormAndWait()
    const col = w.find('[data-testid="disposal-summary-column"]')
    expect(col.exists()).toBe(true)
    expect(col.classes()).toContain('lg:sticky')
    expect(col.classes()).toContain('lg:top-4')
    expect(col.classes()).toContain('self-start')
  })

  it('disables submit until asset + date + method are set, then enables it', async () => {
    const w = await mountFormAndWait()
    const submit = () => w.find('[data-testid="disposal-submit"]')
    expect(submit().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(submit().attributes('disabled')).toBeDefined()

    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')
    expect(submit().attributes('disabled')).toBeUndefined()
  })

  it('NumberInput: typing a formatted-looking value into the proceeds field keeps the submitted payload a raw digit-string', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')

    // The NumberInput shows grouped digits ("1.500.000") but its v-model
    // (proceedsRaw) and the submitted payload must stay the raw digit-string.
    const field = w.find('[data-testid="disposal-proceeds"]')
    await field.setValue('1500000')
    await flushPromises()
    expect((w.vm as unknown as { proceedsRaw: string }).proceedsRaw).toBe('1500000')
    expect((field.element as HTMLInputElement).value).toBe('1.500.000')

    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()
    expect(disposalsSubmitMock).toHaveBeenCalledWith(expect.objectContaining({ proceeds: '1500000' }))
  })

  it('submits the exact body — book_value_at_disposal is server-computed, never sent by the client', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')
    await w.find('[data-testid="disposal-bast-no"]').setValue('BAP/2026/07/200')

    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()

    expect(disposalsSubmitMock).toHaveBeenCalledWith({
      asset_id: 'a1',
      method: 'sale',
      disposal_date: '2026-07-04',
      proceeds: '1500000',
      bast_no: 'BAP/2026/07/200',
      reason: null
    })
    const body = disposalsSubmitMock.mock.calls[0]![0] as Record<string, unknown>
    expect(body).not.toHaveProperty('book_value_at_disposal')
  })

  it('submits with proceeds/bast_no null when omitted, still without book_value_at_disposal', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_MASKED)
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')

    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()

    expect(disposalsSubmitMock).toHaveBeenCalledWith(expect.objectContaining({ proceeds: null, bast_no: null }))
    const body = disposalsSubmitMock.mock.calls[0]![0] as Record<string, unknown>
    expect(body).not.toHaveProperty('book_value_at_disposal')
  })

  it('switches to the post-submit view showing the summary card on success', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')

    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Printer HP LaserJet Pro')
    expect(w.text()).toContain('Menunggu Approval')
  })

  it('keeps submit disabled and shows the no-permission note without disposal.manage', async () => {
    grantSession(['disposal.view'])
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')

    expect(w.find('[data-testid="disposal-submit"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="disposal-no-manage"]').text()).toContain('Anda tidak punya izin untuk mengajukan penghapusan.')
    expect(w.find('[data-testid="disposal-evidence-dropzone"]').attributes('disabled')).toBeDefined()

    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()
    expect(disposalsSubmitMock).not.toHaveBeenCalled()
  })
})

describe('pages/disposals — list-first navigation (Buat Pengajuan / Kembali)', () => {
  it('lands on the Riwayat view by default: history rows + create button, no ajukan form', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="disposal-create"]').exists()).toBe(true)
    // The old tab bar is gone; the form (and its Back button) is not mounted.
    expect(w.find('[data-testid="disposal-back"]').exists()).toBe(false)
    expect(w.find('[data-testid="disposal-submit"]').exists()).toBe(false)
    expect(w.findAll('[data-testid="disposal-history-row"]').length).toBeGreaterThan(0)
  })

  it('hides the create button while the Ajukan form is open and restores it on Back', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="disposal-create"]').exists()).toBe(true)

    await w.find('[data-testid="disposal-create"]').trigger('click')
    await w.vm.$nextTick()
    expect(w.find('[data-testid="disposal-create"]').exists()).toBe(false)
    expect(w.find('[data-testid="disposal-submit"]').exists()).toBe(true)

    await w.find('[data-testid="disposal-back"]').trigger('click')
    await w.vm.$nextTick()
    expect(w.find('[data-testid="disposal-create"]').exists()).toBe(true)
    expect(w.find('[data-testid="disposal-submit"]').exists()).toBe(false)
  })

  it('the Back button returns to Riwayat without submitting', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')
    // Form is now submit-ready — Back must still not submit.
    expect(w.find('[data-testid="disposal-submit"]').attributes('disabled')).toBeUndefined()

    await w.find('[data-testid="disposal-back"]').trigger('click')
    await w.vm.$nextTick()

    expect(disposalsSubmitMock).not.toHaveBeenCalled()
    expect(w.find('[data-testid="disposal-create"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="disposal-history-row"]').length).toBeGreaterThan(0)
  })

  it('reopening the Ajukan form clears the previously selected asset', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-valuation"]').exists()).toBe(true)

    await w.find('[data-testid="disposal-back"]').trigger('click')
    await w.vm.$nextTick()
    await w.find('[data-testid="disposal-create"]').trigger('click')
    await w.vm.$nextTick()

    // openAjukan() calls resetForm(), so the reopened form starts blank.
    expect((w.vm as unknown as { selectedAsset: unknown }).selectedAsset).toBeNull()
    expect(w.find('[data-testid="disposal-valuation"]').exists()).toBe(false)
    expect(w.find('[data-testid="disposal-submit"]').attributes('disabled')).toBeDefined()
  })

  it('Back from the post-submit timeline returns to Riwayat without re-submitting', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')
    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Menunggu Approval')
    expect(disposalsSubmitMock).toHaveBeenCalledTimes(1)

    // The Back button is still rendered above the post-submit timeline.
    await w.find('[data-testid="disposal-back"]').trigger('click')
    await w.vm.$nextTick()

    expect(disposalsSubmitMock).toHaveBeenCalledTimes(1)
    expect(w.find('[data-testid="disposal-create"]').exists()).toBe(true)
    expect(w.findAll('[data-testid="disposal-history-row"]').length).toBeGreaterThan(0)
  })
})

describe('pages/disposals — Timeline Approval Berlapis', () => {
  function stepFixture(over: Partial<ApprovalStep> = {}): ApprovalStep {
    return { step_order: 1, required_level: 'office', approver_id: null, approver_name: null, decision: 'pending', note: null, decided_at: null, ...over }
  }
  function detailFixture(over: Partial<ApprovalRequestDetail> = {}): ApprovalRequestDetail {
    return {
      ...reqRow({ id: 'req9' }),
      steps: [],
      ...over
    }
  }

  it('maps steps to done / current / queued from decision + current_step', async () => {
    approvalGetMock.mockResolvedValue(detailFixture({
      current_step: 2,
      requested_by_name: 'Dewi Lestari',
      created_at: '2026-07-02T09:00:00Z',
      steps: [
        stepFixture({ step_order: 1, required_level: 'office', decision: 'approved', approver_name: 'Kepala Unit A', decided_at: '2026-07-03T10:00:00Z' }),
        stepFixture({ step_order: 2, required_level: 'wilayah', decision: 'pending' }),
        stepFixture({ step_order: 3, required_level: 'pusat', decision: 'pending' })
      ]
    }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await setVmRef(w, 'proceedsRaw', '1500000')
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')
    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()

    const rows = w.findAll('[data-testid="disposal-timeline-row"]')
    // maker + 3 steps
    expect(rows).toHaveLength(4)
    expect(rows.map(r => r.attributes('data-status'))).toEqual(['done', 'done', 'current', 'queued'])
    expect(rows[1]!.text()).toContain('Kepala Unit A')
    expect(rows[2]!.text()).toContain('Menunggu tinjauan')
    expect(rows[3]!.text()).toContain('Menunggu tahap sebelumnya')
  })

  it('"Ajukan Penghapusan Lain" resets to the empty pre-submit form', async () => {
    approvalGetMock.mockResolvedValue(detailFixture({ current_step: 1, steps: [stepFixture()] }))
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    await w.find('[data-testid="disposal-date"]').setValue('2026-07-04')
    await w.find('[data-testid="disposal-submit"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Menunggu Approval')

    await w.find('[data-testid="disposal-reset"]').trigger('click')
    await w.vm.$nextTick()

    expect(w.find('[data-testid="disposal-submit"]').exists()).toBe(true)
    expect((w.vm as unknown as { selectedAsset: unknown }).selectedAsset).toBeNull()
  })
})

describe('pages/disposals — evidence dropzone', () => {
  it('uploads each selected file immediately to the asset\'s attachments and renders a chip', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)

    const file = new File(['x'], 'foto.jpg', { type: 'image/jpeg' })
    const vm = w.vm as unknown as { onEvidenceFileChange: (e: unknown) => Promise<void> }
    await vm.onEvidenceFileChange({ target: { files: [file], value: '' } })
    await flushPromises()

    expect(attachmentsUploadMock).toHaveBeenCalledWith('a1', file)
    expect(w.find('[data-testid="disposal-evidence-chip"]').text()).toContain('foto.jpg')
  })

  it('removes an uploaded chip via remove()', async () => {
    const w = await mountFormAndWait()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    const file = new File(['x'], 'foto.jpg', { type: 'image/jpeg' })
    const vm = w.vm as unknown as { onEvidenceFileChange: (e: unknown) => Promise<void> }
    await vm.onEvidenceFileChange({ target: { files: [file], value: '' } })
    await flushPromises()

    await w.find('[data-testid="disposal-evidence-remove"]').trigger('click')
    await flushPromises()

    expect(attachmentsRemoveMock).toHaveBeenCalledWith('a1', 'att1')
    expect(w.find('[data-testid="disposal-evidence-chip"]').exists()).toBe(false)
  })

  it('is disabled until an asset is selected', async () => {
    const w = await mountFormAndWait()
    expect(w.find('[data-testid="disposal-evidence-dropzone"]').attributes('disabled')).toBeDefined()
    await setVmRef(w, 'selectedAsset', ASSET_LABA)
    expect(w.find('[data-testid="disposal-evidence-dropzone"]').attributes('disabled')).toBeUndefined()
  })
})

describe('pages/disposals — Riwayat', () => {
  it('merges request-sourced (menunggu/ditolak) and disposal-sourced (selesai) rows, method "—" on request rows', async () => {
    approvalListMock.mockResolvedValue(page([
      reqRow({ id: 'req-pending', status: 'pending' }),
      reqRow({ id: 'req-rejected', status: 'rejected' })
    ]))
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="disposal-history-row"]')
    expect(rows).toHaveLength(3)
    const disposalRow = rows.find(r => r.text().includes('Laptop Asus X441 (Lama)'))!
    expect(disposalRow.text()).toContain('Dijual')
    const requestRows = rows.filter(r => !r.text().includes('Laptop Asus X441 (Lama)'))
    expect(requestRows).toHaveLength(2)
    for (const r of requestRows) expect(r.text()).toContain('—')
  })

  it('excludes "Disetujui" from the status filter options (deviation h) and resolves every label', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const vm = w.vm as unknown as { statusFilterItems: Array<{ value: string, label: string }> }
    const values = vm.statusFilterItems.map(i => i.value)
    expect(values).toEqual(['all', 'menunggu', 'ditolak', 'dibatalkan', 'selesai'])
    // Labels must be the resolved i18n strings — a missing locale key would
    // surface here as the raw key path (e.g. "disposal.statusFilter.dibatalkan").
    const labels = vm.statusFilterItems.map(i => i.label)
    expect(labels).toEqual(['Semua Status', 'Menunggu Approval', 'Ditolak', 'Dibatalkan', 'Selesai'])
  })

  it('filters rows by status', async () => {
    approvalListMock.mockResolvedValue(page([reqRow({ id: 'req-pending', status: 'pending' })]))
    const w = await mountAndWait()
    await clickTab(w, 'history')
    await setVmRef(w, 'historyStatus', 'selesai')
    const rows = w.findAll('[data-testid="disposal-history-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Laptop Asus X441 (Lama)')
  })

  it('filters rows by search text', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    await setVmRef(w, 'historyQuery', 'nonexistent-xyz')
    expect(w.text()).toContain('Belum ada riwayat')
  })

  it('shows the empty state when there is no history', async () => {
    disposalsListMock.mockResolvedValue(page([]))
    approvalListMock.mockResolvedValue(page([]))
    const w = await mountAndWait()
    await clickTab(w, 'history')
    expect(w.text()).toContain('Belum ada riwayat')
  })

  it('shows the load-error state with retry when the history call fails', async () => {
    disposalsListMock.mockRejectedValue(new Error('boom'))
    const w = await mountAndWait()
    await clickTab(w, 'history')
    expect(w.text()).toContain('Gagal memuat data.')
  })

  it('shows the footer total count', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    expect(w.text()).toContain('Total 2 pengajuan')
  })

  it('shows a "Lampirkan BAST" kebab action only on the Selesai row and sends multipart via attachDocument', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="disposal-history-row"]')
    const selesaiRow = rows.find(r => r.text().includes('Laptop Asus X441 (Lama)'))!
    const otherRows = rows.filter(r => r !== selesaiRow)

    expect(selesaiRow.find('button[aria-haspopup="menu"]').exists()).toBe(true)
    for (const r of otherRows) {
      expect(r.find('button[aria-haspopup="menu"]').exists()).toBe(false)
    }

    await selesaiRow.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))
    menuItemByText('Lampirkan BAST Penghapusan')!.click()
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('Lampirkan BAST Penghapusan')

    const file = new File(['x'], 'bast.pdf', { type: 'application/pdf' })
    await setVmRef(w, 'attachFile', file)
    await setVmRef(w, 'attachBastNo', 'BAP/2026/07/300')

    bodyButton('disposal-attach-confirm').click()
    await flushPromises()

    expect(disposalsAttachDocumentMock).toHaveBeenCalledWith('d1', expect.objectContaining({
      bast_no: 'BAP/2026/07/300',
      file
    }))
  })

  it('hides the "Lampirkan BAST" kebab on every row without disposal.manage', async () => {
    grantSession(['disposal.view'])
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="disposal-history-row"]')
    expect(rows.length).toBeGreaterThan(0)
    for (const r of rows) {
      expect(r.find('button[aria-haspopup="menu"]').exists()).toBe(false)
    }
  })

  it('right-clicking the Selesai row surfaces "Lampirkan BAST" in the context menu', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="disposal-history-row"]')
    const selesaiRow = rows.find(r => r.text().includes('Laptop Asus X441 (Lama)'))!

    selesaiRow.element.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(menuItemByText('Lampirkan BAST Penghapusan')).toBeTruthy()

    menuItemByText('Lampirkan BAST Penghapusan')!.click()
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('Lampirkan BAST Penghapusan')
  })

  it('right-clicking a non-row area after right-clicking the Selesai row shows no stale context menu', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'history')
    const rows = w.findAll('[data-testid="disposal-history-row"]')
    const selesaiRow = rows.find(r => r.text().includes('Laptop Asus X441 (Lama)'))!

    selesaiRow.element.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(menuItemByText('Lampirkan BAST Penghapusan')).toBeTruthy()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    const thead = w.find('thead tr').element
    thead.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(document.querySelectorAll('[role="menuitem"]').length).toBe(0)
  })
})
