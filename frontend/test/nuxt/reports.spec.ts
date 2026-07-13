// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { parseDate } from '@internationalized/date'
import type { ReportResult } from '~/composables/api/useReports'
import type { Category, Office } from '~/types'
import { useAuthStore } from '~/stores/auth'
import PeriodFilter from '~/components/PeriodFilter.vue'
import ReportsPage from '~/pages/reports.vue'

enableAutoUnmount(afterEach)

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------
const { runMock, exportMock, opnameBaMock, officesListMock, officesGetMock, categoriesTreeMock } = vi.hoisted(() => ({
  runMock: vi.fn(),
  exportMock: vi.fn(),
  opnameBaMock: vi.fn(),
  officesListMock: vi.fn(),
  officesGetMock: vi.fn(),
  categoriesTreeMock: vi.fn()
}))

vi.mock('~/composables/api/useReports', () => ({
  useReports: () => ({ run: runMock, exportReport: exportMock, opnameBa: opnameBaMock })
}))
vi.mock('~/composables/api/useOffices', () => ({
  useOffices: () => ({ list: officesListMock, get: officesGetMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))
vi.mock('~/composables/api/useCategories', () => ({
  useCategories: () => ({ tree: categoriesTreeMock, list: vi.fn(), get: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------
function office(id: string, name: string): Office {
  return { id, name, code: id.toUpperCase() } as Office
}
function category(id: string, name: string): Category {
  return { id, name, code: id.toUpperCase() } as Category
}

const assetsResult: ReportResult = {
  type: 'assets',
  kpis: [
    { key: 'total_assets', value: '2' },
    { key: 'total_acquisition', value: '3820000000' },
    { key: 'total_book', value: '2140000000' }
  ],
  chart: [{ label: 'Elektronik', value: '97600000' }, { label: 'Kendaraan', value: '220000000' }],
  rows: [
    { asset_tag: 'JKT01-ELK-0001', name: 'Laptop Dell Latitude 5440', category_name: 'Elektronik', status: 'available', purchase_cost: '18500000', accum_deprec: '2300000', book_value: '16200000' },
    { asset_tag: 'JKT01-KEN-0007', name: 'Toyota Avanza 1.5 G', category_name: 'Kendaraan', status: 'under_maintenance', purchase_cost: '235000000', accum_deprec: '37000000', book_value: '198000000' }
  ],
  totals: { purchase_cost: '253500000', accum_deprec: '39300000', book_value: '214200000' },
  row_count: 2,
  truncated: false
}

const disposalsResult: ReportResult = {
  type: 'disposals',
  kpis: [
    { key: 'total_disposals', value: '2' },
    { key: 'total_proceeds', value: '9000000' },
    { key: 'total_gain_loss', value: '-1500000' }
  ],
  chart: [{ label: 'sale', value: '9000000' }],
  rows: [
    { asset_name: 'Printer Lama', asset_tag: 'AST-9', method: 'sale', disposal_date: '2026-06-01', book_value: '5000000', proceeds: '7000000', gain_loss: '2000000' },
    { asset_name: 'AC Rusak', asset_tag: 'AST-8', method: 'write_off', disposal_date: '2026-06-02', book_value: '3500000', proceeds: '0', gain_loss: '-3500000' }
  ],
  totals: { book_value: '8500000', proceeds: '7000000', gain_loss: '-1500000' },
  row_count: 2,
  truncated: false
}

const transfersResult: ReportResult = {
  type: 'transfers',
  kpis: [
    { key: 'total', value: '2' },
    { key: 'in_transit', value: '1' },
    { key: 'received', value: '1' }
  ],
  chart: [{ label: 'KC Jakarta', value: '2' }],
  rows: [
    { asset_name: 'Laptop A', asset_tag: 'AST-1', from_office: 'KC Jakarta', to_office: 'KC Bandung', status: 'in_transit', shipped_date: '2026-06-15', received_date: '', bast_no: '' },
    { asset_name: 'Laptop B', asset_tag: 'AST-2', from_office: 'KC Jakarta', to_office: 'KC Bandung', status: 'received', shipped_date: '2026-06-10', received_date: '2026-06-12', bast_no: 'BAST-01' },
    { asset_name: 'Laptop C', asset_tag: 'AST-3', from_office: 'KC Jakarta', to_office: 'KC Bandung', status: 'pending', shipped_date: '', received_date: '', bast_no: '' }
  ],
  totals: {},
  row_count: 3,
  truncated: false
}

const opnameResult: ReportResult = {
  type: 'opname',
  kpis: [
    { key: 'sessions', value: '1' },
    { key: 'total_items', value: '50' },
    { key: 'total_variance', value: '2' }
  ],
  chart: [{ label: 'Opname Juni', value: '2' }],
  rows: [
    { session_id: 'sess-1', name: 'Opname Juni', office_name: 'KC Jakarta', period: '2026-06', status: 'closed', total_items: 50, variance: 2 }
  ],
  totals: {},
  row_count: 1,
  truncated: false
}

function grant(perms: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    perms
  )
}

const OFFICES = [office('o1', 'Kantor A'), office('o2', 'Kantor B')]

beforeEach(() => {
  runMock.mockReset().mockResolvedValue(assetsResult)
  exportMock.mockReset().mockResolvedValue(new Blob(['x']))
  opnameBaMock.mockReset().mockResolvedValue(new Blob(['x']))
  officesListMock.mockReset().mockResolvedValue({ data: OFFICES, total: OFFICES.length, limit: 100, offset: 0 })
  officesGetMock.mockReset().mockImplementation(async (id: string) => {
    const found = OFFICES.find(o => o.id === id)
    if (!found) throw Object.assign(new Error('not found'), { statusCode: 404 })
    return found
  })
  categoriesTreeMock.mockReset().mockResolvedValue([category('c1', 'Elektronik'), category('c2', 'Kendaraan')])
  ;(URL as unknown as { createObjectURL: unknown }).createObjectURL = vi.fn(() => 'blob:mock')
  ;(URL as unknown as { revokeObjectURL: unknown }).revokeObjectURL = vi.fn()
  grant(['*'])
})

type Vm = {
  apply: () => Promise<void>
  doExport: (f: 'pdf' | 'xlsx') => Promise<void>
  doExportGl: (f: 'pdf' | 'xlsx') => Promise<void>
  doOpnameBa: (id: string, f: 'pdf' | 'xlsx') => Promise<void>
  resetFilters: () => void
  selectReport: (k: string) => void
  report: string
  officeId: string
  categoryId: string
  status: string
  basis: string
}

async function mountPage() {
  const wrapper = await mountSuspended(ReportsPage, { route: '/reports' })
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  return wrapper
}

async function applyAndSettle(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  await (wrapper.vm as unknown as Vm).apply()
  await flushPromises()
  await wrapper.vm.$nextTick()
  // A second flush+tick lets the office resolve-cache's on-demand
  // resolveFn(id) promise (kicked off while rendering officeLabel for the
  // first time) settle and re-render with the resolved name.
  await flushPromises()
  await wrapper.vm.$nextTick()
}

// ---------------------------------------------------------------------------
// 1 — cards
// ---------------------------------------------------------------------------
describe('Reports page — cards', () => {
  it('renders all seven report cards with active styling on the default (assets)', async () => {
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('Daftar Aset & Nilai Buku')
    expect(text).toContain('Depresiasi per Periode')
    expect(text).toContain('Utilisasi / Penugasan')
    expect(text).toContain('Biaya Maintenance')
    expect(text).toContain('Mutasi Aset')
    expect(text).toContain('Penghapusan Aset')
    expect(text).toContain('Stock Opname')

    const assetsCard = wrapper.find('[data-testid="reports-card-assets"]')
    expect(assetsCard.classes()).toContain('border-primary')
    const opnameCard = wrapper.find('[data-testid="reports-card-opname"]')
    expect(opnameCard.classes()).not.toContain('border-primary')
  })

  it('moves the active styling when another card is selected', async () => {
    const wrapper = await mountPage()
    await wrapper.find('[data-testid="reports-card-disposals"]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="reports-card-disposals"]').classes()).toContain('border-primary')
    expect(wrapper.find('[data-testid="reports-card-assets"]').classes()).not.toContain('border-primary')
  })
})

// ---------------------------------------------------------------------------
// 2 — conditional filters
// ---------------------------------------------------------------------------
describe('Reports page — conditional filters', () => {
  it('shows the status filter only for assets', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="reports-status-filter"]').exists()).toBe(true)
    await wrapper.find('[data-testid="reports-card-depreciation"]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="reports-status-filter"]').exists()).toBe(false)
  })

  it('shows the basis toggle only for depreciation', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="reports-basis-toggle"]').exists()).toBe(false)
    await wrapper.find('[data-testid="reports-card-depreciation"]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="reports-basis-toggle"]').exists()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// 3 — placeholder before apply
// ---------------------------------------------------------------------------
describe('Reports page — placeholder', () => {
  it('shows the pre-apply placeholder and does not call run', async () => {
    const wrapper = await mountPage()
    expect(wrapper.text()).toContain('Pilih kriteria laporan')
    expect(runMock).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// 3b — office filter is an async search picker (Task 6)
// ---------------------------------------------------------------------------
describe('Reports page — office filter picker', () => {
  it('renders the office filter as an async search picker, not a USelect', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="reports-office-filter-picker-input"]').exists()).toBe(true)
  })

  it('typing in the office filter picker drives GET /offices-style search via useOffices().list', async () => {
    const wrapper = await mountPage()
    vi.useFakeTimers()
    await wrapper.find('[data-testid="reports-office-filter-picker-input"]').setValue('Kantor B')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(officesListMock).toHaveBeenCalledWith(expect.objectContaining({ search: 'Kantor B', limit: 20 }))
  })

  it('clearing the office filter picker resets officeId to null and omits officeId on apply', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as Vm
    vm.officeId = 'o1'
    await wrapper.vm.$nextTick()

    const clearBtn = wrapper.find('[data-testid="reports-office-filter-picker-clear"]')
    expect(clearBtn.exists()).toBe(true)
    await clearBtn.trigger('click')
    await wrapper.vm.$nextTick()

    expect(vm.officeId).toBeNull()
    await applyAndSettle(wrapper)
    expect(runMock.mock.calls[0]![1]).toMatchObject({ officeId: undefined })
  })
})

// ---------------------------------------------------------------------------
// 4 — apply maps filters
// ---------------------------------------------------------------------------
describe('Reports page — apply', () => {
  it('calls run(type, filters) with the mapped office/category/status and the default quarter period', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as Vm
    vm.officeId = 'o1'
    vm.categoryId = 'c2'
    vm.status = 'available'
    await applyAndSettle(wrapper)
    expect(runMock).toHaveBeenCalledTimes(1)
    expect(runMock.mock.calls[0]![0]).toBe('assets')
    expect(runMock.mock.calls[0]![1]).toMatchObject({
      period: { preset: 'this_quarter' },
      officeId: 'o1',
      categoryId: 'c2',
      status: 'available'
    })
  })

  it('omits status/basis when the report type does not use them', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as Vm
    vm.selectReport('maintenance')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    const filters = runMock.mock.calls[0]![1] as Record<string, unknown>
    expect(filters.status).toBeUndefined()
    expect(filters.basis).toBeUndefined()
  })

  it('sends the depreciation basis and a custom period as {preset,from,to}', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as Vm
    vm.selectReport('depreciation')
    await wrapper.vm.$nextTick()
    const pf = wrapper.findComponent(PeriodFilter)
    ;(pf.vm as unknown as { onCalendarUpdate: (r: unknown) => void }).onCalendarUpdate({ start: parseDate('2026-01-01'), end: parseDate('2026-03-31') })
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    expect(runMock.mock.calls[0]![0]).toBe('depreciation')
    expect(runMock.mock.calls[0]![1]).toMatchObject({
      basis: 'commercial',
      period: { preset: 'custom', from: '2026-01-01', to: '2026-03-31' }
    })
  })
})

// ---------------------------------------------------------------------------
// 5 — loading state
// ---------------------------------------------------------------------------
describe('Reports page — loading', () => {
  it('shows a loading state while run is pending', async () => {
    let resolve!: (v: ReportResult) => void
    runMock.mockReturnValue(new Promise<ReportResult>((r) => {
      resolve = r
    }))
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as Vm).apply()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="reports-loading"]').exists()).toBe(true)
    resolve(assetsResult)
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="reports-loading"]').exists()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// 6 — error + retry
// ---------------------------------------------------------------------------
describe('Reports page — error', () => {
  it('renders an error state with a retry that re-runs', async () => {
    runMock.mockRejectedValueOnce(new Error('boom'))
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    expect(wrapper.find('[data-testid="reports-retry"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Gagal memuat laporan')

    await wrapper.find('[data-testid="reports-retry"]').trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-testid="reports-retry"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Total Aset')
    expect(runMock).toHaveBeenCalledTimes(2)
  })
})

// ---------------------------------------------------------------------------
// 7 — empty
// ---------------------------------------------------------------------------
describe('Reports page — empty', () => {
  it('renders the empty state with a reset when run returns no rows', async () => {
    runMock.mockResolvedValue({ ...assetsResult, rows: [], row_count: 0 })
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as Vm
    vm.status = 'lost'
    await applyAndSettle(wrapper)
    expect(wrapper.text()).toContain('Tidak ada data')
    const reset = wrapper.find('[data-testid="reports-empty-reset"]')
    expect(reset.exists()).toBe(true)
    await reset.trigger('click')
    await wrapper.vm.$nextTick()
    expect(vm.status).toBe('all')
  })
})

// ---------------------------------------------------------------------------
// 8 — assets table
// ---------------------------------------------------------------------------
describe('Reports page — assets result', () => {
  it('renders KPIs, a chart, populated rows and a money-formatted TOTAL footer', async () => {
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    const text = wrapper.text()
    expect(text).toContain('Total Aset')
    expect(text).toContain('Rp 3,82 M') // total_acquisition KPI
    expect(text).toContain('Nilai Buku per Kategori') // chart title
    expect(text).toContain('Laptop Dell Latitude 5440') // a row
    expect(text).toContain('TOTAL')
    expect(text).toContain('Rp 214,2 Jt') // book_value footer total
  })
})

// ---------------------------------------------------------------------------
// 9 — disposal gain/loss tones
// ---------------------------------------------------------------------------
describe('Reports page — disposal tones', () => {
  it('tints gain green and loss red', async () => {
    runMock.mockResolvedValue(disposalsResult)
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as Vm).selectReport('disposals')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    const cells = wrapper.findAll('td')
    const gain = cells.find(c => c.text() === 'Rp 2 Jt')
    const loss = cells.find(c => c.text() === 'Rp −3,5 Jt')
    expect(gain).toBeTruthy()
    expect(gain!.classes().join(' ')).toContain('text-success')
    expect(loss).toBeTruthy()
    expect(loss!.classes().join(' ')).toContain('text-error')
  })
})

// ---------------------------------------------------------------------------
// 10 — transfer status labels
// ---------------------------------------------------------------------------
describe('Reports page — transfer labels', () => {
  it('localizes the transfer status and renders — for empty dates/BAST', async () => {
    runMock.mockResolvedValue(transfersResult)
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as Vm).selectReport('transfers')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    const text = wrapper.text()
    expect(text).toContain('Dalam Perjalanan') // reports.kpi.in_transit (KPI tile)
    // status CELLS localize via transfer.status.* — assert on the <td>s, not page text
    const cellTexts = wrapper.findAll('td').map(c => c.text())
    expect(cellTexts).toContain('Dalam Pengiriman') // transfer.status.in_transit
    expect(cellTexts).toContain('Diterima') // transfer.status.received
    expect(cellTexts).toContain('Diajukan') // transfer.status.pending (aliased to diajukan)
    expect(text).toContain('—') // empty received_date / bast_no
    // no money tfoot for transfers
    expect(wrapper.find('tfoot').exists()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// 11 — opname BA download
// ---------------------------------------------------------------------------
describe('Reports page — opname BA', () => {
  it('renders the plan column order Sesi · Kantor · Periode · Total Item · Selisih · Status with the row period', async () => {
    runMock.mockResolvedValue(opnameResult)
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as Vm).selectReport('opname')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    const headers = wrapper.findAll('th').map(h => h.text())
    expect(headers).toEqual(['Sesi', 'Kantor', 'Periode', 'Total Item', 'Selisih', 'Status', 'Berita Acara'])
    const cells = wrapper.findAll('td').map(c => c.text())
    expect(cells[2]).toBe('2026-06') // Periode cell for sess-1
  })

  it('renders per-row BA buttons that call opnameBa(sessionId, format)', async () => {
    runMock.mockResolvedValue(opnameResult)
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as Vm).selectReport('opname')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    const pdfBtn = wrapper.find('[data-testid="reports-opname-ba-pdf-sess-1"]')
    expect(pdfBtn.exists()).toBe(true)
    await pdfBtn.trigger('click')
    await flushPromises()
    expect(opnameBaMock).toHaveBeenCalledWith('sess-1', 'pdf')

    await wrapper.find('[data-testid="reports-opname-ba-xlsx-sess-1"]').trigger('click')
    await flushPromises()
    expect(opnameBaMock).toHaveBeenCalledWith('sess-1', 'xlsx')
  })
})

// ---------------------------------------------------------------------------
// 12 — GL recap (disposals only)
// ---------------------------------------------------------------------------
describe('Reports page — GL recap', () => {
  it('shows the GL recap control only for disposals', async () => {
    const wrapper = await mountPage()
    await applyAndSettle(wrapper) // assets
    expect(wrapper.find('[data-testid="reports-export-gl"]').exists()).toBe(false)

    runMock.mockResolvedValue(disposalsResult)
    ;(wrapper.vm as unknown as Vm).selectReport('disposals')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    expect(wrapper.find('[data-testid="reports-export-gl"]').exists()).toBe(true)
  })

  it('exportReport is called with the gl_recap variant', async () => {
    runMock.mockResolvedValue(disposalsResult)
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as Vm).selectReport('disposals')
    await wrapper.vm.$nextTick()
    await applyAndSettle(wrapper)
    await (wrapper.vm as unknown as Vm).doExportGl('xlsx')
    await flushPromises()
    expect(exportMock).toHaveBeenCalledTimes(1)
    expect(exportMock.mock.calls[0]![0]).toBe('disposals')
    expect(exportMock.mock.calls[0]![2]).toBe('xlsx')
    expect(exportMock.mock.calls[0]![3]).toBe('gl_recap')
  })
})

// ---------------------------------------------------------------------------
// 13 — export flow
// ---------------------------------------------------------------------------
describe('Reports page — export', () => {
  it('doExport calls exportReport(type, filters, format)', async () => {
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    await (wrapper.vm as unknown as Vm).doExport('pdf')
    await flushPromises()
    expect(exportMock).toHaveBeenCalledTimes(1)
    expect(exportMock.mock.calls[0]![0]).toBe('assets')
    expect(exportMock.mock.calls[0]![2]).toBe('pdf')
  })

  it('hides the export buttons without report.export', async () => {
    grant(['report.view'])
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    expect(wrapper.find('[data-testid="reports-export-pdf"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="reports-export-xlsx"]').exists()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// 14 — truncated
// ---------------------------------------------------------------------------
describe('Reports page — truncated', () => {
  it('renders a truncation notice when the backend caps the row set', async () => {
    runMock.mockResolvedValue({ ...assetsResult, truncated: true, row_count: 500 })
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    const notice = wrapper.find('[data-testid="reports-truncated"]')
    expect(notice.exists()).toBe(true)
    expect(notice.text()).toContain('500')
  })

  it('omits the notice when not truncated', async () => {
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    expect(wrapper.find('[data-testid="reports-truncated"]').exists()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// 15 — result meta (honest office label)
// ---------------------------------------------------------------------------
describe('Reports page — result meta', () => {
  it('reflects the selected office (not a hardcoded branch)', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as Vm
    vm.officeId = 'o2'
    await applyAndSettle(wrapper)
    expect(wrapper.text()).toContain('Kantor B')
  })

  it('falls back to All Offices when none is selected', async () => {
    const wrapper = await mountPage()
    await applyAndSettle(wrapper)
    expect(wrapper.text()).toContain('Semua Kantor')
  })
})
