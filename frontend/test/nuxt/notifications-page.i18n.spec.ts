// @vitest-environment nuxt
// Task 16: the /notifications page's Indonesian (default-locale) copy.
//
// This file exists because notifications-page.spec.ts mocks `navigateTo` to
// observe navigation targets, and that short-circuits @nuxtjs/i18n's locale
// redirect — leaving that mount on the English fallback catalog. `id` is the
// default locale and what users actually see, so the Indonesian sentences are
// asserted here, with navigateTo left alone. Same split as
// NotificationBell.spec.ts vs NotificationBell.i18n.spec.ts.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import NotificationsPage from '~/pages/notifications.vue'
import { useAuthStore } from '~/stores/auth'
import { useNotificationsStore } from '~/stores/notifications'
import type { NotificationRow } from '~/composables/api/useNotifications'

const listMock = vi.fn()
const unreadCountMock = vi.fn()

vi.mock('~/composables/api/useNotifications', () => ({
  useNotifications: () => ({
    list: listMock,
    unreadCount: unreadCountMock,
    markRead: vi.fn(),
    markAllRead: vi.fn()
  })
}))

enableAutoUnmount(afterEach)

const TAG = 'INV-2024-0312'

const row = (over: Partial<NotificationRow>): NotificationRow => ({
  id: 'n1',
  type: 'approval_pending',
  params: {},
  entity_type: null,
  entity_id: null,
  read_at: null,
  created_at: new Date(Date.now() - 60 * 60_000).toISOString(),
  ...over
})

async function settle() {
  await flushPromises()
  await new Promise(r => setTimeout(r, 0))
  await flushPromises()
}

function text(w: { text: () => string }): string {
  return w.text().replace(/\s+/g, ' ').trim()
}

async function mountWith(items: NotificationRow[], total = items.length, unread = items.length) {
  listMock.mockResolvedValue({ data: items, total, limit: 20, offset: 0 })
  unreadCountMock.mockResolvedValue(unread)
  const w = await mountSuspended(NotificationsPage)
  await settle()
  return w
}

describe('pages/notifications — Indonesian copy', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const store = useNotificationsStore()
    store.items = []
    store.unreadCount = 0
    store.loading = false
    store.error = false
    useAuthStore().setSession(
      'tok',
      { id: 'u1', name: 'Rina Putri', email: 'rina@e.com', role_id: 'r1', role_name: 'Staf', office_id: 'o1' },
      ['request.decide']
    )
  })

  it('renders the page chrome in Indonesian', async () => {
    const w = await mountWith([])
    const t = text(w)
    expect(t).toContain('Notifikasi')
    expect(t).toContain('Semua notifikasi untuk akun Anda.')
    expect(t).toContain('Tandai semua dibaca')
  })

  it('renders the three filter tabs in Indonesian', async () => {
    const w = await mountWith([])
    const t = text(w)
    expect(t).toContain('Semua')
    expect(t).toContain('Belum dibaca')
    expect(t).toContain('Sudah dibaca')
  })

  it('renders the empty state in Indonesian', async () => {
    const w = await mountWith([])
    const t = text(w)
    expect(t).toContain('Belum ada notifikasi')
    expect(t).toContain('Notifikasi tentang pengajuan, maintenance, dan aset akan muncul di sini.')
  })

  it('renders the per-filter empty copy in Indonesian', async () => {
    const w = await mountWith([])

    await w.find('[data-testid="notifications-tab-unread"]').trigger('click')
    await settle()
    expect(text(w)).toContain('Semua notifikasi sudah dibaca.')

    await w.find('[data-testid="notifications-tab-read"]').trigger('click')
    await settle()
    expect(text(w)).toContain('Belum ada notifikasi yang dibaca.')
  })

  it('renders the error state and retry in Indonesian', async () => {
    listMock.mockRejectedValue(new Error('boom'))
    unreadCountMock.mockRejectedValue(new Error('boom'))
    const w = await mountSuspended(NotificationsPage)
    await settle()

    const t = text(w)
    expect(t).toContain('Gagal memuat notifikasi.')
    expect(t).toContain('Coba lagi')
  })

  it('renders the unread badge in Indonesian', async () => {
    listMock.mockResolvedValue({ data: [row({})], total: 1, limit: 20, offset: 0 })
    unreadCountMock.mockResolvedValue(4)
    await useNotificationsStore().refresh()
    const w = await mountSuspended(NotificationsPage)
    await settle()

    expect(w.find('[data-testid="notifications-unread-badge"]').text()).toBe('4 belum dibaca')
  })

  it.each([
    [
      'approval_pending',
      { type: 'approval_pending' as const, params: { request_type: 'asset_create', step: '1' }, entity_type: 'requests' },
      'Pengajuan Registrasi Aset menunggu persetujuan Anda (tahap 1)'
    ],
    [
      'approval_decided',
      { type: 'approval_decided' as const, params: { request_type: 'assignment', status: 'rejected' }, entity_type: 'requests' },
      'Pengajuan Peminjaman Aset Anda telah Ditolak'
    ],
    [
      'maintenance_due',
      { type: 'maintenance_due' as const, params: { asset_name: 'Toyota Avanza', asset_tag: TAG, due_date: 'besok' }, entity_type: 'assets' },
      `Maintenance Toyota Avanza (${TAG}) jatuh tempo besok`
    ],
    [
      'asset_returned',
      { type: 'asset_returned' as const, params: { asset_name: 'Toyota Avanza', asset_tag: TAG }, entity_type: 'assets' },
      `Aset Toyota Avanza (${TAG}) telah dikembalikan`
    ]
  ])('renders the %s sentence in Indonesian with its params interpolated', async (_name, over, sentence) => {
    const w = await mountWith([row(over)])
    expect(text(w)).toContain(sentence)
  })

  it('renders an unknown type as the Indonesian fallback sentence', async () => {
    const w = await mountWith([row({ type: 'moon_phase' as never })])
    expect(text(w)).toContain('Notifikasi baru')
  })

  it('renders the relative timestamp in Indonesian', async () => {
    const w = await mountWith([row({ type: 'asset_returned', params: { asset_name: 'A', asset_tag: TAG } })])
    // Intl.RelativeTimeFormat('id') renders '1 jam yang lalu'.
    expect(text(w)).toContain('1 jam yang lalu')
  })

  it('renders the pagination range in Indonesian', async () => {
    const w = await mountWith([row({})], 45)
    expect(text(w)).toContain('Menampilkan 1')
    expect(text(w)).toContain('45')
  })
})
