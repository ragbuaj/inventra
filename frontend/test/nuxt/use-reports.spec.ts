// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useReports } from '~/composables/api/useReports'
import type { ReportResult } from '~/composables/api/useReports'

// ---------------------------------------------------------------------------
// Mock the underlying HTTP client (same idiom as use-depreciation.spec.ts).
// request<T>/requestBlob are unchecked assertions at the type level, so these
// tests are the guard that the composable hits the EXACT backend routes
// (report/routes.go: /reports/:type, /reports/:type/export,
// stockopname/routes.go: /stock-opname/sessions/:id/report) with the right
// query params.
// ---------------------------------------------------------------------------

const requestMock = vi.fn()
const requestBlobMock = vi.fn()

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: requestBlobMock, refreshToken: vi.fn() })
}))

const RESULT: ReportResult = {
  type: 'assets',
  kpis: [{ key: 'total_assets', value: '96' }],
  chart: [{ label: 'Elektronik', value: '41' }],
  rows: [],
  totals: { book_value: '0' },
  row_count: 0,
  truncated: false
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useReports — run', () => {
  it('GETs /reports/:type with a preset period', async () => {
    requestMock.mockResolvedValue(RESULT)
    const out = await useReports().run('assets', { period: { preset: 'this_quarter' } })
    expect(requestMock).toHaveBeenCalledWith('/reports/assets', {
      query: { period: 'this_quarter' }
    })
    expect(out).toEqual(RESULT)
  })

  it('GETs /reports/:type with a custom period as date_from/date_to (no period key)', async () => {
    requestMock.mockResolvedValue(RESULT)
    await useReports().run('depreciation', { period: { preset: 'custom', from: '2026-01-01', to: '2026-03-31' } })
    expect(requestMock).toHaveBeenCalledWith('/reports/depreciation', {
      query: { date_from: '2026-01-01', date_to: '2026-03-31' }
    })
    const query = requestMock.mock.calls[0]![1].query
    expect(query).not.toHaveProperty('period')
  })

  it('omits office_id/category_id/status/basis when not provided', async () => {
    requestMock.mockResolvedValue(RESULT)
    await useReports().run('assets', { period: { preset: 'last30' } })
    const query = requestMock.mock.calls[0]![1].query
    expect(query).not.toHaveProperty('office_id')
    expect(query).not.toHaveProperty('category_id')
    expect(query).not.toHaveProperty('status')
    expect(query).not.toHaveProperty('basis')
  })

  it('includes office_id/category_id/status when provided', async () => {
    requestMock.mockResolvedValue(RESULT)
    await useReports().run('assets', {
      period: { preset: 'last30' }, officeId: 'off-1', categoryId: 'cat-1', status: 'available'
    })
    expect(requestMock).toHaveBeenCalledWith('/reports/assets', {
      query: { period: 'last30', office_id: 'off-1', category_id: 'cat-1', status: 'available' }
    })
  })

  it('includes basis when provided (depreciation report)', async () => {
    requestMock.mockResolvedValue(RESULT)
    await useReports().run('depreciation', { period: { preset: 'ytd' }, basis: 'fiscal' })
    expect(requestMock).toHaveBeenCalledWith('/reports/depreciation', {
      query: { period: 'ytd', basis: 'fiscal' }
    })
  })

  it('builds the path from the report type for each of the 7 types', async () => {
    requestMock.mockResolvedValue(RESULT)
    const types = ['assets', 'depreciation', 'utilization', 'maintenance', 'transfers', 'disposals', 'opname'] as const
    for (const t of types) {
      await useReports().run(t, { period: { preset: 'last30' } })
      expect(requestMock).toHaveBeenCalledWith(`/reports/${t}`, { query: { period: 'last30' } })
    }
  })
})

describe('useReports — exportReport', () => {
  it('requestBlobs /reports/:type/export with period+format, no variant key by default', async () => {
    const blob = new Blob(['x'])
    requestBlobMock.mockResolvedValue(blob)
    const out = await useReports().exportReport('assets', { period: { preset: 'this_month' } }, 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/reports/assets/export', {
      query: { period: 'this_month', format: 'xlsx' }
    })
    const query = requestBlobMock.mock.calls[0]![1].query
    expect(query).not.toHaveProperty('variant')
    expect(requestMock).not.toHaveBeenCalled()
    expect(out).toBe(blob)
  })

  it('passes variant=gl_recap through for the disposals GL recap export', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['y']))
    await useReports().exportReport('disposals', { period: { preset: 'ytd' } }, 'pdf', 'gl_recap')
    expect(requestBlobMock).toHaveBeenCalledWith('/reports/disposals/export', {
      query: { period: 'ytd', format: 'pdf', variant: 'gl_recap' }
    })
  })

  it('passes variant=table through explicitly when given', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['z']))
    await useReports().exportReport('assets', { period: { preset: 'last30' } }, 'xlsx', 'table')
    expect(requestBlobMock).toHaveBeenCalledWith('/reports/assets/export', {
      query: { period: 'last30', format: 'xlsx', variant: 'table' }
    })
  })

  it('includes office_id/category_id/status/basis in an export query when provided', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['w']))
    await useReports().exportReport('depreciation', {
      period: { preset: 'last30' }, officeId: 'off-2', categoryId: 'cat-2', status: 'available', basis: 'commercial'
    }, 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/reports/depreciation/export', {
      query: {
        period: 'last30', office_id: 'off-2', category_id: 'cat-2', status: 'available',
        basis: 'commercial', format: 'xlsx'
      }
    })
  })

  it('builds a custom-period export query with date_from/date_to', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['v']))
    await useReports().exportReport('assets', { period: { preset: 'custom', from: '2026-02-01', to: '2026-02-28' } }, 'pdf')
    expect(requestBlobMock).toHaveBeenCalledWith('/reports/assets/export', {
      query: { date_from: '2026-02-01', date_to: '2026-02-28', format: 'pdf' }
    })
  })
})

describe('useReports — opnameBa', () => {
  it('requestBlobs GET /stock-opname/sessions/:id/report with format=xlsx', async () => {
    const blob = new Blob(['ba'])
    requestBlobMock.mockResolvedValue(blob)
    const out = await useReports().opnameBa('session-9', 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/stock-opname/sessions/session-9/report', {
      query: { format: 'xlsx' }
    })
    expect(requestMock).not.toHaveBeenCalled()
    expect(out).toBe(blob)
  })

  it('passes format=pdf through', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['ba2']))
    await useReports().opnameBa('session-9', 'pdf')
    expect(requestBlobMock).toHaveBeenCalledWith('/stock-opname/sessions/session-9/report', {
      query: { format: 'pdf' }
    })
  })
})
