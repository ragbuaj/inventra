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
    // 'assignment' is a valid backend shared.request_type but the frontend's
    // RequestType union (approvalMeta.ts) hasn't been extended to include it yet.
    id: 'req1', type: 'assignment' as ApprovalRequestDetail['type'], status: 'pending', current_step: 1,
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

  it('shows Batalkan only for pending rows; clicking calls cancel then reloads', async () => {
    myRequestsMock.mockResolvedValueOnce({
      data: [
        myRequestRow({ id: 'r-pending', status: 'pending' }),
        myRequestRow({ id: 'r-approved', status: 'approved' })
      ],
      total: 2
    })
    const wrapper = await mountAndWait()

    expect(wrapper.find('[data-testid="peminjaman-cancel-r-pending"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="peminjaman-cancel-r-approved"]').exists()).toBe(false)

    cancelMock.mockResolvedValueOnce({})
    myRequestsMock.mockClear()
    await wrapper.find('[data-testid="peminjaman-cancel-r-pending"]').trigger('click')
    await flushPromises()

    expect(cancelMock).toHaveBeenCalledWith('r-pending')
    expect(myRequestsMock).toHaveBeenCalled()
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
})
