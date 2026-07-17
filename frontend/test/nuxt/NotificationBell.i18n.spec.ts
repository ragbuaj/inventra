// @vitest-environment nuxt
// Task 15: the bell's Indonesian (default-locale) copy.
//
// This file exists because NotificationBell.spec.ts mocks `navigateTo` to
// observe navigation targets, and that short-circuits @nuxtjs/i18n's locale
// redirect — leaving that mount on the English fallback catalog. `id` is the
// default locale and what users actually see, so the Indonesian sentences are
// asserted here, with navigateTo left alone. Same split as
// assets-index.spec.ts (labels) vs assets-index-actions.spec.ts (nav targets).
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import NotificationBell from '~/components/NotificationBell.vue'
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

function panelText(): string {
  const el = document.body.querySelector('[data-testid="notification-mark-all"]')?.closest('div.w-\\[330px\\]')
  if (!el) throw new Error('notification panel is not open')
  return el.textContent?.replace(/\s+/g, ' ').trim() ?? ''
}

async function openWith(items: NotificationRow[], unread = items.length) {
  listMock.mockResolvedValue({ data: items, total: items.length, limit: 20, offset: 0 })
  unreadCountMock.mockResolvedValue(unread)
  await useNotificationsStore().refresh()
  const w = await mountSuspended(NotificationBell)
  await settle()
  await w.find('[data-testid="notification-bell"]').trigger('click')
  await settle()
  return w
}

describe('NotificationBell — Indonesian copy', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
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

  it('renders the chrome in Indonesian', async () => {
    await openWith([])
    const text = panelText()
    expect(text).toContain('Notifikasi')
    expect(text).toContain('Tandai dibaca')
    expect(text).toContain('Lihat semua notifikasi')
  })

  it('renders the empty state in Indonesian', async () => {
    await openWith([])
    expect(panelText()).toContain('Tidak ada notifikasi baru')
  })

  it('renders the error state and retry in Indonesian', async () => {
    listMock.mockRejectedValue(new Error('boom'))
    unreadCountMock.mockRejectedValue(new Error('boom'))
    await useNotificationsStore().refresh()
    const w = await mountSuspended(NotificationBell)
    await settle()
    await w.find('[data-testid="notification-bell"]').trigger('click')
    await settle()

    const text = panelText()
    expect(text).toContain('Gagal memuat notifikasi.')
    expect(text).toContain('Coba lagi')
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
    await openWith([row(over)])
    expect(panelText()).toContain(sentence)
  })

  it('renders an unknown type as the Indonesian fallback sentence', async () => {
    await openWith([row({ type: 'moon_phase' as never })])
    expect(panelText()).toContain('Notifikasi baru')
  })

  it('renders the relative timestamp in Indonesian', async () => {
    await openWith([row({ type: 'asset_returned', params: { asset_name: 'A', asset_tag: TAG } })])
    // Intl.RelativeTimeFormat('id') renders '1 jam yang lalu'.
    expect(panelText()).toContain('1 jam yang lalu')
  })
})
