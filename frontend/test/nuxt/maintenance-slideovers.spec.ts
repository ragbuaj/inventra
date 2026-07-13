// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Asset, ReferenceRow, Paginated } from '~/types'
import type { MaintenanceSchedule, MaintenanceRecord } from '~/composables/api/useMaintenance'
import type { RecordPrefill } from '~/components/maintenance/RecordSlideover.vue'

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

const schedulesMock = vi.fn()
const createScheduleMock = vi.fn()
const updateScheduleMock = vi.fn()
const deleteScheduleMock = vi.fn()
const recordsMock = vi.fn()
const recordMock = vi.fn()
const createRecordMock = vi.fn()
const updateRecordMock = vi.fn()
const attentionMock = vi.fn()
const listByAssetMock = vi.fn()
const submitReportMock = vi.fn()
const myReportsMock = vi.fn()

vi.mock('~/composables/api/useMaintenance', () => ({
  useMaintenance: () => ({
    schedules: schedulesMock,
    createSchedule: createScheduleMock,
    updateSchedule: updateScheduleMock,
    deleteSchedule: deleteScheduleMock,
    records: recordsMock,
    record: recordMock,
    createRecord: createRecordMock,
    updateRecord: updateRecordMock,
    attention: attentionMock,
    listByAsset: listByAssetMock,
    submitReport: submitReportMock,
    myReports: myReportsMock
  })
}))

const refListMock = vi.fn()
const refGetMock = vi.fn()
vi.mock('~/composables/api/useReference', () => ({
  useReference: () => ({ list: refListMock, get: refGetMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

const assetsListMock = vi.fn()
vi.mock('~/composables/api/useAssets', () => ({
  useAssets: () => ({ list: assetsListMock, get: vi.fn(), getByTag: vi.fn(), update: vi.fn() })
}))

// eslint-disable-next-line import/first
import ScheduleSlideover from '~/components/maintenance/ScheduleSlideover.vue'
// eslint-disable-next-line import/first
import RecordSlideover from '~/components/maintenance/RecordSlideover.vue'
// eslint-disable-next-line import/first
import AssetSearchPicker from '~/components/AssetSearchPicker.vue'

enableAutoUnmount(afterEach)
// Belt-and-suspenders: a fake-timers test that fails before reaching its own
// vi.useRealTimers() would otherwise leave every later test's setTimeout-based
// waits hanging forever.
afterEach(() => {
  vi.useRealTimers()
})

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const CATEGORIES: ReferenceRow[] = [
  { id: 'cat1', name: 'Inspeksi', is_active: true },
  { id: 'cat9', name: 'Penggantian Sparepart', is_active: true }
]
const VENDORS: ReferenceRow[] = [
  { id: 'v1', name: 'PT Sinar Komputindo', is_active: true }
]

function page<T>(data: T[]): Paginated<T> {
  return { data, total: data.length, limit: 100, offset: 0 }
}

const ASSET: Asset = {
  id: 'a1', asset_tag: 'JKT01-ELK-2025-00028', name: 'Genset Cummins C22 D5',
  category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible'
}

function schedule(over: Partial<MaintenanceSchedule> = {}): MaintenanceSchedule {
  return {
    id: 'sch1', asset_id: 'a1', maintenance_category_id: 'cat1', interval_months: 6,
    last_done_date: '2026-01-05', next_due_date: '2026-07-05', is_active: true,
    asset_name: 'Genset Cummins C22 D5', asset_tag: 'JKT01-ELK-2025-00028',
    office_name: 'Kantor Cabang Jakarta Selatan', category_name: 'Inspeksi',
    created_at: '2026-01-05T09:00:00Z', updated_at: '2026-01-05T09:00:00Z',
    ...over
  }
}

function record(over: Partial<MaintenanceRecord> = {}): MaintenanceRecord {
  return {
    id: 'rec1', asset_id: 'a1', schedule_id: null, maintenance_category_id: 'cat1',
    problem_category_id: null, type: 'preventive', status: 'in_progress',
    scheduled_date: '2026-06-18', completed_date: null, cost: '350000', vendor_id: 'v1',
    performed_by: null, description: 'Pembersihan filter', reported_by_id: null,
    asset_name: 'AC Daikin FTKC50', asset_tag: 'JKT01-ELK-2023-00009',
    office_name: 'Kantor Cabang Jakarta Selatan', category_name: 'Pembersihan',
    problem_name: null, vendor_name: 'PT Sinar Komputindo', reported_by_name: null,
    created_at: '2026-06-18T09:00:00Z', updated_at: '2026-06-18T09:00:00Z',
    ...over
  }
}

function bodyEl(testid: string): HTMLElement {
  const el = document.body.querySelector(`[data-testid="${testid}"]`)
  expect(el, `expected [data-testid="${testid}"] in document.body`).toBeTruthy()
  return el as HTMLElement
}

function bodyElExists(testid: string): boolean {
  return !!document.body.querySelector(`[data-testid="${testid}"]`)
}

type ScheduleVm = {
  form: {
    assetId: string
    assetName: string
    assetTag: string
    categoryId: string
    intervalMonths: string
    dateValue: string
    isActive: boolean
  }
  canSave: boolean
  onSubmit: () => Promise<void>
}

type RecordVm = {
  form: {
    assetId: string
    assetName: string
    assetTag: string
    type: string
    categoryId: string
    scheduledDate: string
    status: string
    cost: string
    vendorId: string
    description: string
    completedDate: string
  }
  canSave: boolean
  onSubmit: () => Promise<void>
  statusItems: { value: string, label: string }[]
}

async function settle() {
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 50))
  await flushPromises()
}

async function mountSchedule(schedule_: MaintenanceSchedule | null) {
  const wrapper = await mountSuspended(ScheduleSlideover, { props: { open: true, schedule: schedule_ } })
  await settle()
  return wrapper
}

async function mountRecord(record_: MaintenanceRecord | null, prefill: RecordPrefill | null = null) {
  const wrapper = await mountSuspended(RecordSlideover, { props: { open: true, record: record_, prefill } })
  await settle()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  refListMock.mockImplementation((key: string) =>
    Promise.resolve(page(key === 'maintenance-categories' ? CATEGORIES : key === 'vendors' ? VENDORS : [])))
  refGetMock.mockImplementation((key: string, id: string) => {
    const rows = key === 'maintenance-categories' ? CATEGORIES : key === 'vendors' ? VENDORS : []
    const row = rows.find(r => r.id === id)
    return row ? Promise.resolve(row) : Promise.reject(new Error('not found'))
  })
  assetsListMock.mockResolvedValue(page([]))
  createScheduleMock.mockResolvedValue(schedule())
  updateScheduleMock.mockResolvedValue(schedule())
  createRecordMock.mockResolvedValue(record())
  updateRecordMock.mockResolvedValue(record())
})

// ---------------------------------------------------------------------------
// ScheduleSlideover
// ---------------------------------------------------------------------------

describe('MaintenanceScheduleSlideover — create mode', () => {
  it('shows the searchable asset picker (not locked) and cannot save until asset + interval + date are set', async () => {
    const wrapper = await mountSchedule(null)
    const vm = wrapper.vm as unknown as ScheduleVm

    expect(bodyElExists('schedule-slideover-asset-picker')).toBe(true)
    expect(bodyElExists('schedule-slideover-locked-asset')).toBe(false)
    expect(vm.canSave).toBe(false)

    await vm.onSubmit()
    expect(createScheduleMock).not.toHaveBeenCalled()
  })

  it('submits createSchedule with {asset_id, interval_months, start_date} once valid, emits saved, and closes', async () => {
    const wrapper = await mountSchedule(null)
    const vm = wrapper.vm as unknown as ScheduleVm

    const picker = wrapper.findComponent(AssetSearchPicker)
    picker.vm.$emit('select', ASSET)
    await wrapper.vm.$nextTick()

    vm.form.intervalMonths = '6'
    vm.form.categoryId = 'cat1'
    vm.form.dateValue = '2026-08-01'
    await wrapper.vm.$nextTick()

    expect(vm.canSave).toBe(true)
    await vm.onSubmit()
    await settle()

    expect(createScheduleMock).toHaveBeenCalledWith({
      asset_id: 'a1',
      maintenance_category_id: 'cat1',
      interval_months: 6,
      start_date: '2026-08-01'
    })
    expect(wrapper.emitted('saved')).toBeTruthy()
    expect(wrapper.emitted('update:open')).toBeTruthy()
    expect(wrapper.emitted('update:open')![0]).toEqual([false])
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Jadwal maintenance dibuat.' }))
  })

  it('shows an error banner and keeps the slideover open when the API call rejects', async () => {
    createScheduleMock.mockRejectedValue(new Error('boom'))
    const wrapper = await mountSchedule(null)
    const vm = wrapper.vm as unknown as ScheduleVm

    const picker = wrapper.findComponent(AssetSearchPicker)
    picker.vm.$emit('select', ASSET)
    vm.form.intervalMonths = '3'
    vm.form.dateValue = '2026-08-01'
    await wrapper.vm.$nextTick()

    await vm.onSubmit()
    await settle()

    expect(bodyEl('schedule-slideover-error').textContent).toContain('Terjadi kesalahan')
    expect(wrapper.emitted('saved')).toBeFalsy()
    expect(wrapper.emitted('update:open')).toBeFalsy()
  })
})

describe('MaintenanceScheduleSlideover — edit mode', () => {
  it('locks the asset (shows name + tag, no picker) and hydrates the form from the schedule', async () => {
    const wrapper = await mountSchedule(schedule())
    const vm = wrapper.vm as unknown as ScheduleVm

    const locked = bodyEl('schedule-slideover-locked-asset')
    expect(locked.textContent).toContain('Genset Cummins C22 D5')
    expect(locked.textContent).toContain('JKT01-ELK-2025-00028')
    expect(bodyElExists('schedule-slideover-asset-picker')).toBe(false)

    expect(vm.form.intervalMonths).toBe('6')
    expect(vm.form.categoryId).toBe('cat1')
    // "Jatuh Tempo Berikut" is a read-only display of next_due_date, not editable.
    expect(bodyEl('schedule-slideover-date').getAttribute('disabled')).not.toBeNull()
  })

  it('submits updateSchedule with only category/interval/is_active — no asset_id or start_date', async () => {
    const wrapper = await mountSchedule(schedule())
    const vm = wrapper.vm as unknown as ScheduleVm

    vm.form.isActive = false
    vm.form.intervalMonths = '12'
    await wrapper.vm.$nextTick()

    await vm.onSubmit()
    await settle()

    expect(updateScheduleMock).toHaveBeenCalledWith('sch1', {
      maintenance_category_id: 'cat1',
      interval_months: 12,
      is_active: false
    })
    expect(wrapper.emitted('saved')).toBeTruthy()
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Jadwal maintenance diperbarui.' }))
  })
})

describe('MaintenanceScheduleSlideover — category is an AsyncSearchPicker', () => {
  it('renders the picker input instead of the old USelectMenu, driven by GET /maintenance-categories', async () => {
    await mountSchedule(null)
    expect(bodyElExists('schedule-slideover-category')).toBe(false)
    expect(bodyElExists('schedule-slideover-category-picker-input')).toBe(true)

    // The slideover's content is teleported (USlideover) — it lives in
    // document.body, outside `wrapper`'s own DOM subtree, so it must be
    // found and driven via document.body rather than wrapper.find().
    const input = bodyEl('schedule-slideover-category-picker-input') as HTMLInputElement
    vi.useFakeTimers()
    input.value = 'Inspeksi'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(refListMock).toHaveBeenCalledWith('maintenance-categories', { search: 'Inspeksi', limit: 20 })
  })

  it('resolves the hydrated maintenance_category_id to its label via GET /maintenance-categories/:id', async () => {
    await mountSchedule(schedule())
    const input = bodyEl('schedule-slideover-category-picker-input') as HTMLInputElement
    expect(input.value).toBe('Inspeksi')
  })
})

// ---------------------------------------------------------------------------
// RecordSlideover
// ---------------------------------------------------------------------------

describe('MaintenanceRecordSlideover — create mode, no prefill', () => {
  it('shows the asset picker (not locked) and cannot save until asset + date + description are set', async () => {
    const wrapper = await mountRecord(null, null)
    const vm = wrapper.vm as unknown as RecordVm

    expect(bodyElExists('record-slideover-asset-picker')).toBe(true)
    expect(bodyElExists('record-slideover-locked-asset')).toBe(false)
    expect(vm.canSave).toBe(false)

    await vm.onSubmit()
    expect(createRecordMock).not.toHaveBeenCalled()
  })
})

describe('MaintenanceRecordSlideover — create mode, prefilled from a schedule', () => {
  const PREFILL: RecordPrefill = {
    asset: { id: 'a9', name: 'Laptop Dell Latitude 5440', asset_tag: 'JKT01-ELK-2026-00099' },
    scheduleId: 'sch9',
    maintenanceCategoryId: 'cat9',
    type: 'corrective'
  }

  it('locks the asset and hydrates type/category from the prefill', async () => {
    const wrapper = await mountRecord(null, PREFILL)
    const vm = wrapper.vm as unknown as RecordVm

    const locked = bodyEl('record-slideover-locked-asset')
    expect(locked.textContent).toContain('Laptop Dell Latitude 5440')
    expect(locked.textContent).toContain('JKT01-ELK-2026-00099')
    expect(bodyElExists('record-slideover-asset-picker')).toBe(false)
    expect(vm.form.type).toBe('corrective')
    expect(vm.form.categoryId).toBe('cat9')
  })

  it('submits createRecord with asset_id + schedule_id from the prefill', async () => {
    const wrapper = await mountRecord(null, PREFILL)
    const vm = wrapper.vm as unknown as RecordVm

    vm.form.scheduledDate = '2026-07-10'
    vm.form.description = 'Ganti sparepart layar'
    await wrapper.vm.$nextTick()

    expect(vm.canSave).toBe(true)
    await vm.onSubmit()
    await settle()

    expect(createRecordMock).toHaveBeenCalledWith({
      asset_id: 'a9',
      schedule_id: 'sch9',
      maintenance_category_id: 'cat9',
      type: 'corrective',
      status: 'scheduled',
      scheduled_date: '2026-07-10',
      completed_date: null,
      cost: null,
      vendor_id: null,
      description: 'Ganti sparepart layar'
    })
    expect(wrapper.emitted('saved')).toBeTruthy()
    expect(wrapper.emitted('update:open')![0]).toEqual([false])
  })
})

describe('MaintenanceRecordSlideover — completed status reveals Tanggal Selesai', () => {
  it('defaults Tanggal Selesai to today and submits completed_date', async () => {
    const wrapper = await mountRecord(null, { asset: { id: 'a9', name: 'Laptop', asset_tag: 'TAG-1' } })
    const vm = wrapper.vm as unknown as RecordVm

    expect(bodyElExists('record-slideover-completed-date')).toBe(false)

    vm.form.scheduledDate = '2026-07-10'
    vm.form.description = 'Servis penuh'
    vm.form.status = 'completed'
    await wrapper.vm.$nextTick()

    const today = new Date().toISOString().slice(0, 10)
    expect(bodyElExists('record-slideover-completed-date')).toBe(true)
    expect(vm.form.completedDate).toBe(today)

    await vm.onSubmit()
    await settle()

    expect(createRecordMock).toHaveBeenCalledWith(expect.objectContaining({
      status: 'completed',
      completed_date: today
    }))
  })
})

describe('MaintenanceRecordSlideover — category/vendor are AsyncSearchPickers', () => {
  it('renders both picker inputs instead of the old USelectMenus', async () => {
    await mountRecord(null, { asset: { id: 'a9', name: 'Laptop', asset_tag: 'TAG-1' } })
    expect(bodyElExists('record-slideover-category')).toBe(false)
    expect(bodyElExists('record-slideover-vendor')).toBe(false)
    expect(bodyElExists('record-slideover-category-picker-input')).toBe(true)
    expect(bodyElExists('record-slideover-vendor-picker-input')).toBe(true)
  })

  it('searching the vendor picker drives GET /vendors with search+limit=20', async () => {
    await mountRecord(null, { asset: { id: 'a9', name: 'Laptop', asset_tag: 'TAG-1' } })
    const input = bodyEl('record-slideover-vendor-picker-input') as HTMLInputElement
    vi.useFakeTimers()
    input.value = 'Sinar'
    input.dispatchEvent(new Event('input'))
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(refListMock).toHaveBeenCalledWith('vendors', { search: 'Sinar', limit: 20 })
  })

  it('resolves the hydrated category and vendor ids to their labels via GET /:resource/:id', async () => {
    await mountRecord(record({ maintenance_category_id: 'cat1', vendor_id: 'v1' }))
    const categoryInput = bodyEl('record-slideover-category-picker-input') as HTMLInputElement
    const vendorInput = bodyEl('record-slideover-vendor-picker-input') as HTMLInputElement
    expect(categoryInput.value).toBe('Inspeksi')
    expect(vendorInput.value).toBe('PT Sinar Komputindo')
  })
})

describe('MaintenanceRecordSlideover — edit mode status transitions', () => {
  it('offers all four statuses when the current status is scheduled', async () => {
    const wrapper = await mountRecord(record({ status: 'scheduled' }))
    const vm = wrapper.vm as unknown as RecordVm

    expect(vm.statusItems.map(i => i.value)).toEqual(['scheduled', 'in_progress', 'completed', 'cancelled'])
  })

  it('only offers in_progress/completed/cancelled when the current status is in_progress', async () => {
    const wrapper = await mountRecord(record({ status: 'in_progress' }))
    const vm = wrapper.vm as unknown as RecordVm

    expect(vm.statusItems.map(i => i.value)).toEqual(['in_progress', 'completed', 'cancelled'])
    expect(vm.statusItems.map(i => i.value)).not.toContain('scheduled')
  })

  it('locks the asset in edit mode and does not include type/asset_id/schedule_id in the update payload', async () => {
    const wrapper = await mountRecord(record({ status: 'in_progress' }))
    const vm = wrapper.vm as unknown as RecordVm

    expect(bodyElExists('record-slideover-asset-picker')).toBe(false)
    const locked = bodyEl('record-slideover-locked-asset')
    expect(locked.textContent).toContain('AC Daikin FTKC50')

    vm.form.description = 'Catatan diperbarui'
    await wrapper.vm.$nextTick()
    await vm.onSubmit()
    await settle()

    expect(updateRecordMock).toHaveBeenCalledWith('rec1', {
      status: 'in_progress',
      maintenance_category_id: 'cat1',
      scheduled_date: '2026-06-18',
      completed_date: null,
      cost: '350000',
      vendor_id: 'v1',
      description: 'Catatan diperbarui'
    })
  })
})

describe('MaintenanceRecordSlideover — terminal records are read-only', () => {
  it.each(['completed', 'cancelled'] as const)('renders %s records read-only with no save button', async (status) => {
    const wrapper = await mountRecord(record({ status, completed_date: status === 'completed' ? '2026-06-20' : null }))
    const vm = wrapper.vm as unknown as RecordVm

    expect(vm.canSave).toBe(false)
    expect(bodyElExists('record-slideover-readonly-hint')).toBe(true)
    expect(document.body.textContent).not.toContain('Simpan Catatan')

    await vm.onSubmit()
    expect(updateRecordMock).not.toHaveBeenCalled()
  })
})

describe('MaintenanceRecordSlideover — API error', () => {
  it('shows an error banner and keeps the slideover open when updateRecord rejects', async () => {
    updateRecordMock.mockRejectedValue(new Error('boom'))
    const wrapper = await mountRecord(record({ status: 'in_progress' }))
    const vm = wrapper.vm as unknown as RecordVm

    await vm.onSubmit()
    await settle()

    expect(bodyEl('record-slideover-error').textContent).toContain('Terjadi kesalahan')
    expect(wrapper.emitted('saved')).toBeFalsy()
    expect(wrapper.emitted('update:open')).toBeFalsy()
  })
})
