import type { ApprovalRequestDetail } from '~/composables/api/useApproval'
import { formatRupiah } from '~/utils/format'

export interface SummaryRow { label: string, value: string }
export interface DiffRow { label: string, before: string, after: string }
export type PayloadView
  = { layout: 'summary', rows: SummaryRow[] }
    | { layout: 'diff', rows: DiffRow[] }

export interface PayloadLookups {
  categoryName?: (id: string) => string | undefined
  officeName?: (id: string) => string | undefined
  assetName?: (id: string) => string | undefined
  problemCategoryName?: (id: string) => string | undefined
}

type Tfn = (k: string, p?: Record<string, unknown>) => string

function str(p: Record<string, unknown> | null | undefined, key: string): string | undefined {
  const v = p?.[key]
  return typeof v === 'string' && v !== '' ? v : undefined
}

/**
 * Maps a request's raw payload into the mockup's Data section shape.
 * asset_create/asset_transfer render as label:value summaries; asset_disposal
 * and valuation_exclusion render as before→after diffs (their status rows are
 * static — those transitions are implied by the request type, not the payload).
 * A masked/absent payload yields empty rows for payload-dependent fields only.
 */
export function payloadToView(detail: ApprovalRequestDetail, t: Tfn, lookups: PayloadLookups = {}): PayloadView {
  const p = (detail.payload ?? null) as Record<string, unknown> | null

  if (detail.type === 'asset_create') {
    if (!p) return { layout: 'summary', rows: [] }
    const rows: SummaryRow[] = []
    const push = (label: string, value?: string) => {
      if (value) rows.push({ label: t(label), value })
    }
    push('approval.field.assetName', str(p, 'name'))
    const catID = str(p, 'category_id')
    push('approval.field.category', catID ? (lookups.categoryName?.(catID) ?? catID) : undefined)
    push('approval.field.assetClass', str(p, 'asset_class'))
    if (str(p, 'purchase_cost')) push('approval.field.purchaseCost', formatRupiah(str(p, 'purchase_cost')))
    push('approval.field.purchaseDate', str(p, 'purchase_date'))
    push('approval.field.serialNumber', str(p, 'serial_number'))
    push('approval.field.poNumber', str(p, 'po_number'))
    push('approval.field.fundingSource', str(p, 'funding_source'))
    return { layout: 'summary', rows }
  }

  if (detail.type === 'asset_transfer') {
    if (!p) return { layout: 'summary', rows: [] }
    const rows: SummaryRow[] = []
    const office = (id?: string) => (id ? (lookups.officeName?.(id) ?? id) : undefined)
    const from = office(str(p, 'from_office_id'))
    const to = office(str(p, 'to_office_id'))
    if (from) rows.push({ label: t('approval.field.fromOffice'), value: from })
    if (to) rows.push({ label: t('approval.field.toOffice'), value: to })
    if (str(p, 'to_room_id')) rows.push({ label: t('approval.field.toRoom'), value: str(p, 'to_room_id')! })
    return { layout: 'summary', rows }
  }

  if (detail.type === 'asset_disposal') {
    const rows: DiffRow[] = [{
      label: t('approval.field.assetStatus'),
      before: t('approval.field.active'),
      after: t('approval.field.disposed')
    }]
    const add = (label: string, after?: string) => {
      if (after) rows.push({ label: t(label), before: '—', after })
    }
    add('approval.field.method', str(p, 'method'))
    add('approval.field.disposalDate', str(p, 'disposal_date'))
    if (str(p, 'proceeds')) add('approval.field.proceeds', formatRupiah(str(p, 'proceeds')))
    if (str(p, 'book_value_at_disposal')) add('approval.field.bookValue', formatRupiah(str(p, 'book_value_at_disposal')))
    add('approval.field.bastNo', str(p, 'bast_no'))
    return { layout: 'diff', rows }
  }

  if (detail.type === 'asset_import') {
    if (!p) return { layout: 'summary', rows: [] }
    const rows: SummaryRow[] = []
    const push = (label: string, value?: string) => {
      if (value) rows.push({ label: t(label), value })
    }
    push('approval.field.filename', str(p, 'filename'))
    const totalRows = p.total_rows
    if (typeof totalRows === 'number') rows.push({ label: t('approval.field.totalRows'), value: String(totalRows) })
    if (str(p, 'total_value')) push('approval.field.totalValue', formatRupiah(str(p, 'total_value')))
    return { layout: 'summary', rows }
  }

  if (detail.type === 'maintenance') {
    if (!p) return { layout: 'summary', rows: [] }
    const rows: SummaryRow[] = []
    const assetID = str(p, 'asset_id')
    if (assetID) rows.push({ label: t('approval.field.asset'), value: lookups.assetName?.(assetID) ?? assetID })
    const problemID = str(p, 'problem_category_id')
    if (problemID) rows.push({ label: t('approval.field.problemCategory'), value: lookups.problemCategoryName?.(problemID) ?? problemID })
    const desc = str(p, 'description')
    if (desc) rows.push({ label: t('approval.field.description'), value: desc })
    return { layout: 'summary', rows }
  }

  // valuation_exclusion — fully static: the transition is the request itself.
  return {
    layout: 'diff',
    rows: [{
      label: t('approval.field.valuation'),
      before: t('approval.field.included'),
      after: t('approval.field.excluded')
    }]
  }
}
