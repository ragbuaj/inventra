// @vitest-environment nuxt
// Task 15: the real notification bell — badge, the three load states, mark-read
// (single + all), and click-to-navigate with its authorization gate.
//
// `navigateTo` is mocked here to observe its arguments. That short-circuits
// @nuxtjs/i18n's locale-detection redirect (same caveat as
// assets-index-actions.spec.ts), which pins this mount to the English fallback
// catalog — verified, not assumed. Copy is therefore asserted against the real
// resolved English sentences; the default-locale (Indonesian) copy is covered
// by NotificationBell.i18n.spec.ts, which leaves navigateTo alone.
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
import NotificationBell from '~/components/NotificationBell.vue'

enableAutoUnmount(afterEach)

// --- fixtures -------------------------------------------------------------

const TAG = 'INV-2024-0312'

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

// Real resolved sentences from the shipped English catalog (see the file
// header for why this mount lands on `en`).
const EN_TEXT = {
  title: 'Notifications',
  markRead: 'Mark read',
  viewAll: 'View all notifications',
  empty: 'No new notifications',
  loadError: 'Failed to load notifications.',
  retry: 'Retry',
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

/** The popover content is portaled to document.body, not into the wrapper. */
function panel(): HTMLElement {
  const el = document.body.querySelector('[data-testid="notification-mark-all"]')?.closest('div.w-\\[330px\\]')
  if (!el) throw new Error('notification panel is not open')
  return el as HTMLElement
}

function panelText(): string {
  return panel().textContent?.replace(/\s+/g, ' ').trim() ?? ''
}

function rows(): HTMLElement[] {
  return Array.from(document.body.querySelectorAll('[data-testid="notification-row"]'))
}

function byTestId(id: string): HTMLElement | null {
  return document.body.querySelector(`[data-testid="${id}"]`)
}

/** Guards the file-header claim: if the locale caveat ever changes, the copy
 *  assertions below must be revisited rather than silently passing. */
function expected() {
  expect(panelText()).toContain(EN_TEXT.title)
  return EN_TEXT
}

async function settle() {
  await flushPromises()
  await new Promise(r => setTimeout(r, 0))
  await flushPromises()
}

async function mountBell() {
  const w = await mountSuspended(NotificationBell)
  await settle()
  return w
}

/** Mounts and opens the dropdown. The store is primed first (as the fetchMe
 *  choke point does in the real app) so the panel renders a settled state. */
async function openBell() {
  await useNotificationsStore().refresh()
  const w = await mountBell()
  await w.find('[data-testid="notification-bell"]').trigger('click')
  await settle()
  return w
}

function seed(items: NotificationRow[], unread = items.filter(n => !n.read_at).length) {
  listMock.mockResolvedValue({ data: items, total: items.length, limit: 20, offset: 0 })
  unreadCountMock.mockResolvedValue(unread)
}

describe('NotificationBell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
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

  // --- badge --------------------------------------------------------------

  describe('badge', () => {
    it('is hidden when there is nothing unread', async () => {
      seed([])
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(false)
    })

    it('is hidden when rows exist but all of them are read', async () => {
      seed([row({ read_at: '2026-07-17T09:00:00Z' })], 0)
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(false)
    })

    it('shows the real unread count', async () => {
      seed([pendingRow(), maintenanceRow()], 2)
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').text()).toBe('2')
    })

    it('renders a large count verbatim, without 9+ clamping', async () => {
      seed([pendingRow()], 42)
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').text()).toBe('42')
    })

    it('drops to hidden after the count refreshes to zero', async () => {
      seed([pendingRow()], 1)
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(true)

      seed([], 0)
      await useNotificationsStore().refresh()
      await settle()
      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(false)
    })

    it('is not rendered at all for a signed-out user', async () => {
      useAuthStore().clear()
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(false)
      expect(listMock).not.toHaveBeenCalled()
    })
  })

  // --- states -------------------------------------------------------------

  describe('states', () => {
    it('renders the empty state, not a row, when the feed is empty', async () => {
      seed([])
      await openBell()
      expect(byTestId('notification-empty')).not.toBeNull()
      expect(rows()).toHaveLength(0)
      expect(panelText()).toContain(expected().empty)
    })

    it('renders skeletons while a refresh is in flight', async () => {
      let release: (v: unknown) => void = () => {}
      listMock.mockReturnValue(new Promise((r) => {
        release = r
      }))
      unreadCountMock.mockResolvedValue(0)

      const w = await mountBell()
      const refreshing = useNotificationsStore().refresh()
      await w.find('[data-testid="notification-bell"]').trigger('click')
      await settle()

      expect(byTestId('notification-loading')).not.toBeNull()
      expect(byTestId('notification-empty')).toBeNull()
      expect(byTestId('notification-load-error')).toBeNull()

      release({ data: [], total: 0, limit: 20, offset: 0 })
      await refreshing
      await settle()
      expect(byTestId('notification-loading')).toBeNull()
      expect(byTestId('notification-empty')).not.toBeNull()
    })

    it('renders the error state with a retry button when the feed fails to load', async () => {
      listMock.mockRejectedValue(new Error('boom'))
      unreadCountMock.mockRejectedValue(new Error('boom'))
      await openBell()

      expect(byTestId('notification-load-error')).not.toBeNull()
      expect(byTestId('notification-empty')).toBeNull()
      const text = panelText()
      expect(text).toContain(expected().loadError)
      expect(text).toContain(expected().retry)
    })

    it('retry re-fetches and replaces the error state with the loaded rows', async () => {
      listMock.mockRejectedValue(new Error('boom'))
      unreadCountMock.mockRejectedValue(new Error('boom'))
      await openBell()
      expect(byTestId('notification-load-error')).not.toBeNull()

      seed([maintenanceRow()], 1)
      byTestId('notification-retry')!.click()
      await settle()

      expect(byTestId('notification-load-error')).toBeNull()
      expect(rows()).toHaveLength(1)
      expect(panelText()).toContain(expected().maintenance_due)
    })

    it('keeps the last known rows visible when a later refresh fails', async () => {
      seed([maintenanceRow()], 1)
      await openBell()
      expect(rows()).toHaveLength(1)

      listMock.mockRejectedValue(new Error('boom'))
      unreadCountMock.mockRejectedValue(new Error('boom'))
      await useNotificationsStore().refresh()
      await settle()

      // Stale-but-useful beats an error wall (precedent: pages/approval.vue).
      expect(rows()).toHaveLength(1)
      expect(byTestId('notification-load-error')).toBeNull()
    })
  })

  // --- populated rows -----------------------------------------------------

  describe('rows', () => {
    it('renders the resolved sentence and relative time for every type', async () => {
      seed([pendingRow(), decidedRow(), maintenanceRow(), returnedRow()], 4)
      await openBell()
      const e = expected()

      expect(rows()).toHaveLength(4)
      const text = panelText()
      expect(text).toContain(e.approval_pending)
      expect(text).toContain(e.approval_decided)
      expect(text).toContain(e.maintenance_due)
      expect(text).toContain(e.asset_returned)
      // Enum-valued params are translated, never interpolated raw.
      expect(text).not.toContain('asset_create')
      expect(text).not.toContain('approved')
    })

    it('renders a relative timestamp, not the raw ISO string', async () => {
      seed([pendingRow()], 1)
      await openBell()
      const text = panelText()
      expect(text).toContain('5 minutes ago')
      expect(text).not.toContain('T00:')
    })

    it.each([
      ['approval_pending', pendingRow, 'i-lucide:check-square', 'bg-primary/10', 'text-primary'],
      ['approval_decided', decidedRow, 'i-lucide:clipboard-check', 'bg-primary/10', 'text-primary'],
      ['maintenance_due', maintenanceRow, 'i-lucide:wrench', 'bg-warning/15', 'text-warning'],
      ['asset_returned', returnedRow, 'i-lucide:package', 'bg-muted', 'text-muted']
    ])('renders the %s icon and tint from the catalog', async (_type, make, icon, bg, fg) => {
      seed([make()], 1)
      await openBell()
      const badge = rows()[0]!.querySelector('span')!
      expect(badge.className).toContain(bg)
      const iconEl = badge.querySelector('.iconify')!
      expect(iconEl.className).toContain(icon)
      expect(iconEl.className).toContain(fg)
    })

    it('renders an unknown type as a neutral fallback instead of throwing', async () => {
      seed([row({ type: 'moon_phase' as never, params: {}, entity_type: null, entity_id: null })], 1)
      await openBell()
      expect(rows()).toHaveLength(1)
      expect(panelText()).toMatch(/Notifikasi baru|New notification/)
    })

    it('tints unread rows and leaves read rows untinted', async () => {
      seed([pendingRow(), row({ id: 'n-read', read_at: '2026-07-17T09:00:00Z' })], 1)
      await openBell()
      expect(rows()[0]!.className).toContain('bg-primary/5')
      expect(rows()[1]!.className).not.toContain('bg-primary/5')
    })
  })

  // --- mark read ----------------------------------------------------------

  describe('mark all read', () => {
    it('calls the endpoint and refreshes the feed', async () => {
      seed([pendingRow(), maintenanceRow()], 2)
      await openBell()
      listMock.mockClear()

      byTestId('notification-mark-all')!.click()
      await settle()

      expect(markAllReadMock).toHaveBeenCalledTimes(1)
      expect(listMock).toHaveBeenCalled()
    })

    it('clears the badge once the refreshed count comes back zero', async () => {
      seed([pendingRow()], 1)
      const w = await openBell()
      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(true)

      seed([row({ read_at: '2026-07-17T10:00:00Z' })], 0)
      byTestId('notification-mark-all')!.click()
      await settle()

      expect(w.find('[data-testid="notification-badge"]').exists()).toBe(false)
    })

    it('still refreshes when the endpoint fails, and does not navigate', async () => {
      seed([pendingRow()], 1)
      await openBell()
      markAllReadMock.mockRejectedValue(new Error('boom'))
      listMock.mockClear()

      byTestId('notification-mark-all')!.click()
      await settle()

      expect(listMock).toHaveBeenCalled()
      expect(navigateToMock).not.toHaveBeenCalled()
    })
  })

  // --- row click ----------------------------------------------------------

  describe('row click', () => {
    it('marks the row read and navigates to the linked entity', async () => {
      seed([maintenanceRow()], 1)
      await openBell()

      rows()[0]!.click()
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-maint')
      expect(navigateToMock).toHaveBeenCalledTimes(1)
      expect(navigateToMock.mock.calls[0]![0]).toMatch(new RegExp(`/assets/${TAG}$`))
    })

    it('navigates an approval_pending row to /approval for a user who can decide', async () => {
      login(['request.decide'])
      seed([pendingRow()], 1)
      await openBell()

      rows()[0]!.click()
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-pending')
      expect(navigateToMock.mock.calls[0]![0]).toMatch(/\/approval$/)
    })

    it('treats a wildcard permission as sufficient to decide', async () => {
      login(['*'])
      seed([pendingRow()], 1)
      await openBell()

      rows()[0]!.click()
      await settle()

      expect(navigateToMock.mock.calls[0]![0]).toMatch(/\/approval$/)
    })

    it('marks an approval_decided row read but does NOT navigate the maker into a 403', async () => {
      // The maker receives approval_decided but typically lacks request.decide,
      // which /approval is gated on — the click must stop at mark-read.
      login(['asset.view'])
      seed([decidedRow()], 1)
      await openBell()

      rows()[0]!.click()
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-decided')
      expect(navigateToMock).not.toHaveBeenCalled()
    })

    it('marks a row with no derivable link read without navigating', async () => {
      // assets links key off params.asset_tag (the detail route is /assets/[tag]);
      // without one there is no target, so the row is mark-read only.
      seed([row({ id: 'n-nolink', type: 'asset_returned', params: { asset_name: 'Laptop' }, entity_type: 'assets', entity_id: 'a1' })], 1)
      await openBell()

      rows()[0]!.click()
      await settle()

      expect(markReadMock).toHaveBeenCalledWith('n-nolink')
      expect(navigateToMock).not.toHaveBeenCalled()
    })

    it('does not re-mark a row that is already read, but still navigates', async () => {
      seed([maintenanceRow(), row({ id: 'n-read', type: 'maintenance_due', params: { asset_name: 'A', asset_tag: TAG }, entity_type: 'assets', read_at: '2026-07-17T09:00:00Z' })], 1)
      await openBell()

      rows()[1]!.click()
      await settle()

      expect(markReadMock).not.toHaveBeenCalled()
      expect(navigateToMock).toHaveBeenCalledTimes(1)
    })

    it('still navigates when mark-read fails', async () => {
      seed([maintenanceRow()], 1)
      await openBell()
      markReadMock.mockRejectedValue(new Error('boom'))

      rows()[0]!.click()
      await settle()

      expect(navigateToMock).toHaveBeenCalledTimes(1)
    })

    it('closes the dropdown on click', async () => {
      seed([maintenanceRow()], 1)
      await openBell()
      expect(rows()).toHaveLength(1)

      rows()[0]!.click()
      await settle()

      expect(byTestId('notification-mark-all')).toBeNull()
    })
  })

  // --- view all -----------------------------------------------------------

  describe('view all', () => {
    it('navigates to the full feed page', async () => {
      seed([pendingRow()], 1)
      await openBell()

      byTestId('notification-view-all')!.click()
      await settle()

      expect(navigateToMock).toHaveBeenCalledTimes(1)
      expect(navigateToMock.mock.calls[0]![0]).toMatch(/\/notifications$/)
    })

    it('is offered even when the feed is empty', async () => {
      seed([])
      await openBell()
      expect(panelText()).toContain(expected().viewAll)
      expect(byTestId('notification-view-all')).not.toBeNull()
    })
  })
})
