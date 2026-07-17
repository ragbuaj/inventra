import { describe, it, expect } from 'vitest'
import {
  NOTIFICATION_META,
  NOTIFICATION_TYPE_KEYS,
  UNKNOWN_NOTIFICATION_META,
  notificationMeta,
  notificationI18nParams,
  notificationLink,
  type NotificationType
} from '~/constants/notificationMeta'

// Mirrors the params each consumer handler writes
// (backend/internal/notification/consumer.go). If the backend changes them,
// these fixtures are what should break first.
const PARAMS: Record<NotificationType, Record<string, string>> = {
  approval_pending: { request_type: 'assignment', step: '1' },
  approval_decided: { request_type: 'asset_disposal', status: 'approved' },
  maintenance_due: { asset_tag: 'INV-2024-0312', asset_name: 'Toyota Avanza', due_date: '2026-07-20' },
  asset_returned: { asset_tag: 'INV-2024-0312', asset_name: 'Toyota Avanza' }
}

describe('notificationMeta catalog', () => {
  it('covers every backend notification type', () => {
    expect(NOTIFICATION_TYPE_KEYS).toEqual(['approval_pending', 'approval_decided', 'maintenance_due', 'asset_returned'])
    for (const type of NOTIFICATION_TYPE_KEYS) {
      expect(NOTIFICATION_META[type]).toBeDefined()
    }
  })

  it('every type maps to a complete, non-empty presentation', () => {
    for (const type of NOTIFICATION_TYPE_KEYS) {
      const meta = notificationMeta(type)
      expect(meta.icon).toMatch(/^i-lucide-/)
      expect(meta.iconBg.length).toBeGreaterThan(0)
      expect(meta.iconColor.length).toBeGreaterThan(0)
      expect(meta.i18nKey).toBe(`notifications.item.${type}`)
    }
  })

  it('uses the mockup icons and tints', () => {
    expect(notificationMeta('approval_pending')).toMatchObject({ icon: 'i-lucide-check-square', iconBg: 'bg-primary/10', iconColor: 'text-primary' })
    expect(notificationMeta('maintenance_due')).toMatchObject({ icon: 'i-lucide-wrench', iconBg: 'bg-warning/15', iconColor: 'text-warning' })
    expect(notificationMeta('asset_returned')).toMatchObject({ icon: 'i-lucide-package', iconBg: 'bg-muted', iconColor: 'text-muted' })
  })

  it('uses semantic color tokens, never literal Tailwind colors', () => {
    for (const type of NOTIFICATION_TYPE_KEYS) {
      const { iconBg, iconColor } = notificationMeta(type)
      expect(`${iconBg} ${iconColor}`).not.toMatch(/\b(red|green|blue|slate|gray|amber|emerald)-\d{2,3}\b/)
    }
  })

  it('degrades an unknown type to a neutral bell instead of throwing', () => {
    expect(notificationMeta('some_future_type')).toEqual(UNKNOWN_NOTIFICATION_META)
    expect(notificationMeta('')).toEqual(UNKNOWN_NOTIFICATION_META)
    expect(UNKNOWN_NOTIFICATION_META.i18nKey).toBe('notifications.item.unknown')
  })
})

describe('notificationI18nParams', () => {
  // Stand-in for vue-i18n's t(): echoes the key so the assertions prove which
  // key was looked up.
  const translate = (key: string) => `T(${key})`

  it('translates request_type through approval.type for approval_pending', () => {
    const out = notificationI18nParams({ type: 'approval_pending', params: PARAMS.approval_pending }, translate)
    expect(out).toEqual({ request_type: 'T(approval.type.assignment)', step: '1' })
  })

  it('translates both request_type and status for approval_decided', () => {
    const out = notificationI18nParams({ type: 'approval_decided', params: PARAMS.approval_decided }, translate)
    expect(out).toEqual({ request_type: 'T(approval.type.asset_disposal)', status: 'T(approval.status.approved)' })
  })

  it('passes display-ready params through untranslated', () => {
    expect(notificationI18nParams({ type: 'maintenance_due', params: PARAMS.maintenance_due }, translate))
      .toEqual({ asset_tag: 'INV-2024-0312', asset_name: 'Toyota Avanza', due_date: '2026-07-20' })
    expect(notificationI18nParams({ type: 'asset_returned', params: PARAMS.asset_returned }, translate))
      .toEqual({ asset_tag: 'INV-2024-0312', asset_name: 'Toyota Avanza' })
  })

  it('handles absent, null and empty params without throwing', () => {
    expect(notificationI18nParams({ type: 'asset_returned' }, translate)).toEqual({})
    expect(notificationI18nParams({ type: 'asset_returned', params: null }, translate)).toEqual({})
    expect(notificationI18nParams({ type: 'asset_returned', params: {} }, translate)).toEqual({})
  })

  it('does not translate params of an unknown type', () => {
    expect(notificationI18nParams({ type: 'some_future_type', params: { request_type: 'assignment' } }, translate))
      .toEqual({ request_type: 'assignment' })
  })
})

describe('notificationLink', () => {
  it('links a requests notification to /approval', () => {
    expect(notificationLink({ type: 'approval_pending', entity_type: 'requests', entity_id: 'req-1', params: PARAMS.approval_pending })).toBe('/approval')
    expect(notificationLink({ type: 'approval_decided', entity_type: 'requests', entity_id: 'req-1', params: PARAMS.approval_decided })).toBe('/approval')
  })

  it('links an assets notification by asset_tag, not by entity_id', () => {
    // The route is /assets/[tag] (keyed by asset_tag), while entity_id is the
    // asset UUID -- linking by entity_id would 404.
    const link = notificationLink({ type: 'asset_returned', entity_type: 'assets', entity_id: '0f8a-uuid', params: PARAMS.asset_returned })
    expect(link).toBe('/assets/INV-2024-0312')
    expect(link).not.toContain('0f8a-uuid')
  })

  it('url-encodes an asset tag containing unsafe characters', () => {
    expect(notificationLink({ type: 'asset_returned', entity_type: 'assets', params: { asset_tag: 'INV/2024 #12' } }))
      .toBe('/assets/INV%2F2024%20%2312')
  })

  it('returns null for an assets notification with no asset_tag param', () => {
    expect(notificationLink({ type: 'asset_returned', entity_type: 'assets', entity_id: 'uuid', params: {} })).toBeNull()
    expect(notificationLink({ type: 'asset_returned', entity_type: 'assets', entity_id: 'uuid' })).toBeNull()
  })

  it('returns null for a null, absent or unrecognized entity_type', () => {
    expect(notificationLink({ type: 'approval_pending', entity_type: null })).toBeNull()
    expect(notificationLink({ type: 'approval_pending' })).toBeNull()
    expect(notificationLink({ type: 'approval_pending', entity_type: 'widgets', entity_id: 'x' })).toBeNull()
  })
})
