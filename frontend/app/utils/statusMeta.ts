import type { BadgeColor } from '~/types'

interface StatusMeta { color: BadgeColor, labelKey: string }

// Asset statuses from PRD: tersedia/dipinjam/maintenance/dilepas/hilang
export const assetStatusMeta: Record<string, StatusMeta> = {
  available: { color: 'success', labelKey: 'status.asset.available' },
  assigned: { color: 'info', labelKey: 'status.asset.assigned' },
  under_maintenance: { color: 'warning', labelKey: 'status.asset.under_maintenance' },
  disposed: { color: 'neutral', labelKey: 'status.asset.disposed' },
  lost: { color: 'error', labelKey: 'status.asset.lost' }
}

// Approval statuses: pending/approved/rejected
export const approvalStatusMeta: Record<string, StatusMeta> = {
  pending: { color: 'warning', labelKey: 'status.approval.pending' },
  approved: { color: 'success', labelKey: 'status.approval.approved' },
  rejected: { color: 'error', labelKey: 'status.approval.rejected' }
}
