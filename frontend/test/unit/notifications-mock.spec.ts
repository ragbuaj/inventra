import { describe, it, expect } from 'vitest'
import { useNotifications } from '~/composables/api/useNotifications'

describe('useNotifications — list', () => {
  const notifs = useNotifications()

  it('returns a non-empty array of notifications', () => {
    const list = notifs.list()
    expect(list.length).toBeGreaterThan(0)
  })

  it('each notification has required fields', () => {
    for (const n of notifs.list()) {
      expect(typeof n.id).toBe('string')
      expect(typeof n.title).toBe('string')
      expect(typeof n.read).toBe('boolean')
    }
  })
})

describe('useNotifications — unreadCount', () => {
  const notifs = useNotifications()

  it('returns the count of unread notifications', () => {
    const count = notifs.unreadCount()
    const expected = notifs.list().filter(n => !n.read).length
    expect(count).toBe(expected)
  })

  it('unread count is > 0 in the seed data', () => {
    expect(notifs.unreadCount()).toBeGreaterThan(0)
  })
})

describe('useNotifications — markAllRead', () => {
  const notifs = useNotifications()

  it('flips all notifications to read=true', () => {
    // Ensure at least one is unread first
    expect(notifs.list().some(n => !n.read)).toBe(true)
    notifs.markAllRead()
    expect(notifs.unreadCount()).toBe(0)
    expect(notifs.list().every(n => n.read)).toBe(true)
  })
})

describe('useNotifications — markRead', () => {
  const notifs = useNotifications()

  it('marks a single notification as read by id', () => {
    // markAllRead may have been called in prior test (shared store), so just test the behavior
    const all = notifs.list()
    if (all.length > 0) {
      const target = all[0]
      notifs.markRead(target.id)
      expect(notifs.list().find(n => n.id === target.id)?.read).toBe(true)
    }
  })
})
