// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useDepreciation } from '~/composables/api/useDepreciation'
import type {
  DepreciationPeriod,
  ScheduleResponse,
  JournalResponse,
  AssetDepreciationResponse,
  ClosedPeriod,
  ImpairmentResult
} from '~/composables/api/useDepreciation'

// ---------------------------------------------------------------------------
// Mock the underlying HTTP client. request<T>/requestBlob are unchecked
// assertions at the type level, so these tests are the only guard that the
// composable hits the EXACT backend routes (routes.go / openapi.yaml) with the
// right verb/query/body and unwraps envelopes correctly.
// ---------------------------------------------------------------------------

const requestMock = vi.fn()
const requestBlobMock = vi.fn()

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: requestBlobMock, refreshToken: vi.fn() })
}))

const PERIOD: DepreciationPeriod = {
  period: '2026-07', status: 'computed', asset_count: 6, total_amount: '1250000', skipped_count: 1
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useDepreciation — periods', () => {
  it('GETs /depreciation/periods and unwraps the {data} envelope', async () => {
    requestMock.mockResolvedValue({ data: [PERIOD] })
    const out = await useDepreciation().periods()
    expect(requestMock).toHaveBeenCalledWith('/depreciation/periods')
    expect(out).toEqual([PERIOD])
  })

  it('returns the inner array, not the envelope object', async () => {
    requestMock.mockResolvedValue({ data: [] })
    const out = await useDepreciation().periods()
    expect(Array.isArray(out)).toBe(true)
    expect(out).toEqual([])
  })
})

describe('useDepreciation — compute', () => {
  it('POSTs /depreciation/periods/:period/compute with period as a PATH param and no body', async () => {
    requestMock.mockResolvedValue(PERIOD)
    const out = await useDepreciation().compute('2026-07')
    expect(requestMock).toHaveBeenCalledWith('/depreciation/periods/2026-07/compute', { method: 'POST' })
    // No body key at all.
    expect(requestMock.mock.calls[0]![1]).not.toHaveProperty('body')
    expect(out).toEqual(PERIOD)
  })
})

describe('useDepreciation — close', () => {
  it('POSTs /depreciation/periods/:period/close with period as a PATH param and no body', async () => {
    const closed: ClosedPeriod = { period: '2026-07', status: 'closed' }
    requestMock.mockResolvedValue(closed)
    const out = await useDepreciation().close('2026-07')
    expect(requestMock).toHaveBeenCalledWith('/depreciation/periods/2026-07/close', { method: 'POST' })
    expect(requestMock.mock.calls[0]![1]).not.toHaveProperty('body')
    expect(out).toEqual(closed)
  })
})

describe('useDepreciation — schedule', () => {
  const RESULT: ScheduleResponse = {
    kpi: { total_cost: '10', total_accumulated: '2', total_book_value: '8', period_expense: '1' },
    rows: [],
    totals: { opening: '0', amount: '0', accumulated: '0', closing: '0' },
    total: 0
  }

  it('GETs /depreciation/schedule with required period+basis and omits undefined optionals', async () => {
    requestMock.mockResolvedValue(RESULT)
    const out = await useDepreciation().schedule({ period: '2026-07', basis: 'commercial' })
    expect(requestMock).toHaveBeenCalledWith('/depreciation/schedule', {
      query: { period: '2026-07', basis: 'commercial' }
    })
    const query = requestMock.mock.calls[0]![1].query
    expect(query).not.toHaveProperty('search')
    expect(query).not.toHaveProperty('category_id')
    expect(query).not.toHaveProperty('office_id')
    expect(query).not.toHaveProperty('limit')
    expect(query).not.toHaveProperty('offset')
    expect(out).toEqual(RESULT)
  })

  it('includes search/category_id/office_id in the query when provided', async () => {
    requestMock.mockResolvedValue(RESULT)
    await useDepreciation().schedule({
      period: '2026-07', basis: 'fiscal', search: 'laptop', category_id: 'cat-1', office_id: 'off-1'
    })
    expect(requestMock).toHaveBeenCalledWith('/depreciation/schedule', {
      query: { period: '2026-07', basis: 'fiscal', search: 'laptop', category_id: 'cat-1', office_id: 'off-1' }
    })
  })

  it('forwards limit/offset as stringified query params for server pagination', async () => {
    requestMock.mockResolvedValue(RESULT)
    await useDepreciation().schedule({
      period: '2026-07', basis: 'commercial', limit: 10, offset: 10
    })
    expect(requestMock).toHaveBeenCalledWith('/depreciation/schedule', {
      query: { period: '2026-07', basis: 'commercial', limit: '10', offset: '10' }
    })
  })
})

describe('useDepreciation — journal', () => {
  it('GETs /depreciation/journal with period+basis in the query', async () => {
    const result: JournalResponse = { rows: [], total_debit: '0', total_credit: '0', balanced: true }
    requestMock.mockResolvedValue(result)
    const out = await useDepreciation().journal('2026-07', 'commercial')
    expect(requestMock).toHaveBeenCalledWith('/depreciation/journal', {
      query: { period: '2026-07', basis: 'commercial' }
    })
    expect(out).toEqual(result)
  })
})

describe('useDepreciation — exportJournal', () => {
  it('requestBlobs /depreciation/journal/export with period+basis+format', async () => {
    const blob = new Blob(['x'])
    requestBlobMock.mockResolvedValue(blob)
    const out = await useDepreciation().exportJournal('2026-07', 'fiscal', 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/depreciation/journal/export', {
      query: { period: '2026-07', basis: 'fiscal', format: 'xlsx' }
    })
    expect(requestMock).not.toHaveBeenCalled()
    expect(out).toBe(blob)
  })

  it('passes format=pdf through', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['y']))
    await useDepreciation().exportJournal('2026-07', 'commercial', 'pdf')
    expect(requestBlobMock).toHaveBeenCalledWith('/depreciation/journal/export', {
      query: { period: '2026-07', basis: 'commercial', format: 'pdf' }
    })
  })
})

describe('useDepreciation — assetSchedule', () => {
  it('GETs /assets/:id/depreciation (under the /assets prefix)', async () => {
    const result: AssetDepreciationResponse = { masked: false, computed_book_value: '100', entries: [] }
    requestMock.mockResolvedValue(result)
    const out = await useDepreciation().assetSchedule('asset-9')
    expect(requestMock).toHaveBeenCalledWith('/assets/asset-9/depreciation')
    expect(out).toEqual(result)
  })
})

describe('useDepreciation — recordImpairment', () => {
  it('POSTs /assets/:id/impairment with {recoverable_amount, reason}', async () => {
    const result: ImpairmentResult = { book_value: '80', impairment_loss: '20' }
    requestMock.mockResolvedValue(result)
    const out = await useDepreciation().recordImpairment('asset-9', '80', 'Banjir')
    expect(requestMock).toHaveBeenCalledWith('/assets/asset-9/impairment', {
      method: 'POST',
      body: { recoverable_amount: '80', reason: 'Banjir' }
    })
    expect(out).toEqual(result)
  })
})
