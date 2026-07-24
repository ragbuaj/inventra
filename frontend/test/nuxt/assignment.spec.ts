// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Paginated, Employee } from '~/types'
import type { Assignment, AvailableAsset } from '~/composables/api/useAssignment'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

function employee(over: Partial<Employee> = {}): Employee {
  return {
    id: 'e1', code: 'EMP001', name: 'Andi Saputra', email: 'andi@test.com', phone: null,
    department_id: null, position_id: null, office_id: 'o-mine', status: 'active',
    // Legacy-parity Fase 6 fields.
    company_id: null, executor_division_id: null,
    created_at: null, updated_at: null,
    ...over
  }
}

const EMPLOYEES: Employee[] = [
  employee({ id: 'e1', code: 'EMP001', name: 'Andi Saputra' }),
  employee({ id: 'e2', code: 'EMP002', name: 'Rina Putri' })
]

const AVAILABLE_ASSETS: AvailableAsset[] = [
  { id: 'as1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440' },
  { id: 'as2', asset_tag: 'JKT01-ITX-2025-00014', name: 'Router MikroTik RB4011' }
]

function assignment(over: Partial<Assignment> = {}): Assignment {
  return {
    id: 'a1', asset_id: 'as3', employee_id: 'e1', assigned_by_id: 'u1',
    checkout_date: '2026-01-20', due_date: null, checkin_date: null,
    condition_out: 'baik', condition_in: null, status: 'active', notes: null,
    asset_name: 'Proyektor Epson EB-X51', asset_tag: 'JKT01-ELK-2026-00002',
    employee_name: 'Andi Saputra', assigned_by_name: 'Manager Cabang', office_name: 'Kantor Cabang Jakarta Selatan',
    created_at: '2026-01-20T09:00:00Z', updated_at: '2026-01-20T09:00:00Z',
    ...over
  }
}

const ASSIGNMENTS: Assignment[] = [
  assignment({ id: 'a1', status: 'active', asset_name: 'Proyektor Epson EB-X51', asset_tag: 'JKT01-ELK-2026-00002', employee_name: 'Andi Saputra', checkout_date: '2026-01-20' }),
  assignment({
    id: 'r1', status: 'returned', asset_name: 'Televisi Samsung 55" Crystal', asset_tag: 'JKT01-ELK-2024-00030',
    employee_name: 'Dewi Lestari', checkout_date: '2025-12-03', checkin_date: '2025-12-18', condition_in: 'baik'
  })
]

function page<T>(data: T[]): Paginated<T> {
  return { data, total: data.length, limit: 100, offset: 0 }
}

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const assignmentListMock = vi.fn()
const assignmentAvailableMock = vi.fn()
const assignmentCheckoutMock = vi.fn()
const assignmentCheckinMock = vi.fn()

vi.mock('~/composables/api/useAssignment', () => ({
  useAssignment: () => ({
    list: assignmentListMock,
    available: assignmentAvailableMock,
    checkout: assignmentCheckoutMock,
    checkin: assignmentCheckinMock,
    borrow: vi.fn(),
    myRequests: vi.fn(),
    cancel: vi.fn()
  })
}))

const employeesListMock = vi.fn()
const employeesGetMock = vi.fn()
vi.mock('~/composables/api/useEmployees', () => ({
  useEmployees: () => ({
    list: employeesListMock,
    get: employeesGetMock,
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn()
  })
}))

// eslint-disable-next-line import/first
import AssignmentPage from '~/pages/assignment.vue'

enableAutoUnmount(afterEach)
afterEach(() => {
  vi.useRealTimers()
})

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
}

async function mountAndWait() {
  const wrapper = await mountSuspended(AssignmentPage, { route: '/assignment' })
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

function clickTab(wrapper: Wrapper, label: string) {
  const btn = wrapper.findAll('button').find(b => b.text().includes(label))
  return btn!.trigger('click')
}

beforeEach(() => {
  vi.clearAllMocks()
  employeesListMock.mockResolvedValue(page(EMPLOYEES))
  employeesGetMock.mockImplementation((id: string) => {
    const e = EMPLOYEES.find(emp => emp.id === id)
    return e ? Promise.resolve(e) : Promise.reject(new Error('not found'))
  })
  assignmentListMock.mockResolvedValue(page(ASSIGNMENTS))
  assignmentAvailableMock.mockResolvedValue({ data: AVAILABLE_ASSETS })
  assignmentCheckoutMock.mockResolvedValue(assignment({ id: 'new1', status: 'active' }))
  assignmentCheckinMock.mockResolvedValue(assignment({ id: 'a1', status: 'returned' }))
  grantAdmin()
})

// ---------------------------------------------------------------------------

describe('Assignment page — mount', () => {
  it('renders the 3 tabs with Check-out as the default tab', async () => {
    const w = await mountAndWait()
    const text = w.text()
    expect(text).toContain('Penugasan Aset')
    expect(text).toContain('Check-out')
    expect(text).toContain('Check-in')
    expect(text).toContain('Riwayat')
    // Default tab content: check-out form fields visible.
    expect(text).toContain('Pegawai Penerima')
    expect(text).toContain('Aset (hanya yang tersedia)')
  })

  it('loads assignment list/available on mount without an eager employees list', async () => {
    await mountAndWait()
    // The recipient field is now an async search picker — no more eager
    // `{ limit: 100 }` employees fetch on mount.
    expect(employeesListMock).not.toHaveBeenCalled()
    expect(assignmentListMock).toHaveBeenCalled()
    expect(assignmentAvailableMock).toHaveBeenCalled()
  })

  it('shows the load-error state with retry when list() rejects, and retry re-calls it', async () => {
    assignmentListMock.mockRejectedValueOnce(new Error('boom'))
    const w = await mountAndWait()
    expect(w.text()).toContain('Gagal memuat data.')

    assignmentListMock.mockResolvedValueOnce(page(ASSIGNMENTS))
    const retryBtn = w.findAll('button').find(b => b.text().includes('Coba lagi'))
    await retryBtn!.trigger('click')
    await flushPromises()

    expect(assignmentListMock).toHaveBeenCalledTimes(2)
    expect(w.text()).not.toContain('Gagal memuat data.')
    expect(w.text()).toContain('Check-out')
  })
})

describe('Assignment page — Check-out tab: recipient is an AsyncSearchPicker', () => {
  it('renders the recipient picker input (no more eager-options USelect)', async () => {
    const w = await mountAndWait()
    expect(w.find('[data-testid="employee-picker-input"]').exists()).toBe(true)
  })

  it('typing drives useEmployees().list with { search, limit: 20 }', async () => {
    const w = await mountAndWait()
    vi.useFakeTimers()
    await w.find('[data-testid="employee-picker-input"]').setValue('Rina')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    vi.useRealTimers()
    expect(employeesListMock).toHaveBeenCalledWith({ search: 'Rina', limit: 20 })
  })

  it('resolves a preselected employee id to its label via useEmployees().get', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'coEmployeeId', 'e1')
    const input = w.find('[data-testid="employee-picker-input"]').element as HTMLInputElement
    expect(input.value).toBe('Andi Saputra')
  })
})

describe('Assignment page — Check-out tab', () => {
  it('disables submit until asset + employee + date are set, then enables it', async () => {
    const w = await mountAndWait()
    // Locate the submit button precisely via its label text (exact match, not the tab).
    const submitBtn = () => w.findAll('button').filter(b => b.text().trim() === 'Check-out').at(-1)!
    expect(submitBtn().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'coAssetId', 'as1')
    expect(submitBtn().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'coEmployeeId', 'e1')
    expect(submitBtn().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'coTgl', '2026-07-08')
    expect(submitBtn().attributes('disabled')).toBeUndefined()
  })

  it('lists assets from available() in the asset picker', async () => {
    const w = await mountAndWait()
    expect((w.vm as unknown as { availableAssets: Array<{ label: string, value: string }> }).availableAssets).toEqual([
      { label: 'Laptop Dell Latitude 5440 · JKT01-ELK-2026-00001', value: 'as1' },
      { label: 'Router MikroTik RB4011 · JKT01-ITX-2025-00014', value: 'as2' }
    ])
  })

  it('calls checkout with the chosen asset_id/employee_id on submit', async () => {
    const w = await mountAndWait()
    await setVmRef(w, 'coAssetId', 'as1')
    await setVmRef(w, 'coEmployeeId', 'e2')
    await setVmRef(w, 'coTgl', '2026-07-08')

    const submitBtn = w.findAll('button').filter(b => b.text().trim() === 'Check-out').at(-1)!
    await submitBtn.trigger('click')
    await flushPromises()

    expect(assignmentCheckoutMock).toHaveBeenCalledWith({
      asset_id: 'as1',
      employee_id: 'e2',
      checkout_date: '2026-07-08',
      condition_out: 'baik',
      notes: null
    })
    expect(w.text()).toContain('berhasil di-check-out')
  })
})

describe('Assignment page — Check-in tab', () => {
  it('shows the empty state when there are no active assignments', async () => {
    assignmentListMock.mockResolvedValue(page(ASSIGNMENTS.filter(a => a.status === 'returned')))
    const w = await mountAndWait()
    await clickTab(w, 'Check-in')
    expect(w.text()).toContain('Tidak ada penugasan aktif')
  })

  it('enables submit once an active assignment + return date are chosen, and needs_maintenance toggles', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Check-in')
    expect(w.text()).not.toContain('Tidak ada penugasan aktif')
    expect(w.text()).toContain('Perlu maintenance')

    const submitBtn = () => w.findAll('button').filter(b => b.text().trim() === 'Check-in').at(-1)!
    expect(submitBtn().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'ciId', 'a1')
    expect(submitBtn().attributes('disabled')).toBeDefined()

    await setVmRef(w, 'ciTgl', '2026-07-08')
    expect(submitBtn().attributes('disabled')).toBeUndefined()

    await setVmRef(w, 'ciMaint', true)
    await submitBtn().trigger('click')
    await flushPromises()

    expect(assignmentCheckinMock).toHaveBeenCalledWith('a1', {
      checkin_date: '2026-07-08',
      condition_in: 'baik',
      needs_maintenance: true
    })
  })
})

describe('Assignment page — Riwayat tab', () => {
  it('renders rows with resolved asset_name/employee_name and status/condition text', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Riwayat')
    const text = w.text()
    expect(text).toContain('Proyektor Epson EB-X51')
    expect(text).toContain('Andi Saputra')
    expect(text).toContain('Televisi Samsung 55" Crystal')
    expect(text).toContain('Dewi Lestari')
    expect(text).toContain('Aktif')
    expect(text).toContain('Dikembalikan')
    expect(text).toContain('Baik')
    expect(text).toContain('Total 2 penugasan')
  })

  it('narrows rows via the search term', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Riwayat')
    const search = w.find('input[type="text"]')
    await search.setValue('Televisi')
    await w.vm.$nextTick()
    const text = w.text()
    expect(text).toContain('Televisi Samsung 55" Crystal')
    expect(text).not.toContain('Proyektor Epson EB-X51')
    expect(text).toContain('Total 1 penugasan')
  })

  it('narrows rows via the status filter', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Riwayat')
    await setVmRef(w, 'hStatus', 'returned')
    const text = w.text()
    expect(text).toContain('Televisi Samsung 55" Crystal')
    expect(text).not.toContain('Proyektor Epson EB-X51')
    expect(text).toContain('Total 1 penugasan')
  })

  it('shows the empty state when no rows match', async () => {
    const w = await mountAndWait()
    await clickTab(w, 'Riwayat')
    const search = w.find('input[type="text"]')
    await search.setValue('tidak-ada-yang-cocok')
    await w.vm.$nextTick()
    expect(w.text()).toContain('Tidak ada riwayat')
  })
})
