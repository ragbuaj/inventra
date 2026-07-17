// @vitest-environment nuxt
// Task 16: the full /notifications feed — the four load states, the filter
// tabs (asserted on the outgoing query, not just the rendered rows),
// server-side pagination, mark-all-read, and row click with its authorization
// gate.
//
// `navigateTo` is mocked here to observe its arguments. That short-circuits
// @nuxtjs/i18n's locale-detection redirect (same caveat as
// NotificationBell.spec.ts), which pins this mount to the English fallback
// catalog — verified by `expected()`, not assumed. Copy is therefore asserted
// against the real resolved English sentences; the default-locale (Indonesian)
// copy is covered by notifications-page.i18n.spec.ts, which leaves navigateTo
// alone.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'
import { useNotificationsStore } from '~/stores/notifications'
import type { NotificationRow } from '~/composables/api/useNotifications'

const listMock = vi.fn()
const unreadCountMock = vi.fn()
const markReadMock = vi.fn()
const markAllReadMock = vi.fn()

vi.mock('~/composables/api/useNotifications', () => ({
  useNotifications: () => ({
    list: listMock,
    unreadCount: unreadCountMock,
    markRead: markReadMock,
    markAllRead: markAllReadMock
  })
}))

const { navigateToMock } = vi.hoisted(() => ({ navigateToMock: vi.fn() }))
mockNuxtImport('navigateTo', () => navigateToMock)

// eslint-disable-next-line import/first
import NotificationsPage from '~/pages/notifications.vue'

enableAutoUnmount(afterEach)

// --- fixtures -------------------------------------------------------------

const TAG = 'INV-2024-0312'
const PAGE_SIZE = 20

const row = (over: Partial<NotificationRow> = {}): NotificationRow => ({
  id: 'n1',
  type: 'approval_pending',
  params: { request_type: 'asset_create', step: '1' },
  entity_type: 'requests',
  entity_id: 'r1',
  read_at: null,
  created_at: new Date(Date.now() - 5 * 60_000).toISOString(),
  ...over
})

const pendingRow = () => row({ id: 'n-pending' })
const decidedRow = () => row({
  id: 'n-decided',
  type: 'approval_decided',
  params: { request_type: 'asset_create', status: 'approved' },
  entity_type: 'requests'
})
const maintenanceRow = () => row({
  id: 'n-maint',
  type: 'maintenance_due',
  params: { asset_name: 'Toyota Avanza', asset_tag: TAG, due_date: 'besok' },
  entity_type: 'assets',
  entity_id: 'a1'
})
const returnedRow = () => row({
  id: 'n-returned',
  type: 'asset_returned',
  params: { asset_name: 'Toyota Avanza', asset_tag: TAG },
  entity_type: 'assets',
  entity_id: 'a1'
})

/** Real resolved sentences from the shipped English catalog (see file header). */
const EN_TEXT = {
  title: 'Notifications',
  subtitle: 'Every notification for your account.',
  markAllRead: 'Mark all as read',
  emptyTitle: 'No notifications yet',
  emptySubAll: 'Notifications about requests, maintenance, and assets will show up here.',
  emptySubUnread: 'You have read every notification.',
  emptySubRead: 'No notification has been read yet.',
  loadError: 'Failed to load notifications.',
  retry: 'Retry',
  filterAll: 'All',
  filterUnread: 'Unread',
  filterRead: 'Read',
  approval_pending: 'A Asset Registration request is awaiting your approval (step 1)',
  approval_decided: 'Your Asset Registration request was Approved',
  maintenance_due: `Maintenance for Toyota Avanza (${TAG}) is due besok`,
  asset_returned: `Asset Toyota Avanza (${TAG}) has been returned`
}

// --- helpers --------------------------------------------------------------

function login(permissions: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: 'u1', name: 'Rina Putri', email: 'rina@e.com', role_id: 'r1', role_name: 'Staf', office_id: 'o1' },
    permissions
  )
}

async function settle() {
  await flushPromises()
  await new Promise(r => setTimeout(r, 0))
  await flushPromises()
}

/**
 * The page's own list() calls, separated from the store's. The page always
 * sends limit AND offset; stores/notifications.ts sends limit only — so the
 * presence of `offset` is an exact discriminator.
 */
function pageQueries(): Array<{ read?: boolean, limit?: number, offset?: number }> {
  return listMock.mock.calls
    .map(c => (c[0] ?? {}) as { read?: boolean, limit?: number, offset?: number })
    .filter(q => q.offset !== undefined)
}

function lastPageQuery() {
  const qs = pageQueries()
  return qs[qs.length - 1]
}

/** Seeds one page of the feed; `total` defaults to the rows given. */
function seed(items: NotificationRow[], total = items.length, unread = items.filter(n => !n.read_at).length) {
  listMock.mockResolvedValue({ data: items, total, limit: PAGE_SIZE, offset: 0 })
  unreadCountMock.mockResolvedValue(unread)
}

async function mountPage() {
  const w = await mountSuspended(NotificationsPage)
  await settle()
  return w
}

function text(w: { text: () => string }): string {
  return w.text().replace(/\s+/g, ' ').trim()
}

/** Guards the file-header locale claim: if the fallback ever changes, the copy
 *  assertions below must be revisited rather than silently passing. */
async function expected(w: { text: () => string }) {
  expect(text(w)).toContain(EN_TEXT.title)
  return EN_TEXT
}

describe('pages/notifications', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore().clear()
    const store = useNotificationsStore()
    store.items = []
    store.unreadCount = 0
    store.loading = false
    store.error = false
    login(['request.decide'])
    seed([])
    markReadMock.mockImplementation(async (id: string) => row({ id, read_at: new Date().toISOString() }))
    markAllReadMock.mockResolvedValue(undefined)
  })

  // --- states -------------------------------------------------------------

  describe('states', () => {
    it('renders a skeleton while the first page is in flight, then the rows', async () => {
      let release: (v: unknown) => void = () => {}
      listMock.mockReturnValue(new Promise((r) => {
        release = r
      }))

      const w = await mountSuspended(NotificationsPage)
      await flushPromises()

      expect(w.find('[data-testid="notifications-loading"]').exists()).toBe(true)
      expect(w.find('[data-testid="notifications-empty"]').exists()).toBe(false)
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(0)

      release({ data: [maintenanceRow()], total: 1, limit: PAGE_SIZE, offset: 0 })
      await settle()

      expect(w.find('[data-testid="notifications-loading"]').exists()).toBe(false)
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(1)
    })

    it('renders the empty state, not a row, when the feed is empty', async () => {
      seed([])
      const w = await mountPage()
      const e = await expected(w)

      expect(w.find('[data-testid="notifications-empty"]').exists()).toBe(true)
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(0)
      expect(text(w)).toContain(e.emptyTitle)
      expect(text(w)).toContain(e.emptySubAll)
    })

    it('renders the error state with a retry button when the feed fails to load', async () => {
      listMock.mockRejectedValue(new Error('boom'))
      const w = await mountPage()

      expect(w.find('[data-testid="notifications-load-error"]').exists()).toBe(true)
      expect(w.find('[data-testid="notifications-empty"]').exists()).toBe(false)
      const t = text(w)
      expect(t).toContain(EN_TEXT.loadError)
      expect(t).toContain(EN_TEXT.retry)
    })

    it('retry re-fetches and replaces the error state with the loaded rows', async () => {
      listMock.mockRejectedValue(new Error('boom'))
      const w = await mountPage()
      expect(w.find('[data-testid="notifications-load-error"]').exists()).toBe(true)

      seed([maintenanceRow()], 1)
      await w.find('[data-testid="notifications-retry"]').trigger('click')
      await settle()

      expect(w.find('[data-testid="notifications-load-error"]').exists()).toBe(false)
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(1)
      expect(text(w)).toContain(EN_TEXT.maintenance_due)
    })

    it('shows the error wall, not the empty state, when a filtered load fails', async () => {
      seed([maintenanceRow()], 1)
      const w = await mountPage()
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(1)

      listMock.mockRejectedValue(new Error('boom'))
      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()

      expect(w.find('[data-testid="notifications-load-error"]').exists()).toBe(true)
      expect(w.find('[data-testid="notifications-empty"]').exists()).toBe(false)
    })

    it('renders the page chrome: title, subtitle and the three filter tabs', async () => {
      const w = await mountPage()
      const e = await expected(w)
      const t = text(w)
      expect(t).toContain(e.subtitle)
      expect(t).toContain(e.filterAll)
      expect(t).toContain(e.filterUnread)
      expect(t).toContain(e.filterRead)
    })
  })

  // --- rows ---------------------------------------------------------------

  describe('rows', () => {
    it('renders the resolved sentence for all four types', async () => {
      seed([pendingRow(), decidedRow(), maintenanceRow(), returnedRow()], 4)
      const w = await mountPage()
      const e = await expected(w)

      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(4)
      const t = text(w)
      expect(t).toContain(e.approval_pending)
      expect(t).toContain(e.approval_decided)
      expect(t).toContain(e.maintenance_due)
      expect(t).toContain(e.asset_returned)
      // Enum-valued params are translated, never interpolated raw.
      expect(t).not.toContain('asset_create')
      expect(t).not.toContain('approved')
    })

    it.each([
      ['approval_pending', pendingRow, 'i-lucide:check-square', 'bg-primary/10', 'text-primary'],
      ['approval_decided', decidedRow, 'i-lucide:clipboard-check', 'bg-primary/10', 'text-primary'],
      ['maintenance_due', maintenanceRow, 'i-lucide:wrench', 'bg-warning/15', 'text-warning'],
      ['asset_returned', returnedRow, 'i-lucide:package', 'bg-muted', 'text-muted']
    ])('renders the %s icon and tint from the catalog', async (_type, make, icon, bg, fg) => {
      seed([make()], 1)
      const w = await mountPage()
      const badge = w.find('[data-testid="notifications-row"]').find('span')
      expect(badge.classes().join(' ')).toContain(bg)
      const iconEl = badge.find('.iconify')
      expect(iconEl.classes().join(' ')).toContain(icon)
      expect(iconEl.classes().join(' ')).toContain(fg)
    })

    it('renders a relative timestamp, not the raw ISO string', async () => {
      seed([pendingRow()], 1)
      const w = await mountPage()
      expect(text(w)).toContain('5 minutes ago')
      expect(text(w)).not.toContain('T00:')
    })

    it('tints unread rows and marks them with a dot; read rows get neither', async () => {
      seed([pendingRow(), row({ id: 'n-read', read_at: '2026-07-17T09:00:00Z' })], 2)
      const w = await mountPage()
      const rows = w.findAll('[data-testid="notifications-row"]')

      expect(rows[0]!.classes().join(' ')).toContain('bg-primary/5')
      expect(rows[0]!.find('[data-testid="notification-unread-dot"]').exists()).toBe(true)
      expect(rows[1]!.classes().join(' ')).not.toContain('bg-primary/5')
      expect(rows[1]!.find('[data-testid="notification-unread-dot"]').exists()).toBe(false)
    })

    it('renders an unknown type as a neutral fallback instead of throwing', async () => {
      seed([row({ type: 'moon_phase' as never, params: {}, entity_type: null, entity_id: null })], 1)
      const w = await mountPage()
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(1)
      expect(text(w)).toMatch(/Notifikasi baru|New notification/)
    })
  })

  // --- filter -------------------------------------------------------------

  describe('filter', () => {
    it('loads the whole feed on mount, omitting `read` entirely', async () => {
      await mountPage()
      expect(lastPageQuery()).toEqual({ read: undefined, limit: PAGE_SIZE, offset: 0 })
    })

    it('sends read=false for the unread tab', async () => {
      const w = await mountPage()
      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()
      expect(lastPageQuery()).toEqual({ read: false, limit: PAGE_SIZE, offset: 0 })
    })

    it('sends read=true for the read tab', async () => {
      const w = await mountPage()
      await w.find('[data-testid="notifications-tab-read"]').trigger('click')
      await settle()
      expect(lastPageQuery()).toEqual({ read: true, limit: PAGE_SIZE, offset: 0 })
    })

    it('goes back to omitting `read` when returning to the all tab', async () => {
      const w = await mountPage()
      await w.find('[data-testid="notifications-tab-read"]').trigger('click')
      await settle()
      await w.find('[data-testid="notifications-tab-all"]').trigger('click')
      await settle()
      expect(lastPageQuery()).toEqual({ read: undefined, limit: PAGE_SIZE, offset: 0 })
    })

    it('resets to the first page when the filter changes', async () => {
      seed([maintenanceRow()], 60)
      const w = await mountPage()

      await w.findAll('[data-testid="pagination-page"]')[1]!.trigger('click')
      await settle()
      expect(lastPageQuery()!.offset).toBe(PAGE_SIZE)

      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()
      // Page 2 of "all" is not page 2 of "unread" — offset must go back to 0.
      expect(lastPageQuery()).toEqual({ read: false, limit: PAGE_SIZE, offset: 0 })
    })

    it('issues exactly one page query when the filter changes from a later page', async () => {
      seed([maintenanceRow()], 60)
      const w = await mountPage()

      await w.findAll('[data-testid="pagination-page"]')[1]!.trigger('click')
      await settle()
      const before = pageQueries().length

      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()
      // The filter watcher resets the offset and lets the offset watcher reload:
      // one query, not the two a redundant load() in both watchers would fire.
      expect(pageQueries().length - before).toBe(1)
    })

    it('falls back to page 1 when a page beyond the shrunken data loads empty', async () => {
      // Page 1 has data (total says 2 pages); any offset>0 comes back empty, as
      // if a mark-read shrank the set below the current page. Keyed on offset
      // rather than mockResolvedValueOnce, so the store's own refresh() list()
      // calls cannot consume the queued value out from under the page.
      listMock.mockImplementation((q: { offset?: number } = {}) =>
        Promise.resolve((q.offset ?? 0) > 0
          ? { data: [], total: 20, limit: PAGE_SIZE, offset: q.offset }
          : { data: [maintenanceRow()], total: 40, limit: PAGE_SIZE, offset: 0 }))
      unreadCountMock.mockResolvedValue(1)
      const w = await mountPage()

      await w.findAll('[data-testid="pagination-page"]')[1]!.trigger('click')
      await settle()

      // load() saw an empty page at offset>0 and fell back to page 1 instead of
      // showing a false empty-state.
      expect(lastPageQuery()!.offset).toBe(0)
      expect(w.find('[data-testid="notifications-empty"]').exists()).toBe(false)
      expect(w.findAll('[data-testid="notifications-row"]')).toHaveLength(1)
    })

    it('shows the filter-specific empty copy on each tab', async () => {
      const w = await mountPage()
      expect(text(w)).toContain(EN_TEXT.emptySubAll)

      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()
      expect(text(w)).toContain(EN_TEXT.emptySubUnread)

      await w.find('[data-testid="notifications-tab-read"]').trigger('click')
      await settle()
      expect(text(w)).toContain(EN_TEXT.emptySubRead)
    })
  })

  // --- pagination ---------------------------------------------------------

  describe('pagination', () => {
    it('is not rendered when the feed is empty', async () => {
      seed([])
      const w = await mountPage()
      expect(w.find('[data-testid="pagination-next"]').exists()).toBe(false)
    })

    it('renders a single page when total fits in one page', async () => {
      seed([maintenanceRow()], 1)
      const w = await mountPage()
      expect(w.findAll('[data-testid="pagination-page"]')).toHaveLength(1)
      expect(w.find('[data-testid="pagination-next"]').attributes('disabled')).toBeDefined()
    })

    it('derives the page count from `total`, not from the rows on screen', async () => {
      // One row rendered, but the server says 45 -> ceil(45/20) = 3 pages.
      seed([maintenanceRow()], 45)
      const w = await mountPage()
      expect(w.findAll('[data-testid="pagination-page"]')).toHaveLength(3)
    })

    it('requests the right offset when paging forward and back', async () => {
      seed([maintenanceRow()], 45)
      const w = await mountPage()
      expect(lastPageQuery()!.offset).toBe(0)

      await w.find('[data-testid="pagination-next"]').trigger('click')
      await settle()
      expect(lastPageQuery()).toEqual({ read: undefined, limit: PAGE_SIZE, offset: 20 })

      await w.find('[data-testid="pagination-next"]').trigger('click')
      await settle()
      expect(lastPageQuery()!.offset).toBe(40)

      await w.find('[data-testid="pagination-prev"]').trigger('click')
      await settle()
      expect(lastPageQuery()!.offset).toBe(20)
    })

    it('jumps straight to a page number', async () => {
      seed([maintenanceRow()], 45)
      const w = await mountPage()

      await w.findAll('[data-testid="pagination-page"]')[2]!.trigger('click')
      await settle()
      expect(lastPageQuery()!.offset).toBe(40)
    })

    it('reports the visible range from the server total', async () => {
      seed([maintenanceRow()], 45)
      const w = await mountPage()
      expect(text(w)).toContain('Showing 1')
      expect(text(w)).toContain('45')
    })
  })

  // --- mark all read ------------------------------------------------------

  describe('mark all read', () => {
    it('is disabled when nothing is unread', async () => {
      seed([row({ read_at: '2026-07-17T09:00:00Z' })], 1, 0)
      await useNotificationsStore().refresh()
      const w = await mountPage()
      expect(w.find('[data-testid="notifications-mark-all"]').attributes('disabled')).toBeDefined()
    })

    it('is enabled and shows the unread badge when something is unread', async () => {
      seed([pendingRow()], 1, 3)
      await useNotificationsStore().refresh()
      const w = await mountPage()

      expect(w.find('[data-testid="notifications-mark-all"]').attributes('disabled')).toBeUndefined()
      expect(w.find('[data-testid="notifications-unread-badge"]').text()).toContain('3')
      expect(text(w)).toContain(EN_TEXT.markAllRead)
    })

    it('calls the endpoint, then reloads the page and the badge', async () => {
      seed([pendingRow()], 1, 1)
      await useNotificationsStore().refresh()
      const w = await mountPage()
      listMock.mockClear()
      unreadCountMock.mockClear()

      seed([row({ id: 'n-pending', read_at: '2026-07-17T10:00:00Z' })], 1, 0)
      await w.find('[data-testid="notifications-mark-all"]').trigger('click')
      await settle()

      expect(markAllReadMock).toHaveBeenCalledTimes(1)
      expect(pageQueries().length).toBeGreaterThan(0)
      expect(unreadCountMock).toHaveBeenCalled()
      expect(w.find('[data-testid="notifications-unread-badge"]').exists()).toBe(false)
      expect(w.find('[data-testid="notifications-mark-all"]').attributes('disabled')).toBeDefined()
    })

    it('still resyncs when the endpoint fails, and does not navigate', async () => {
      seed([pendingRow()], 1, 1)
      await useNotificationsStore().refresh()
      const w = await mountPage()
      markAllReadMock.mockRejectedValue(new Error('boom'))
      listMock.mockClear()

      await w.find('[data-testid="notifications-mark-all"]').trigger('click')
      await settle()

      expect(pageQueries().length).toBeGreaterThan(0)
      expect(navigateToMock).not.toHaveBeenCalled()
    })

    it('keeps the current filter when reloading after mark-all', async () => {
      seed([pendingRow()], 1, 1)
      await useNotificationsStore().refresh()
      const w = await mountPage()
      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()

      await w.find('[data-testid="notifications-mark-all"]').trigger('click')
      await settle()

      expect(lastPageQuery()!.read).toBe(false)
    })
  })

  // --- row click ----------------------------------------------------------

  describe('row click', () => {
    it('marks the row read and navigates to the linked asset', async () => {
      seed([maintenanceRow()], 1)
      const w = await mountPage()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-maint')
      expect(navigateToMock).toHaveBeenCalledTimes(1)
      expect(navigateToMock.mock.calls[0]![0]).toMatch(new RegExp(`/assets/${TAG}$`))
    })

    it('navigates an approval_pending row to /approval for a user who can decide', async () => {
      login(['request.decide'])
      seed([pendingRow()], 1)
      const w = await mountPage()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-pending')
      expect(navigateToMock.mock.calls[0]![0]).toMatch(/\/approval$/)
    })

    it('treats a wildcard permission as sufficient to decide', async () => {
      login(['*'])
      seed([pendingRow()], 1)
      const w = await mountPage()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(navigateToMock.mock.calls[0]![0]).toMatch(/\/approval$/)
    })

    it('marks read but does NOT navigate a requests row when the caller cannot decide', async () => {
      // The maker receives approval_decided but often lacks request.decide;
      // /approval would 403 on them, so the row is mark-read only.
      login(['request.create'])
      seed([decidedRow()], 1)
      const w = await mountPage()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-decided')
      expect(navigateToMock).not.toHaveBeenCalled()
    })

    it('does not navigate an assets row that carries no asset_tag param', async () => {
      seed([row({ id: 'n-noTag', type: 'asset_returned', params: {}, entity_type: 'assets', entity_id: 'a1' })], 1)
      const w = await mountPage()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-noTag')
      expect(navigateToMock).not.toHaveBeenCalled()
    })

    it('drops the unread tint in place, without a reload, on the all tab', async () => {
      seed([maintenanceRow()], 1)
      const w = await mountPage()
      listMock.mockClear()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      const rowEl = w.find('[data-testid="notifications-row"]')
      expect(rowEl.classes().join(' ')).not.toContain('bg-primary/5')
      expect(rowEl.find('[data-testid="notification-unread-dot"]').exists()).toBe(false)
      // The badge is resynced, but the list itself is not refetched.
      expect(unreadCountMock).toHaveBeenCalled()
      expect(pageQueries()).toHaveLength(0)
    })

    it('reloads on the unread tab, where a read row no longer belongs', async () => {
      seed([maintenanceRow()], 1)
      const w = await mountPage()
      await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
      await settle()
      listMock.mockClear()

      seed([], 0)
      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(lastPageQuery()!.read).toBe(false)
      expect(w.find('[data-testid="notifications-empty"]').exists()).toBe(true)
    })

    it('does not re-mark an already-read row', async () => {
      seed([row({ id: 'n-read', type: 'maintenance_due', params: { asset_name: 'A', asset_tag: TAG }, entity_type: 'assets', read_at: '2026-07-17T09:00:00Z' })], 1, 0)
      const w = await mountPage()

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(markReadMock).not.toHaveBeenCalled()
      // ... but the click still navigates.
      expect(navigateToMock).toHaveBeenCalledTimes(1)
    })

    it('still navigates when the mark-read call fails', async () => {
      seed([maintenanceRow()], 1)
      const w = await mountPage()
      markReadMock.mockRejectedValue(new Error('boom'))

      await w.find('[data-testid="notifications-row"]').trigger('click')
      await settle()

      expect(navigateToMock).toHaveBeenCalledTimes(1)
      expect(navigateToMock.mock.calls[0]![0]).toMatch(new RegExp(`/assets/${TAG}$`))
    })
  })
})
