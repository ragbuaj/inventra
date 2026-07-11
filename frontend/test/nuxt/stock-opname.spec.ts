// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { Office, ReferenceRow, Paginated, Floor, Room } from '~/types'
import type { OpnameSession, OpnameSessionDetail, OpnameItem } from '~/composables/api/useStockOpname'
import { useAuthStore } from '~/stores/auth'

// useToast's real toast portal isn't mounted in these component tests (no
// UApp wrapper), so success/error toast text never lands in the DOM. Mock it
// and assert on the call args instead (mirrors assets-form.spec.ts).
const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const OFFICES: Office[] = [
  { id: 'o-mine', parent_id: null, office_type_id: 'ot-cabang', province_id: null, city_id: null, name: 'Kantor Cabang Jakarta Selatan', code: 'JKS', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null },
  { id: 'o-other', parent_id: null, office_type_id: 'ot-cabang', province_id: null, city_id: null, name: 'Kantor Cabang Bandung', code: 'BDG', address: null, is_active: true, latitude: null, longitude: null, created_at: null, updated_at: null }
]

function session(over: Partial<OpnameSession> = {}): OpnameSession {
  return {
    id: 's1', office_id: 'o-mine', name: 'Opname Semester I 2026', period: '2026-06', status: 'counting',
    started_by_id: 'u1', started_at: '2026-06-01T09:00:00Z', closed_by_id: null, closed_at: null,
    created_at: '2026-06-01T09:00:00Z', updated_at: '2026-06-01T09:00:00Z',
    office_name: 'Kantor Cabang Jakarta Selatan', started_by_name: 'Dewi Lestari', closed_by_name: null,
    ...over
  }
}

function detail(over: Partial<OpnameSessionDetail> = {}): OpnameSessionDetail {
  return { ...session(), total: 8, found: 5, pending: 1, variance: 2, ...over }
}

function item(over: Partial<OpnameItem> = {}): OpnameItem {
  return {
    id: 'i1', session_id: 's1', asset_id: 'a1', asset_name: 'Laptop Dell Latitude 5440', asset_tag: 'JKT01-ELK-2026-00001',
    office_name: 'Kantor Cabang Jakarta Selatan', room_name: 'Ruang IT', floor_name: 'L3', expected: true, result: 'found',
    note: null, counted_by_name: null, counted_at: null, followup_request_id: null, followup_record_id: null,
    ...over
  }
}

function page<T>(data: T[]): Paginated<T> {
  // Clone so the page component's in-place array writes (e.g. after
  // setResult) never leak back into shared module-level fixtures.
  return { data: [...data], total: data.length, limit: 100, offset: 0 }
}

const ITEMS: OpnameItem[] = [
  item({ id: 'i1', asset_name: 'Laptop Dell Latitude 5440', asset_tag: 'JKT01-ELK-2026-00001', result: 'found' }),
  item({ id: 'i2', asset_name: 'Proyektor Epson EB-X51', asset_tag: 'JKT01-ELK-2026-00002', result: 'found' }),
  item({ id: 'i3', asset_name: 'UPS APC Smart-UPS 1500', asset_tag: 'JKT01-ELK-2025-00018', result: 'damaged' }),
  item({ id: 'i4', asset_name: 'Kursi Ergonomis Ergotec', asset_tag: 'JKT01-MBL-2024-00033', result: 'misplaced' }),
  item({ id: 'i5', asset_name: 'Scanner Fujitsu fi-7160', asset_tag: 'JKT01-ELK-2025-00025', result: 'not_found' }),
  item({ id: 'i6', asset_name: 'Switch Cisco Catalyst 1000', asset_tag: 'JKT01-ITX-2025-00022', result: 'pending' })
]

// ---------------------------------------------------------------------------
// Composable mocks
// ---------------------------------------------------------------------------

const listMock = vi.fn()
const getMock = vi.fn()
const itemsMock = vi.fn()
const createMock = vi.fn()
const startMock = vi.fn()
const scanMock = vi.fn()
const setResultMock = vi.fn()
const reconcileMock = vi.fn()
const followupMock = vi.fn()
const closeMock = vi.fn()
const reportUrlMock = vi.fn()

vi.mock('~/composables/api/useStockOpname', () => ({
  useStockOpname: () => ({
    list: listMock,
    get: getMock,
    items: itemsMock,
    create: createMock,
    start: startMock,
    scan: scanMock,
    setResult: setResultMock,
    reconcile: reconcileMock,
    followup: followupMock,
    close: closeMock,
    reportUrl: reportUrlMock
  })
}))

const officesListMock = vi.fn()
vi.mock('~/composables/api/useOffices', () => ({
  useOffices: () => ({ list: officesListMock, get: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

const listByOfficeMock = vi.fn()
const roomsByFloorMock = vi.fn()
vi.mock('~/composables/api/useFloors', () => ({
  useFloors: () => ({
    listByOffice: listByOfficeMock, roomsByFloor: roomsByFloorMock,
    createFloor: vi.fn(), updateFloor: vi.fn(), removeFloor: vi.fn(),
    createRoom: vi.fn(), updateRoom: vi.fn(), removeRoom: vi.fn()
  })
}))

const refListMock = vi.fn()
vi.mock('~/composables/api/useReference', () => ({
  useReference: () => ({ list: refListMock, create: vi.fn(), update: vi.fn(), remove: vi.fn() })
}))

const assetsListMock = vi.fn()
vi.mock('~/composables/api/useAssets', () => ({
  useAssets: () => ({ list: assetsListMock, get: vi.fn(), getByTag: vi.fn(), update: vi.fn() })
}))

// eslint-disable-next-line import/first
import StockOpnamePage from '~/pages/stock-opname.vue'

enableAutoUnmount(afterEach)

function grantSession(officeId: string | null, permissions: string[] = ['stockopname.view', 'stockopname.manage']) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Dewi Lestari', email: 'dewi@test.com', role_id: 'r1', role_name: 'Kepala Unit', office_id: officeId },
    permissions
  )
}

async function mountAndWait() {
  const wrapper = await mountSuspended(StockOpnamePage, { route: '/stock-opname' })
  await flushPromises()
  await new Promise(r => setTimeout(r, 50))
  await flushPromises()
  await wrapper.vm.$nextTick()
  return wrapper
}

type Wrapper = Awaited<ReturnType<typeof mountAndWait>>

function bodyEl(testid: string): HTMLElement {
  const el = document.body.querySelector(`[data-testid="${testid}"]`)
  expect(el, `expected [data-testid="${testid}"] in document.body`).toBeTruthy()
  return el as HTMLElement
}

// USelect in Nuxt UI v4 renders a fully custom listbox portalled into
// document.body; reka-ui's PointerEvent-based item selection is not reliable
// in JSDOM. Per the established convention in this codebase (see
// settings-audit.spec.ts), the correct approach is to set the *component's*
// reactive ref directly — for modal sub-components that means reaching into
// the child component's vm via findComponent, since the ref does not live on
// the page itself.
function findModalVm(w: Wrapper, name: string) {
  const comp = w.findComponent({ name })
  expect(comp.exists(), `expected <${name}> to be mounted`).toBe(true)
  return comp.vm as unknown as Record<string, unknown>
}

async function setModalRef(w: Wrapper, name: string, key: string, value: unknown) {
  const vm = findModalVm(w, name)
  vm[key] = value
  await flushPromises()
  await w.vm.$nextTick()
  await flushPromises()
}

async function openDetail(w: Wrapper, id = 's1') {
  await w.find(`#opname-session-row-${id}`).trigger('click')
  await flushPromises()
  await w.vm.$nextTick()
}

const FLOORS: Floor[] = [{ id: 'f1', office_id: 'o-other', name: 'Lantai 1', level: 1, created_at: null, updated_at: null }]
const ROOMS: Room[] = [{ id: 'room1', floor_id: 'f1', name: 'Ruang Server', code: null, created_at: null, updated_at: null }]

beforeEach(() => {
  vi.clearAllMocks()
  officesListMock.mockResolvedValue(page(OFFICES))
  refListMock.mockResolvedValue(page([] as ReferenceRow[]))
  listByOfficeMock.mockResolvedValue(FLOORS)
  roomsByFloorMock.mockResolvedValue(ROOMS)
  assetsListMock.mockResolvedValue(page([]))
  listMock.mockResolvedValue(page([session()]))
  getMock.mockResolvedValue(detail())
  itemsMock.mockResolvedValue(page(ITEMS))
  createMock.mockResolvedValue(detail({ id: 's-new', name: 'Sesi Baru', status: 'open' }))
  startMock.mockResolvedValue(detail({ status: 'counting' }))
  scanMock.mockResolvedValue({ id: 'i6', session_id: 's1', asset_id: 'a6', expected: true, result: 'found' })
  setResultMock.mockResolvedValue({ id: 'i1', session_id: 's1', asset_id: 'a1', expected: true, result: 'found', note: null, counted_at: '2026-07-01T09:00:00Z' })
  reconcileMock.mockResolvedValue(detail({ status: 'reconciling' }))
  followupMock.mockResolvedValue({ request_id: 'req1', type: 'disposal' })
  closeMock.mockResolvedValue(detail({ status: 'closed' }))
  reportUrlMock.mockResolvedValue(new Blob(['pdf'], { type: 'application/pdf' }))
  toastAddMock.mockClear()
  grantSession('o-mine')
})

// ---------------------------------------------------------------------------
// 1. List view
// ---------------------------------------------------------------------------

describe('pages/stock-opname — list', () => {
  it('loads and renders sessions with resolved status text', async () => {
    listMock.mockResolvedValue(page([
      session({ id: 's1', status: 'counting' }),
      session({ id: 's2', status: 'reconciling' }),
      session({ id: 's3', status: 'closed' }),
      session({ id: 's4', status: 'open' })
    ]))
    const w = await mountAndWait()
    const rows = w.findAll('[data-testid="opname-session-row"]')
    expect(rows).toHaveLength(4)
    expect(w.text()).toContain('Berjalan')
    expect(w.text()).toContain('Rekonsiliasi')
    expect(w.text()).toContain('Selesai')
    expect(w.text()).toContain('Draf')
  })

  it('shows session scope (office name) and period on each card', async () => {
    const w = await mountAndWait()
    const row = w.find('[data-testid="opname-session-row"]')
    expect(row.text()).toContain('Kantor Cabang Jakarta Selatan')
    expect(row.text()).toContain('Opname Semester I 2026')
  })

  it('shows the empty state when there are no sessions', async () => {
    listMock.mockResolvedValue(page([]))
    const w = await mountAndWait()
    expect(w.find('[data-testid="opname-empty"]').exists()).toBe(true)
    expect(w.text()).toContain('Belum ada sesi opname')
  })

  it('shows the loading skeleton then the error+retry state on failure, and recovers on retry', async () => {
    listMock.mockRejectedValueOnce(new Error('boom'))
    const w = await mountAndWait()
    expect(w.text()).toContain('Gagal memuat data')
    listMock.mockResolvedValue(page([session()]))
    await w.find('[data-testid="opname-retry"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="opname-session-row"]').exists()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// 2/3. Create session modal
// ---------------------------------------------------------------------------

describe('pages/stock-opname — create session', () => {
  it('creates a session with the exact payload and shows a success toast', async () => {
    const w = await mountAndWait()
    await w.find('[data-testid="opname-create-open"]').trigger('click')
    await w.vm.$nextTick()

    await setModalRef(w, 'StockopnameCreateSessionModal', 'name', 'Opname Baru Juli')
    await setModalRef(w, 'StockopnameCreateSessionModal', 'officeId', 'o-other')
    await setModalRef(w, 'StockopnameCreateSessionModal', 'period', '2026-07')

    bodyEl('opname-create-confirm').dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith({ office_id: 'o-other', name: 'Opname Baru Juli', period: '2026-07' })
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: expect.stringContaining('berhasil dibuat') }))
  })

  it('shows the snapshot info note in the create modal', async () => {
    const w = await mountAndWait()
    await w.find('[data-testid="opname-create-open"]').trigger('click')
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('snapshot seluruh aset')
  })

  it('hides the create button without stockopname.manage', async () => {
    grantSession('o-mine', ['stockopname.view'])
    const w = await mountAndWait()
    const btn = w.find('[data-testid="opname-create-open"]')
    expect(!btn.exists() || btn.attributes('disabled') !== undefined).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// 4. Detail — open state
// ---------------------------------------------------------------------------

describe('pages/stock-opname — detail (open)', () => {
  it('shows KPI tiles and no scan bar while the session is still open', async () => {
    getMock.mockResolvedValue(detail({ status: 'open', total: 8, found: 0, pending: 8, variance: 0 }))
    itemsMock.mockResolvedValue(page(ITEMS.map(i => ({ ...i, result: 'pending' }))))
    const w = await mountAndWait()
    await openDetail(w)

    expect(w.find('[data-testid="opname-kpi-total"]').text()).toContain('8')
    expect(w.find('[data-testid="opname-kpi-found"]').text()).toContain('0')
    expect(w.find('[data-testid="opname-kpi-pending"]').text()).toContain('8')
    expect(w.find('[data-testid="opname-kpi-variance"]').text()).toContain('0')
    expect(w.find('[data-testid="opname-scan-input"]').exists()).toBe(false)
  })

  it('shows a "Mulai" button that calls start() when open', async () => {
    getMock.mockResolvedValue(detail({ status: 'open' }))
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-start"]').trigger('click')
    await flushPromises()
    expect(startMock).toHaveBeenCalledWith('s1')
  })
})

// ---------------------------------------------------------------------------
// 5. Detail — counting state (scan + editable results)
// ---------------------------------------------------------------------------

describe('pages/stock-opname — detail (counting)', () => {
  it('shows the scan bar and manual code input while counting', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    expect(w.find('[data-testid="opname-scan-input"]').exists()).toBe(true)
  })

  it('manual scan check calls scan() with the session id and code', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-scan-input"]').setValue('JKT01-ELK-2026-00099')
    await w.find('[data-testid="opname-scan-check"]').trigger('click')
    await flushPromises()
    expect(scanMock).toHaveBeenCalledWith('s1', 'JKT01-ELK-2026-00099')
  })

  it('clicking a result segment calls setResult() with the item id and result', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    const row = w.findAll('[data-testid="opname-item-row"]').find(r => r.text().includes('Switch Cisco Catalyst 1000'))!
    await row.find('[data-testid="opname-result-damaged"]').trigger('click')
    await flushPromises()
    expect(setResultMock).toHaveBeenCalledWith('s1', 'i6', { result: 'damaged' })
  })

  it('shows the "Rekonsiliasi" button that calls reconcile() when counting', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-reconcile"]').trigger('click')
    await flushPromises()
    expect(reconcileMock).toHaveBeenCalledWith('s1')
  })
})

// ---------------------------------------------------------------------------
// 6/7. Detail — reconciling state (read-only + variance panel + follow-up)
// ---------------------------------------------------------------------------

describe('pages/stock-opname — detail (reconciling)', () => {
  beforeEach(() => {
    getMock.mockResolvedValue(detail({ status: 'reconciling', total: 6, found: 2, pending: 0, variance: 3 }))
  })

  it('renders result cells as read-only badges (no segmented buttons)', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    expect(w.find('[data-testid="opname-result-found"]').exists()).toBe(false)
    const row = w.findAll('[data-testid="opname-item-row"]').find(r => r.text().includes('Laptop Dell Latitude 5440'))!
    expect(row.text()).toContain('Ditemukan')
  })

  it('shows read-only labels using DB enum semantics for pending/not_found (deviation c)', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    const pendingRow = w.findAll('[data-testid="opname-item-row"]').find(r => r.text().includes('Switch Cisco Catalyst 1000'))!
    expect(pendingRow.text()).toContain('Belum Dicek')
    const notFoundRow = w.findAll('[data-testid="opname-item-row"]').find(r => r.text().includes('Scanner Fujitsu fi-7160'))!
    expect(notFoundRow.text()).toContain('Tidak Ditemukan')
  })

  it('shows the variance panel listing not_found/damaged/misplaced items with follow-up buttons', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    expect(w.find('[data-testid="opname-followup-not_found"]').exists()).toBe(true)
    expect(w.find('[data-testid="opname-followup-damaged"]').exists()).toBe(true)
    expect(w.find('[data-testid="opname-followup-misplaced"]').exists()).toBe(true)
  })

  it('not_found follow-up calls followup() for disposal without an office', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-followup-not_found"]').trigger('click')
    await flushPromises()
    expect(followupMock).toHaveBeenCalledWith('s1', 'i5', {})
  })

  it('misplaced follow-up opens a modal requiring a destination office before submit', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-followup-misplaced"]').trigger('click')
    await w.vm.$nextTick()

    const confirmBtn = bodyEl('opname-followup-confirm') as HTMLButtonElement
    expect(confirmBtn.disabled).toBe(true)

    await setModalRef(w, 'StockopnameFollowupModal', 'officeId', 'o-other')
    expect((bodyEl('opname-followup-confirm') as HTMLButtonElement).disabled).toBe(false)

    bodyEl('opname-followup-confirm').dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    expect(followupMock).toHaveBeenCalledWith('s1', 'i4', expect.objectContaining({ to_office_id: 'o-other' }))
  })

  it('damaged follow-up calls followup() for a maintenance record and shows a success toast', async () => {
    followupMock.mockResolvedValue({ record_id: 'rec1', type: 'maintenance_record' })
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-followup-damaged"]').trigger('click')
    await flushPromises()
    expect(followupMock).toHaveBeenCalledWith('s1', 'i3', {})
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({
      title: expect.stringContaining('UPS APC Smart-UPS 1500')
    }))
  })

  it('disables the damaged follow-up button once the item already has a linked maintenance record', async () => {
    itemsMock.mockResolvedValue(page(ITEMS.map(i => i.id === 'i3' ? { ...i, followup_record_id: 'rec1' } : i)))
    const w = await mountAndWait()
    await openDetail(w)
    const btn = w.find('[data-testid="opname-followup-damaged"]')
    expect(btn.attributes('disabled')).toBeDefined()
  })

  it('disables the not_found/misplaced follow-up buttons once already linked to a follow-up request', async () => {
    itemsMock.mockResolvedValue(page(ITEMS.map((i) => {
      if (i.id === 'i5') return { ...i, followup_request_id: 'req-nf' } // not_found
      if (i.id === 'i4') return { ...i, followup_request_id: 'req-mv' } // misplaced
      return i
    })))
    const w = await mountAndWait()
    await openDetail(w)
    expect(w.find('[data-testid="opname-followup-not_found"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="opname-followup-misplaced"]').attributes('disabled')).toBeDefined()
  })

  it('disables the not_found/misplaced/damaged follow-up buttons without stockopname.manage', async () => {
    grantSession('o-mine', ['stockopname.view'])
    const w = await mountAndWait()
    await openDetail(w)
    const notFoundBtn = w.find('[data-testid="opname-followup-not_found"]')
    const misplacedBtn = w.find('[data-testid="opname-followup-misplaced"]')
    const damagedBtn = w.find('[data-testid="opname-followup-damaged"]')
    expect(notFoundBtn.attributes('disabled')).toBeDefined()
    expect(misplacedBtn.attributes('disabled')).toBeDefined()
    expect(damagedBtn.attributes('disabled')).toBeDefined()
  })
})

// ---------------------------------------------------------------------------
// 8. Detail — closed state
// ---------------------------------------------------------------------------

describe('pages/stock-opname — detail (closed)', () => {
  it('hides "Selesaikan" and shows the Berita Acara export button', async () => {
    getMock.mockResolvedValue(detail({ status: 'closed', total: 6, found: 4, pending: 0, variance: 2 }))
    const w = await mountAndWait()
    await openDetail(w)
    expect(w.find('[data-testid="opname-finish-open"]').exists()).toBe(false)
    expect(w.find('[data-testid="opname-export"]').exists()).toBe(true)
  })

  it('result cells are read-only on a closed session too', async () => {
    getMock.mockResolvedValue(detail({ status: 'closed' }))
    const w = await mountAndWait()
    await openDetail(w)
    expect(w.find('[data-testid="opname-result-found"]').exists()).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// 9. Loading / error states on detail fetch
// ---------------------------------------------------------------------------

describe('pages/stock-opname — detail loading/error', () => {
  it('shows an error+retry state when the detail fetch fails, and recovers on retry', async () => {
    const w = await mountAndWait()
    // Fail only the detail-open fetch that follows, not the list's own
    // per-row KPI lookups already consumed during mountAndWait().
    getMock.mockRejectedValueOnce(new Error('boom'))
    await openDetail(w)
    expect(w.text()).toContain('Gagal memuat data')

    getMock.mockResolvedValue(detail())
    await w.find('[data-testid="opname-detail-retry"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="opname-kpi-total"]').exists()).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Finish modal (close then export)
// ---------------------------------------------------------------------------

describe('pages/stock-opname — finish modal', () => {
  it('confirming finish calls close() and shows the Berita Acara preview', async () => {
    getMock.mockResolvedValue(detail({ status: 'reconciling', total: 6, found: 3, pending: 0, variance: 3 }))
    const w = await mountAndWait()
    await openDetail(w)
    await w.find('[data-testid="opname-finish-open"]').trigger('click')
    await w.vm.$nextTick()
    expect(document.body.textContent).toContain('BERITA ACARA STOCK OPNAME')

    bodyEl('opname-finish-confirm').dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(closeMock).toHaveBeenCalledWith('s1')
  })
})

// ---------------------------------------------------------------------------
// Client-side item search / room filter
// ---------------------------------------------------------------------------

describe('pages/stock-opname — item table client-side filters', () => {
  it('filters items by search text without an extra API call', async () => {
    const w = await mountAndWait()
    await openDetail(w)
    itemsMock.mockClear()
    await w.find('[data-testid="opname-item-search"]').setValue('Scanner')
    await w.vm.$nextTick()
    const rows = w.findAll('[data-testid="opname-item-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0]!.text()).toContain('Scanner Fujitsu fi-7160')
    expect(itemsMock).not.toHaveBeenCalled()
  })
})
