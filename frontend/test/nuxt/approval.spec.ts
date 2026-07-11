// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import type { ApprovalRequestRow, ApprovalRequestDetail } from '~/composables/api/useApproval'

const row = (over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow => ({
  id: 'r1', type: 'asset_create', status: 'pending', amount: '1500000',
  current_step: 1, office_id: 'o1', office_name: 'Cabang Alpha',
  target_id: null, target_entity: null, reason: 'pengadaan',
  requested_by_id: 'u1', requested_by_name: 'Andi Saputra', requested_by_role: 'Kepala Unit',
  decided_by_id: null, decision_note: null, created_at: '2026-07-04T09:00:00Z',
  ...over
})
const detail = (over: Partial<ApprovalRequestDetail> = {}): ApprovalRequestDetail => ({
  ...row(), payload: { name: 'Laptop A', purchase_cost: '1500000' },
  steps: [{ step_order: 1, required_level: 'office', approver_id: null, approver_name: null, decision: 'pending', note: null, decided_at: null }],
  ...over
})

const inboxMock = vi.fn()
const listMock = vi.fn()
const getMock = vi.fn()
const approveMock = vi.fn()
const rejectMock = vi.fn()

vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({ inbox: inboxMock, list: listMock, get: getMock, approve: approveMock, reject: rejectMock })
}))
// useCategories()/useOffices() lookups both go through useApiClient — stub it to avoid network.
type RequestHandler = (path: string, opts?: Record<string, unknown>) => unknown

let _blobHandler: RequestHandler = () => new Blob(['x'], { type: 'image/jpeg' })

function setBlobHandler(fn: RequestHandler) {
  _blobHandler = fn
}

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: vi.fn().mockResolvedValue({ data: [] }),
    requestBlob: (path: string, opts?: Record<string, unknown>) => {
      const res = _blobHandler(path, opts)
      return res instanceof Promise ? res : Promise.resolve(res)
    },
    refreshToken: vi.fn()
  })
}))

// eslint-disable-next-line import/first
import ApprovalPage from '~/pages/approval.vue'

beforeEach(() => {
  vi.clearAllMocks()
  inboxMock.mockResolvedValue([row()])
  listMock.mockResolvedValue({ data: [], total: 0, limit: 100, offset: 0 })
  getMock.mockResolvedValue(detail())
  approveMock.mockResolvedValue(row({ status: 'approved' }))
  rejectMock.mockResolvedValue(row({ status: 'rejected' }))
  // Reset blob handler to default (pass-through)
  setBlobHandler(() => new Blob(['x'], { type: 'image/jpeg' }))
})

describe('pages/approval — wired', () => {
  it('loads the inbox on mount and renders a request card', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    expect(inboxMock).toHaveBeenCalled()
    expect(w.text()).toContain('Andi Saputra')
    expect(w.text()).toContain('Cabang Alpha')
  })

  it('shows the empty state when the inbox is empty', async () => {
    inboxMock.mockResolvedValue([])
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    expect(w.text()).toContain('Tidak ada pengajuan')
  })

  it('shows the load-error state with retry when the inbox call fails', async () => {
    inboxMock.mockRejectedValue(new Error('boom'))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    expect(w.find('[data-testid="approval-load-error"]').exists()).toBe(true)
  })

  it('switching to the approved tab queries the list endpoint with status', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-approved"]').trigger('click')
    await flushPromises()
    expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'approved' }))
  })

  it('has a cancelled tab that queries status=cancelled', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-cancelled"]').trigger('click')
    await flushPromises()
    expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'cancelled' }))
  })

  it('selecting a card fetches the detail and renders payload data + timeline', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(getMock).toHaveBeenCalledWith('r1')
    expect(w.text()).toContain('Laptop A')
    expect(w.text()).toContain('Mengajukan permintaan')
  })

  it('approve sends the note and refreshes', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-note"]').setValue('ok!')
    await w.find('[data-testid="approval-approve"]').trigger('click')
    await flushPromises()
    expect(approveMock).toHaveBeenCalledWith('r1', 'ok!')
    expect(inboxMock.mock.calls.length).toBeGreaterThanOrEqual(2)
  })

  it('reject sends the note', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-reject"]').trigger('click')
    await flushPromises()
    expect(rejectMock).toHaveBeenCalledWith('r1', undefined)
  })

  it('a pending request NOT in the inbox shows the not-eligible lock instead of buttons', async () => {
    inboxMock.mockResolvedValue([])
    listMock.mockResolvedValue({ data: [row()], total: 1, limit: 100, offset: 0 })
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-all"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="approval-not-eligible"]').exists()).toBe(true)
    expect(w.find('[data-testid="approval-approve"]').exists()).toBe(false)
  })

  it('a cancelled request renders the neutral result banner', async () => {
    inboxMock.mockResolvedValue([])
    listMock.mockResolvedValue({ data: [row({ status: 'cancelled' })], total: 1, limit: 100, offset: 0 })
    getMock.mockResolvedValue(detail({ status: 'cancelled' }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-cancelled"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Dibatalkan oleh pengaju')
  })

  it('lampiran section always renders the permanent empty state', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tidak ada lampiran')
  })

  it('a sensitive-type request shows the sensitive warning banner in detail', async () => {
    inboxMock.mockResolvedValue([row({ id: 'r2', type: 'asset_disposal' })])
    getMock.mockResolvedValue(detail({ id: 'r2', type: 'asset_disposal', payload: { method: 'lelang' } }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tindakan sensitif')
  })

  it('an approved request shows the result banner with the actor and hides the decision buttons', async () => {
    inboxMock.mockResolvedValue([])
    listMock.mockResolvedValue({ data: [row({ id: 'r9', status: 'approved' })], total: 1, limit: 100, offset: 0 })
    getMock.mockResolvedValue(detail({
      id: 'r9',
      status: 'approved',
      steps: [{ step_order: 1, required_level: 'office', approver_id: 'u2', approver_name: 'Rudi Hartono', decision: 'approved', note: null, decided_at: '2026-07-02T08:00:00Z' }]
    }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-approved"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Rudi Hartono')
    expect(w.find('[data-testid="approval-approve"]').exists()).toBe(false)
    expect(w.find('[data-testid="approval-reject"]').exists()).toBe(false)
  })

  it('switching tabs clears the selection and shows the placeholder again', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Laptop A')
    await w.find('[data-testid="approval-tab-approved"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tidak ada pengajuan dipilih')
  })
})

// ---------------------------------------------------------------------------
// Maintenance payload rendering (Task 12)
// ---------------------------------------------------------------------------

describe('pages/approval — maintenance payload', () => {
  it('renders the asset (fallback to raw id), problem category (fallback to raw id) and description', async () => {
    inboxMock.mockResolvedValue([row({ id: 'm1', type: 'maintenance' })])
    getMock.mockResolvedValue(detail({
      id: 'm1',
      type: 'maintenance',
      payload: { asset_id: 'asset-xyz', problem_category_id: 'pc-1', description: 'Layar retak setelah jatuh' }
    }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()

    const text = w.text()
    expect(text).toContain('asset-xyz')
    expect(text).toContain('pc-1')
    expect(text).toContain('Layar retak setelah jatuh')
    expect(text).toContain('Laporan Kerusakan') // approval.type.maintenance label
  })

  it('shows a "Lihat Lampiran" button when the payload has an attachment_id, instead of "Tidak ada lampiran"', async () => {
    inboxMock.mockResolvedValue([row({ id: 'm2', type: 'maintenance' })])
    getMock.mockResolvedValue(detail({
      id: 'm2',
      type: 'maintenance',
      payload: { asset_id: 'asset-1', problem_category_id: 'pc-1', description: 'x', attachment_id: 'att-9' }
    }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()

    expect(w.find('[data-testid="approval-view-attachment"]').exists()).toBe(true)
    expect(w.text()).not.toContain('Tidak ada lampiran')
  })

  it('clicking "Lihat Lampiran" fetches the attachment content and opens it in a new tab', async () => {
    inboxMock.mockResolvedValue([row({ id: 'm3', type: 'maintenance' })])
    getMock.mockResolvedValue(detail({
      id: 'm3',
      type: 'maintenance',
      payload: { asset_id: 'asset-1', problem_category_id: 'pc-1', description: 'x', attachment_id: 'att-9' }
    }))

    // Stub requestBlob to verify exact path and reject any other path
    const expectedPath = '/assets/asset-1/attachments/att-9/content'
    setBlobHandler((path) => {
      if (path !== expectedPath) {
        throw new Error(`Expected requestBlob path "${expectedPath}" but got "${path}"`)
      }
      return new Blob(['mock-attachment-content'], { type: 'application/pdf' })
    })

    URL.createObjectURL = vi.fn(() => 'blob:mock-attachment')
    const openMock = vi.fn()
    window.open = openMock

    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-view-attachment"]').trigger('click')
    await flushPromises()

    expect(openMock).toHaveBeenCalledWith('blob:mock-attachment', '_blank')
  })

  it('a maintenance request with no attachment_id shows the permanent empty state', async () => {
    inboxMock.mockResolvedValue([row({ id: 'm4', type: 'maintenance' })])
    getMock.mockResolvedValue(detail({
      id: 'm4',
      type: 'maintenance',
      payload: { asset_id: 'asset-1', problem_category_id: 'pc-1', description: 'x' }
    }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()

    expect(w.find('[data-testid="approval-view-attachment"]').exists()).toBe(false)
    expect(w.text()).toContain('Tidak ada lampiran')
  })
})
