import { describe, it, expect } from 'vitest'
import { formatRupiah } from '~/utils/money'
import { payloadToView } from '~/utils/approvalPayload'
import type { ApprovalRequestDetail } from '~/composables/api/useApproval'

const t = (k: string, p?: Record<string, unknown>) => p ? `${k}:${JSON.stringify(p)}` : k

function detail(partial: Partial<ApprovalRequestDetail>): ApprovalRequestDetail {
  return {
    id: 'x', type: 'asset_create', status: 'pending', current_step: 1,
    office_id: 'o1', office_name: 'Cabang A', target_id: null, target_entity: null,
    requested_by_id: 'u1', requested_by_name: 'Andi', requested_by_role: 'Kepala Unit',
    decided_by_id: null, decision_note: null, created_at: '2026-07-04T09:00:00Z',
    steps: [],
    ...partial
  }
}

describe('formatRupiah', () => {
  it('formats a decimal string as IDR without fraction digits', () => {
    expect(formatRupiah('1500000')).toMatch(/^Rp\s?1\.500\.000$/)
  })
  it('returns em-dash for null, undefined and non-numeric input', () => {
    expect(formatRupiah(null)).toBe('—')
    expect(formatRupiah(undefined)).toBe('—')
    expect(formatRupiah('abc')).toBe('—')
  })
})

describe('payloadToView — asset_create', () => {
  it('maps the payload into summary rows with resolved names', () => {
    const v = payloadToView(detail({
      payload: {
        name: 'Laptop A', category_id: 'c1', asset_class: 'tangible',
        purchase_cost: '1500000', purchase_date: '2026-07-01', serial_number: 'SN1'
      }
    }), t, { categoryName: id => (id === 'c1' ? 'Elektronik' : undefined) })
    expect(v.layout).toBe('summary')
    const byLabel = Object.fromEntries(v.rows.map(r => [r.label, (r as { value: string }).value]))
    expect(byLabel['approval.field.assetName']).toBe('Laptop A')
    expect(byLabel['approval.field.category']).toBe('Elektronik')
    expect(byLabel['approval.field.purchaseCost']).toMatch(/1\.500\.000/)
  })

  it('falls back to the raw id when a lookup misses', () => {
    const v = payloadToView(detail({ payload: { name: 'X', category_id: 'c9' } }), t)
    const cat = v.rows.find(r => r.label === 'approval.field.category')
    expect((cat as { value: string } | undefined)?.value).toBe('c9')
  })

  it('returns empty rows for a null/masked payload', () => {
    expect(payloadToView(detail({ payload: null }), t).rows).toEqual([])
    expect(payloadToView(detail({}), t).rows).toEqual([])
  })
})

describe('payloadToView — asset_disposal', () => {
  it('renders a static status diff plus payload fields', () => {
    const v = payloadToView(detail({
      type: 'asset_disposal',
      payload: { method: 'sale', disposal_date: '2026-07-01', proceeds: '500000' }
    }), t)
    expect(v.layout).toBe('diff')
    const status = v.rows.find(r => r.label === 'approval.field.assetStatus') as { before: string, after: string }
    expect(status.before).toBe('approval.field.active')
    expect(status.after).toBe('approval.field.disposed')
    const method = v.rows.find(r => r.label === 'approval.field.method') as { after: string }
    expect(method.after).toBe('sale')
  })

  it('keeps the static status row even when the payload is missing', () => {
    const v = payloadToView(detail({ type: 'asset_disposal', payload: null }), t)
    expect(v.layout).toBe('diff')
    expect(v.rows.some(r => r.label === 'approval.field.assetStatus')).toBe(true)
  })
})

describe('payloadToView — asset_transfer', () => {
  it('maps offices through the lookup with raw-id fallback', () => {
    const v = payloadToView(detail({
      type: 'asset_transfer',
      payload: { from_office_id: 'o1', to_office_id: 'o2', reason: 'relokasi' }
    }), t, { officeName: id => (id === 'o1' ? 'Cabang A' : undefined) })
    expect(v.layout).toBe('summary')
    const byLabel = Object.fromEntries(v.rows.map(r => [r.label, (r as { value: string }).value]))
    expect(byLabel['approval.field.fromOffice']).toBe('Cabang A')
    expect(byLabel['approval.field.toOffice']).toBe('o2')
  })
})

describe('payloadToView — valuation_exclusion', () => {
  it('is a static diff that needs no payload', () => {
    const v = payloadToView(detail({ type: 'valuation_exclusion', payload: null }), t)
    expect(v.layout).toBe('diff')
    const row = v.rows[0] as { label: string, before: string, after: string }
    expect(row.label).toBe('approval.field.valuation')
    expect(row.before).toBe('approval.field.included')
    expect(row.after).toBe('approval.field.excluded')
  })
})
