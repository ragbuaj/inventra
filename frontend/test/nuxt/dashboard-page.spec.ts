// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, beforeAll, afterEach, afterAll } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { DashboardSummary } from '~/composables/api/useDashboard'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'
import type { Office } from '~/types'
import { useAuthStore } from '~/stores/auth'
import IndexPage from '~/pages/index.vue'

enableAutoUnmount(afterEach)

// dueLabel() compares local-midnight dates; isoDaysFromNow() builds UTC dates.
// Pin TZ=UTC so the two agree regardless of the host machine's timezone.
const ORIGINAL_TZ = process.env.TZ
beforeAll(() => {
  process.env.TZ = 'UTC'
})
afterAll(() => {
  process.env.TZ = ORIGINAL_TZ
})

// ---------------------------------------------------------------------------
// Composable mocks (controllable per test)
// ---------------------------------------------------------------------------
const { summaryMock, exportMock, inboxMock, approveMock, rejectMock, officesListMock, officesGetMock, toastAddMock } = vi.hoisted(() => ({
  summaryMock: vi.fn(),
  exportMock: vi.fn(),
  inboxMock: vi.fn(),
  approveMock: vi.fn(),
  rejectMock: vi.fn(),
  officesListMock: vi.fn(),
  officesGetMock: vi.fn(),
  toastAddMock: vi.fn()
}))

vi.mock('~/composables/api/useDashboard', () => ({
  useDashboard: () => ({ summary: summaryMock, exportSummary: exportMock })
}))
vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({ inbox: inboxMock, approve: approveMock, reject: rejectMock, list: vi.fn(), get: vi.fn() })
}))
vi.mock('~/composables/api/useOffices', () => ({
  useOffices: () => ({ list: officesListMock, get: officesGetMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------
function isoDaysFromNow(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() + n)
  return d.toISOString().slice(0, 10)
}

function office(id: string, name: string): Office {
  return { id, name, code: id.toUpperCase() } as Office
}

function makeSummary(over: Partial<DashboardSummary> = {}): DashboardSummary {
  return {
    office_name: 'Kantor Cabang Jakarta Selatan',
    kpi: {
      total_assets: 96,
      acquisition_value: '3820000000', // Rp 3,82 M
      book_value: '2140000000',
      overdue_assets: 4,
      maintenance_due: 3,
      maintenance_cost: '42500000', // Rp 42,5 Jt
      trends: { acquisition_pct: 8.3, book_value_pct: null, maintenance_cost_pct: 3.1 }
    },
    by_status: [
      { status: 'available', count: 58 },
      { status: 'assigned', count: 22 },
      { status: 'under_maintenance', count: 9 },
      { status: 'in_transfer', count: 2 },
      { status: 'retired', count: 1 },
      { status: 'disposed', count: 4 },
      { status: 'lost', count: 3 }
    ],
    by_category: [
      { name: 'Elektronik', count: 41 },
      { name: 'Furnitur', count: 28 },
      { name: null, count: 3 }
    ],
    location_kind: 'office',
    by_location: [
      { name: 'Cabang Jaksel', count: 60 },
      { name: 'Gudang Aset', count: 24 }
    ],
    maintenance_due_list: [
      { id: 'm1', asset_name: 'Toyota Avanza', asset_tag: 'B 1234 XYZ', category_name: 'Servis berkala', next_due_date: isoDaysFromNow(1) },
      { id: 'm2', asset_name: 'AC Daikin', asset_tag: 'R.301', category_name: null, next_due_date: isoDaysFromNow(5) }
    ],
    excluded_count: 0,
    ...over
  }
}

function inboxRow(over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow {
  return {
    id: 'req1', type: 'assignment', status: 'pending', current_step: 1,
    office_id: 'o1', office_name: 'Cabang Blok M', target_id: null, target_entity: null,
    requested_by_id: 'u2', requested_by_name: 'Andi Saputra', requested_by_role: 'Staf Ops',
    decided_by_id: null, decision_note: null, created_at: '2026-07-10T09:00:00Z',
    ...over
  }
}

function grant(perms: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    perms
  )
}

beforeEach(() => {
  summaryMock.mockReset().mockResolvedValue(makeSummary())
  inboxMock.mockReset().mockResolvedValue([])
  exportMock.mockReset().mockResolvedValue(new Blob(['x']))
  approveMock.mockReset().mockResolvedValue({})
  rejectMock.mockReset().mockResolvedValue({})
  officesListMock.mockReset().mockResolvedValue({ data: [office('o1', 'Kantor A'), office('o2', 'Kantor B')], total: 2, limit: 100, offset: 0 })
  officesGetMock.mockReset().mockImplementation(async (id: string) => {
    const found = [office('o1', 'Kantor A'), office('o2', 'Kantor B')].find(o => o.id === id)
    if (!found) throw Object.assign(new Error('not found'), { statusCode: 404 })
    return found
  })
  toastAddMock.mockReset()
  ;(URL as unknown as { createObjectURL: unknown }).createObjectURL = vi.fn(() => 'blob:mock')
  ;(URL as unknown as { revokeObjectURL: unknown }).revokeObjectURL = vi.fn()
  grant(['*'])
})

async function mountPage() {
  const wrapper = await mountSuspended(IndexPage, { route: '/' })
  await flushPromises()
  await wrapper.vm.$nextTick()
  await flushPromises()
  return wrapper
}

function bodyEl(testid: string): HTMLElement | null {
  return document.body.querySelector(`[data-testid="${testid}"]`)
}

function setInputValue(el: HTMLElement, value: string) {
  const input = el as HTMLTextAreaElement
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

// ---------------------------------------------------------------------------
// 1 — loading
// ---------------------------------------------------------------------------
describe('Dashboard page — loading', () => {
  it('shows the header immediately but skeletons (no KPI figures) while summary is pending', async () => {
    let resolve!: (v: DashboardSummary) => void
    summaryMock.mockReturnValue(new Promise<DashboardSummary>((r) => {
      resolve = r
    }))
    const wrapper = await mountSuspended(IndexPage, { route: '/' })
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Dashboard')
    expect(wrapper.text()).not.toContain('96')
    resolve(makeSummary())
    await flushPromises()
  })
})

// ---------------------------------------------------------------------------
// 2/3 — KPIs + trends
// ---------------------------------------------------------------------------
describe('Dashboard page — KPIs', () => {
  it('renders KPI figures with short-money formatting', async () => {
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('Total Aset')
    expect(text).toContain('96')
    expect(text).toContain('Nilai Perolehan')
    expect(text).toContain('Rp 3,82 M')
    expect(text).toContain('Total Biaya Maintenance')
    expect(text).toContain('Rp 42,5 Jt')
  })

  it('renders a computed trend as a signed percentage and a null trend as the static descriptor', async () => {
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('+8,3%') // acquisition_pct 8.3
    expect(text).toContain('Relatif stabil') // book_value_pct null → fallback
  })
})

// ---------------------------------------------------------------------------
// 4/5 — charts
// ---------------------------------------------------------------------------
describe('Dashboard page — charts', () => {
  it('renders the status donut and category bars from the summary', async () => {
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('Aset per Status')
    expect(text).toContain('Tersedia') // available
    expect(text).toContain('Digunakan') // assigned (renamed enum key)
    expect(text).toContain('Aset per Kategori')
    expect(text).toContain('Elektronik')
    expect(text).toContain('41')
  })

  it('switches the location card title between office and room kinds', async () => {
    const wrapper = await mountPage()
    expect(wrapper.text()).toContain('Aset per Kantor')

    summaryMock.mockResolvedValue(makeSummary({ location_kind: 'room', by_location: [{ name: 'Ruang 301', count: 5 }, { name: null, count: 2 }] }))
    await (wrapper.vm as unknown as { load: () => Promise<void> }).load()
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('Aset per Ruangan')
    expect(text).toContain('Tanpa ruangan') // null room bucket localized
  })
})

// ---------------------------------------------------------------------------
// 6 — maintenance panel
// ---------------------------------------------------------------------------
describe('Dashboard page — maintenance panel', () => {
  it('renders due rows with a localized due label and an urgent badge for tomorrow', async () => {
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('Maintenance Jatuh Tempo')
    expect(text).toContain('Toyota Avanza · B 1234 XYZ')
    expect(text).toContain('Besok') // due tomorrow
    expect(text).toContain('5 hari lagi') // due in 5 days
    // the "Besok" pill is the urgent (warning) variant
    const pill = wrapper.findAll('span').find(s => s.text() === 'Besok')
    expect(pill).toBeTruthy()
    expect(pill!.classes().join(' ')).toContain('text-warning')
  })

  it('falls back to a generic task label when the category is null', async () => {
    const wrapper = await mountPage()
    expect(wrapper.text()).toContain('Maintenance terjadwal')
  })
})

// ---------------------------------------------------------------------------
// 7 — approval panel gating
// ---------------------------------------------------------------------------
describe('Dashboard page — approval panel', () => {
  it('is hidden without request.decide', async () => {
    grant(['dashboard.view'])
    const wrapper = await mountPage()
    expect(wrapper.text()).not.toContain('Pengajuan Menunggu Approval')
    expect(inboxMock).not.toHaveBeenCalled()
  })

  it('renders inbox rows and a total-count badge when granted', async () => {
    inboxMock.mockResolvedValue([
      inboxRow({ id: 'req1', type: 'assignment', office_name: 'Cabang Blok M' }),
      inboxRow({ id: 'req2', type: 'asset_transfer', office_name: null, requested_by_name: 'Rina' })
    ])
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).toContain('Pengajuan Menunggu Approval')
    expect(text).toContain('Peminjaman Aset — Cabang Blok M') // type + office
    expect(text).toContain('Mutasi Aset') // no office suffix
    expect(text).toContain('Andi Saputra · Staf Ops')
  })
})

// ---------------------------------------------------------------------------
// 8 — approve
// ---------------------------------------------------------------------------
describe('Dashboard page — approve', () => {
  it('calls approve(id) then reloads the summary + inbox', async () => {
    inboxMock.mockResolvedValue([inboxRow({ id: 'req1' })])
    const wrapper = await mountPage()
    expect(summaryMock).toHaveBeenCalledTimes(1)
    expect(inboxMock).toHaveBeenCalledTimes(1)

    await wrapper.find('[aria-label="approve-req1"]').trigger('click')
    await flushPromises()

    expect(approveMock).toHaveBeenCalledWith('req1')
    expect(toastAddMock).toHaveBeenCalled()
    expect(summaryMock).toHaveBeenCalledTimes(2) // reloaded
    expect(inboxMock).toHaveBeenCalledTimes(2)
  })
})

// ---------------------------------------------------------------------------
// 9 — reject modal
// ---------------------------------------------------------------------------
describe('Dashboard page — reject modal', () => {
  it('opens the modal, keeps confirm disabled until a note is entered, then calls reject(id, note)', async () => {
    inboxMock.mockResolvedValue([inboxRow({ id: 'req1' })])
    const wrapper = await mountPage()

    await wrapper.find('[aria-label="reject-req1"]').trigger('click')
    await flushPromises()
    await new Promise(r => setTimeout(r, 400))
    await wrapper.vm.$nextTick()

    const confirm = bodyEl('dashboard-reject-confirm') as HTMLButtonElement
    expect(confirm).toBeTruthy()
    expect(confirm.hasAttribute('disabled')).toBe(true)
    expect(rejectMock).not.toHaveBeenCalled()

    const note = bodyEl('dashboard-reject-note') as HTMLTextAreaElement
    setInputValue(note, 'Data tidak lengkap')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(confirm.hasAttribute('disabled')).toBe(false)
    confirm.click()
    await flushPromises()

    expect(rejectMock).toHaveBeenCalledWith('req1', 'Data tidak lengkap')
    expect(summaryMock).toHaveBeenCalledTimes(2) // reloaded
  })
})

// ---------------------------------------------------------------------------
// 10 — export
// ---------------------------------------------------------------------------
describe('Dashboard page — export', () => {
  it('hides the export control without report.export', async () => {
    grant(['request.decide'])
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-export"]').exists()).toBe(false)
  })

  it('shows the export control and exports a PDF with the current query', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-export"]').exists()).toBe(true)
    await (wrapper.vm as unknown as { doExport: (f: 'pdf' | 'xlsx') => Promise<void> }).doExport('pdf')
    await flushPromises()
    expect(exportMock).toHaveBeenCalledTimes(1)
    expect(exportMock.mock.calls[0]![1]).toBe('pdf')
    expect(exportMock.mock.calls[0]![0]).toMatchObject({ period: { preset: 'last30' } })
  })
})

// ---------------------------------------------------------------------------
// 11 — load failure + retry
// ---------------------------------------------------------------------------
describe('Dashboard page — load failure', () => {
  it('renders an error state with a retry that reloads', async () => {
    summaryMock.mockRejectedValueOnce(new Error('boom'))
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-retry"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Total Aset')

    await wrapper.find('[data-testid="dashboard-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="dashboard-retry"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Total Aset')
  })
})

// ---------------------------------------------------------------------------
// 12 — empty summary
// ---------------------------------------------------------------------------
describe('Dashboard page — empty summary', () => {
  it('renders a zero-state donut without NaN when every count is zero', async () => {
    summaryMock.mockResolvedValue(makeSummary({
      office_name: null,
      kpi: {
        total_assets: 0, acquisition_value: '0', book_value: '0', overdue_assets: 0,
        maintenance_due: 0, maintenance_cost: '0',
        trends: { acquisition_pct: null, book_value_pct: null, maintenance_cost_pct: null }
      },
      by_status: [
        { status: 'available', count: 0 }, { status: 'assigned', count: 0 },
        { status: 'under_maintenance', count: 0 }, { status: 'in_transfer', count: 0 },
        { status: 'retired', count: 0 }, { status: 'disposed', count: 0 }, { status: 'lost', count: 0 }
      ],
      by_category: [], by_location: [], maintenance_due_list: []
    }))
    const wrapper = await mountPage()
    const text = wrapper.text()
    expect(text).not.toContain('NaN')
    expect(text).toContain('Seluruh scope Anda') // office_name null → scopeAll
    expect(text).toContain('Aset per Status')
  })
})

// ---------------------------------------------------------------------------
// 13 — office select visibility
// ---------------------------------------------------------------------------
describe('Dashboard page — office select', () => {
  it('is hidden when the scope holds a single office', async () => {
    officesListMock.mockResolvedValue({ data: [office('o1', 'Kantor A')], total: 1, limit: 100, offset: 0 })
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-office-picker-input"]').exists()).toBe(false)
  })

  it('is shown when the scope holds more than one office', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-office-picker-input"]').exists()).toBe(true)
  })

  it('selecting an office (via the picker) reloads with officeId', async () => {
    const wrapper = await mountPage()
    summaryMock.mockClear()

    vi.useFakeTimers()
    await wrapper.find('[data-testid="dashboard-office-picker-input"]').setValue('Kantor B')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await wrapper.vm.$nextTick()

    const item = wrapper.findAll('[data-testid="dashboard-office-picker-item"]').find(i => i.text().includes('Kantor B'))
    expect(item).toBeDefined()
    // vi.useRealTimers() must run *after* the click — see the equivalent
    // comment in async-search-picker.spec.ts (Vue's own-event guard silently
    // swallows a click whose timeStamp lands behind the fake-advanced clock).
    await item!.trigger('click')
    vi.useRealTimers()
    await flushPromises()
    expect(summaryMock).toHaveBeenLastCalledWith(expect.objectContaining({ officeId: 'o2' }))
  })

  it('clearing the office filter picker resets officeId to null and reloads without officeId', async () => {
    const wrapper = await mountPage()
    ;(wrapper.vm as unknown as { officeId: string | null }).officeId = 'o1'
    await (wrapper.vm as unknown as { load: () => Promise<void> }).load()
    await flushPromises()
    await wrapper.vm.$nextTick()
    summaryMock.mockClear()

    const clearBtn = wrapper.find('[data-testid="dashboard-office-picker-clear"]')
    expect(clearBtn.exists()).toBe(true)
    await clearBtn.trigger('click')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect((wrapper.vm as unknown as { officeId: string | null }).officeId).toBeNull()
    expect(summaryMock).toHaveBeenLastCalledWith(expect.objectContaining({ officeId: undefined }))
  })
})

// ---------------------------------------------------------------------------
// 14 — excluded note
// ---------------------------------------------------------------------------
describe('Dashboard page — valuation exclusion note', () => {
  it('renders the excluded-count note when excluded_count > 0', async () => {
    summaryMock.mockResolvedValue(makeSummary({ excluded_count: 3 }))
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-excluded-note"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('3 aset dikecualikan')
  })

  it('omits the note when excluded_count is 0', async () => {
    const wrapper = await mountPage()
    expect(wrapper.find('[data-testid="dashboard-excluded-note"]').exists()).toBe(false)
  })
})
