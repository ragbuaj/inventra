// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useDashboard } from '~/composables/api/useDashboard'
import type { DashboardSummary } from '~/composables/api/useDashboard'

// ---------------------------------------------------------------------------
// Mock the underlying HTTP client (same idiom as use-depreciation.spec.ts).
// request<T>/requestBlob are unchecked assertions at the type level, so these
// tests are the guard that the composable hits the EXACT backend routes
// (report/routes.go, report/handler.go) with the right query params.
// ---------------------------------------------------------------------------

const requestMock = vi.fn()
const requestBlobMock = vi.fn()

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: requestBlobMock, refreshToken: vi.fn() })
}))

const SUMMARY: DashboardSummary = {
  office_name: 'Kantor Cabang Jakarta Selatan',
  kpi: {
    total_assets: 96,
    acquisition_value: '3820000000',
    book_value: '2140000000',
    overdue_assets: 4,
    maintenance_due: 3,
    maintenance_cost: '42500000',
    trends: { acquisition_pct: 8.3, book_value_pct: -6.4, maintenance_cost_pct: null }
  },
  by_status: [{ status: 'available', count: 58 }],
  by_category: [{ name: 'Elektronik', count: 41 }, { name: null, count: 2 }],
  location_kind: 'office',
  by_location: [{ name: 'Lantai 2', count: 31 }],
  maintenance_due_list: [
    { id: 'm1', asset_name: 'Toyota Avanza', asset_tag: 'JKT01-KEN-2025-00007', category_name: 'Kendaraan', next_due_date: '2026-07-12' }
  ],
  excluded_count: 0
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useDashboard — summary', () => {
  it('GETs /dashboard/summary with a preset period', async () => {
    requestMock.mockResolvedValue(SUMMARY)
    const out = await useDashboard().summary({ period: { preset: 'this_quarter' } })
    expect(requestMock).toHaveBeenCalledWith('/dashboard/summary', {
      query: { period: 'this_quarter' }
    })
    expect(out).toEqual(SUMMARY)
  })

  it('GETs /dashboard/summary with a custom period as date_from/date_to (no period key)', async () => {
    requestMock.mockResolvedValue(SUMMARY)
    await useDashboard().summary({ period: { preset: 'custom', from: '2026-01-01', to: '2026-03-31' } })
    expect(requestMock).toHaveBeenCalledWith('/dashboard/summary', {
      query: { date_from: '2026-01-01', date_to: '2026-03-31' }
    })
    const query = requestMock.mock.calls[0]![1].query
    expect(query).not.toHaveProperty('period')
  })

  it('omits office_id when officeId is not provided', async () => {
    requestMock.mockResolvedValue(SUMMARY)
    await useDashboard().summary({ period: { preset: 'last30' } })
    const query = requestMock.mock.calls[0]![1].query
    expect(query).not.toHaveProperty('office_id')
  })

  it('includes office_id when officeId is provided', async () => {
    requestMock.mockResolvedValue(SUMMARY)
    await useDashboard().summary({ period: { preset: 'last30' }, officeId: 'off-1' })
    expect(requestMock).toHaveBeenCalledWith('/dashboard/summary', {
      query: { period: 'last30', office_id: 'off-1' }
    })
  })

  it('returns the response unchanged (no envelope to unwrap)', async () => {
    requestMock.mockResolvedValue(SUMMARY)
    const out = await useDashboard().summary({ period: { preset: 'ytd' } })
    expect(out).toBe(SUMMARY)
    expect(requestBlobMock).not.toHaveBeenCalled()
  })
})

describe('useDashboard — exportSummary', () => {
  it('requestBlobs /dashboard/export with period+format', async () => {
    const blob = new Blob(['x'])
    requestBlobMock.mockResolvedValue(blob)
    const out = await useDashboard().exportSummary({ period: { preset: 'this_month' } }, 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/dashboard/export', {
      query: { period: 'this_month', format: 'xlsx' }
    })
    expect(requestMock).not.toHaveBeenCalled()
    expect(out).toBe(blob)
  })

  it('passes format=pdf and office_id through together', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['y']))
    await useDashboard().exportSummary({ period: { preset: 'last30' }, officeId: 'off-9' }, 'pdf')
    expect(requestBlobMock).toHaveBeenCalledWith('/dashboard/export', {
      query: { period: 'last30', office_id: 'off-9', format: 'pdf' }
    })
  })

  it('builds a custom-period export query with date_from/date_to', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['z']))
    await useDashboard().exportSummary({ period: { preset: 'custom', from: '2026-02-01', to: '2026-02-28' } }, 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/dashboard/export', {
      query: { date_from: '2026-02-01', date_to: '2026-02-28', format: 'xlsx' }
    })
  })
})
