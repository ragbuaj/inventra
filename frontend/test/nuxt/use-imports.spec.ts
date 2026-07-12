// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useImports } from '~/composables/api/useImports'
import type { ImportJob, ImportRow } from '~/composables/api/useImports'

// ---------------------------------------------------------------------------
// Mock the underlying HTTP client (same idiom as use-reports.spec.ts /
// use-asset-attachments.spec.ts). request<T>/requestBlob are unchecked
// assertions at the type level, so these tests are the guard that the
// composable hits the EXACT backend routes (importer/routes.go: POST
// /imports, GET /imports, GET /imports/:id, GET /imports/:id/rows, POST
// /imports/:id/confirm, POST /imports/:id/cancel, GET /imports/template,
// GET /imports/:id/error-report) with the right method/query/body.
// ---------------------------------------------------------------------------

const requestMock = vi.fn()
const requestBlobMock = vi.fn()

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: requestBlobMock, refreshToken: vi.fn() })
}))

const JOB: ImportJob = {
  id: 'job-1',
  target: 'asset',
  format: 'csv',
  filename: 'assets.csv',
  status: 'validated',
  total_rows: 10,
  success_rows: 9,
  failed_rows: 1,
  created_at: '2026-07-01T00:00:00Z'
}

// Backend/OpenAPI ImportRow shape: target-column values are flat siblings of
// id/row_no/valid/errors (additionalProperties), NOT nested under a `data`
// key — see useImports.ts's ImportRow doc comment.
const ROW: ImportRow = {
  id: 'row-1',
  row_no: 2,
  valid: false,
  asset_tag: 'A-001',
  name: 'Kursi',
  errors: [{ column: 'category_code', error_key: 'not_found' }]
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useImports — uploadImport', () => {
  it('POSTs multipart FormData with "file" and "target" to /imports', async () => {
    requestMock.mockResolvedValue(JOB)
    const file = new File(['a,b\n1,2'], 'assets.csv', { type: 'text/csv' })
    const out = await useImports().uploadImport('asset', file)

    expect(requestMock).toHaveBeenCalledTimes(1)
    const [path, opts] = requestMock.mock.calls[0] as [string, { method: string, body: FormData }]
    expect(path).toBe('/imports')
    expect(opts.method).toBe('POST')
    expect(opts.body).toBeInstanceOf(FormData)
    expect(opts.body.get('file')).toBe(file)
    expect(opts.body.get('target')).toBe('asset')
    expect(out).toEqual(JOB)
  })

  it('propagates errors from request', async () => {
    requestMock.mockRejectedValue(new Error('file too large'))
    const file = new File(['x'], 'big.csv', { type: 'text/csv' })
    await expect(useImports().uploadImport('asset', file)).rejects.toThrow('file too large')
  })
})

describe('useImports — getJob', () => {
  it('GETs /imports/:id', async () => {
    requestMock.mockResolvedValue(JOB)
    const out = await useImports().getJob('job-1')
    expect(requestMock).toHaveBeenCalledWith('/imports/job-1')
    expect(out).toEqual(JOB)
  })

  it('propagates errors from request', async () => {
    requestMock.mockRejectedValue(new Error('not found'))
    await expect(useImports().getJob('nope')).rejects.toThrow('not found')
  })
})

describe('useImports — getRows', () => {
  it('GETs /imports/:id/rows with no query params when opts is empty', async () => {
    requestMock.mockResolvedValue({ data: [ROW], total: 1, limit: 20, offset: 0 })
    const out = await useImports().getRows('job-1')
    expect(requestMock).toHaveBeenCalledWith('/imports/job-1/rows', { query: {} })
    expect(out.data).toEqual([ROW])
    expect(out.total).toBe(1)
  })

  it('includes only_errors/limit/offset when provided', async () => {
    requestMock.mockResolvedValue({ data: [], total: 0, limit: 5, offset: 10 })
    await useImports().getRows('job-1', { onlyErrors: true, limit: 5, offset: 10 })
    expect(requestMock).toHaveBeenCalledWith('/imports/job-1/rows', {
      query: { only_errors: 'true', limit: '5', offset: '10' }
    })
  })

  it('sends only_errors=false explicitly when onlyErrors is false', async () => {
    requestMock.mockResolvedValue({ data: [], total: 0, limit: 20, offset: 0 })
    await useImports().getRows('job-1', { onlyErrors: false })
    expect(requestMock).toHaveBeenCalledWith('/imports/job-1/rows', { query: { only_errors: 'false' } })
  })

  it('propagates errors from request', async () => {
    requestMock.mockRejectedValue(new Error('forbidden'))
    await expect(useImports().getRows('job-1')).rejects.toThrow('forbidden')
  })
})

describe('useImports — listJobs', () => {
  it('GETs /imports with the target query param', async () => {
    requestMock.mockResolvedValue({ data: [JOB], total: 1, limit: 20, offset: 0 })
    const out = await useImports().listJobs('asset')
    expect(requestMock).toHaveBeenCalledWith('/imports', { query: { target: 'asset' } })
    expect(out.data).toEqual([JOB])
  })

  it('includes limit/offset when provided', async () => {
    requestMock.mockResolvedValue({ data: [], total: 0, limit: 5, offset: 15 })
    await useImports().listJobs('employee', { limit: 5, offset: 15 })
    expect(requestMock).toHaveBeenCalledWith('/imports', {
      query: { target: 'employee', limit: '5', offset: '15' }
    })
  })

  it('propagates errors from request', async () => {
    requestMock.mockRejectedValue(new Error('boom'))
    await expect(useImports().listJobs('asset')).rejects.toThrow('boom')
  })
})

describe('useImports — confirmJob', () => {
  it('POSTs /imports/:id/confirm', async () => {
    const confirmed = { ...JOB, status: 'confirmed' }
    requestMock.mockResolvedValue(confirmed)
    const out = await useImports().confirmJob('job-1')
    expect(requestMock).toHaveBeenCalledWith('/imports/job-1/confirm', { method: 'POST' })
    expect(out).toEqual(confirmed)
  })

  it('propagates errors from request', async () => {
    requestMock.mockRejectedValue(new Error('bad state'))
    await expect(useImports().confirmJob('job-1')).rejects.toThrow('bad state')
  })
})

describe('useImports — cancelJob', () => {
  it('POSTs /imports/:id/cancel', async () => {
    const cancelled = { ...JOB, status: 'cancelled' }
    requestMock.mockResolvedValue(cancelled)
    const out = await useImports().cancelJob('job-1')
    expect(requestMock).toHaveBeenCalledWith('/imports/job-1/cancel', { method: 'POST' })
    expect(out).toEqual(cancelled)
  })

  it('propagates errors from request', async () => {
    requestMock.mockRejectedValue(new Error('conflict'))
    await expect(useImports().cancelJob('job-1')).rejects.toThrow('conflict')
  })
})

describe('useImports — getTemplate', () => {
  it('requestBlobs /imports/template with target+format', async () => {
    const blob = new Blob(['tag,name'], { type: 'text/csv' })
    requestBlobMock.mockResolvedValue(blob)
    const out = await useImports().getTemplate('asset', 'csv')
    expect(requestBlobMock).toHaveBeenCalledWith('/imports/template', { query: { target: 'asset', format: 'csv' } })
    expect(requestMock).not.toHaveBeenCalled()
    expect(out).toBe(blob)
  })

  it('passes format=xlsx through', async () => {
    requestBlobMock.mockResolvedValue(new Blob(['x']))
    await useImports().getTemplate('employee', 'xlsx')
    expect(requestBlobMock).toHaveBeenCalledWith('/imports/template', { query: { target: 'employee', format: 'xlsx' } })
  })

  it('propagates errors from requestBlob', async () => {
    requestBlobMock.mockRejectedValue(new Error('unknown target'))
    await expect(useImports().getTemplate('bogus', 'csv')).rejects.toThrow('unknown target')
  })
})

describe('useImports — getErrorReport', () => {
  it('requestBlobs /imports/:id/error-report', async () => {
    const blob = new Blob(['errors'], { type: 'text/csv' })
    requestBlobMock.mockResolvedValue(blob)
    const out = await useImports().getErrorReport('job-1')
    expect(requestBlobMock).toHaveBeenCalledWith('/imports/job-1/error-report')
    expect(out).toBe(blob)
  })

  it('propagates errors from requestBlob', async () => {
    requestBlobMock.mockRejectedValue(new Error('not found'))
    await expect(useImports().getErrorReport('nope')).rejects.toThrow('not found')
  })
})
