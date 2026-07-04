/**
 * Pure merge of approval requests + disposal rows into a single unified
 * history feed for the Penghapusan (disposal) "Riwayat" tab. Mirrors
 * `transferHistory.ts`'s design: framework-free, callers supply resolved
 * lookup callbacks, no i18n/composables/reactivity here.
 *
 * A pending/rejected/cancelled request has no Disposal row yet, so it is
 * rendered from the request itself; an approved disposal is always executed
 * immediately (no intermediate "approved but not yet disposed" state), so
 * every Disposal row reports the terminal 'selesai' status and the matching
 * request row is dropped to avoid a duplicate line.
 */

import type { DisposalMethod } from '~/constants/disposalMeta'
import type { Disposal } from '~/composables/api/useDisposals'
import type { ApprovalRequestRow } from '~/composables/api/useApproval'

export type DisposalHistoryStatus = 'menunggu' | 'ditolak' | 'dibatalkan' | 'selesai'

export interface DisposalHistoryRow {
  key: string // request:<id> | disposal:<id>
  source: 'request' | 'disposal'
  id: string
  status: DisposalHistoryStatus
  assetLabel: string
  assetTag: string | null
  methodKey: DisposalMethod | null
  proceeds: string | null
  gainLoss: string | null
  dateLabel: string
  canAttach: boolean
  raw: Disposal | ApprovalRequestRow
}

export interface MergeDisposalHistoryOpts {
  fmtDate: (iso: string | null) => string
  assetName: (targetId: string | null) => string | null
  canAttach: boolean
}

const REQUEST_STATUS_MAP: Record<string, DisposalHistoryStatus | undefined> = {
  pending: 'menunggu',
  rejected: 'ditolak',
  cancelled: 'dibatalkan'
}

/** Parses an ISO date string to a sortable timestamp; null/invalid sorts as oldest. */
function sortTs(iso: string | null | undefined): number {
  if (!iso) return -Infinity
  const parsed = Date.parse(iso)
  return Number.isNaN(parsed) ? -Infinity : parsed
}

function fromRequest(r: ApprovalRequestRow, opts: MergeDisposalHistoryOpts): DisposalHistoryRow | null {
  const status = REQUEST_STATUS_MAP[r.status]
  if (!status) return null

  return {
    key: `request:${r.id}`,
    source: 'request',
    id: r.id,
    status,
    assetLabel: opts.assetName(r.target_id) ?? '—',
    assetTag: null,
    methodKey: null,
    proceeds: null,
    gainLoss: null,
    dateLabel: opts.fmtDate(r.created_at),
    canAttach: false,
    raw: r
  }
}

function fromDisposal(d: Disposal, opts: MergeDisposalHistoryOpts): DisposalHistoryRow {
  return {
    key: `disposal:${d.id}`,
    source: 'disposal',
    id: d.id,
    status: 'selesai',
    assetLabel: d.asset_name,
    assetTag: d.asset_tag,
    methodKey: d.method,
    proceeds: d.proceeds ?? null,
    gainLoss: d.gain_loss ?? null,
    dateLabel: opts.fmtDate(d.disposal_date ?? d.created_at),
    canAttach: opts.canAttach,
    raw: d
  }
}

export function mergeDisposalHistory(
  requests: ApprovalRequestRow[],
  disposals: Disposal[],
  opts: MergeDisposalHistoryOpts
): DisposalHistoryRow[] {
  const requestRows = requests
    .filter(r => r.type === 'asset_disposal')
    .map(r => fromRequest(r, opts))
    .filter((row): row is DisposalHistoryRow => row !== null)

  const disposalRows = disposals.map(d => fromDisposal(d, opts))

  return [...requestRows, ...disposalRows].sort(
    (a, b) => sortTs(b.raw.created_at) - sortTs(a.raw.created_at)
  )
}
