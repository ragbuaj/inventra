// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Asset } from '~/types'
import type { AvailableAsset } from '~/composables/api/useAssignment'
import type { ApprovalRequestDetail } from '~/composables/api/useApproval'
import { useAuthStore } from '~/stores/auth'

// useToast's real toast portal isn't mounted here (no UApp wrapper) — mock
// and assert on call args, per the established convention in this codebase.
const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const AVAILABLE_ASSETS: AvailableAsset[] = [
  { id: 'as1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440' },
  { id: 'as2', asset_tag: 'JKT01-ELK-2026-00002', name: 'Proyektor Epson EB-X51' }
]

function myRequestRow(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'req1',
    type: 'assignment',
    status: 'pending',
    target_id: 'as1',
    target_entity: 'asset',
    created_at: '2026-07-06T09:00:00Z',
    decision_note: null,
    payload: { asset_id: 'as1', due_date: '2026-07-11', notes: 'Presentasi ke nasabah prioritas' },
    ...over
  }
}

function detail(over: Partial<ApprovalRequestDetail> = {}): ApprovalRequestDetail {
  return {
    id: 'req1', type: 'assignment', status: 'pending', current_step: 1,
    office_id: 'o1', office_name: 'Kantor Cabang Jakarta Selatan',
    target_id: 'as1', target_entity: 'asset', reason: 'Presentasi ke nasabah prioritas',
    requested_by_id: 'u1', requested_by_name: 'Andi Saputra', requested_by_role: 'Staf',
    decided_by_id: null, decision_note: null, created_at: '2026-07-06T09:00:00Z',
    payload: { asset_id: 'as1', due_date: '2026-07-11', notes: 'Presentasi ke nasabah prioritas' },
    steps: [
      { step_order: 1, required_level: 'manager', approver_id: null, approver_name: null, decision: 'pending', note: null, decided_at: null }
    ],
    ...over
  }
}

function asset(over: Partial<Asset> = {}): Asset {
  return {
    id: 'as1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440',
    category_id: 'c1', office_id: 'o1', status: 'available', asset_class: 'tangible',
    ...over
  } as Asset
}

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const availableMock = vi.fn()
const borrowMock = vi.fn()
const myRequestsMock = vi.fn()
const cancelMock = vi.fn()

vi.mock('~/composables/api/useAssignment', () => ({
  useAssignment: () => ({
    list: vi.fn(),
    available: availableMock,
    checkout: vi.fn(),
    checkin: vi.fn(),
    borrow: borrowMock,
    myRequests: myRequestsMock,
    cancel: cancelMock
  })
}))

const approvalGetMock = vi.fn()
vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({
    inbox: vi.fn(),
    list: vi.fn(),
    get: approvalGetMock,
    approve: vi.fn(),
    reject: vi.fn()
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

// eslint-disable-next-line import/first
import PeminjamanPage from '~/pages/peminjaman.vue'

enableAutoUnmount(afterEach)

function grantAdmin() {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Andi Saputra', email: 'andi@test.com', role_id: 'r1', role_name: 'Staf', office_id: 'o1' },
    ['*']
  )
}

async function mountAndWait() {
  grantAdmin()
  const wrapper = await mountSuspended(PeminjamanPage)
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 50))
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

// Row-actions kebab/context-menu items are portaled to document.body; locale
// here is 'id', so matching on the resolved Indonesian label text is reliable.
function menuItemByText(text: string): HTMLElement | undefined {
  return Array.from(document.querySelectorAll('[role="menuitem"]'))
    .find(el => el.textContent?.trim() === text) as HTMLElement | undefined
}

beforeEach(() => {
  availableMock.mockReset()
  borrowMock.mockReset()
  myRequestsMock.mockReset()
  cancelMock.mockReset()
  approvalGetMock.mockReset()
  assetsGetMock.mockReset()

  availableMock.mockResolvedValue({ data: AVAILABLE_ASSETS })
  myRequestsMock.mockResolvedValue({ data: [myRequestRow()], total: 1 })
  assetsGetMock.mockResolvedValue(asset())
  toastAddMock.mockReset()
})

describe('Peminjaman page — Ajukan Peminjaman card', () => {
  it('blocks submit when asset is not picked, even with Alasan filled', async () => {
    const wrapper = await mountAndWait()
    await wrapper.find('[data-testid="peminjaman-notes"]').setValue('Butuh untuk rapat')
    await wrapper.find('[data-testid="peminjaman-submit"]').trigger('click')
    await flushPromises()

    expect(borrowMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Aset wajib dipilih.')
  })

  it('blocks submit when Alasan is empty, even with an asset picked', async () => {
    const wrapper = await mountAndWait()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).assetId = 'as1'
    await wrapper.vm.$nextTick()
    await wrapper.find('[data-testid="peminjaman-submit"]').trigger('click')
    await flushPromises()

    expect(borrowMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Alasan wajib diisi.')
  })

  it('on success calls borrow, reloads myRequests + available, and shows a success toast', async () => {
    borrowMock.mockResolvedValueOnce({ request_id: 'req9', status: 'pending' })
    const wrapper = await mountAndWait()

    myRequestsMock.mockClear()
    availableMock.mockClear()

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(wrapper.vm as any).assetId = 'as1'
    await wrapper.vm.$nextTick()
    await wrapper.find('[data-testid="peminjaman-notes"]').setValue('Presentasi ke nasabah prioritas')
    await wrapper.find('[data-testid="peminjaman-submit"]').trigger('click')
    await flushPromises()

    expect(borrowMock).toHaveBeenCalledWith({ asset_id: 'as1', due_date: null, notes: 'Presentasi ke nasabah prioritas' })
    expect(myRequestsMock).toHaveBeenCalled()
    expect(availableMock).toHaveBeenCalled()
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Pengajuan peminjaman terkirim' }))
  })
})

describe('Peminjaman page — Pengajuan Peminjaman Saya list', () => {
  it('renders rows from myRequests', async () => {
    const wrapper = await mountAndWait()
    expect(myRequestsMock).toHaveBeenCalled()
    expect(wrapper.find('[data-testid="peminjaman-row-req1"]').exists()).toBe(true)
  })

  it('switches status tabs and re-queries with the mapped status', async () => {
    const wrapper = await mountAndWait()
    myRequestsMock.mockClear()

    await wrapper.find('[data-testid="peminjaman-filter-approved"]').trigger('click')
    await flushPromises()
    expect(myRequestsMock).toHaveBeenCalledWith({ status: 'approved' })

    myRequestsMock.mockClear()
    await wrapper.find('[data-testid="peminjaman-filter-rejected"]').trigger('click')
    await flushPromises()
    expect(myRequestsMock).toHaveBeenCalledWith({ status: 'rejected' })

    myRequestsMock.mockClear()
    await wrapper.find('[data-testid="peminjaman-filter-pending"]').trigger('click')
    await flushPromises()
    expect(myRequestsMock).toHaveBeenCalledWith({ status: 'pending' })

    myRequestsMock.mockClear()
    await wrapper.find('[data-testid="peminjaman-filter-all"]').trigger('click')
    await flushPromises()
    expect(myRequestsMock).toHaveBeenCalledWith({})
  })

  it('shows the correct badge tone/text for pending/approved/rejected', async () => {
    myRequestsMock.mockResolvedValueOnce({
      data: [
        myRequestRow({ id: 'r-pending', status: 'pending' }),
        myRequestRow({ id: 'r-approved', status: 'approved', decision_note: 'Disetujui, harap kembalikan tepat waktu.' }),
        myRequestRow({ id: 'r-rejected', status: 'rejected', decision_note: 'Aset dijadwalkan untuk unit lain.' })
      ],
      total: 3
    })
    const wrapper = await mountAndWait()

    expect(wrapper.find('[data-testid="peminjaman-status-r-pending"]').text()).toContain('Menunggu')
    expect(wrapper.find('[data-testid="peminjaman-status-r-approved"]').text()).toContain('Disetujui')
    expect(wrapper.find('[data-testid="peminjaman-status-r-rejected"]').text()).toContain('Ditolak')
    expect(wrapper.text()).toContain('Disetujui, harap kembalikan tepat waktu.')
  })

  it('shows a Batalkan kebab action only for pending rows; selecting it calls cancel then reloads', async () => {
    myRequestsMock.mockResolvedValueOnce({
      data: [
        myRequestRow({ id: 'r-pending', status: 'pending' }),
        myRequestRow({ id: 'r-approved', status: 'approved' })
      ],
      total: 2
    })
    const wrapper = await mountAndWait()

    const pendingRow = wrapper.find('[data-testid="peminjaman-row-r-pending"]')
    const approvedRow = wrapper.find('[data-testid="peminjaman-row-r-approved"]')
    expect(pendingRow.find('button[aria-haspopup="menu"]').exists()).toBe(true)
    expect(approvedRow.find('button[aria-haspopup="menu"]').exists()).toBe(false)

    cancelMock.mockResolvedValueOnce({})
    myRequestsMock.mockClear()
    await pendingRow.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))
    menuItemByText('Batalkan')!.click()
    await flushPromises()

    expect(cancelMock).toHaveBeenCalledWith('r-pending')
    expect(myRequestsMock).toHaveBeenCalled()
  })

  it('opening the kebab / selecting Batalkan does not toggle the row timeline', async () => {
    myRequestsMock.mockResolvedValueOnce({ data: [myRequestRow({ id: 'r-pending', status: 'pending' })], total: 1 })
    approvalGetMock.mockResolvedValue(detail())
    const wrapper = await mountAndWait()

    const pendingRow = wrapper.find('[data-testid="peminjaman-row-r-pending"]')
    await pendingRow.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(resolve => setTimeout(resolve, 0))

    // Opening the kebab (a click inside the actions cell, which stops
    // propagation) must not have expanded the timeline row.
    expect(wrapper.find('[data-testid="peminjaman-timeline-r-pending"]').exists()).toBe(false)
    expect(approvalGetMock).not.toHaveBeenCalled()

    cancelMock.mockResolvedValueOnce({})
    menuItemByText('Batalkan')!.click()
    await flushPromises()

    expect(cancelMock).toHaveBeenCalledWith('r-pending')
    expect(wrapper.find('[data-testid="peminjaman-timeline-r-pending"]').exists()).toBe(false)
  })

  it('the row itself (outside the actions cell) still toggles the timeline on click', async () => {
    approvalGetMock.mockResolvedValueOnce(detail())
    const wrapper = await mountAndWait()

    await wrapper.find('[data-testid="peminjaman-row-req1"]').trigger('click')
    await flushPromises()

    expect(approvalGetMock).toHaveBeenCalledWith('req1')
    expect(wrapper.find('[data-testid="peminjaman-timeline-req1"]').exists()).toBe(true)
  })

  it('right-clicking a pending row surfaces Batalkan in the context menu', async () => {
    myRequestsMock.mockResolvedValueOnce({ data: [myRequestRow({ id: 'r-pending', status: 'pending' })], total: 1 })
    const wrapper = await mountAndWait()

    const pendingRow = wrapper.find('[data-testid="peminjaman-row-r-pending"]').element
    pendingRow.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(menuItemByText('Batalkan')).toBeTruthy()
  })

  it('right-clicking a non-row area (or the expanded timeline row) after right-clicking a pending row shows no stale context menu', async () => {
    myRequestsMock.mockResolvedValueOnce({ data: [myRequestRow({ id: 'r-pending', status: 'pending' })], total: 1 })
    approvalGetMock.mockResolvedValueOnce(detail())
    const wrapper = await mountAndWait()

    const pendingRow = wrapper.find('[data-testid="peminjaman-row-r-pending"]').element
    pendingRow.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(menuItemByText('Batalkan')).toBeTruthy()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    // Expand the row's timeline, then right-click inside it — the timeline
    // row is a `tbody tr` too, but not a request row, so this must not
    // resurface the pending row's stale "Batalkan" item.
    await wrapper.find('[data-testid="peminjaman-row-r-pending"]').trigger('click')
    await flushPromises()
    const timeline = wrapper.find('[data-testid="peminjaman-timeline-r-pending"]').element
    timeline.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true }))
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(document.querySelectorAll('[role="menuitem"]').length).toBe(0)
  })

  it('shows the empty state when myRequests returns []', async () => {
    myRequestsMock.mockResolvedValueOnce({ data: [], total: 0 })
    const wrapper = await mountAndWait()
    expect(wrapper.text()).toContain('Belum ada pengajuan peminjaman')
  })

  it('shows a loading skeleton before myRequests resolves', async () => {
    let resolve: (v: unknown) => void = () => {}
    myRequestsMock.mockReturnValueOnce(new Promise((r) => {
      resolve = r
    }))
    grantAdmin()
    const wrapper = await mountSuspended(PeminjamanPage)
    await wrapper.vm.$nextTick()

    expect(wrapper.findComponent({ name: 'USkeleton' }).exists() || wrapper.html().includes('animate-pulse')).toBeTruthy()

    resolve({ data: [], total: 0 })
    await flushPromises()
  })

  it('shows an error state + retry when myRequests rejects', async () => {
    myRequestsMock.mockReset()
    myRequestsMock.mockRejectedValueOnce(new Error('network'))
    const wrapper = await mountAndWait()

    expect(wrapper.find('[data-testid="peminjaman-load-error"]').exists()).toBe(true)

    myRequestsMock.mockResolvedValueOnce({ data: [myRequestRow()], total: 1 })
    await wrapper.find('[data-testid="peminjaman-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="peminjaman-load-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="peminjaman-row-req1"]').exists()).toBe(true)
  })

  it('row expand fetches the approval detail and renders the timeline', async () => {
    approvalGetMock.mockResolvedValueOnce(detail({
      status: 'approved',
      decision_note: 'Disetujui, harap kembalikan tepat waktu.',
      steps: [
        { step_order: 1, required_level: 'manager', approver_id: 'u2', approver_name: 'Rina Putri', decision: 'approved', note: 'Disetujui, harap kembalikan tepat waktu.', decided_at: '2026-07-02T09:14:00Z' }
      ]
    }))
    const wrapper = await mountAndWait()

    await wrapper.find('[data-testid="peminjaman-row-req1"]').trigger('click')
    await flushPromises()

    expect(approvalGetMock).toHaveBeenCalledWith('req1')
    const timeline = wrapper.find('[data-testid="peminjaman-timeline-req1"]')
    expect(timeline.exists()).toBe(true)
    expect(timeline.text()).toContain('Rina Putri')
    expect(timeline.text()).toContain('Disetujui, harap kembalikan tepat waktu.')
  })

  it('collapses the row again on a second click without re-fetching', async () => {
    approvalGetMock.mockResolvedValueOnce(detail())
    const wrapper = await mountAndWait()

    await wrapper.find('[data-testid="peminjaman-row-req1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="peminjaman-timeline-req1"]').exists()).toBe(true)

    await wrapper.find('[data-testid="peminjaman-row-req1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="peminjaman-timeline-req1"]').exists()).toBe(false)

    approvalGetMock.mockClear()
    await wrapper.find('[data-testid="peminjaman-row-req1"]').trigger('click')
    await flushPromises()
    expect(approvalGetMock).not.toHaveBeenCalled()
  })

  it('prepends a synthesized "Diajukan oleh saya" entry built from the detail requester/date, before the approval steps', async () => {
    approvalGetMock.mockResolvedValueOnce(detail({
      requested_by_name: 'Andi Saputra',
      requested_by_role: 'Staf',
      created_at: '2026-07-06T09:00:00Z',
      status: 'approved',
      steps: [
        { step_order: 1, required_level: 'manager', approver_id: 'u2', approver_name: 'Rina Putri', decision: 'approved', note: null, decided_at: '2026-07-02T09:14:00Z' }
      ]
    }))
    const wrapper = await mountAndWait()

    await wrapper.find('[data-testid="peminjaman-row-req1"]').trigger('click')
    await flushPromises()

    const submittedEntry = wrapper.find('[data-testid="peminjaman-timeline-submitted-req1"]')
    expect(submittedEntry.exists()).toBe(true)
    expect(submittedEntry.text()).toContain('Diajukan oleh saya')
    expect(submittedEntry.text()).toContain('Andi Saputra (Staf)')
    expect(submittedEntry.text()).toContain('6 Jul 2026')

    // The synthesized entry must appear before the real approval step in DOM order.
    const timeline = wrapper.find('[data-testid="peminjaman-timeline-req1"]')
    const submittedIdx = timeline.html().indexOf('Diajukan oleh saya')
    const stepIdx = timeline.html().indexOf('Rina Putri')
    expect(submittedIdx).toBeGreaterThanOrEqual(0)
    expect(stepIdx).toBeGreaterThan(submittedIdx)
  })

  it('shows the resolved asset name after the async lookup settles', async () => {
    assetsGetMock.mockResolvedValueOnce(asset({ id: 'as1', name: 'Laptop Dell Latitude 5440', asset_tag: 'JKT01-ELK-2026-00001' }))
    const wrapper = await mountAndWait()

    expect(assetsGetMock).toHaveBeenCalledWith('as1')
    const row = wrapper.find('[data-testid="peminjaman-row-req1"]')
    expect(row.text()).toContain('Laptop Dell Latitude 5440')
  })

  it('falls back to the id/tag without crashing when the asset lookup rejects (e.g. 403 out of scope)', async () => {
    assetsGetMock.mockReset()
    assetsGetMock.mockRejectedValueOnce(new Error('403 Forbidden'))
    const wrapper = await mountAndWait()

    expect(assetsGetMock).toHaveBeenCalledWith('as1')
    const row = wrapper.find('[data-testid="peminjaman-row-req1"]')
    // Falls back to showing the raw asset id (target_id) — no name resolved, no thrown error.
    expect(row.text()).toContain('as1')
    expect(row.text()).not.toContain('Laptop Dell Latitude 5440')
  })
})
