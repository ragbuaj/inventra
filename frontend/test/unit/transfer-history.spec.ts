import { describe, it, expect, vi } from 'vitest'
import { mergeTransferHistory } from '~/utils/transferHistory'
import type { Transfer } from '~/composables/api/useTransfers'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'

function req(over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow {
  return {
    id: 'r1',
    type: 'asset_transfer',
    status: 'pending',
    amount: null,
    current_step: 1,
    office_id: 'o-src',
    office_name: 'Cabang Alpha',
    target_id: 'a1',
    target_entity: 'asset',
    reason: 'realokasi',
    requested_by_id: 'u1',
    requested_by_name: 'Andi Saputra',
    requested_by_role: 'Kepala Unit',
    decided_by_id: null,
    decision_note: null,
    created_at: '2026-07-01T09:00:00Z',
    ...over
  }
}

function transfer(over: Partial<Transfer> = {}): Transfer {
  return {
    id: 't1',
    asset_id: 'a1',
    from_office_id: 'o-src',
    to_office_id: 'o-dst',
    to_room_id: null,
    status: 'approved',
    reason: 'realokasi',
    requested_by_id: 'u1',
    approved_by_id: 'u2',
    shipped_date: null,
    received_date: null,
    received_by_id: null,
    bast_no: null,
    request_id: 'r1',
    condition_sent: null,
    transfer_date: '2026-07-02',
    return_note: null,
    asset_name: 'Laptop Dell Latitude 5440',
    asset_tag: 'JKT01-ELK-2026-00001',
    from_office_name: 'Cabang Alpha',
    to_office_name: 'Cabang Beta',
    to_room_name: null,
    requested_by_name: 'Andi Saputra',
    received_by_name: null,
    created_at: '2026-07-01T09:05:00Z',
    updated_at: '2026-07-01T09:05:00Z',
    ...over
  }
}

function opts(over: Partial<Parameters<typeof mergeTransferHistory>[2]> = {}) {
  return {
    fmtDate: (iso: string | null) => (iso ? `fmt(${iso})` : '—'),
    assetName: (targetId: string | null) => (targetId ? `Lookup ${targetId}` : null),
    officeName: (id: string | null) => (id ? `Office ${id}` : null),
    interRegion: (a: string, b: string) => a !== b,
    canShip: (t: Transfer) => t.id === 'ship-ok',
    ...over
  }
}

describe('mergeTransferHistory — request-row status mapping', () => {
  it.each([
    ['pending', 'diajukan'],
    ['rejected', 'ditolak_pengajuan'],
    ['cancelled', 'dibatalkan']
  ] as const)('maps request status %s to history status %s', (status, expected) => {
    const rows = mergeTransferHistory([req({ status })], [], opts())
    expect(rows).toHaveLength(1)
    expect(rows[0]!.status).toBe(expected)
    expect(rows[0]!.source).toBe('request')
    expect(rows[0]!.key).toBe('request:r1')
  })

  it('drops request rows whose status is approved — the transfer row already covers that stage', () => {
    const rows = mergeTransferHistory([req({ status: 'approved' })], [], opts())
    expect(rows).toHaveLength(0)
  })

  it('drops requests that are not of type asset_transfer', () => {
    const rows = mergeTransferHistory([req({ type: 'asset_disposal' })], [], opts())
    expect(rows).toHaveLength(0)
  })

  it('resolves the asset label via the assetName lookup callback with the request target id', () => {
    const rows = mergeTransferHistory([req({ target_id: 'a9' })], [], opts())
    expect(rows[0]!.assetLabel).toBe('Lookup a9')
    expect(rows[0]!.assetTag).toBeNull()
  })

  it('falls back to em dash when the asset lookup cannot resolve a target', () => {
    const rows = mergeTransferHistory([req({ target_id: null })], [], opts())
    expect(rows[0]!.assetLabel).toBe('—')
  })

  it('a request row is never shippable, has no BAST, and no known destination', () => {
    const rows = mergeTransferHistory([req()], [], opts())
    expect(rows[0]!.canShip).toBe(false)
    expect(rows[0]!.bastNo).toBeNull()
    expect(rows[0]!.toLabel).toBe('—')
    expect(rows[0]!.interRegion).toBeNull()
  })

  it('uses the request created_at for the date label', () => {
    const rows = mergeTransferHistory([req({ created_at: '2026-07-01T09:00:00Z' })], [], opts())
    expect(rows[0]!.dateLabel).toBe('fmt(2026-07-01T09:00:00Z)')
  })

  it('resolves fromLabel from the request office_name, falling back to the officeName callback', () => {
    const withName = mergeTransferHistory([req({ office_name: 'Cabang Alpha' })], [], opts())
    expect(withName[0]!.fromLabel).toBe('Cabang Alpha')

    const withoutName = mergeTransferHistory([req({ office_name: null, office_id: 'o-src' })], [], opts())
    expect(withoutName[0]!.fromLabel).toBe('Office o-src')
  })
})

describe('mergeTransferHistory — transfer-row mapping', () => {
  it.each(['approved', 'in_transit', 'received', 'returned'] as const)(
    'passes the transfer status %s straight through',
    (status) => {
      const rows = mergeTransferHistory([], [transfer({ status })], opts())
      expect(rows[0]!.status).toBe(status)
      expect(rows[0]!.source).toBe('transfer')
      expect(rows[0]!.key).toBe('transfer:t1')
    }
  )

  it('uses the enriched asset_name/asset_tag fields directly', () => {
    const rows = mergeTransferHistory([], [transfer()], opts())
    expect(rows[0]!.assetLabel).toBe('Laptop Dell Latitude 5440')
    expect(rows[0]!.assetTag).toBe('JKT01-ELK-2026-00001')
  })

  it('falls back to em dash when asset_name is absent', () => {
    const rows = mergeTransferHistory([], [transfer({ asset_name: null, asset_tag: null })], opts())
    expect(rows[0]!.assetLabel).toBe('—')
    expect(rows[0]!.assetTag).toBeNull()
  })

  it('uses the enriched from/to office names, falling back to the officeName callback', () => {
    const rows = mergeTransferHistory([], [transfer({ from_office_name: null, to_office_name: null })], opts())
    expect(rows[0]!.fromLabel).toBe('Office o-src')
    expect(rows[0]!.toLabel).toBe('Office o-dst')
  })

  it('falls back to em dash for office labels when both the enriched name and the lookup are unresolved', () => {
    const rows = mergeTransferHistory(
      [],
      [transfer({ from_office_name: null, to_office_name: null })],
      opts({ officeName: () => null })
    )
    expect(rows[0]!.fromLabel).toBe('—')
    expect(rows[0]!.toLabel).toBe('—')
  })

  it('prefers transfer_date over created_at for the date label', () => {
    const rows = mergeTransferHistory([], [transfer({ transfer_date: '2026-07-02', created_at: '2026-07-01T09:05:00Z' })], opts())
    expect(rows[0]!.dateLabel).toBe('fmt(2026-07-02)')
  })

  it('falls back to created_at for the date label when transfer_date is absent', () => {
    const rows = mergeTransferHistory([], [transfer({ transfer_date: null, created_at: '2026-07-01T09:05:00Z' })], opts())
    expect(rows[0]!.dateLabel).toBe('fmt(2026-07-01T09:05:00Z)')
  })

  it('calls interRegion with the from/to office ids', () => {
    const interRegion = vi.fn(() => true)
    mergeTransferHistory([], [transfer({ from_office_id: 'o-src', to_office_id: 'o-dst' })], opts({ interRegion }))
    expect(interRegion).toHaveBeenCalledWith('o-src', 'o-dst')
  })

  it('canShip is true only when status is approved AND the canShip callback allows it', () => {
    const allowed = mergeTransferHistory([], [transfer({ id: 'ship-ok', status: 'approved' })], opts())
    expect(allowed[0]!.canShip).toBe(true)

    const wrongStatus = mergeTransferHistory([], [transfer({ id: 'ship-ok', status: 'in_transit' })], opts())
    expect(wrongStatus[0]!.canShip).toBe(false)

    const notAllowed = mergeTransferHistory([], [transfer({ id: 'other', status: 'approved' })], opts())
    expect(notAllowed[0]!.canShip).toBe(false)
  })

  it('uses bast_no and requested_by_name straight from the transfer row', () => {
    const rows = mergeTransferHistory([], [transfer({ bast_no: 'BAST-001', requested_by_name: 'Budi' })], opts())
    expect(rows[0]!.bastNo).toBe('BAST-001')
    expect(rows[0]!.actorName).toBe('Budi')
  })
})

describe('mergeTransferHistory — merge + sort', () => {
  it('sorts merged rows by underlying created_at descending across both sources', () => {
    const rows = mergeTransferHistory(
      [
        req({ id: 'r-old', created_at: '2026-06-01T00:00:00Z' }),
        req({ id: 'r-new', created_at: '2026-07-10T00:00:00Z' })
      ],
      [
        transfer({ id: 't-mid', created_at: '2026-06-15T00:00:00Z' })
      ],
      opts()
    )
    expect(rows.map(r => r.key)).toEqual(['request:r-new', 'transfer:t-mid', 'request:r-old'])
  })

  it('treats a null/missing created_at as oldest so it sorts last', () => {
    const rows = mergeTransferHistory(
      [req({ id: 'r-null', created_at: null })],
      [transfer({ id: 't-dated', created_at: '2026-06-15T00:00:00Z' })],
      opts()
    )
    expect(rows.map(r => r.key)).toEqual(['transfer:t-dated', 'request:r-null'])
  })

  it('returns an empty array when both inputs are empty', () => {
    expect(mergeTransferHistory([], [], opts())).toEqual([])
  })
})
