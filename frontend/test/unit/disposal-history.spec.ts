import { describe, it, expect } from 'vitest'
import { mergeDisposalHistory } from '~/utils/disposalHistory'
import type { Disposal } from '~/composables/api/useDisposals'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'

function req(over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow {
  return {
    id: 'r1',
    type: 'asset_disposal',
    status: 'pending',
    amount: null,
    current_step: 1,
    office_id: 'o1',
    office_name: 'Cabang Alpha',
    target_id: 'a1',
    target_entity: 'asset',
    reason: 'rusak berat',
    requested_by_id: 'u1',
    requested_by_name: 'Andi Saputra',
    requested_by_role: 'Kepala Unit',
    decided_by_id: null,
    decision_note: null,
    created_at: '2026-07-01T09:00:00Z',
    ...over
  }
}

function disposal(over: Partial<Disposal> = {}): Disposal {
  return {
    id: 'd1',
    asset_id: 'a1',
    method: 'sale',
    disposal_date: '2026-07-05',
    proceeds: '500000',
    book_value_at_disposal: '200000',
    gain_loss: '300000',
    bast_no: 'BAST-100',
    approved_by_id: 'u2',
    request_id: 'r1',
    created_by_id: 'u1',
    asset_name: 'Meja Kerja Ergonomis',
    asset_tag: 'JKT01-FUR-2025-00011',
    office_name: 'Cabang Alpha',
    created_by_name: 'Andi Saputra',
    created_at: '2026-07-01T09:05:00Z',
    updated_at: '2026-07-05T00:00:00Z',
    ...over
  }
}

function opts(over: Partial<Parameters<typeof mergeDisposalHistory>[2]> = {}) {
  return {
    fmtDate: (iso: string | null) => (iso ? `fmt(${iso})` : '—'),
    assetName: (targetId: string | null) => (targetId ? `Lookup ${targetId}` : null),
    canAttach: false,
    ...over
  }
}

describe('mergeDisposalHistory — request-row status mapping', () => {
  it.each([
    ['pending', 'menunggu'],
    ['rejected', 'ditolak'],
    ['cancelled', 'dibatalkan']
  ] as const)('maps request status %s to history status %s', (status, expected) => {
    const rows = mergeDisposalHistory([req({ status })], [], opts())
    expect(rows).toHaveLength(1)
    expect(rows[0]!.status).toBe(expected)
    expect(rows[0]!.source).toBe('request')
    expect(rows[0]!.key).toBe('request:r1')
  })

  it('drops request rows whose status is approved — the disposal row already covers that stage', () => {
    const rows = mergeDisposalHistory([req({ status: 'approved' })], [], opts())
    expect(rows).toHaveLength(0)
  })

  it('drops requests that are not of type asset_disposal', () => {
    const rows = mergeDisposalHistory([req({ type: 'asset_transfer' })], [], opts())
    expect(rows).toHaveLength(0)
  })

  it('methodKey is null for request rows — the disposal payload is absent on the list endpoint', () => {
    const rows = mergeDisposalHistory([req()], [], opts())
    expect(rows[0]!.methodKey).toBeNull()
  })

  it('proceeds/gainLoss are null for request rows', () => {
    const rows = mergeDisposalHistory([req()], [], opts())
    expect(rows[0]!.proceeds).toBeNull()
    expect(rows[0]!.gainLoss).toBeNull()
  })

  it('resolves the asset label via the assetName lookup callback, falling back to em dash', () => {
    const withTarget = mergeDisposalHistory([req({ target_id: 'a9' })], [], opts())
    expect(withTarget[0]!.assetLabel).toBe('Lookup a9')
    expect(withTarget[0]!.assetTag).toBeNull()

    const withoutTarget = mergeDisposalHistory([req({ target_id: null })], [], opts())
    expect(withoutTarget[0]!.assetLabel).toBe('—')
  })

  it('a request row can never be attached to, regardless of canAttach', () => {
    const rows = mergeDisposalHistory([req()], [], opts({ canAttach: true }))
    expect(rows[0]!.canAttach).toBe(false)
  })

  it('uses the request created_at for the date label', () => {
    const rows = mergeDisposalHistory([req({ created_at: '2026-07-01T09:00:00Z' })], [], opts())
    expect(rows[0]!.dateLabel).toBe('fmt(2026-07-01T09:00:00Z)')
  })
})

describe('mergeDisposalHistory — disposal-row mapping', () => {
  it('disposal rows always report status selesai', () => {
    const rows = mergeDisposalHistory([], [disposal()], opts())
    expect(rows[0]!.status).toBe('selesai')
    expect(rows[0]!.source).toBe('disposal')
    expect(rows[0]!.key).toBe('disposal:d1')
  })

  it('uses the disposal method directly as methodKey', () => {
    const rows = mergeDisposalHistory([], [disposal({ method: 'auction' })], opts())
    expect(rows[0]!.methodKey).toBe('auction')
  })

  it('uses the enriched asset_name/asset_tag fields directly', () => {
    const rows = mergeDisposalHistory([], [disposal()], opts())
    expect(rows[0]!.assetLabel).toBe('Meja Kerja Ergonomis')
    expect(rows[0]!.assetTag).toBe('JKT01-FUR-2025-00011')
  })

  it('exposes proceeds and gainLoss straight from the disposal row', () => {
    const rows = mergeDisposalHistory([], [disposal({ proceeds: '750000', gain_loss: '-50000' })], opts())
    expect(rows[0]!.proceeds).toBe('750000')
    expect(rows[0]!.gainLoss).toBe('-50000')
  })

  it('treats null proceeds/gain_loss as null (not coerced to a string)', () => {
    const rows = mergeDisposalHistory([], [disposal({ proceeds: null, gain_loss: null })], opts())
    expect(rows[0]!.proceeds).toBeNull()
    expect(rows[0]!.gainLoss).toBeNull()
  })

  it('prefers disposal_date over created_at for the date label', () => {
    const rows = mergeDisposalHistory([], [disposal({ disposal_date: '2026-07-05', created_at: '2026-07-01T09:05:00Z' })], opts())
    expect(rows[0]!.dateLabel).toBe('fmt(2026-07-05)')
  })

  it('falls back to created_at for the date label when disposal_date is absent', () => {
    const rows = mergeDisposalHistory([], [disposal({ disposal_date: null, created_at: '2026-07-01T09:05:00Z' })], opts())
    expect(rows[0]!.dateLabel).toBe('fmt(2026-07-01T09:05:00Z)')
  })

  it('canAttach is true only when the source is disposal AND the opts.canAttach flag is set', () => {
    const allowed = mergeDisposalHistory([], [disposal()], opts({ canAttach: true }))
    expect(allowed[0]!.canAttach).toBe(true)

    const notAllowed = mergeDisposalHistory([], [disposal()], opts({ canAttach: false }))
    expect(notAllowed[0]!.canAttach).toBe(false)
  })
})

describe('mergeDisposalHistory — merge + sort', () => {
  it('sorts merged rows by underlying created_at descending across both sources', () => {
    const rows = mergeDisposalHistory(
      [
        req({ id: 'r-old', created_at: '2026-06-01T00:00:00Z' }),
        req({ id: 'r-new', created_at: '2026-07-10T00:00:00Z' })
      ],
      [
        disposal({ id: 'd-mid', created_at: '2026-06-15T00:00:00Z' })
      ],
      opts()
    )
    expect(rows.map(r => r.key)).toEqual(['request:r-new', 'disposal:d-mid', 'request:r-old'])
  })

  it('treats a null/missing created_at as oldest so it sorts last', () => {
    const rows = mergeDisposalHistory(
      [req({ id: 'r-null', created_at: null })],
      [disposal({ id: 'd-dated', created_at: '2026-06-15T00:00:00Z' })],
      opts()
    )
    expect(rows.map(r => r.key)).toEqual(['disposal:d-dated', 'request:r-null'])
  })

  it('returns an empty array when both inputs are empty', () => {
    expect(mergeDisposalHistory([], [], opts())).toEqual([])
  })
})
