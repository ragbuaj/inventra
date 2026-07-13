// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Category, Office, Paginated } from '~/types'
import type { DepreciationPeriod, JournalResponse, ScheduleResponse, ScheduleRow } from '~/composables/api/useDepreciation'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

function page<T>(data: T[]): Paginated<T> {
  return { data, total: data.length, limit: 100, offset: 0 }
}

const PERIODS: DepreciationPeriod[] = [
  { period: '2026-05', status: 'closed', asset_count: 6, total_amount: '11500000', skipped_count: 0 },
  { period: '2026-06', status: 'computed', asset_count: 6, total_amount: '12500000', skipped_count: 0 },
  { period: '2026-07', status: 'open', asset_count: 0, total_amount: '0', skipped_count: 0 }
]

const CATEGORIES: Category[] = [
  {
    id: 'c1', name: 'Elektronik', code: 'ELK', parent_id: null,
    default_depreciation_method: 'straight_line', default_useful_life_months: 48, default_salvage_rate: '0',
    asset_class: 'tangible', default_fiscal_group: null, default_fiscal_life_months: null,
    gl_account_code: null, capitalization_threshold: null, is_active: true,
    created_at: '2026-01-01T00:00:00Z', updated_at: null
  }
]

const OFFICES: Office[] = [
  { id: 'o1', parent_id: null, office_type_id: 'ot1', province_id: null, city_id: null, name: 'Kantor Cabang Jakarta Selatan', code: 'JKS', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null }
]

function scheduleRow(over: Partial<ScheduleRow> = {}): ScheduleRow {
  return {
    asset_id: 'a1', asset_name: 'Laptop Dell Latitude 5440', asset_tag: 'JKT01-ELK-2026-00001',
    category_name: 'Elektronik', office_name: 'Kantor Cabang Jakarta Selatan',
    method: 'straight_line', life_months: 48, opening: '18500000', amount: '385417',
    accumulated: '2697917', closing: '15802083', impaired: false, fully_depreciated: false,
    ...over
  }
}

const SCHEDULE_ROWS: ScheduleRow[] = [
  scheduleRow(),
  scheduleRow({
    asset_id: 'a2', asset_name: 'Genset Cummins C22 D5', asset_tag: 'JKT01-ELK-2025-00028',
    method: 'declining_balance', life_months: 96, opening: '67437500', amount: '1687000',
    accumulated: '10562500', closing: '67437500', impaired: true
  }),
  scheduleRow({
    asset_id: 'a3', asset_name: 'Kursi Ergonomis (20 unit)', asset_tag: 'JKT01-MBL-2024-00033',
    method: 'straight_line', life_months: 48, opening: '0', amount: '999',
    accumulated: '24000000', closing: '0', impaired: false, fully_depreciated: true
  })
]

function scheduleResponse(rows: ScheduleRow[] = SCHEDULE_ROWS): ScheduleResponse {
  return {
    kpi: { total_cost: '120500000', total_accumulated: '37260417', total_book_value: '83239583', period_expense: '1198917' },
    rows,
    totals: { opening: '85937500', amount: '1198917', accumulated: '37260417', closing: '83239583' }
  }
}

// A *filtered* schedule response: fewer rows AND a smaller KPI block. The page
// must never surface this KPI block on the tiles (those come from the unfiltered
// call) — so if the tiles ever read from a filtered call, this distinct value
// would leak and the KPI-invariance test would fail.
function filteredScheduleResponse(): ScheduleResponse {
  return {
    kpi: { total_cost: '67437500', total_accumulated: '10562500', total_book_value: '67437500', period_expense: '1687000' },
    rows: [SCHEDULE_ROWS[1]!],
    totals: { opening: '67437500', amount: '1687000', accumulated: '10562500', closing: '67437500' }
  }
}

function isFilteredScheduleCall(q: { search?: string, category_id?: string, office_id?: string }): boolean {
  return Boolean(q.search || q.category_id || q.office_id)
}

function journalResponse(balanced = true): JournalResponse {
  return {
    rows: [
      { account_code: '8.1.01.001', account_name: 'Beban Penyusutan — Elektronik', debit: '1198917', credit: '0.00' },
      { account_code: '1.2.9.001', account_name: 'Akumulasi Penyusutan', debit: '0.00', credit: '1198917' }
    ],
    total_debit: '1198917',
    total_credit: '1198917',
    balanced
  }
}

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const periodsMock = vi.fn()
const computeMock = vi.fn()
const closeMock = vi.fn()
const scheduleMock = vi.fn()
const journalMock = vi.fn()
const exportJournalMock = vi.fn()
const recordImpairmentMock = vi.fn()

vi.mock('~/composables/api/useDepreciation', () => ({
  useDepreciation: () => ({
    periods: periodsMock,
    compute: computeMock,
    close: closeMock,
    schedule: scheduleMock,
    journal: journalMock,
    exportJournal: exportJournalMock,
    assetSchedule: vi.fn(),
    recordImpairment: recordImpairmentMock
  })
}))

const categoriesTreeMock = vi.fn()
vi.mock('~/composables/api/useCategories', () => ({
  useCategories: () => ({ list: vi.fn(), get: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn(), tree: categoriesTreeMock })
}))

const officesListMock = vi.fn()
const officesGetMock = vi.fn()
vi.mock('~/composables/api/useOffices', () => ({
  useOffices: () => ({ list: officesListMock, get: officesGetMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

// eslint-disable-next-line import/first
import DepreciationPage from '~/pages/depreciation.vue'

enableAutoUnmount(afterEach)

function grantSession(permissions: string[] = ['depreciation.view', 'depreciation.manage']) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Dewi Lestari', email: 'dewi@test.com', role_id: 'r1', role_name: 'Kepala Unit', office_id: 'o1' },
    permissions
  )
}

async function mountAndWait() {
  const wrapper = await mountSuspended(DepreciationPage, { route: '/depreciation' })
  await flushPromises()
  await new Promise(r => setTimeout(r, 50))
  await flushPromises()
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

function bodyEl(testid: string): HTMLElement {
  const el = document.body.querySelector(`[data-testid="${testid}"]`)
  expect(el, `expected [data-testid="${testid}"] in document.body`).toBeTruthy()
  return el as HTMLElement
}

beforeEach(() => {
  vi.clearAllMocks()
  periodsMock.mockResolvedValue([...PERIODS])
  categoriesTreeMock.mockResolvedValue(CATEGORIES)
  officesListMock.mockResolvedValue(page(OFFICES))
  officesGetMock.mockImplementation(async (id: string) => {
    const found = OFFICES.find(o => o.id === id)
    if (!found) throw Object.assign(new Error('not found'), { statusCode: 404 })
    return found
  })
  scheduleMock.mockImplementation(async (q: { search?: string, category_id?: string, office_id?: string }) =>
    isFilteredScheduleCall(q) ? filteredScheduleResponse() : scheduleResponse())
  journalMock.mockResolvedValue(journalResponse())
  computeMock.mockImplementation(async (p: string) => ({ period: p, status: 'computed', asset_count: 6, total_amount: '1198917', skipped_count: 0 }))
  closeMock.mockImplementation(async (p: string) => ({ period: p, status: 'closed' }))
  recordImpairmentMock.mockResolvedValue({ book_value: '40000000', impairment_loss: '27437500' })
  grantSession()
})

// ---------------------------------------------------------------------------

describe('pages/depreciation — mount + KPI', () => {
  it('loads periods and defaults the selected period to the latest one', async () => {
    await mountAndWait()
    expect(periodsMock).toHaveBeenCalled()
    expect(scheduleMock).toHaveBeenCalledWith(expect.objectContaining({ period: '2026-07', basis: 'commercial' }))
  })

  it('renders the four KPI tiles with values from the schedule response', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-kpi-acquisition"]').text()).toContain('120.500.000')
    expect(w.find('[data-testid="depr-kpi-accumulated"]').text()).toContain('37.260.417')
    expect(w.find('[data-testid="depr-kpi-book-value"]').text()).toContain('83.239.583')
    expect(w.find('[data-testid="depr-kpi-period-expense"]').text()).toContain('1.198.917')
  })

  it('fetches KPIs from an UNFILTERED schedule() call (period + basis only)', async () => {
    await mountAndWait()
    // The KPI fetch carries no search/category/office keys.
    expect(scheduleMock).toHaveBeenCalledWith({ period: '2026-07', basis: 'commercial' })
  })

  it('keeps the KPI tiles unchanged when a table filter is applied (only the rows shrink)', async () => {
    const w = await mountAndWait()
    expect(w.findAll('[data-testid="depr-schedule-row"]').length).toBe(3)
    const kpiBefore = w.find('[data-testid="depr-kpi-acquisition"]').text()
    expect(kpiBefore).toContain('120.500.000')

    // Apply a category filter — the table now uses the filtered call (1 row,
    // a different KPI block), but the tiles must stay on the unfiltered totals.
    await setVmRef(w, 'categoryId', 'c1')
    expect(w.findAll('[data-testid="depr-schedule-row"]').length).toBe(1)
    const kpiAfter = w.find('[data-testid="depr-kpi-acquisition"]').text()
    expect(kpiAfter).toContain('120.500.000')
    expect(kpiAfter).not.toContain('67.437.500') // the filtered call's total_cost
    expect(kpiAfter).toBe(kpiBefore)
  })
})

describe('pages/depreciation — basis toggle', () => {
  it('refetches schedule and journal with basis: fiscal when the Fiskal chip is clicked', async () => {
    const w = await mountAndWait()
    scheduleMock.mockClear()
    journalMock.mockClear()
    await w.find('[data-testid="depr-basis-fiscal"]').trigger('click')
    await flushPromises()
    expect(scheduleMock).toHaveBeenCalledWith(expect.objectContaining({ basis: 'fiscal' }))
    expect(journalMock).toHaveBeenCalledWith(expect.any(String), 'fiscal')
  })
})

describe('pages/depreciation — period states', () => {
  it('shows Hitung Periode when the selected period is open', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-compute"]').exists()).toBe(true)
    expect(w.find('[data-testid="depr-close"]').exists()).toBe(false)
  })

  it('shows Tutup Periode + the computed note when the selected period is computed', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'period', '2026-06')
    expect(w.find('[data-testid="depr-compute"]').exists()).toBe(false)
    expect(w.find('[data-testid="depr-close"]').exists()).toBe(true)
    expect(w.text()).toContain('Sudah dihitung')
  })

  it('shows the closed badge and disables the period select when the selected period is closed', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'period', '2026-05')
    expect(w.find('[data-testid="depr-compute"]').exists()).toBe(false)
    expect(w.find('[data-testid="depr-close"]').exists()).toBe(false)
    expect(w.text()).toContain('Periode Ditutup')
    expect(w.find('[data-testid="depr-period-select"]').attributes('disabled')).toBeDefined()
  })
})

describe('pages/depreciation — compute/close', () => {
  it('calling Hitung Periode invokes compute() and refreshes schedule + journal', async () => {
    const w = await mountAndWait()
    scheduleMock.mockClear()
    journalMock.mockClear()
    await w.find('[data-testid="depr-compute"]').trigger('click')
    await flushPromises()
    expect(computeMock).toHaveBeenCalledWith('2026-07')
    expect(scheduleMock).toHaveBeenCalled()
    expect(journalMock).toHaveBeenCalled()
    // The period transitions to "computed" — Tutup now shows instead of Hitung.
    expect(w.find('[data-testid="depr-close"]').exists()).toBe(true)
  })

  it('calling Tutup Periode invokes close(), refreshes schedule + journal, and shows the closed state', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'period', '2026-06')
    scheduleMock.mockClear()
    journalMock.mockClear()
    await w.find('[data-testid="depr-close"]').trigger('click')
    await flushPromises()
    expect(closeMock).toHaveBeenCalledWith('2026-06')
    // Close refreshes symmetrically with compute.
    expect(scheduleMock).toHaveBeenCalled()
    expect(journalMock).toHaveBeenCalled()
    expect(w.text()).toContain('Periode Ditutup')
  })
})

describe('pages/depreciation — reminder banner', () => {
  it('shows the reminder when the latest known period is still open', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-reminder"]').exists()).toBe(true)
  })

  it('hides the reminder when the latest known period is already computed/closed', async () => {
    periodsMock.mockResolvedValue([
      { period: '2026-05', status: 'closed', asset_count: 6, total_amount: '11500000', skipped_count: 0 },
      { period: '2026-06', status: 'closed', asset_count: 6, total_amount: '12500000', skipped_count: 0 }
    ])
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-reminder"]').exists()).toBe(false)
  })
})

describe('pages/depreciation — manage gate', () => {
  it('disables Hitung/Tutup and shows the no-manage note without depreciation.manage', async () => {
    grantSession(['depreciation.view'])
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-compute"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="depr-no-manage"]').exists()).toBe(true)
  })

  it('keeps Hitung enabled and hides the no-manage note with depreciation.manage', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-compute"]').attributes('disabled')).toBeUndefined()
    expect(w.find('[data-testid="depr-no-manage"]').exists()).toBe(false)
  })

  it('disables the Tutup Periode button on a computed period without depreciation.manage', async () => {
    grantSession(['depreciation.view'])
    const w = await mountAndWait()
    await setVmRef(w, 'period', '2026-06')
    expect(w.find('[data-testid="depr-close"]').exists()).toBe(true)
    expect(w.find('[data-testid="depr-close"]').attributes('disabled')).toBeDefined()
  })

  it('disables the impairment row action without depreciation.manage (commercial basis)', async () => {
    grantSession(['depreciation.view'])
    const w = await mountAndWait()
    const btn = w.findAll('[data-testid="depr-impair"]')[0]!
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.attributes('title')).toBe('Anda tidak punya izin untuk mengelola depresiasi.')
  })
})

describe('pages/depreciation — Jadwal per Aset', () => {
  it('renders schedule rows including the impaired icon', async () => {
    const w = await mountAndWait()
    const rows = w.findAll('[data-testid="depr-schedule-row"]')
    expect(rows.length).toBe(3)
    expect(rows[1]!.find('[title="Aset telah di-impair"]').exists()).toBe(true)
    expect(rows[0]!.find('[title="Aset telah di-impair"]').exists()).toBe(false)
  })

  it('shows a zero period-expense for a fully-depreciated row regardless of the backend value (deviation a)', async () => {
    const w = await mountAndWait()
    const rows = w.findAll('[data-testid="depr-schedule-row"]')
    // Row 3 (a3) is fully_depreciated with a nonzero backend "amount" of 999.
    expect(rows[2]!.text()).not.toContain('999')
    // formatRupiah uses Intl currency formatting, which inserts a NBSP (U+00A0)
    // between "Rp" and the digits — not a plain space.
    expect(rows[2]!.text()).toContain('Rp 0')
  })

  it('shows the empty state when the schedule has no rows', async () => {
    scheduleMock.mockResolvedValue(scheduleResponse([]))
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-schedule-empty"]').exists()).toBe(true)
  })

  it('calls schedule() with the filter params when search/category/office change', async () => {
    const w = await mountAndWait()
    scheduleMock.mockClear()
    await setVmRef(w, 'debouncedSearch', 'Genset')
    await setVmRef(w, 'categoryId', 'c1')
    await setVmRef(w, 'officeId', 'o1')
    expect(scheduleMock).toHaveBeenLastCalledWith(expect.objectContaining({
      period: '2026-07', basis: 'commercial', search: 'Genset', category_id: 'c1', office_id: 'o1'
    }))
  })
})

describe('pages/depreciation — office filter picker', () => {
  it('renders the office filter as an async search picker, not a USelect', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-filter-office-picker-input"]').exists()).toBe(true)
  })

  it('typing in the office filter picker drives useOffices().list with search+limit=20', async () => {
    const w = await mountAndWait()
    vi.useFakeTimers()
    await w.find('[data-testid="depr-filter-office-picker-input"]').setValue('Jakarta')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(officesListMock).toHaveBeenCalledWith(expect.objectContaining({ search: 'Jakarta', limit: 20 }))
  })

  it('clearing the office filter picker resets officeId to null and drops office_id from schedule()', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'officeId', 'o1')
    scheduleMock.mockClear()

    const clearBtn = w.find('[data-testid="depr-filter-office-picker-clear"]')
    expect(clearBtn.exists()).toBe(true)
    await clearBtn.trigger('click')
    await flushPromises()
    await w.vm.$nextTick()

    expect((w.vm as unknown as { officeId: string | null }).officeId).toBeNull()
    expect(scheduleMock).toHaveBeenLastCalledWith(expect.objectContaining({ office_id: undefined }))
  })
})

describe('pages/depreciation — impairment modal', () => {
  it('is disabled (with a fiscal tooltip) when the basis is fiscal', async () => {
    const w = await mountAndWait()
    await w.find('[data-testid="depr-basis-fiscal"]').trigger('click')
    await flushPromises()
    const btn = w.findAll('[data-testid="depr-impair"]')[0]!
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.attributes('title')).toBe('Fiskal tidak mengakui impairment — PSAK 48 hanya basis komersial')
  })

  it('computes the loss preview from the row closing value and the recoverable input', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'impairTarget', SCHEDULE_ROWS[0])
    await setVmRef(w, 'impairOpen', true)
    await setVmRef(w, 'impairRecoverRaw', '10000000')
    // closing (15,802,083) - recoverable (10,000,000) = 5,802,083
    expect(bodyEl('depr-impair-loss').textContent).toContain('5.802.083')
  })

  it('NumberInput: typing into the recoverable field keeps recordImpairment fed the raw digit-string', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'impairTarget', SCHEDULE_ROWS[0])
    await setVmRef(w, 'impairOpen', true)

    // The recoverable field is rendered inside a UModal (teleported to
    // document.body), so it's queried/driven via the raw DOM, not w.find().
    const input = bodyEl('depr-impair-recoverable') as HTMLInputElement
    input.value = '10000000'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await w.vm.$nextTick()
    await flushPromises()
    // NumberInput groups the display ("10.000.000") but the underlying
    // v-model (impairRecoverRaw) stays the raw digit-string.
    expect((w.vm as unknown as { impairRecoverRaw: string }).impairRecoverRaw).toBe('10000000')
    expect(input.value).toBe('10.000.000')

    bodyEl('depr-impair-save').click()
    await flushPromises()
    expect(recordImpairmentMock).toHaveBeenCalledWith('a1', '10000000', '')
  })

  it('saves with the exact recordImpairment args and refreshes the schedule', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'impairTarget', SCHEDULE_ROWS[0])
    await setVmRef(w, 'impairOpen', true)
    await setVmRef(w, 'impairRecoverRaw', '10000000')
    await setVmRef(w, 'impairReason', 'Kerusakan permanen akibat banjir')
    scheduleMock.mockClear()
    bodyEl('depr-impair-save').click()
    await flushPromises()
    expect(recordImpairmentMock).toHaveBeenCalledWith('a1', '10000000', 'Kerusakan permanen akibat banjir')
    expect(scheduleMock).toHaveBeenCalled()
    expect((w.vm as unknown as { impairOpen: boolean }).impairOpen).toBe(false)
  })
})

describe('pages/depreciation — Rekap Siap-Jurnal', () => {
  it('renders journal rows and the balanced banner', async () => {
    const w = await mountAndWait()
    await w.find('[data-testid="depr-tab-journal"]').trigger('click')
    await flushPromises()
    const rows = w.findAll('[data-testid="depr-journal-row"]')
    expect(rows.length).toBe(2)
    expect(w.text()).toContain('Jurnal seimbang — debit = kredit.')
  })

  it('hides the balanced banner when the journal is not balanced', async () => {
    journalMock.mockResolvedValue(journalResponse(false))
    const w = await mountAndWait()
    await w.find('[data-testid="depr-tab-journal"]').trigger('click')
    await flushPromises()
    expect(w.text()).not.toContain('Jurnal seimbang — debit = kredit.')
  })

  it('ignores a stale journal response from a superseded basis (seq guard)', async () => {
    const deferred: Record<string, (v: JournalResponse) => void> = {}
    journalMock.mockImplementation((_period: string, b: string) =>
      new Promise<JournalResponse>((resolve) => { deferred[b] = resolve }))

    const w = await mountAndWait() // issues the commercial journal (pending)
    await w.find('[data-testid="depr-tab-journal"]').trigger('click')
    await w.find('[data-testid="depr-basis-fiscal"]').trigger('click') // issues the fiscal journal (pending)
    await flushPromises()

    const fiscalResp: JournalResponse = {
      rows: [{ account_code: 'F', account_name: 'FISCAL JOURNAL ROW', debit: '1', credit: '0' }],
      total_debit: '1', total_credit: '1', balanced: true
    }
    const commercialResp: JournalResponse = {
      rows: [{ account_code: 'C', account_name: 'COMMERCIAL JOURNAL ROW', debit: '1', credit: '0' }],
      total_debit: '1', total_credit: '1', balanced: true
    }

    // Resolve the latest (fiscal) request first, then the stale (commercial) one.
    deferred.fiscal!(fiscalResp)
    await flushPromises()
    deferred.commercial!(commercialResp)
    await flushPromises()

    expect(w.text()).toContain('FISCAL JOURNAL ROW')
    expect(w.text()).not.toContain('COMMERCIAL JOURNAL ROW')
  })

  it('exports the journal as a blob download via a temporary anchor', async () => {
    const blob = new Blob(['pdf-bytes'], { type: 'application/pdf' })
    exportJournalMock.mockResolvedValue(blob)
    const createObjectURLSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:mock-url')
    const revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const w = await mountAndWait()
    await w.find('[data-testid="depr-tab-journal"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="depr-export-pdf"]').trigger('click')
    await flushPromises()

    expect(exportJournalMock).toHaveBeenCalledWith('2026-07', 'commercial', 'pdf')
    expect(createObjectURLSpy).toHaveBeenCalledWith(blob)
    expect(clickSpy).toHaveBeenCalled()
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:mock-url')

    createObjectURLSpy.mockRestore()
    revokeObjectURLSpy.mockRestore()
    clickSpy.mockRestore()
  })
})

describe('pages/depreciation — loading/error states', () => {
  it('shows a loading skeleton for the run panel while periods() is pending', async () => {
    periodsMock.mockImplementation(() => new Promise(() => {}))
    const wrapper = await mountSuspended(DepreciationPage, { route: '/depreciation' })
    await flushPromises()
    expect(wrapper.find('[data-testid="depr-period-select"]').exists()).toBe(false)
  })

  it('shows a retry banner when periods() fails, and retry reloads it', async () => {
    periodsMock.mockRejectedValueOnce(new Error('network'))
    const w = await mountAndWait()
    expect(w.text()).toContain('Gagal memuat data')
    periodsMock.mockResolvedValueOnce([...PERIODS])
    await w.find('[data-testid="depr-periods-retry"]').trigger('click')
    await flushPromises()
    expect(periodsMock).toHaveBeenCalledTimes(2)
    expect(w.find('[data-testid="depr-period-select"]').exists()).toBe(true)
  })

  it('shows a retry banner when schedule() fails', async () => {
    scheduleMock.mockRejectedValue(new Error('network'))
    const w = await mountAndWait()
    expect(w.find('[data-testid="depr-schedule-row"]').exists()).toBe(false)
    expect(w.text()).toContain('Gagal memuat data')
  })
})
