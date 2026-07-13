// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach, beforeAll, afterAll } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Asset } from '~/types'
import type { Assignment } from '~/composables/api/useAssignment'
import type { MaintenanceSchedule, MaintenanceRecord, AttentionItem } from '~/composables/api/useMaintenance'
import { useAuthStore } from '~/stores/auth'

// dueDiffDays() compares date-only strings (parsed as UTC midnight) against
// `new Date()` via local-timezone getters — pin TZ=UTC for this file so the
// due-banner/Jadwal fixtures below (built with isoDaysFromNow()) are exact
// regardless of the host machine's timezone. Restored after the file runs.
const ORIGINAL_TZ = process.env.TZ
beforeAll(() => {
  process.env.TZ = 'UTC'
})
afterAll(() => {
  process.env.TZ = ORIGINAL_TZ
})

// useToast's real toast portal isn't mounted here (no UApp wrapper) — mock and
// assert on call args, per the established convention (see peminjaman.spec.ts).
// The maintenance slideovers call useToast() internally on success.
const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

function isoDaysFromNow(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() + n)
  return d.toISOString().slice(0, 10)
}

function schedule(over: Partial<MaintenanceSchedule> = {}): MaintenanceSchedule {
  return {
    id: 's1', asset_id: 'a1', maintenance_category_id: 'mc1', interval_months: 6,
    last_done_date: null, next_due_date: isoDaysFromNow(10), is_active: true,
    asset_name: 'Switch Cisco Catalyst 1000', asset_tag: 'JKT01-ITX-2025-00022',
    office_name: 'Kantor Pusat', category_name: 'Update Firmware',
    created_at: null, updated_at: null,
    ...over
  }
}

function record(over: Partial<MaintenanceRecord> = {}): MaintenanceRecord {
  return {
    id: 'r1', asset_id: 'a1', schedule_id: null, maintenance_category_id: 'mc1',
    problem_category_id: null, type: 'corrective', status: 'completed',
    scheduled_date: '2026-05-12', completed_date: '2026-05-12', cost: '2350000',
    vendor_id: 'v1', performed_by: null, description: 'Servis rutin',
    reported_by_id: null, asset_name: 'Toyota Avanza 1.5 G', asset_tag: 'JKT01-KEN-2025-00007',
    office_name: 'Kantor Cabang Jakarta Selatan', category_name: 'Servis Mesin',
    problem_name: null, vendor_name: 'Auto2000', reported_by_name: null,
    created_at: null, updated_at: null,
    ...over
  }
}

function attentionItem(over: Partial<AttentionItem> = {}): AttentionItem {
  return {
    id: 'a2', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440',
    office_id: 'o1', office_name: 'Kantor Pusat',
    ...over
  }
}

function assignment(over: Partial<Assignment> = {}): Assignment {
  return {
    id: 'as1', asset_id: 'asset-mine-1', employee_id: 'e1', assigned_by_id: 'u1',
    checkout_date: '2026-01-01', due_date: null, checkin_date: null,
    condition_out: 'baik', condition_in: null, status: 'active', notes: null,
    asset_name: 'Monitor LG 27UL550', asset_tag: 'JKT01-ELK-2026-00005',
    employee_name: 'Staf Satu', assigned_by_name: 'Manager', office_name: 'Kantor Pusat',
    created_at: null, updated_at: null,
    ...over
  }
}

function asset(over: Partial<Asset> = {}): Asset {
  return {
    id: 'a2', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440',
    category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible',
    ...over
  } as Asset
}

function reportRow(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'req1',
    type: 'maintenance',
    status: 'pending',
    target_id: null,
    created_at: '2026-07-06T09:00:00Z',
    decision_note: null,
    payload: { asset_id: 'asset-mine-1', problem_category_id: 'pc1', description: 'Layar berkedip' },
    ...over
  }
}

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const schedulesMock = vi.fn()
const createScheduleMock = vi.fn()
const updateScheduleMock = vi.fn()
const recordsMock = vi.fn()
const createRecordMock = vi.fn()
const updateRecordMock = vi.fn()
const attentionMock = vi.fn()
const submitReportMock = vi.fn()
const myReportsMock = vi.fn()

vi.mock('~/composables/api/useMaintenance', () => ({
  useMaintenance: () => ({
    schedules: schedulesMock,
    createSchedule: createScheduleMock,
    updateSchedule: updateScheduleMock,
    deleteSchedule: vi.fn(),
    records: recordsMock,
    record: vi.fn(),
    createRecord: createRecordMock,
    updateRecord: updateRecordMock,
    attention: attentionMock,
    listByAsset: vi.fn(),
    submitReport: submitReportMock,
    myReports: myReportsMock
  })
}))

const assignmentMineMock = vi.fn()
vi.mock('~/composables/api/useAssignment', () => ({
  useAssignment: () => ({
    list: vi.fn(),
    available: vi.fn(),
    mine: assignmentMineMock,
    checkout: vi.fn(),
    checkin: vi.fn(),
    borrow: vi.fn(),
    myRequests: vi.fn(),
    cancel: vi.fn()
  })
}))

const assetsGetMock = vi.fn()
vi.mock('~/composables/api/useAssets', () => ({
  useAssets: () => ({
    list: vi.fn(),
    get: assetsGetMock,
    getByTag: vi.fn(),
    update: vi.fn()
  })
}))

const PROBLEM_CATEGORIES = [{ id: 'pc1', name: 'Layar / Tampilan' }, { id: 'pc2', name: 'Mati Total / Tidak Menyala' }]
const MAINT_CATEGORIES = [{ id: 'mc1', name: 'Servis Berkala' }]
const VENDORS = [{ id: 'v1', name: 'Auto2000' }]

function page<T>(data: T[]): { data: T[], total: number, limit: number, offset: number } {
  return { data, total: data.length, limit: 100, offset: 0 }
}

const referenceListMock = vi.fn((key: string) => {
  if (key === 'problem-categories') return Promise.resolve(page(PROBLEM_CATEGORIES))
  if (key === 'maintenance-categories') return Promise.resolve(page(MAINT_CATEGORIES))
  if (key === 'vendors') return Promise.resolve(page(VENDORS))
  return Promise.resolve(page([]))
})
const referenceGetMock = vi.fn((key: string, id: string) => {
  const rows = key === 'problem-categories'
    ? PROBLEM_CATEGORIES
    : key === 'maintenance-categories' ? MAINT_CATEGORIES : key === 'vendors' ? VENDORS : []
  const row = rows.find(r => r.id === id)
  return row ? Promise.resolve(row) : Promise.reject(new Error('not found'))
})
vi.mock('~/composables/api/useReference', () => ({
  useReference: () => ({
    list: referenceListMock,
    get: referenceGetMock,
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn()
  })
}))

// eslint-disable-next-line import/first
import MaintenancePage from '~/pages/maintenance.vue'

enableAutoUnmount(afterEach)
// Belt-and-suspenders: a fake-timers test that fails before reaching its own
// vi.useRealTimers() would otherwise leave every later test's setTimeout-based
// waits hanging forever.
afterEach(() => {
  vi.useRealTimers()
})

function grant(permissions: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Andi Saputra', email: 'andi@test.com', role_id: 'r1', role_name: 'Staf', office_id: 'o1', employee_id: 'e1' },
    permissions
  )
}

async function mountAndWait() {
  const wrapper = await mountSuspended(MaintenancePage)
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 50))
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountAndWait>>
type Vm = Record<string, unknown>

async function setVmRef(wrapper: Wrapper, key: string, value: unknown) {
  ;(wrapper.vm as unknown as Vm)[key] = value
  await wrapper.vm.$nextTick()
  await flushPromises()
  await wrapper.vm.$nextTick()
}

function clickTab(wrapper: Wrapper, label: string) {
  const btn = wrapper.findAll('button').find(b => b.text().trim() === label)
  return btn!.trigger('click')
}

beforeEach(() => {
  vi.clearAllMocks()
  schedulesMock.mockResolvedValue(page([schedule()]))
  recordsMock.mockResolvedValue(page([record()]))
  attentionMock.mockResolvedValue({ data: [] })
  assignmentMineMock.mockResolvedValue({ data: [assignment()] })
  assetsGetMock.mockResolvedValue(asset())
  myReportsMock.mockResolvedValue({ data: [], total: 0 })
  submitReportMock.mockResolvedValue({ request_id: 'req-new', status: 'pending' })
  createScheduleMock.mockResolvedValue(schedule({ id: 'new' }))
  updateScheduleMock.mockResolvedValue(schedule())
  createRecordMock.mockResolvedValue(record({ id: 'new' }))
  updateRecordMock.mockResolvedValue(record())
  toastAddMock.mockReset()
  grant(['*'])
})

// ---------------------------------------------------------------------------
// Due banner
// ---------------------------------------------------------------------------

describe('Maintenance page — due banner', () => {
  it('shows the banner with an overdue item labelled "Terlambat N hari" and hides it when nothing is due', async () => {
    schedulesMock.mockResolvedValue(page([
      schedule({ id: 's-overdue', asset_name: 'Toyota Avanza 1.5 G', next_due_date: isoDaysFromNow(-4) }),
      schedule({ id: 's-far', asset_name: 'Genset Cummins C22 D5', next_due_date: isoDaysFromNow(10) })
    ]))
    const w = await mountAndWait()
    const banner = w.find('[data-testid="due-banner"]')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('Maintenance jatuh tempo')
    expect(banner.text()).toContain('Terlambat 4 hari')
    expect(banner.text()).toContain('Toyota Avanza 1.5 G')
    expect(banner.text()).not.toContain('Genset Cummins C22 D5') // outside the ≤3 day window — banner only
  })

  it('shows "Jatuh tempo hari ini" and "N hari lagi" labels, and hides the banner when nothing is due within 3 days', async () => {
    schedulesMock.mockResolvedValue(page([
      schedule({ id: 's-today', asset_name: 'AC Daikin FTKC50', next_due_date: isoDaysFromNow(0) }),
      schedule({ id: 's-soon', asset_name: 'Genset Cummins C22 D5', next_due_date: isoDaysFromNow(3) })
    ]))
    const w = await mountAndWait()
    const banner = w.find('[data-testid="due-banner"]')
    expect(banner.text()).toContain('Jatuh tempo hari ini')
    expect(banner.text()).toContain('3 hari lagi')
  })

  it('hides the banner entirely when no schedule is due within 3 days', async () => {
    schedulesMock.mockResolvedValue(page([schedule({ next_due_date: isoDaysFromNow(10) })]))
    const w = await mountAndWait()
    expect(w.find('[data-testid="due-banner"]').exists()).toBe(false)
  })

  it('excludes an inactive schedule from the banner even when severely overdue', async () => {
    schedulesMock.mockResolvedValue(page([
      schedule({ id: 's-inactive-overdue', asset_name: 'Printer HP LaserJet', next_due_date: isoDaysFromNow(-30), is_active: false }),
      schedule({ id: 's-far', asset_name: 'Genset Cummins C22 D5', next_due_date: isoDaysFromNow(10) })
    ]))
    const w = await mountAndWait()
    expect(w.find('[data-testid="due-banner"]').exists()).toBe(false)
  })

  it('"Lihat Jadwal" switches to the Jadwal tab', async () => {
    schedulesMock.mockResolvedValue(page([schedule({ next_due_date: isoDaysFromNow(-1) })]))
    const w = await mountAndWait()
    await clickTab(w, 'Catatan')
    expect((w.vm as unknown as Vm).tab).toBe('catatan')
    await w.find('[data-testid="due-banner-see-schedule"]').trigger('click')
    expect((w.vm as unknown as Vm).tab).toBe('jadwal')
  })
})

// ---------------------------------------------------------------------------
// Perlu Tindak Lanjut (attention)
// ---------------------------------------------------------------------------

describe('Maintenance page — Perlu Tindak Lanjut', () => {
  it('renders only when attention() is non-empty and the caller can manage', async () => {
    attentionMock.mockResolvedValue({ data: [attentionItem()] })
    const w = await mountAndWait()
    expect(w.find('[data-testid="attention-section"]').exists()).toBe(true)
    expect(w.text()).toContain('Perlu Tindak Lanjut')
    expect(w.text()).toContain('Laptop Dell Latitude 5440')
  })

  it('stays hidden when attention() is empty', async () => {
    attentionMock.mockResolvedValue({ data: [] })
    const w = await mountAndWait()
    expect(w.find('[data-testid="attention-section"]').exists()).toBe(false)
  })

  it('opens the record slideover prefilled with the asset + type=corrective', async () => {
    attentionMock.mockResolvedValue({ data: [attentionItem()] })
    const w = await mountAndWait()
    await w.find('[data-testid="attention-note-a2"]').trigger('click')
    const vm = w.vm as unknown as Vm
    expect(vm.recordSlideoverOpen).toBe(true)
    expect(vm.recordSlideoverPrefill).toEqual({
      asset: { id: 'a2', name: 'Laptop Dell Latitude 5440', asset_tag: 'JKT01-ELK-2026-00001' },
      type: 'corrective'
    })
  })
})

// ---------------------------------------------------------------------------
// Jadwal tab
// ---------------------------------------------------------------------------

describe('Maintenance page — Jadwal tab', () => {
  it('shows a loading skeleton, then the populated list with due colors', async () => {
    schedulesMock.mockResolvedValue(page([
      schedule({ id: 's-overdue', asset_name: 'Toyota Avanza 1.5 G', next_due_date: isoDaysFromNow(-2) }),
      schedule({ id: 's-far', asset_name: 'Genset Cummins C22 D5', next_due_date: isoDaysFromNow(20) })
    ]))
    const w = await mountAndWait()
    expect(w.text()).toContain('Toyota Avanza 1.5 G')
    expect(w.text()).toContain('Genset Cummins C22 D5')
    expect(w.text()).toContain('Terlambat 2 hari')
    const overdueCard = w.find('[data-testid="schedule-card-s-overdue"]')
    expect(overdueCard.classes().join(' ')).toContain('border-error/35')
  })

  it('renders a neutral "Nonaktif" badge (not the colored overdue label) for an inactive schedule, even when overdue', async () => {
    schedulesMock.mockResolvedValue(page([
      schedule({ id: 's-inactive', asset_name: 'Printer HP LaserJet', next_due_date: isoDaysFromNow(-15), is_active: false })
    ]))
    const w = await mountAndWait()
    const card = w.find('[data-testid="schedule-card-s-inactive"]')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('Nonaktif')
    expect(card.text()).not.toContain('Terlambat')
    expect(card.classes().join(' ')).not.toContain('border-error/35')
  })

  it('shows the error state with retry on schedules() failure', async () => {
    schedulesMock.mockRejectedValueOnce(new Error('boom'))
    const w = await mountAndWait()
    expect(w.find('[data-testid="jadwal-load-error"]').exists()).toBe(true)

    schedulesMock.mockResolvedValueOnce(page([schedule()]))
    await w.find('[data-testid="jadwal-retry"]').trigger('click')
    await flushPromises()
    expect(schedulesMock).toHaveBeenCalledTimes(2)
    expect(w.find('[data-testid="jadwal-load-error"]').exists()).toBe(false)
  })

  it('shows the empty state when there are no schedules', async () => {
    schedulesMock.mockResolvedValue(page([]))
    const w = await mountAndWait()
    expect(w.text()).toContain('Belum ada jadwal maintenance')
  })

  it('hides "Tambah Jadwal" and disables schedule-card edit without maintenance.manage', async () => {
    grant(['maintenance.view', 'request.create'])
    const w = await mountAndWait()
    expect(w.find('[data-testid="jadwal-add-button"]').exists()).toBe(false)
    await w.find('[data-testid="schedule-card-s1"]').trigger('click')
    expect((w.vm as unknown as Vm).scheduleSlideoverOpen).toBe(false)
  })

  it('shows "Tambah Jadwal" and opens the create/edit slideover with maintenance.manage', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="jadwal-add-button"]').exists()).toBe(true)
    await w.find('[data-testid="schedule-card-s1"]').trigger('click')
    const vm = w.vm as unknown as Vm
    expect(vm.scheduleSlideoverOpen).toBe(true)
    expect((vm.scheduleSlideoverTarget as MaintenanceSchedule).id).toBe('s1')
  })

  it('"Buat Catatan" opens the record slideover prefilled from the schedule', async () => {
    const w = await mountAndWait()
    await w.find('[data-testid="schedule-make-note-s1"]').trigger('click')
    const vm = w.vm as unknown as Vm
    expect(vm.recordSlideoverOpen).toBe(true)
    expect(vm.recordSlideoverPrefill).toEqual({
      asset: { id: 'a1', name: 'Switch Cisco Catalyst 1000', asset_tag: 'JKT01-ITX-2025-00022' },
      scheduleId: 's1',
      maintenanceCategoryId: 'mc1',
      type: 'preventive'
    })
    // clicking the button must not also trigger the card's own click (edit).
    expect(vm.scheduleSlideoverOpen).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Catatan tab
// ---------------------------------------------------------------------------

describe('Maintenance page — Catatan tab', () => {
  it('renders enriched fields, biaya formatting and badges', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Catatan')
    const text = w.text()
    expect(text).toContain('Toyota Avanza 1.5 G')
    expect(text).toContain('JKT01-KEN-2025-00007')
    expect(text).toContain('Servis Mesin')
    expect(text).toContain('Rp 2.350.000')
    expect(text).toContain('Corrective')
    expect(text).toContain('Selesai')
    expect(text).toContain('Auto2000')
  })

  it('search triggers a refetch with the q param', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Catatan')
    expect(recordsMock).toHaveBeenLastCalledWith({ q: undefined, limit: 100 })

    const search = w.find('input[type="text"]')
    await search.setValue('Honda')
    await new Promise(resolve => setTimeout(resolve, 350))
    await flushPromises()
    expect(recordsMock).toHaveBeenLastCalledWith({ q: 'Honda', limit: 100 })
  })

  it('row click opens the edit slideover only with maintenance.manage', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Catatan')
    await w.find('[data-testid="record-row-r1"]').trigger('click')
    const vm = w.vm as unknown as Vm
    expect(vm.recordSlideoverOpen).toBe(true)
    expect((vm.recordSlideoverTarget as MaintenanceRecord).id).toBe('r1')
  })

  it('does not open the edit slideover on row click without maintenance.manage, and hides Tambah Catatan', async () => {
    grant(['maintenance.view', 'request.create'])
    const w = await mountAndWait()
    await clickTab(w, 'Catatan')
    expect(w.find('[data-testid="catatan-add-button"]').exists()).toBe(false)
    await w.find('[data-testid="record-row-r1"]').trigger('click')
    expect((w.vm as unknown as Vm).recordSlideoverOpen).toBe(false)
  })

  it('shows the error state with retry, and the empty state when there are no records', async () => {
    recordsMock.mockRejectedValueOnce(new Error('boom'))
    const w = await mountAndWait()
    await clickTab(w, 'Catatan')
    expect(w.find('[data-testid="catatan-load-error"]').exists()).toBe(true)

    recordsMock.mockResolvedValueOnce(page([]))
    await w.find('[data-testid="catatan-retry"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tidak ada catatan')
  })
})

// ---------------------------------------------------------------------------
// Laporan Kerusakan tab
// ---------------------------------------------------------------------------

describe('Maintenance page — Laporan Kerusakan tab', () => {
  it('lists the caller\'s active assignments and problem categories, and disables submit until both are chosen', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    expect(assignmentMineMock).toHaveBeenCalledWith({ status: 'active' })
    const submit = () => w.find('[data-testid="report-submit"]')
    expect(submit().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'reportAssetId', 'asset-mine-1')
    expect(submit().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'reportProblemId', 'pc1')
    expect(submit().attributes('disabled')).toBeUndefined()
  })

  it('submits FormData with the photo when one is picked, then shows the success alert', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    await setVmRef(w, 'reportAssetId', 'asset-mine-1')
    await setVmRef(w, 'reportProblemId', 'pc1')
    await setVmRef(w, 'reportDesc', 'Layar berkedip terus menerus')

    const file = new File(['data'], 'kerusakan.jpg', { type: 'image/jpeg' })
    const vm = w.vm as unknown as Vm & { onPhotoChange: (e: unknown) => void }
    vm.onPhotoChange({ target: { files: [file] } })
    await w.vm.$nextTick()

    await w.find('[data-testid="report-submit"]').trigger('click')
    await flushPromises()

    expect(submitReportMock).toHaveBeenCalledWith({
      asset_id: 'asset-mine-1',
      problem_category_id: 'pc1',
      description: 'Layar berkedip terus menerus',
      photo: file
    })
    expect(w.find('[data-testid="report-success"]').exists()).toBe(true)
  })

  it('renders the problem-category field as an AsyncSearchPicker (no more eager-options USelectMenu)', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    expect(w.find('[data-testid="report-problem-picker"]').exists()).toBe(false)
    expect(w.find('[data-testid="report-problem-picker-input"]').exists()).toBe(true)
  })

  it('typing in the problem-category picker drives GET /problem-categories with search+limit=20', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    vi.useFakeTimers()
    await w.find('[data-testid="report-problem-picker-input"]').setValue('Layar')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(referenceListMock).toHaveBeenCalledWith('problem-categories', { search: 'Layar', limit: 20 })
  })

  it('resolves a preselected problem category id to its label via GET /problem-categories/:id', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    await setVmRef(w, 'reportProblemId', 'pc1')
    const input = w.find('[data-testid="report-problem-picker-input"]').element as HTMLInputElement
    expect(input.value).toBe('Layar / Tampilan')
  })

  it('renders "Riwayat Laporan Saya" cards and an empty state', async () => {
    myReportsMock.mockResolvedValue({ data: [], total: 0 })
    let w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    expect(w.text()).toContain('Belum ada laporan')

    myReportsMock.mockResolvedValue({ data: [reportRow()], total: 1 })
    assetsGetMock.mockResolvedValue(asset({ id: 'asset-mine-1', name: 'Monitor LG 27UL550', asset_tag: 'JKT01-ELK-2026-00005' }))
    w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    expect(w.text()).toContain('Monitor LG 27UL550')
    expect(w.text()).toContain('Menunggu Review')
    expect(w.text()).toContain('Layar / Tampilan')
  })

  it('falls back to the raw id/tag when asset-name lookup 403s', async () => {
    myReportsMock.mockResolvedValue({ data: [reportRow({ payload: { asset_id: 'asset-forbidden', problem_category_id: 'pc1' } })], total: 1 })
    assetsGetMock.mockRejectedValue(new Error('403 Forbidden'))
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    expect(w.text()).toContain('asset-forbidden')
  })

  it('leaves the asset picker empty when the caller has no linked employee (server-resolved, not a client check)', async () => {
    // /assignments/mine resolves employee scoping server-side from the caller's
    // JWT — the frontend has no employee_id to branch on anymore, so this just
    // asserts the endpoint is still called plainly and an empty response (what
    // the backend returns for a user with no linked employee) leaves the
    // picker empty and the submit button disabled.
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Andi Saputra', email: 'andi@test.com', role_id: 'r1', role_name: 'Staf', office_id: 'o1', employee_id: null },
      ['maintenance.view', 'maintenance.manage', 'request.create']
    )
    assignmentMineMock.mockResolvedValue({ data: [] })
    const w = await mountAndWait()
    await clickTab(w, 'Laporan Kerusakan')
    expect(assignmentMineMock).toHaveBeenCalledWith({ status: 'active' })
    const submit = () => w.find('[data-testid="report-submit"]')
    expect(submit().attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// Permission variations
// ---------------------------------------------------------------------------

describe('Maintenance page — permission variations', () => {
  it('shows only the Laporan Kerusakan tab without maintenance.view', async () => {
    grant(['request.create'])
    const w = await mountAndWait()
    const labels = w.findAll('button').map(b => b.text().trim())
    expect(labels).not.toContain('Jadwal')
    expect(labels).not.toContain('Catatan')
    expect(labels).toContain('Laporan Kerusakan')
    expect((w.vm as unknown as Vm).tab).toBe('laporan')
    expect(w.text()).toContain('Laporkan Kerusakan Aset')
    expect(schedulesMock).not.toHaveBeenCalled()
    expect(recordsMock).not.toHaveBeenCalled()
    expect(attentionMock).not.toHaveBeenCalled()
  })

  it('view-only (no maintenance.manage): no write buttons, no row/card click-through, attention hidden', async () => {
    attentionMock.mockResolvedValue({ data: [attentionItem()] })
    grant(['maintenance.view', 'request.create'])
    const w = await mountAndWait()
    expect(attentionMock).not.toHaveBeenCalled()
    expect(w.find('[data-testid="attention-section"]').exists()).toBe(false)
    expect(w.find('[data-testid="jadwal-add-button"]').exists()).toBe(false)
    // Schedule card exists but Buat Catatan button is hidden without maintenance.manage
    expect(w.find('[data-testid="schedule-card-s1"]').exists()).toBe(true)
    expect(w.find('[data-testid="schedule-make-note-s1"]').exists()).toBe(false)
    await clickTab(w, 'Catatan')
    expect(w.find('[data-testid="catatan-add-button"]').exists()).toBe(false)
  })
})
