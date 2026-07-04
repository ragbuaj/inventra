/**
 * Pure merge of approval requests + transfer rows into a single unified
 * history feed for the Mutasi (transfer) "Riwayat" tab. Kept framework-free
 * so the mapping/sort rules are unit-testable without mounting; callers
 * (Vue pages) supply already-resolved lookup callbacks (date formatting,
 * asset/office name resolution, inter-region check, ship permission) so this
 * module never touches i18n, composables, or reactivity.
 *
 * A pending/rejected/cancelled request has no corresponding Transfer row yet,
 * so it is rendered from the request itself; once a transfer is approved (and
 * beyond), the enriched Transfer row is the source of truth and the request
 * row for that same submission is dropped to avoid a duplicate line.
 */

import type { Transfer } from '~/composables/api/useTransfers'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'

export type TransferHistoryStatus
  = | 'diajukan'
    | 'ditolak_pengajuan'
    | 'dibatalkan'
    | 'approved'
    | 'in_transit'
    | 'received'
    | 'returned'

export interface TransferHistoryRow {
  key: string // request:<id> | transfer:<id>
  source: 'request' | 'transfer'
  id: string
  status: TransferHistoryStatus
  assetLabel: string
  assetTag: string | null
  fromLabel: string
  toLabel: string
  dateLabel: string
  actorName: string | null
  bastNo: string | null
  interRegion: boolean | null
  canShip: boolean
  raw: Transfer | ApprovalRequestRow
}

export interface MergeTransferHistoryOpts {
  fmtDate: (iso: string | null) => string
  assetName: (targetId: string | null) => string | null
  officeName: (id: string | null) => string | null
  interRegion: (a: string, b: string) => boolean | null
  canShip: (t: Transfer) => boolean
}

const REQUEST_STATUS_MAP: Record<string, TransferHistoryStatus | undefined> = {
  pending: 'diajukan',
  rejected: 'ditolak_pengajuan',
  cancelled: 'dibatalkan'
}

/** Parses an ISO date string to a sortable timestamp; null/invalid sorts as oldest. */
function sortTs(iso: string | null | undefined): number {
  if (!iso) return -Infinity
  const parsed = Date.parse(iso)
  return Number.isNaN(parsed) ? -Infinity : parsed
}

function fromRequest(r: ApprovalRequestRow, opts: MergeTransferHistoryOpts): TransferHistoryRow | null {
  const status = REQUEST_STATUS_MAP[r.status]
  if (!status) return null

  return {
    key: `request:${r.id}`,
    source: 'request',
    id: r.id,
    status,
    assetLabel: opts.assetName(r.target_id) ?? '—',
    assetTag: null,
    fromLabel: r.office_name ?? opts.officeName(r.office_id) ?? '—',
    toLabel: '—',
    dateLabel: opts.fmtDate(r.created_at),
    actorName: r.requested_by_name ?? null,
    bastNo: null,
    interRegion: null,
    canShip: false,
    raw: r
  }
}

function fromTransfer(t: Transfer, opts: MergeTransferHistoryOpts): TransferHistoryRow {
  return {
    key: `transfer:${t.id}`,
    source: 'transfer',
    id: t.id,
    status: t.status,
    assetLabel: t.asset_name ?? '—',
    assetTag: t.asset_tag ?? null,
    fromLabel: t.from_office_name ?? opts.officeName(t.from_office_id) ?? '—',
    toLabel: t.to_office_name ?? opts.officeName(t.to_office_id) ?? '—',
    dateLabel: opts.fmtDate(t.transfer_date ?? t.created_at),
    actorName: t.requested_by_name ?? null,
    bastNo: t.bast_no ?? null,
    interRegion: opts.interRegion(t.from_office_id, t.to_office_id),
    canShip: t.status === 'approved' && opts.canShip(t),
    raw: t
  }
}

export function mergeTransferHistory(
  requests: ApprovalRequestRow[],
  transfers: Transfer[],
  opts: MergeTransferHistoryOpts
): TransferHistoryRow[] {
  const requestRows = requests
    .filter(r => r.type === 'asset_transfer')
    .map(r => fromRequest(r, opts))
    .filter((row): row is TransferHistoryRow => row !== null)

  const transferRows = transfers.map(t => fromTransfer(t, opts))

  return [...requestRows, ...transferRows].sort(
    (a, b) => sortTs(b.raw.created_at) - sortTs(a.raw.created_at)
  )
}
